package config

import "k8s.io/apimachinery/pkg/types"

const (
	PrometheusControllerID          = "acm-observability"
	PlatformMetricsCollectorApp     = "acm-platform-metrics-collector"
	UserWorkloadMetricsCollectorApp = "acm-user-workload-metrics-collector"
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
	ImagesConfigMap = types.NamespacedName{
		Name:      "images-list",
		Namespace: "open-cluster-management-observability",
	}
)
