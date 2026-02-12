package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AddonName                       = "multicluster-observability-addon"
	PrometheusControllerID          = "acm-observability"
	PlatformMetricsCollectorApp     = "platform-metrics-collector"
	UserWorkloadMetricsCollectorApp = "user-workload-metrics-collector"
	HubCASecretName                 = "observability-managed-cluster-certs"
	ClientCertSecretName            = "observability-controller-open-cluster-management.io-observability-signer-client-cert"
	PrometheusCAConfigMapName       = "prometheus-server-ca"
	PrometheusServerName            = "acm-prometheus-k8s" // For non ocp spokes
	HubInstallNamespace             = "open-cluster-management-observability"

	ManagedClusterLabelClusterID      = "clusterID"
	ManagedClusterLabelVendorKey      = "vendor"
	ManagedClusterLabelVendorOCPValue = "OpenShift"

	// Monitoring resources (meta monitoring)
	PlatformRBACProxyTLSSecret     = "prometheus-agent-platform-kube-rbac-proxy-tls"
	UserWorkloadRBACProxyTLSSecret = "prometheus-agent-user-workload-kube-rbac-proxy-tls"
	RBACProxyPort                  = 9092

	// Standard metrics label names
	ClusterNameMetricLabel           = "cluster"
	ClusterIDMetricLabel             = "clusterID"
	ManagementClusterNameMetricLabel = "managementcluster"
	ManagementClusterIDMetricLabel   = "managementclusterID"

	TargetNamespaceAnnotation = "observability.open-cluster-management.io/target-namespace"

	// Hypershift
	LocalManagedClusterLabel              = "local-cluster"
	HypershiftAddonStateLabel             = "feature.open-cluster-management.io/addon-hypershift-addon"
	HypershiftEtcdServiceMonitorName      = "etcd"
	HypershiftApiServerServiceMonitorName = "kube-apiserver"
	AcmEtcdServiceMonitorName             = "acm-etcd"
	AcmApiServerServiceMonitorName        = "acm-kube-apiserver"

	RemoteWriteCfgName        = "acm-observability"
	ScrapeClassCfgName        = "ocp-monitoring"
	NonOCPScrapeClassName     = "non-ocp-monitoring"
	ScrapeClassPlatformTarget = "prometheus-k8s.openshift-monitoring.svc:9091"
	ScrapeClassUWLTarget      = "prometheus-user-workload.openshift-user-workload-monitoring.svc:9092"
	MonitoringStackCRDName    = "monitoringstacks.monitoring.rhobs"

	AlertmanagerAccessorSecretName = "observability-alertmanager-accessor"
	AlertmanagerRouterCASecretName = "hub-alertmanager-router-ca"
	AlertmanagerRouteBYOCAName     = "alertmanager-byo-ca"
	AlertmanagerRouteBYOCERTName   = "alertmanager-byo-cert"
	AlertmanagerPlatformNamespace  = "openshift-monitoring"
	AlertmanagerUWLNamespace       = "openshift-user-workload-monitoring"
)

var (
	PlatformPrometheusMatchLabels = map[string]string{
		addoncfg.ComponentK8sLabelKey: "platform-metrics-collector",
	}
	UserWorkloadPrometheusMatchLabels = map[string]string{
		addoncfg.ComponentK8sLabelKey: "user-workload-metrics-collector",
	}
	EtcdHcpUserWorkloadPrometheusMatchLabels = map[string]string{
		addoncfg.ComponentK8sLabelKey: "etcd-hcp-user-workload-metrics-collector",
	}
	ApiserverHcpUserWorkloadPrometheusMatchLabels = map[string]string{
		addoncfg.ComponentK8sLabelKey: "apiserver-hcp-user-workload-metrics-collector",
	}

	ImagesConfigMapObjKey = types.NamespacedName{
		Name:      "images-list",
		Namespace: "open-cluster-management-observability",
	}

	RouterDefaultCertsConfigMapObjKey = types.NamespacedName{
		Name:      "router-certs-default",
		Namespace: "openshift-ingress",
	}

	ErrMissingImageOverride = errors.New("missing image override")
)

type ImageOverrides struct {
	PrometheusConfigReloader   string `json:"prometheus_config_reloader"`
	KubeRBACProxy              string `json:"kube_rbac_proxy"`
	CooPrometheusOperatorImage string `json:"obo_prometheus_rhel9_operator"`
	KubeStateMetrics           string `json:"kube_state_metrics"`
	NodeExporter               string `json:"node_exporter"`
	Prometheus                 string `json:"prometheus"`
}

func GetImageOverrides(ctx context.Context, c client.Client, registries []addonapiv1alpha1.ImageMirror, logger logr.Logger) (ImageOverrides, error) {
	ret := ImageOverrides{}
	// Get the ACM images overrides
	imagesList := &corev1.ConfigMap{}
	if err := c.Get(ctx, ImagesConfigMapObjKey, imagesList); err != nil {
		return ret, fmt.Errorf("failed to get image overrides configmap: %w", err)
	}

	jsonData, err := json.Marshal(imagesList.Data)
	if err != nil {
		return ret, fmt.Errorf("failed to marshal image overrides data: %w", err)
	}

	if err := json.Unmarshal(jsonData, &ret); err != nil {
		return ret, fmt.Errorf("failed to unmarshal image overrides: %w", err)
	}

	if ret.CooPrometheusOperatorImage == "" ||
		ret.PrometheusConfigReloader == "" ||
		ret.KubeRBACProxy == "" ||
		ret.KubeStateMetrics == "" ||
		ret.Prometheus == "" ||
		ret.NodeExporter == "" {
		return ret, fmt.Errorf("%w: %+v", ErrMissingImageOverride, ret)
	}

	// Apply registry overrides
	if len(registries) > 0 {
		ret.PrometheusConfigReloader = overrideImage(ret.PrometheusConfigReloader, registries, logger)
		ret.KubeRBACProxy = overrideImage(ret.KubeRBACProxy, registries, logger)
		ret.CooPrometheusOperatorImage = overrideImage(ret.CooPrometheusOperatorImage, registries, logger)
		ret.KubeStateMetrics = overrideImage(ret.KubeStateMetrics, registries, logger)
		ret.NodeExporter = overrideImage(ret.NodeExporter, registries, logger)
		ret.Prometheus = overrideImage(ret.Prometheus, registries, logger)
	}

	return ret, nil
}

func overrideImage(image string, registries []addonapiv1alpha1.ImageMirror, logger logr.Logger) string {
	for _, registry := range registries {
		if !strings.HasPrefix(image, registry.Source) {
			continue
		}

		// If lengths are equal, it's an exact match (e.g. image has no tag/digest, or source includes them)
		if len(image) == len(registry.Source) {
			return strings.Replace(image, registry.Source, registry.Mirror, 1)
		}

		// Check the character immediately following the match to ensure we matched a full image name component.
		// Allowed boundaries for an image override are ':' (tag) or '@' (digest).
		// We explicitly do NOT allow '/' as that would imply a registry or org level override.
		nextChar := image[len(registry.Source)]
		if nextChar == ':' || nextChar == '@' {
			return strings.Replace(image, registry.Source, registry.Mirror, 1)
		}

		// It matches as a prefix but it is not a full image override (e.g. matched "quay.io/org" against "quay.io/org/repo")
		logger.Info("Registry override ignored as it does not reference a full image", "source", registry.Source, "mirror", registry.Mirror, "image", image)
	}
	return image
}

func HasHostedCLusters(ctx context.Context, c client.Client, logger logr.Logger) bool {
	hostedClusters := &hyperv1.HostedClusterList{}
	if err := c.List(ctx, hostedClusters, &client.ListOptions{}); err != nil {
		logger.Error(err, "failed to list HostedClusterList")
		return false
	}

	return len(hostedClusters.Items) != 0
}

func GetTrimmedClusterID(clusterID string) string {
	// We use this ID later to postfix the follow secrets:
	// hub-alertmanager-router-ca
	// observability-alertmanager-accessor
	//
	// when prom-opreator mounts these secrets to the prometheus-k8s pod
	// it will take the name of the secret, and prepend `secret-` to the
	// volume mount name. However since this is volume mount name is a label
	// that must be at most 63 chars. Therefore we trim it here to 19 chars.
	idTrim := strings.ReplaceAll(clusterID, "-", "")
	return fmt.Sprintf("%.19s", idTrim)
}

func GetAlertmanagerRouterCASecretName(trimmedClusterID string) string {
	return fmt.Sprintf("%s-%s", AlertmanagerRouterCASecretName, trimmedClusterID)
}

func GetAlertmanagerAccessorSecretName(trimmedClusterID string) string {
	return fmt.Sprintf("%s-%s", AlertmanagerAccessorSecretName, trimmedClusterID)
}
