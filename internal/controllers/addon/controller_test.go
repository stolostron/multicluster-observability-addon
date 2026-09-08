package addon

import (
	"context"
	"testing"

	monitoringv1alpha1 "github.com/rhobs/observability-operator/pkg/apis/monitoring/v1alpha1"
	addoncommon "github.com/stolostron/multicluster-observability-addon/internal/addon/common"
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
	"open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	fakeaddon "open-cluster-management.io/api/client/addon/clientset/versioned/fake"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

type mockAgent struct {
	manifests []runtime.Object
	called    bool
	options   agent.AgentAddonOptions
}

func (m *mockAgent) Manifests(_ context.Context, cluster *clusterv1.ManagedCluster, addon *addonapiv1beta1.ManagedClusterAddOn) ([]runtime.Object, error) {
	m.called = true
	return m.manifests, nil
}

func (m *mockAgent) GetAgentAddonOptions() agent.AgentAddonOptions {
	return m.options
}

func TestAgentConfigNamespaces(t *testing.T) {
	for _, resource := range []addonapiv1beta1.ConfigGroupResource{
		{Group: "addon.open-cluster-management.io", Resource: addoncfg.AddonDeploymentConfigResource},
		{Group: "observability.openshift.io", Resource: addoncfg.ClusterLogForwardersResource},
		{Group: "opentelemetry.io", Resource: addoncfg.OpenTelemetryCollectorsResource},
		{Group: "opentelemetry.io", Resource: addoncfg.InstrumentationResource},
		{Group: "monitoring.rhobs", Resource: "prometheusagents"},
		{Group: "monitoring.rhobs", Resource: "scrapeconfigs"},
		{Group: "monitoring.coreos.com", Resource: "prometheusrules"},
		{Group: "monitoring.rhobs", Resource: "prometheusrules"},
		{Resource: "configmaps"},
	} {
		for _, namespace := range []string{addoncfg.InstallNamespace, "cluster-a", "openshift-logging", "cluster-b", ""} {
			t.Run(resource.Group+"/"+resource.Resource+"/"+namespace, func(t *testing.T) {
				mcAddon := &addonapiv1beta1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name, Namespace: "cluster-a"},
					Status: addonapiv1beta1.ManagedClusterAddOnStatus{
						ConfigReferences: []addonapiv1beta1.ConfigReference{
							{
								ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
									Group: "addon.open-cluster-management.io", Resource: addoncfg.AddonDeploymentConfigResource,
								},
								DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
									ConfigReferent: addonapiv1beta1.ConfigReferent{Name: addoncfg.Name, Namespace: addoncfg.InstallNamespace},
									SpecHash:       "test-hash",
								},
							},
							{
								ConfigGroupResource: resource,
								DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
									ConfigReferent: addonapiv1beta1.ConfigReferent{Name: "instance", Namespace: namespace},
								},
							},
						},
					},
				}
				//nolint:staticcheck // The generated client does not provide NewClientset.
				client := fakeaddon.NewSimpleClientset(&addonapiv1beta1.AddOnDeploymentConfig{
					ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name, Namespace: addoncfg.InstallNamespace},
					Spec:       addonapiv1beta1.AddOnDeploymentConfigSpec{AgentInstallNamespace: "custom-agent-namespace"},
				})
				mock := &mockAgent{
					manifests: []runtime.Object{&corev1.Secret{}},
					options: agent.AgentAddonOptions{
						AgentInstallNamespace: utils.AgentInstallNamespaceFromDeploymentConfigFunc(utils.NewAddOnDeploymentConfigGetter(client)),
					},
				}
				wrapper := &AgentAddonWithSortedManifests{agent: mock}
				installNamespace, installErr := wrapper.GetAgentAddonOptions().AgentInstallNamespace(t.Context(), mcAddon)
				objects, err := wrapper.Manifests(t.Context(), nil, mcAddon)
				if namespace == addoncfg.InstallNamespace || namespace == mcAddon.Namespace {
					require.NoError(t, installErr)
					require.Equal(t, "custom-agent-namespace", installNamespace)
					require.Len(t, client.Actions(), 1)
					require.NoError(t, err)
					require.True(t, mock.called)
					require.Len(t, objects, 1)
					return
				}
				require.ErrorIs(t, err, addoncommon.ErrInvalidConfigNamespace)
				require.ErrorIs(t, installErr, addoncommon.ErrInvalidConfigNamespace)
				require.Empty(t, installNamespace)
				require.Empty(t, client.Actions(), "registration must reject references before fetching configuration")
				require.Nil(t, objects)
				require.False(t, mock.called, "reject the entire reference list before framework config reads or rendering")
			})
		}
	}
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

	sorted, err := prober.Manifests(t.Context(), nil, &addonapiv1beta1.ManagedClusterAddOn{})
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

	objects, err := agent.Manifests(t.Context(), nil, &addonapiv1beta1.ManagedClusterAddOn{})
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
	require.Len(t, objects, 4, "expected ThanosStore, ThanosReceive, ThanosQuery, and ThanosRuler objects")

	var store *thanosv1alpha1.ThanosStore
	var receive *thanosv1alpha1.ThanosReceive
	var query *thanosv1alpha1.ThanosQuery
	var ruler *thanosv1alpha1.ThanosRuler
	for _, obj := range objects {
		switch o := obj.(type) {
		case *thanosv1alpha1.ThanosStore:
			store = o
		case *thanosv1alpha1.ThanosReceive:
			receive = o
		case *thanosv1alpha1.ThanosQuery:
			query = o
		case *thanosv1alpha1.ThanosRuler:
			ruler = o
		}
	}

	require.NotNil(t, store, "expected ThanosStore object")
	assert.Equal(t, "mcoa", store.Name)
	assert.Equal(t, addoncfg.InstallNamespace, store.Namespace)

	require.NotNil(t, receive, "expected ThanosReceive object")
	assert.Equal(t, "mcoa", receive.Name)
	assert.Equal(t, addoncfg.InstallNamespace, receive.Namespace)
	require.Len(t, receive.Spec.Ingester.Hashrings, 1)
	assert.Equal(t, "default", receive.Spec.Ingester.Hashrings[0].Name)

	require.NotNil(t, query, "expected ThanosQuery object")
	assert.Equal(t, "mcoa", query.Name)
	assert.Equal(t, addoncfg.InstallNamespace, query.Namespace)
	assert.Equal(t, int32(2), query.Spec.Replicas)
	require.NotNil(t, query.Spec.QueryFrontend)
	assert.Equal(t, int32(2), query.Spec.QueryFrontend.Replicas)

	require.NotNil(t, ruler, "expected ThanosRuler object")
	assert.Equal(t, "mcoa", ruler.Name)
	assert.Equal(t, addoncfg.InstallNamespace, ruler.Namespace)
	assert.Equal(t, int32(3), ruler.Spec.Replicas)
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
