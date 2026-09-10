package addon

import (
	"testing"

	addoncommon "github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"open-cluster-management.io/addon-framework/pkg/agent"
	"open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	fakeaddon "open-cluster-management.io/api/client/addon/clientset/versioned/fake"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

type mockAgent struct {
	manifests []runtime.Object
	called    bool
	options   agent.AgentAddonOptions
}

func (m *mockAgent) Manifests(cluster *clusterv1.ManagedCluster, addon *addonapiv1alpha1.ManagedClusterAddOn) ([]runtime.Object, error) {
	m.called = true
	return m.manifests, nil
}

func (m *mockAgent) GetAgentAddonOptions() agent.AgentAddonOptions {
	return m.options
}

func TestAgentConfigNamespaces(t *testing.T) {
	for _, resource := range []addonapiv1alpha1.ConfigGroupResource{
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
				mcAddon := &addonapiv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name, Namespace: "cluster-a"},
					Status: addonapiv1alpha1.ManagedClusterAddOnStatus{
						ConfigReferences: []addonapiv1alpha1.ConfigReference{
							{
								ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
									Group: "addon.open-cluster-management.io", Resource: addoncfg.AddonDeploymentConfigResource,
								},
								DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
									ConfigReferent: addonapiv1alpha1.ConfigReferent{Name: addoncfg.Name, Namespace: addoncfg.InstallNamespace},
									SpecHash:       "test-hash",
								},
							},
							{
								ConfigGroupResource: resource,
								DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
									ConfigReferent: addonapiv1alpha1.ConfigReferent{Name: "instance", Namespace: namespace},
								},
							},
						},
					},
				}
				//nolint:staticcheck // The generated client does not provide NewClientset.
				client := fakeaddon.NewSimpleClientset(&addonapiv1alpha1.AddOnDeploymentConfig{
					ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name, Namespace: addoncfg.InstallNamespace},
					Spec:       addonapiv1alpha1.AddOnDeploymentConfigSpec{AgentInstallNamespace: "custom-agent-namespace"},
				})
				mock := &mockAgent{
					manifests: []runtime.Object{&corev1.Secret{}},
					options: agent.AgentAddonOptions{
						Registration: &agent.RegistrationOption{
							AgentInstallNamespace: utils.AgentInstallNamespaceFromDeploymentConfigFunc(utils.NewAddOnDeploymentConfigGetter(client)),
						},
					},
				}
				wrapper := &AgentAddonWithSortedManifests{agent: mock}
				installNamespace, installErr := wrapper.GetAgentAddonOptions().Registration.AgentInstallNamespace(mcAddon)
				objects, err := wrapper.Manifests(nil, mcAddon)
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

	sorted, err := prober.Manifests(nil, &addonapiv1alpha1.ManagedClusterAddOn{})
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
