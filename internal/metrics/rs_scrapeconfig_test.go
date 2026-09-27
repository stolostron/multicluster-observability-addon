package metrics_test

import (
	"encoding/json"
	"os"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	monitoringv1alpha1 "github.com/rhobs/observability-operator/pkg/apis/monitoring/v1alpha1"
	clusterinfov1beta1 "github.com/stolostron/cluster-lifecycle-api/clusterinfo/v1beta1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	addonhelm "github.com/stolostron/multicluster-observability-addon/internal/addon/helm"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	internalres "github.com/stolostron/multicluster-observability-addon/internal/metrics/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	kubescheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager/addontesting"
	"open-cluster-management.io/addon-framework/pkg/agent"
	"open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	controllerIDAnnotation = "operator.prometheus.io/controller-id"
	managedByLabel         = "app.kubernetes.io/managed-by"
)

type objectIdentity struct {
	gk        schema.GroupKind
	namespace string
	name      string
}

type rsSetup struct {
	cooInstalled   bool
	namespaceRS    string
	virtRS         string
	withMCOCopy    bool
	platformSCName string
}

// setupFullMCOA wires the full MCOA chart with the real values function. When withMCOCopy is
// set, the right-sizing ScrapeConfig shipped by MCO (testdata/mco-rs-scrape-config.yaml) reaches
// the addon through the configuration references, as it does on a hub where MCO deploys it.
func setupFullMCOA(t *testing.T, s rsSetup) (agent.AgentAddon, *clusterv1.ManagedCluster, *addonapiv1beta1.ManagedClusterAddOn) {
	t.Helper()
	hubNamespace := "open-cluster-management-observability"

	scheme := runtime.NewScheme()
	require.NoError(t, kubescheme.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	require.NoError(t, configv1.AddToScheme(scheme))
	require.NoError(t, cooprometheusv1alpha1.AddToScheme(scheme))
	require.NoError(t, monitoringv1alpha1.AddToScheme(scheme))
	require.NoError(t, prometheusv1.AddToScheme(scheme))
	require.NoError(t, cooprometheusv1.AddToScheme(scheme))
	require.NoError(t, clusterv1.Install(scheme))
	require.NoError(t, addonapiv1beta1.Install(scheme))
	require.NoError(t, workv1.Install(scheme))

	platformSC := &cooprometheusv1alpha1.ScrapeConfig{
		TypeMeta: metav1.TypeMeta{Kind: cooprometheusv1alpha1.ScrapeConfigsKind, APIVersion: cooprometheusv1alpha1.SchemeGroupVersion.Identifier()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.platformSCName,
			Namespace: hubNamespace,
			Labels:    config.PlatformPrometheusMatchLabels,
		},
	}
	clusterVersion := &configv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{Name: "version"},
		Spec:       configv1.ClusterVersionSpec{ClusterID: configv1.ClusterID(testClusterID)},
	}

	aodc := newAddonDeploymentConfig()
	aodc.Spec.CustomizedVariables = append(aodc.Spec.CustomizedVariables,
		addonapiv1beta1.CustomizedVariable{Name: addon.KeyPlatformMetricsCollection, Value: string(addon.PrometheusAgentV1alpha1)},
		addonapiv1beta1.CustomizedVariable{Name: addon.KeyMetricsHubHostname, Value: "metrics.example.com"},
		addonapiv1beta1.CustomizedVariable{Name: addon.KeyRightSizingDelegated, Value: "true"},
		addonapiv1beta1.CustomizedVariable{Name: addon.KeyPlatformNamespaceRightSizing, Value: s.namespaceRS},
		addonapiv1beta1.CustomizedVariable{Name: addon.KeyPlatformVirtualizationRightSizing, Value: s.virtRS},
	)

	managedCluster := addontesting.NewManagedCluster("cluster-1")
	managedCluster.Labels = map[string]string{
		addoncfg.ManagedClusterLabelClusterID: testClusterID,
		clusterinfov1beta1.LabelKubeVendor:    string(clusterinfov1beta1.KubeVendorOpenShift),
	}
	cmao := newCMOA()

	clientObjects := []client.Object{
		platformSC, clusterVersion, aodc, managedCluster, cmao,
		newSecret(config.HubCASecretName, hubNamespace),
		newSecret(config.ClientCertSecretName, hubNamespace),
		newSecret(config.AlertmanagerAccessorSecretName, hubNamespace),
		newManifestWork("cluster-1", s.cooInstalled),
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.ImagesConfigMapObjKey.Name, Namespace: config.ImagesConfigMapObjKey.Namespace},
			Data: map[string]string{
				"obo_prometheus_rhel9_operator": "quay.io/prometheus/obo-operator",
				"prometheus_config_reloader":    "quay.io/prometheus/config-reloader",
				"kube_rbac_proxy":               "quay.io/kube/rbac-proxy",
				"kube_state_metrics":            "quay.io/kube/kube-state-metrics",
				"node_exporter":                 "quay.io/kube/node-exporter",
				"prometheus":                    "quay.io/prometheus/prometheus",
				"endpoint_monitoring_operator":  "quay.io/stolostron/endpoint-monitoring-operator",
			},
		},
	}
	configReferences := []addonapiv1beta1.ConfigReference{newConfigReference(platformSC), newConfigReference(aodc)}

	if s.withMCOCopy {
		raw, err := os.ReadFile("testdata/mco-rs-scrape-config.yaml")
		require.NoError(t, err)
		obj, _, err := serializer.NewCodecFactory(scheme).UniversalDeserializer().Decode(raw, nil, nil)
		require.NoError(t, err)
		mcoCopy, ok := obj.(*cooprometheusv1alpha1.ScrapeConfig)
		require.True(t, ok)
		require.Equal(t, rightsizing.ScrapeConfigName, mcoCopy.Name)
		clientObjects = append(clientObjects, mcoCopy)
		configReferences = append(configReferences, newConfigReference(mcoCopy))
	}

	c := fakeclient.NewClientBuilder().
		WithInterceptorFuncs(ensureGVKIsSet(scheme)).
		WithScheme(scheme).
		WithObjects(clientObjects...).
		Build()

	defaultStack := internalres.DefaultStackResources{
		Client:       c,
		CMAO:         cmao,
		AddonOptions: newAddonOptions(true, false),
		Logger:       klog.Background(),
	}
	dc, err := defaultStack.Reconcile(t.Context())
	require.NoError(t, err)
	require.NoError(t, common.EnsureAddonConfig(t.Context(), klog.Background(), c, dc))

	agents := cooprometheusv1alpha1.PrometheusAgentList{}
	require.NoError(t, c.List(t.Context(), &agents))
	for i := range agents.Items {
		configReferences = append(configReferences, newConfigReference(&agents.Items[i]))
	}

	mca := addontesting.NewAddon("test", "cluster-1")
	mca.Status.ConfigReferences = configReferences

	getter := mockAODCGetter{aodc}
	agentAddon, err := addonfactory.NewAgentAddonFactory(addoncfg.Name, addon.FS, addoncfg.McoaChartDir).
		WithGetValuesFuncs(addonhelm.GetValuesFunc(t.Context(), c, getter, klog.Background())).
		WithAgentRegistrationOption(&agent.RegistrationOption{}).
		WithAgentInstallNamespace(utils.AgentInstallNamespaceFromDeploymentConfigFunc(getter)).
		WithScheme(scheme).
		BuildHelmAgentAddon()
	require.NoError(t, err)

	return agentAddon, managedCluster, mca
}

func identityOf(t *testing.T, obj runtime.Object) objectIdentity {
	t.Helper()
	acc, err := meta.Accessor(obj)
	require.NoError(t, err)
	return objectIdentity{gk: obj.GetObjectKind().GroupVersionKind().GroupKind(), namespace: acc.GetNamespace(), name: acc.GetName()}
}

// lastObjectPerIdentity mimics the ManifestWork builder, which keeps the last object it sees for
// each kind, namespace and name.
func lastObjectPerIdentity(t *testing.T, objects []runtime.Object) map[objectIdentity]string {
	t.Helper()
	ret := map[objectIdentity]string{}
	for _, obj := range objects {
		content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		require.NoError(t, err)
		raw, err := json.Marshal(content)
		require.NoError(t, err)
		ret[identityOf(t, obj)] = string(raw)
	}
	return ret
}

func matchParams(metrics ...[]string) []string {
	var ret []string
	for _, list := range metrics {
		for _, m := range list {
			ret = append(ret, `{__name__="`+m+`"}`)
		}
	}
	return ret
}

func TestHelmBuild_MCOA_RightSizingScrapeConfig(t *testing.T) {
	t.Setenv("UNIT_TEST", "true")

	for _, tc := range []struct {
		name             string
		setup            rsSetup
		wantMatch        []string // nil means no right-sizing ScrapeConfig is expected
		wantControllerID string
		wantManagedBy    string
	}{
		{
			name:             "MCO copy present, both right-sizing features, MCOA prometheus operator",
			setup:            rsSetup{namespaceRS: "enabled", virtRS: "enabled", withMCOCopy: true},
			wantMatch:        matchParams(rightsizing.NamespaceMetrics, rightsizing.VirtualizationMetrics),
			wantControllerID: config.PrometheusControllerID,
			wantManagedBy:    "multicluster-observability-addon-manager",
		},
		{
			name:             "MCO copy present, both right-sizing features, COO installed",
			setup:            rsSetup{cooInstalled: true, namespaceRS: "enabled", virtRS: "enabled", withMCOCopy: true},
			wantMatch:        matchParams(rightsizing.NamespaceMetrics, rightsizing.VirtualizationMetrics),
			wantControllerID: "",
			wantManagedBy:    "observability-operator",
		},
		{
			name:             "MCO copy present, namespace right-sizing only",
			setup:            rsSetup{namespaceRS: "enabled", virtRS: "disabled", withMCOCopy: true},
			wantMatch:        matchParams(rightsizing.NamespaceMetrics),
			wantControllerID: config.PrometheusControllerID,
			wantManagedBy:    "multicluster-observability-addon-manager",
		},
		{
			name:             "MCO copy present, virtualization right-sizing only",
			setup:            rsSetup{namespaceRS: "disabled", virtRS: "enabled", withMCOCopy: true},
			wantMatch:        matchParams(rightsizing.VirtualizationMetrics),
			wantControllerID: config.PrometheusControllerID,
			wantManagedBy:    "multicluster-observability-addon-manager",
		},
		{
			name:  "MCO copy present, right-sizing disabled",
			setup: rsSetup{namespaceRS: "disabled", virtRS: "disabled", withMCOCopy: true},
		},
		{
			name:             "MCO copy absent, both right-sizing features",
			setup:            rsSetup{namespaceRS: "enabled", virtRS: "enabled"},
			wantMatch:        matchParams(rightsizing.NamespaceMetrics, rightsizing.VirtualizationMetrics),
			wantControllerID: config.PrometheusControllerID,
			wantManagedBy:    "multicluster-observability-addon-manager",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup.platformSCName = "platform-metrics-default"
			agentAddon, cluster, mca := setupFullMCOA(t, tc.setup)

			// Raw helm output, as handed to the ManifestWork builder.
			objects, err := agentAddon.Manifests(t.Context(), cluster, mca)
			require.NoError(t, err)

			seen := map[objectIdentity]int{}
			var rs []*cooprometheusv1alpha1.ScrapeConfig
			var platformSC *cooprometheusv1alpha1.ScrapeConfig
			for _, obj := range objects {
				seen[identityOf(t, obj)]++
				sc, ok := obj.(*cooprometheusv1alpha1.ScrapeConfig)
				if !ok {
					continue
				}
				switch sc.Name {
				case rightsizing.ScrapeConfigName:
					rs = append(rs, sc)
				case tc.setup.platformSCName:
					platformSC = sc
				}
			}
			for id, n := range seen {
				assert.Equal(t, 1, n, "object rendered more than once: %+v", id)
			}

			if tc.wantMatch == nil {
				assert.Empty(t, rs)
			} else {
				// MCOA's own copy is the only one and the platform agent selects it.
				require.Len(t, rs, 1)
				assert.Equal(t, config.PlatformPrometheusMatchLabels[addoncfg.ComponentK8sLabelKey], rs[0].Labels[addoncfg.ComponentK8sLabelKey])
				assert.Equal(t, tc.wantManagedBy, rs[0].Labels[managedByLabel])
				assert.Equal(t, tc.wantControllerID, rs[0].Annotations[controllerIDAnnotation])
				assert.ElementsMatch(t, tc.wantMatch, rs[0].Spec.Params["match[]"])

				// It must be labeled and annotated like the platform ScrapeConfigs rendered by the
				// metrics chart, which follow the same COO rules.
				require.NotNil(t, platformSC)
				assert.Equal(t, platformSC.Labels[addoncfg.ComponentK8sLabelKey], rs[0].Labels[addoncfg.ComponentK8sLabelKey])
				assert.Equal(t, platformSC.Labels[managedByLabel], rs[0].Labels[managedByLabel])
				assert.Equal(t, platformSC.Annotations[controllerIDAnnotation], rs[0].Annotations[controllerIDAnnotation])
			}

			// Every render must produce the same ManifestWork content.
			want := lastObjectPerIdentity(t, objects)
			for range 10 {
				again, err := agentAddon.Manifests(t.Context(), cluster, mca)
				require.NoError(t, err)
				assert.Equal(t, want, lastObjectPerIdentity(t, again))
			}
		})
	}
}
