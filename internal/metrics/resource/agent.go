package resource

import (
	"fmt"

	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultPlatformMetricsCollectorApp     = "acm-platform-metrics-collector-default"
	defaultUserWorkloadMetricsCollectorApp = "acm-user-workload-metrics-collector-default"
	platformPrometheusService              = "prometheus-k8s.openshift-monitoring.svc.cluster.local:9091"
	userWorkloadPrometheusService          = "prometheus-user-workload.openshift-user-workload-monitoring.svc.cluster.local:9091"
	haProxyPort                            = 8090
)

func NewDefaultResources(ns string) ([]client.Object, error) {
	ret := []client.Object{}

	// Create platform resources
	haProxyCm := fmt.Sprintf("%s-haproxy-config", defaultPlatformMetricsCollectorApp)
	ret = append(ret, newPrometheusAgent(ns, defaultPlatformMetricsCollectorApp, &metav1.LabelSelector{}, haProxyCm)) // listen only to the same namespace
	ret = append(ret, newHaproxyConfig(ns, haProxyCm, platformPrometheusService))

	// Create user workload resources
	haProxyCm = fmt.Sprintf("%s-haproxy-config", defaultUserWorkloadMetricsCollectorApp)
	ret = append(ret, newPrometheusAgent(ns, defaultUserWorkloadMetricsCollectorApp, nil, haProxyCm)) // listen to all namespaces
	ret = append(ret, newHaproxyConfig(ns, haProxyCm, userWorkloadPrometheusService))

	return ret, nil
}

// newPrometheusAgent is a helper function to create a PrometheusAgent resource with given parameters
func newPrometheusAgent(ns, appName string, namespaceSelector *metav1.LabelSelector, haProxyCm string) *prometheusalpha1.PrometheusAgent {
	agent := newDefaultPrometheusAgent()
	agent.ObjectMeta.Name = appName
	agent.ObjectMeta.Namespace = ns
	if agent.ObjectMeta.Labels == nil {
		agent.ObjectMeta.Labels = map[string]string{}
	}
	agent.ObjectMeta.Labels["app"] = appName
	agent.Spec.CommonPrometheusFields.ScrapeConfigNamespaceSelector = namespaceSelector

	return agent
}

func newDefaultPrometheusAgent() *prometheusalpha1.PrometheusAgent {
	intPtr := func(i int32) *int32 {
		return &i
	}

	return &prometheusalpha1.PrometheusAgent{
		TypeMeta: metav1.TypeMeta{
			Kind:       prometheusalpha1.PrometheusAgentsKind,
			APIVersion: prometheusalpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{},
		Spec: prometheusalpha1.PrometheusAgentSpec{
			CommonPrometheusFields: prometheusv1.CommonPrometheusFields{
				Replicas:       intPtr(1),
				LogLevel:       "info",
				ScrapeInterval: "300s",
				ScrapeTimeout:  "30s",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
			},
		},
	}
}

func newHaproxyConfig(ns, name, prometheusURL string) *corev1.ConfigMap {
	cfg := fmt.Sprintf(`global
	log stdout format raw daemon
	maxconn 100

defaults
	log global
	mode http
	option httplog
	option http-server-close
	option forwardfor
	timeout connect 5s
	timeout client  10s
	timeout server  10s

frontend metrics
	bind *:8082
	mode http
	http-request use-service prometheus-exporter if { path /metrics }

frontend healthcheck
	bind *:8081
	mode http
	monitor-uri /healthz

frontend prometheus_frontend
	bind 127.0.0.1:%d
	mode http
	default_backend prometheus_backend

backend prometheus_backend
	mode http
	# Point to the Prometheus backend server (TLS-enabled)
	server prometheus-server %s ssl ca-file /etc/haproxy/certs/service-ca.crt verify required
	http-request set-header Authorization "Bearer ${BEARER_TOKEN}"
	`, haProxyPort, prometheusURL)

	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Data: map[string]string{
			"haproxy.cfg": cfg,
		},
	}
}
