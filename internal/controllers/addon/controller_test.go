package addon

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"open-cluster-management.io/addon-framework/pkg/agent"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

type mockAgent struct {
	manifests []runtime.Object
}

func (m *mockAgent) Manifests(cluster *clusterv1.ManagedCluster, addon *addonapiv1alpha1.ManagedClusterAddOn) ([]runtime.Object, error) {
	return m.manifests, nil
}

func (m *mockAgent) GetAgentAddonOptions() agent.AgentAddonOptions {
	return agent.AgentAddonOptions{}
}

func TestManifestsSorting(t *testing.T) {
	// Create unsorted manifests
	manifests := []runtime.Object{
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "b-config",
				Namespace: "ns1",
			},
		},
		&corev1.Secret{
			TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "a-secret",
				Namespace: "ns1",
			},
		},
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "a-config",
				Namespace: "ns1",
			},
		},
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "a-config",
				Namespace: "ns2",
			},
		},
	}

	mock := &mockAgent{manifests: manifests}
	// We don't need logger or client for Manifests call as per current implementation
	prober := &AgentAddonWithDynamicHealthProber{
		agent: mock,
	}

	sorted, err := prober.Manifests(nil, nil)
	assert.NoError(t, err)
	assert.Len(t, sorted, 4)

	// Expected order:
	// 1. ConfigMap ns1/a-config
	// 2. ConfigMap ns1/b-config
	// 3. ConfigMap ns2/a-config
	// 4. Secret ns1/a-secret

	// Check 1: ConfigMap ns1/a-config
	assert.Equal(t, "ConfigMap", sorted[0].GetObjectKind().GroupVersionKind().Kind)
	assert.Equal(t, "a-config", sorted[0].(*corev1.ConfigMap).Name)
	assert.Equal(t, "ns1", sorted[0].(*corev1.ConfigMap).Namespace)

	// Check 2: ConfigMap ns1/b-config
	assert.Equal(t, "ConfigMap", sorted[1].GetObjectKind().GroupVersionKind().Kind)
	assert.Equal(t, "b-config", sorted[1].(*corev1.ConfigMap).Name)
	assert.Equal(t, "ns1", sorted[1].(*corev1.ConfigMap).Namespace)

	// Check 3: ConfigMap ns2/a-config
	assert.Equal(t, "ConfigMap", sorted[2].GetObjectKind().GroupVersionKind().Kind)
	assert.Equal(t, "a-config", sorted[2].(*corev1.ConfigMap).Name)
	assert.Equal(t, "ns2", sorted[2].(*corev1.ConfigMap).Namespace)

	// Check 4: Secret
	assert.Equal(t, "Secret", sorted[3].GetObjectKind().GroupVersionKind().Kind)
}
