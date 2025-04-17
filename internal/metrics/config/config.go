package config

import "k8s.io/apimachinery/pkg/types"

const (
	PrometheusControllerID          = "acm-observability"
	PlatformMetricsCollectorApp     = "acm-platform-metrics-collector"
	UserWorkloadMetricsCollectorApp = "acm-user-workload-metrics-collector"
	HubCASecretName                 = "observability-managed-cluster-certs"
	ClientCertSecretName            = "observability-controller-open-cluster-management.io-observability-signer-client-cert"
	PrometheusCAConfigMapName       = "prometheus-server-ca"
	EnvoyImage                      = "registry.redhat.io/openshift-service-mesh/proxyv2-rhel9@sha256:153130dd485f60c9b1e120d51b8228fc3100afa9a7f500c3caa13ccd41520e99"
	EnvoyAdminPort                  = 9901
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
