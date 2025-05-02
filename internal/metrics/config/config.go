package config

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	PrometheusControllerID          = "acm-observability"
	PlatformMetricsCollectorApp     = "platform-metrics-collector"
	UserWorkloadMetricsCollectorApp = "user-workload-metrics-collector"
	HubCASecretName                 = "observability-managed-cluster-certs"
	ClientCertSecretName            = "observability-controller-open-cluster-management.io-observability-signer-client-cert"
	PrometheusCAConfigMapName       = "prometheus-server-ca"
	HubInstallNamespace             = "open-cluster-management-observability"

	ManagedClusterLabelClusterID = "clusterID"

	ClusterNameMetricLabel           = "cluster"
	ClusterIDMetricLabel             = "clusterID"
	ManagementClusterNameMetricLabel = "managementcluster"
	ManagementClusterIDMetricLabel   = "managementclusterID"

	LocalManagedClusterLabel              = "local-cluster"
	HypershiftAddonStateLabel             = "feature.open-cluster-management.io/addon-hypershift-addon"
	HypershiftEtcdServiceMonitorName      = "etcd"
	HypershiftApiServerServiceMonitorName = "kube-apiserver"
	AcmEtcdServiceMonitorName             = "acm-etcd"
	AcmApiServerServiceMonitorName        = "acm-kube-apiserver"

	PlacementRefNameLabelKey      = "placement-ref-name"
	PlacementRefNamespaceLabelKey = "placement-ref-namespace"
	RemoteWriteCfgName            = "acm-observability"
	ScrapeClassCfgName            = "ocp-monitoring"
)

var (
	PlatformPrometheusMatchLabels = map[string]string{
		"app.kubernetes.io/component": "platform-metrics-collector",
	}
	UserWorkloadPrometheusMatchLabels = map[string]string{
		"app.kubernetes.io/component": "user-workload-metrics-collector",
	}
	EtcdHcpUserWorkloadPrometheusMatchLabels = map[string]string{
		"app.kubernetes.io/component": "etcd-hcp-user-workload-metrics-collector",
	}
	ApiserverHcpUserWorkloadPrometheusMatchLabels = map[string]string{
		"app.kubernetes.io/component": "apiserver-hcp-user-workload-metrics-collector",
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
		return ret, err
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
