package resource

import (
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	_ "embed"
)

const (
	DefaultPlatformMetricsCollectorApp     = config.PlatformMetricsCollectorApp + "-default"
	defaultUserWorkloadMetricsCollectorApp = config.UserWorkloadMetricsCollectorApp + "-default"
	scrapeTimeout                          = "30s"
)

func DefaultPlaftformAgentResources(ns string) []client.Object {
	ret := []client.Object{}

	// Create platform resources
	agent := newPrometheusAgent(ns, DefaultPlatformMetricsCollectorApp, config.PlatformPrometheusMatchLabels, &metav1.LabelSelector{})
	ret = append(ret, agent) // listen only to the same namespace
	return ret
}

func DefaultUserWorkloadAgentResources(ns string) []client.Object {
	ret := []client.Object{}

	// Create user workload resources
	agent := newPrometheusAgent(ns, defaultUserWorkloadMetricsCollectorApp, config.UserWorkloadPrometheusMatchLabels, nil)
	ret = append(ret, agent) // listen to all namespaces

	return ret
}

// newPrometheusAgent is a helper function to create a PrometheusAgent resource with given parameters
func newPrometheusAgent(ns, appName string, labels map[string]string, namespaceSelector *metav1.LabelSelector) *prometheusalpha1.PrometheusAgent {
	agent := newDefaultPrometheusAgent()
	agent.Name = appName
	agent.Namespace = ns
	if agent.Labels == nil {
		agent.Labels = labels
	} else {
		for k, v := range labels {
			agent.Labels[k] = v
		}
	}
	agent.Spec.ScrapeConfigNamespaceSelector = namespaceSelector

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
				Replicas: intPtr(1),
				LogLevel: "info",
				NodeSelector: map[string]string{
					"kubernetes.io/os": "linux",
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("3m"),
						corev1.ResourceMemory: resource.MustParse("150Mi"),
					},
				},
				ScrapeInterval: "300s",
				SecurityContext: &corev1.PodSecurityContext{
					RunAsNonRoot: ptr.To(true),
				},
				ScrapeTimeout: scrapeTimeout,
				PortName:      "web", // set this value to the default to avoid triggering update when comparing the spec
			},
		},
	}
}
