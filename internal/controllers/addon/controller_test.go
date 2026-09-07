package addon

import (
	"context"
	"testing"

	monitoringv1alpha1 "github.com/rhobs/observability-operator/pkg/apis/monitoring/v1alpha1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	thanosbuilder "github.com/stolostron/multicluster-observability-addon/internal/metrics/thanos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	thanosv1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"open-cluster-management.io/addon-framework/pkg/agent"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

type mockAgent struct {
	manifests []runtime.Object
}

func (m *mockAgent) Manifests(_ context.Context, cluster *clusterv1.ManagedCluster, addon *addonapiv1beta1.ManagedClusterAddOn) ([]runtime.Object, error) {
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
	prober := &AgentAddonWithSortedManifests{
		agent: mock,
	}

	sorted, err := prober.Manifests(t.Context(), nil, nil)
	require.NoError(t, err)
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

func TestManifestsConvertsMonitoringStackToUnstructured(t *testing.T) {
	ms := &monitoringv1alpha1.MonitoringStack{
		TypeMeta: metav1.TypeMeta{
			Kind:       "MonitoringStack",
			APIVersion: "monitoring.rhobs/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec:       monitoringv1alpha1.MonitoringStackSpec{
			// resources and alertmanagerConfig are zero-value (empty structs)
		},
	}

	mock := &mockAgent{manifests: []runtime.Object{ms}}
	agent := &AgentAddonWithSortedManifests{agent: mock}

	objects, err := agent.Manifests(t.Context(), nil, nil)
	require.NoError(t, err)
	require.Len(t, objects, 1)

	u, ok := objects[0].(*unstructured.Unstructured)
	require.True(t, ok, "MonitoringStack should be converted to *unstructured.Unstructured")

	spec, _, _ := unstructured.NestedMap(u.Object, "spec")
	assert.NotContains(t, spec, "resources", "empty spec.resources should be stripped")
	assert.NotContains(t, spec, "alertmanagerConfig", "empty spec.alertmanagerConfig should be stripped")
}

func TestToUnstructuredMonitoringStackPreservesNonEmptyFields(t *testing.T) {
	ms := &monitoringv1alpha1.MonitoringStack{
		TypeMeta: metav1.TypeMeta{
			Kind:       "MonitoringStack",
			APIVersion: "monitoring.rhobs/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: monitoringv1alpha1.MonitoringStackSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				},
			},
		},
	}

	a := &AgentAddonWithSortedManifests{agent: &mockAgent{}}
	obj := a.toUnstructuredMonitoringStack(ms)

	u, ok := obj.(*unstructured.Unstructured)
	require.True(t, ok)

	resources, found, err := unstructured.NestedMap(u.Object, "spec", "resources")
	require.NoError(t, err)
	assert.True(t, found, "non-empty spec.resources should be preserved")
	assert.NotEmpty(t, resources)
}

type mockAODCGetter struct {
	aodc *addonapiv1beta1.AddOnDeploymentConfig
}

func (m *mockAODCGetter) Get(_ context.Context, _, _ string) (*addonapiv1beta1.AddOnDeploymentConfig, error) {
	return m.aodc, nil
}

func TestManifestsWithObjectBuilders(t *testing.T) {
	hubCluster := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "local-cluster",
			Labels: map[string]string{"local-cluster": "true"},
		},
	}

	aodc := &addonapiv1beta1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:        addoncfg.Name,
			Namespace:   addoncfg.InstallNamespace,
			Annotations: map[string]string{"mcoa-thanos-operator": "true"},
		},
	}

	mcAddon := &addonapiv1beta1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addoncfg.Name,
			Namespace: "local-cluster",
		},
		Status: addonapiv1beta1.ManagedClusterAddOnStatus{
			ConfigReferences: []addonapiv1beta1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
						Group:    "addon.open-cluster-management.io",
						Resource: addoncfg.AddonDeploymentConfigResource,
					},
					DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
						ConfigReferent: addonapiv1beta1.ConfigReferent{
							Namespace: addoncfg.InstallNamespace,
							Name:      addoncfg.Name,
						},
					},
				},
			},
		},
	}

	getter := &mockAODCGetter{aodc: aodc}
	thanosBuilder := &thanosbuilder.ObjectBuilder{
		Logger: klog.Background(),
	}

	wrapper := &AgentAddonWithSortedManifests{
		agent:          &mockAgent{manifests: []runtime.Object{}},
		getter:         getter,
		objectBuilders: []ObjectBuilder{thanosBuilder},
	}

	objects, err := wrapper.Manifests(t.Context(), hubCluster, mcAddon)
	require.NoError(t, err)
	require.Len(t, objects, 1, "expected 1 ThanosStore object")

	store, ok := objects[0].(*thanosv1alpha1.ThanosStore)
	require.True(t, ok, "expected *ThanosStore, got %T", objects[0])
	assert.Equal(t, "mcoa", store.Name)
	assert.Equal(t, addoncfg.InstallNamespace, store.Namespace)
}

func TestManifestsObjectBuildersSkippedForNonHub(t *testing.T) {
	spokeCluster := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "spoke-1"},
	}

	aodc := &addonapiv1beta1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:        addoncfg.Name,
			Namespace:   addoncfg.InstallNamespace,
			Annotations: map[string]string{"mcoa-thanos-operator": "true"},
		},
	}

	mcAddon := &addonapiv1beta1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addoncfg.Name,
			Namespace: "spoke-1",
		},
		Status: addonapiv1beta1.ManagedClusterAddOnStatus{
			ConfigReferences: []addonapiv1beta1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
						Group:    "addon.open-cluster-management.io",
						Resource: addoncfg.AddonDeploymentConfigResource,
					},
					DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
						ConfigReferent: addonapiv1beta1.ConfigReferent{
							Namespace: addoncfg.InstallNamespace,
							Name:      addoncfg.Name,
						},
					},
				},
			},
		},
	}

	getter := &mockAODCGetter{aodc: aodc}
	thanosBuilder := &thanosbuilder.ObjectBuilder{
		Logger: klog.Background(),
	}

	wrapper := &AgentAddonWithSortedManifests{
		agent:          &mockAgent{manifests: []runtime.Object{}},
		getter:         getter,
		objectBuilders: []ObjectBuilder{thanosBuilder},
	}

	objects, err := wrapper.Manifests(t.Context(), spokeCluster, mcAddon)
	require.NoError(t, err)
	assert.Empty(t, objects, "no Thanos objects should be built for non-hub clusters")
}
