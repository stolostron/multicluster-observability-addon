package config

import (
	"context"
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
	HubInstallNamespace             = "open-cluster-management-observability"

	ManagedClusterLabelClusterID = "clusterID"

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
	ScrapeClassPlatformTarget = "prometheus-k8s.openshift-monitoring.svc:9091"
	ScrapeClassUWLTarget      = "prometheus-user-workload.openshift-user-workload-monitoring.svc:9092"
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

	ErrMissingImageOverride = errors.New("missing image override")
)

type ImageOverrides struct {
	PrometheusOperator       string
	PrometheusConfigReloader string
	KubeRBACProxy            string
	Prometheus               string
}

func GetImageOverrides(ctx context.Context, c client.Client) (ImageOverrides, error) {
	ret := ImageOverrides{}
	// Get the ACM images overrides
	imagesList := &corev1.ConfigMap{}
	if err := c.Get(ctx, ImagesConfigMapObjKey, imagesList); err != nil {
		return ret, fmt.Errorf("failed to get image overrides configmap: %w", err)
	}

	for key, value := range imagesList.Data {
		switch key {
		case "prometheus_operator":
			ret.PrometheusOperator = value
		case "prometheus_config_reloader":
			ret.PrometheusConfigReloader = value
		case "kube_rbac_proxy":
			ret.KubeRBACProxy = value
		case "prometheus":
			ret.Prometheus = value
		default:
		}
	}

	if ret.PrometheusOperator == "" || ret.PrometheusConfigReloader == "" || ret.KubeRBACProxy == "" || ret.Prometheus == "" {
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
