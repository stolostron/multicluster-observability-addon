package resource

import (
	"fmt"
	"strings"

	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/metrics/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	_ "embed"
)

//go:embed resources/envoy.yaml
var envoyConfig string

const (
	DefaultPlatformMetricsCollectorApp     = config.PlatformMetricsCollectorApp + "-default"
	DefaultPlatformEnvoyConfigMap          = DefaultPlatformMetricsCollectorApp + "-envoy-config"
	defaultUserWorkloadMetricsCollectorApp = config.UserWorkloadMetricsCollectorApp + "-default"
	DefaultUserWorkloadEnvoyConfigMap      = defaultUserWorkloadMetricsCollectorApp + "-envoy-config"
	platformPrometheusService              = "prometheus-k8s.openshift-monitoring.svc.cluster.local:9091"
	userWorkloadPrometheusService          = "prometheus-user-workload.openshift-user-workload-monitoring.svc.cluster.local:9092"
	envoyAdminPort                         = 9091
	envoyProxyPortForPrometheus            = 8090
	scrapeTimeout                          = "30s"
)

func DefaultPlaftformAgentResources(ns string) []client.Object {
	ret := []client.Object{}

	// Create platform resources
	agent := newPrometheusAgent(ns, DefaultPlatformMetricsCollectorApp, config.PlatformPrometheusMatchLabels, &metav1.LabelSelector{})
	proxyCfg := newEnvoyConfigMap(ns, DefaultPlatformEnvoyConfigMap, platformPrometheusService, config.PlatformPrometheusMatchLabels)
	agent.Spec.ConfigMaps = []string{proxyCfg.Name}
	ret = append(ret, agent) // listen only to the same namespace
	ret = append(ret, proxyCfg)
	return ret
}

func DefaultUserWorkloadAgentResources(ns string) []client.Object {
	ret := []client.Object{}

	// Create user workload resources
	agent := newPrometheusAgent(ns, defaultUserWorkloadMetricsCollectorApp, config.UserWorkloadPrometheusMatchLabels, nil)
	proxyCfg := newEnvoyConfigMap(ns, DefaultUserWorkloadEnvoyConfigMap, userWorkloadPrometheusService, config.UserWorkloadPrometheusMatchLabels)
	agent.Spec.ConfigMaps = []string{proxyCfg.Name}
	ret = append(ret, agent) // listen to all namespaces
	ret = append(ret, proxyCfg)

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
				Replicas: intPtr(1),
				LogLevel: "info",
				NodeSelector: map[string]string{
					"kubernetes.io/os": "linux",
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("6m"),
						corev1.ResourceMemory: resource.MustParse("200Mi"),
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

func newEnvoyConfigMap(ns, name string, prometheusURL string, labels map[string]string) *corev1.ConfigMap {
	splitPromUrl := strings.Split(prometheusURL, ":")
	cfg := fmt.Sprintf(envoyConfig, envoyAdminPort, envoyProxyPortForPrometheus, scrapeTimeout, splitPromUrl[0], splitPromUrl[1])

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
			"envoy.yaml": cfg,
		},
	}
}
