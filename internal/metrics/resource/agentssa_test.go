package resource_test

import (
	"slices"
	"testing"

	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/resource"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestPrometheusAgentSSA(t *testing.T) {
	testCases := []struct {
		Name          string
		ExistingAgent *cooprometheusv1alpha1.PrometheusAgent
		Labels        map[string]string
		Expect        func(*testing.T, *cooprometheusv1alpha1.PrometheusAgent)
	}{
		{
			Name: "mandatory fields are set",
			ExistingAgent: &cooprometheusv1alpha1.PrometheusAgent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: cooprometheusv1alpha1.PrometheusAgentSpec{},
			},
			Expect: func(t *testing.T, agent *cooprometheusv1alpha1.PrometheusAgent) {
				assert.NotEmpty(t, agent.Spec.ServiceAccountName)
				assert.Empty(t, agent.Spec.Image)                                         // is set by the obo-prometheus operator
				assert.Equal(t, agent.Spec.Containers[0].Image, "kube-rbac-proxy:latest") // kube-rbac-proxy image
				assert.NotEmpty(t, agent.Spec.RemoteWrite)
				assert.NotEmpty(t, agent.Spec.RemoteWrite[0].URL)
			},
		},
		{
			Name: "labels are set",
			Labels: map[string]string{
				"placement": "a",
			},
			ExistingAgent: &cooprometheusv1alpha1.PrometheusAgent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Labels:    map[string]string{"dummy": "dummy"},
				},
				Spec: cooprometheusv1alpha1.PrometheusAgentSpec{},
			},
			Expect: func(t *testing.T, agent *cooprometheusv1alpha1.PrometheusAgent) {
				assert.NotEmpty(t, agent.Labels["dummy"])
				assert.NotEmpty(t, agent.Labels["placement"])
			},
		},
		{
			Name: "remote write config is set",
			ExistingAgent: &cooprometheusv1alpha1.PrometheusAgent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: cooprometheusv1alpha1.PrometheusAgentSpec{
					CommonPrometheusFields: cooprometheusv1.CommonPrometheusFields{
						Secrets: []string{"test", config.HubCASecretName},
						RemoteWrite: []cooprometheusv1.RemoteWriteSpec{
							{
								Name: ptr.To("custom"),
							},
							{
								URL: "url",
							},
							{
								Name: ptr.To(config.RemoteWriteCfgName),
								URL:  "another",
								QueueConfig: &cooprometheusv1.QueueConfig{
									Capacity: 1,
								},
							},
						},
					},
				},
			},
			Expect: func(t *testing.T, agent *cooprometheusv1alpha1.PrometheusAgent) {
				// Ensure user secrets are kept, and remote write ones are set
				assert.Contains(t, agent.Spec.Secrets, "test")
				assert.Contains(t, agent.Spec.Secrets, config.HubCASecretName)
				assert.Contains(t, agent.Spec.Secrets, config.ClientCertSecretName)
				assert.Len(t, agent.Spec.Secrets, 3)
				// Ensure that user custom remote write configs are kept
				assert.Len(t, agent.Spec.RemoteWrite, 3)
				// Ensure that user custom queue config is maintained and required fields are enforced
				index := slices.IndexFunc(agent.Spec.RemoteWrite, func(e cooprometheusv1.RemoteWriteSpec) bool {
					return e.Name != nil && *e.Name == config.RemoteWriteCfgName
				})
				assert.Equal(t, 1, agent.Spec.RemoteWrite[index].QueueConfig.Capacity)
				assert.Equal(t, 1, agent.Spec.RemoteWrite[index].QueueConfig.Capacity)
				assert.Equal(t, "https://example.com/write", agent.Spec.RemoteWrite[index].URL)
			},
		},
		{
			Name: "scrapeClasses are set",
			ExistingAgent: &cooprometheusv1alpha1.PrometheusAgent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: cooprometheusv1alpha1.PrometheusAgentSpec{
					CommonPrometheusFields: cooprometheusv1.CommonPrometheusFields{
						ConfigMaps: []string{"cm"},
						ScrapeClasses: []cooprometheusv1.ScrapeClass{
							{
								Name: "one",
							},
							{
								Name: config.ScrapeClassCfgName,
								Authorization: &cooprometheusv1.Authorization{
									CredentialsFile: "dummy",
								},
								MetricRelabelings: []cooprometheusv1.RelabelConfig{
									{
										TargetLabel: "test",
									},
								},
							},
						},
					},
				},
			},
			Expect: func(t *testing.T, agent *cooprometheusv1alpha1.PrometheusAgent) {
				assert.Equal(t, agent.Spec.ConfigMaps, []string{"cm", config.PrometheusCAConfigMapName})
				assert.Len(t, agent.Spec.ScrapeClasses, 2)
				index := slices.IndexFunc(agent.Spec.ScrapeClasses, func(e cooprometheusv1.ScrapeClass) bool {
					return e.Name == config.ScrapeClassCfgName
				})
				assert.Equal(t, "/var/run/secrets/kubernetes.io/serviceaccount/token", agent.Spec.ScrapeClasses[index].Authorization.CredentialsFile)
				assert.Len(t, agent.Spec.ScrapeClasses[index].MetricRelabelings, 1)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			builder := resource.PrometheusAgentSSA{
				ExistingAgent:       tc.ExistingAgent,
				RemoteWriteEndpoint: "https://example.com/write",
				KubeRBACProxyImage:  "kube-rbac-proxy:latest",
				Labels:              tc.Labels,
			}

			result := builder.Build()
			tc.Expect(t, result)
		})
	}
}
