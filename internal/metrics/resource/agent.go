package resource

import (
	"fmt"

	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/metrics/config"
	"github.com/rhobs/multicluster-observability-addon/internal/metrics/handlers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultPlatformMetricsCollectorApp     = config.PlatformMetricsCollectorApp + "-default"
	defaultUserWorkloadMetricsCollectorApp = config.UserWorkloadMetricsCollectorApp + "-default"
	platformPrometheusService              = "prometheus-k8s.openshift-monitoring.svc.cluster.local:9091"
	userWorkloadPrometheusService          = "prometheus-user-workload.openshift-user-workload-monitoring.svc.cluster.local:9091"
	haProxyPort                            = 8090
)

func DefaultPlaftformAgentResources(ns string) []client.Object {
	ret := []client.Object{}

	// Create platform resources
	ret = append(ret, newPrometheusAgent(ns, defaultPlatformMetricsCollectorApp, handlers.PlatformMatchLabels, &metav1.LabelSelector{})) // listen only to the same namespace
	haProxyCm := fmt.Sprintf("%s-haproxy-config", defaultPlatformMetricsCollectorApp)
	ret = append(ret, newHaproxyConfig(ns, haProxyCm, platformPrometheusService, handlers.PlatformMatchLabels))

	return ret
}

func DefaultUserWorkloadAgentResources(ns string) []client.Object {
	ret := []client.Object{}

	// Create user workload resources
	ret = append(ret, newPrometheusAgent(ns, defaultUserWorkloadMetricsCollectorApp, handlers.UserWorkloadMatchLabels, nil)) // listen to all namespaces
	haProxyCm := fmt.Sprintf("%s-haproxy-config", defaultUserWorkloadMetricsCollectorApp)
	ret = append(ret, newHaproxyConfig(ns, haProxyCm, userWorkloadPrometheusService, handlers.UserWorkloadMatchLabels))

	return ret
}

// newPrometheusAgent is a helper function to create a PrometheusAgent resource with given parameters
func newPrometheusAgent(ns, appName string, labels map[string]string, namespaceSelector *metav1.LabelSelector) *prometheusalpha1.PrometheusAgent {
	agent := newDefaultPrometheusAgent()
	agent.ObjectMeta.Name = appName
	agent.ObjectMeta.Namespace = ns
	if agent.ObjectMeta.Labels == nil {
		agent.ObjectMeta.Labels = labels
	} else {
		for k, v := range labels {
			agent.ObjectMeta.Labels[k] = v
		}
	}
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

func newHaproxyConfig(ns, name, prometheusURL string, labels map[string]string) *corev1.ConfigMap {
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
			Labels:    labels,
		},
		Data: map[string]string{
			"haproxy.cfg": cfg,
		},
	}
}
