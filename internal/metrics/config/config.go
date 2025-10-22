package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
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

func GetImageOverrides(ctx context.Context, c client.Client) (ImageOverrides, error) {
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

	return ret, nil
}

func HasHostedCLusters(ctx context.Context, c client.Client, logger logr.Logger) bool {
	hostedClusters := &hyperv1.HostedClusterList{}
	if err := c.List(ctx, hostedClusters, &client.ListOptions{}); err != nil {
		logger.Error(err, "failed to list HostedClusterList")
		return false
	}

	return len(hostedClusters.Items) != 0
}
