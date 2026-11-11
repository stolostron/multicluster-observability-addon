package handlers

import (
	"net/url"
	"testing"

	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubescheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// TestBuildOptions_ClusterImageRegistriesAgentImages checks that the
// registry mirrors from the ManagedCluster image-registries annotation also
// reach the PrometheusAgents. The agents are copied from the hub-wide default
// agents, whose images the resource creator sets once for all clusters.
func TestBuildOptions_ClusterImageRegistriesAgentImages(t *testing.T) {
	const (
		hubNamespace = "test-hub"
		spokeName    = "test-spoke"
		source       = "registry.redhat.io/rhacm2"
		mirror       = "registry.stage.redhat.io/rhacm2"
		prometheus   = "/prometheus-rhel9@sha256:aaaa"
		rbacProxy    = "/kube-rbac-proxy-rhel9@sha256:bbbb"
	)
	mirrorAnnotation := `{"registries":[{"mirror":"` + mirror + `","source":"` + source + `"}]}`

	scheme := runtime.NewScheme()
	require.NoError(t, kubescheme.AddToScheme(scheme))
	require.NoError(t, configv1.AddToScheme(scheme))
	require.NoError(t, cooprometheusv1alpha1.AddToScheme(scheme))
	require.NoError(t, cooprometheusv1.AddToScheme(scheme))
	require.NoError(t, prometheusv1.AddToScheme(scheme))
	require.NoError(t, clusterv1.Install(scheme))
	require.NoError(t, addonapiv1beta1.Install(scheme))
	require.NoError(t, workv1.Install(scheme))

	cmao := &addonapiv1beta1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name: addoncfg.Name,
			UID:  types.UID("test-cmao-uid"),
		},
	}

	// Hub agents as the resource creator builds them: the image and the
	// kube-rbac-proxy sidecar come from the hub images list.
	newHubAgent := func(name string, labels map[string]string) *cooprometheusv1alpha1.PrometheusAgent {
		agent := &cooprometheusv1alpha1.PrometheusAgent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: hubNamespace,
				Labels:    labels,
			},
			Spec: cooprometheusv1alpha1.PrometheusAgentSpec{
				CommonPrometheusFields: cooprometheusv1.CommonPrometheusFields{
					Image: ptr.To(source + prometheus),
					Containers: []corev1.Container{
						{Name: "kube-rbac-proxy", Image: source + rbacProxy},
					},
					RemoteWrite: []cooprometheusv1.RemoteWriteSpec{
						{Name: ptr.To(config.RemoteWriteCfgName)},
					},
				},
			},
		}
		require.NoError(t, controllerutil.SetOwnerReference(cmao, agent, scheme))
		return agent
	}
	platformAgent := newHubAgent("platform-agent", config.PlatformPrometheusMatchLabels)
	uwlAgent := newHubAgent("uwl-agent", config.UserWorkloadPrometheusMatchLabels)

	agentRef := func(agent *cooprometheusv1alpha1.PrometheusAgent) addonapiv1beta1.ConfigReference {
		return addonapiv1beta1.ConfigReference{
			ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
				Group:    "monitoring.rhobs",
				Resource: cooprometheusv1alpha1.PrometheusAgentName,
			},
			DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
				ConfigReferent: addonapiv1beta1.ConfigReferent{
					Name:      agent.Name,
					Namespace: agent.Namespace,
				},
			},
		}
	}

	resources := func() []client.Object {
		return []client.Object{
			&configv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "version"},
				Spec:       configv1.ClusterVersionSpec{ClusterID: "hub-cluster-id"},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      config.ImagesConfigMapObjKey.Name,
					Namespace: config.ImagesConfigMapObjKey.Namespace,
				},
				Data: map[string]string{
					"obo_prometheus_rhel9_operator": source + "/obo-prometheus-rhel9-operator@sha256:cccc",
					"kube_rbac_proxy":               source + rbacProxy,
					"prometheus_config_reloader":    source + "/prometheus-config-reloader-rhel9@sha256:dddd",
					"kube_state_metrics":            source + "/kube-state-metrics-rhel9@sha256:eeee",
					"node_exporter":                 source + "/node-exporter-rhel9@sha256:ffff",
					"prometheus":                    source + prometheus,
					"endpoint_monitoring_operator":  source + "/endpoint-monitoring-rhel9-operator@sha256:1111",
				},
			},
			platformAgent.DeepCopy(),
			uwlAgent.DeepCopy(),
			cmao.DeepCopy(),
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: config.HubCASecretName, Namespace: config.HubInstallNamespace},
				Data:       map[string][]byte{"ca.crt": []byte("test-ca")},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: config.ClientCertSecretName, Namespace: config.HubInstallNamespace},
				Data:       map[string][]byte{"tls.crt": []byte("test-cert"), "tls.key": []byte("test-key")},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: config.AlertmanagerAccessorSecretName, Namespace: config.HubInstallNamespace},
				Data:       map[string][]byte{"key": []byte("data"), "pass": []byte("data")},
			},
		}
	}

	testCases := map[string]struct {
		vendor       string
		annotation   string
		withUWL      bool
		wantRegistry string
	}{
		"non-OpenShift cluster with registry mirrors": {
			vendor:       "GKE",
			annotation:   mirrorAnnotation,
			wantRegistry: mirror,
		},
		"OpenShift cluster with registry mirrors": {
			vendor:       "OpenShift",
			annotation:   mirrorAnnotation,
			withUWL:      true,
			wantRegistry: mirror,
		},
		"OpenShift cluster without the annotation": {
			vendor:       "OpenShift",
			withUWL:      true,
			wantRegistry: source,
		},
		"mirror for another registry": {
			vendor:       "GKE",
			annotation:   `{"registries":[{"mirror":"mirror.example.com/other","source":"quay.io/other"}]}`,
			wantRegistry: source,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			managedCluster := &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:   spokeName,
					Labels: map[string]string{"vendor": tc.vendor},
				},
			}
			if tc.annotation != "" {
				managedCluster.Annotations = map[string]string{clusterv1.ClusterImageRegistriesAnnotationKey: tc.annotation}
			}

			mcAddon := &addonapiv1beta1.ManagedClusterAddOn{
				ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name, Namespace: spokeName},
				Status: addonapiv1beta1.ManagedClusterAddOnStatus{
					ConfigReferences: []addonapiv1beta1.ConfigReference{agentRef(platformAgent), agentRef(uwlAgent)},
				},
			}

			opts := addon.Options{
				InstallNamespace: "test-install-ns",
				Platform: addon.PlatformOptions{
					Metrics: addon.MetricsOptions{
						CollectionEnabled: true,
						HubEndpoint:       url.URL{Host: "hub-endpoint"},
					},
				},
				UserWorkloads: addon.UserWorkloadOptions{
					Metrics: addon.MetricsOptions{CollectionEnabled: tc.withUWL},
				},
			}

			builder := &OptionsBuilder{
				Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(resources()...).Build(),
				Logger: logr.Discard(),
			}
			retOpts, err := builder.Build(t.Context(), mcAddon, managedCluster, opts)
			require.NoError(t, err)

			// The images list already follows the mirrors.
			assert.Equal(t, tc.wantRegistry+prometheus, retOpts.Images.Prometheus)
			assert.Equal(t, tc.wantRegistry+rbacProxy, retOpts.Images.KubeRBACProxy)

			agents := []*cooprometheusv1alpha1.PrometheusAgent{retOpts.Platform.PrometheusAgent}
			if tc.withUWL {
				agents = append(agents, retOpts.UserWorkloads.PrometheusAgent)
			} else {
				assert.Nil(t, retOpts.UserWorkloads.PrometheusAgent)
			}
			for _, agent := range agents {
				require.NotNil(t, agent)
				require.NotNil(t, agent.Spec.Image)
				assert.Equal(t, tc.wantRegistry+prometheus, *agent.Spec.Image, "agent %s image", agent.Name)
				require.Len(t, agent.Spec.Containers, 1)
				assert.Equal(t, tc.wantRegistry+rbacProxy, agent.Spec.Containers[0].Image, "agent %s sidecar image", agent.Name)
			}
		})
	}
}
