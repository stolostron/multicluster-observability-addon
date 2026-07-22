package handlers

import (
	"context"
	"net/url"
	"testing"

	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
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
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestBuildOptionsRawMetrics(t *testing.T) {
	const (
		hubNamespace = "test-hub"
		spokeName    = "test-spoke"
		clusterID    = "test-cluster-id"
	)

	scheme := runtime.NewScheme()
	require.NoError(t, kubescheme.AddToScheme(scheme))
	require.NoError(t, configv1.AddToScheme(scheme))
	require.NoError(t, cooprometheusv1alpha1.AddToScheme(scheme))
	require.NoError(t, cooprometheusv1.AddToScheme(scheme))
	require.NoError(t, prometheusv1.AddToScheme(scheme))
	require.NoError(t, clusterv1.Install(scheme))
	require.NoError(t, addonapiv1beta1.Install(scheme))
	require.NoError(t, workv1.Install(scheme))

	clusterVersion := &configv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name: "version",
		},
		Spec: configv1.ClusterVersionSpec{
			ClusterID: configv1.ClusterID(clusterID),
		},
	}

	platformAgent := &cooprometheusv1alpha1.PrometheusAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-prometheus-agent",
			Namespace: hubNamespace,
			Labels:    config.PlatformPrometheusMatchLabels,
		},
		Spec: cooprometheusv1alpha1.PrometheusAgentSpec{
			CommonPrometheusFields: cooprometheusv1.CommonPrometheusFields{
				RemoteWrite: []cooprometheusv1.RemoteWriteSpec{
					{
						Name: ptr.To(config.RemoteWriteCfgName),
					},
				},
			},
		},
	}

	cmao := &addonapiv1beta1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name: "multicluster-observability-addon",
			UID:  types.UID("test-cmao-uid"),
		},
	}
	require.NoError(t, controllerutil.SetOwnerReference(cmao, platformAgent, scheme))

	rawScrapeConfig := &cooprometheusv1alpha1.ScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "raw-scrape-config",
			Namespace: hubNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/component": "platform-metrics-collector",
			},
			Annotations: map[string]string{
				config.RawResolutionAnnotation:       config.RawResolutionValue,
				config.COOMonitoringStacksAnnotation: "stack-namespace/stack-name",
			},
		},
		Spec: cooprometheusv1alpha1.ScrapeConfigSpec{
			Params: map[string][]string{
				"match[]": {
					"up",
				},
			},
		},
	}

	// mTLS Secrets on Hub
	caSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.HubCASecretName,
			Namespace: config.HubInstallNamespace,
		},
		Data: map[string][]byte{"ca.crt": []byte("test-ca")},
	}
	certSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ClientCertSecretName,
			Namespace: config.HubInstallNamespace,
		},
		Data: map[string][]byte{"tls.crt": []byte("test-cert"), "tls.key": []byte("test-key")},
	}
	accessorSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.AlertmanagerAccessorSecretName,
			Namespace: config.HubInstallNamespace,
		},
		Data: map[string][]byte{
			"key":  []byte("data"),
			"pass": []byte("data"),
		},
	}

	addonCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ImagesConfigMapObjKey.Name,
			Namespace: config.ImagesConfigMapObjKey.Namespace,
		},
		Data: map[string]string{
			"obo_prometheus_rhel9_operator": "obo-prom-operator-image",
			"kube_rbac_proxy":               "kube-rbac-proxy-image",
			"prometheus_config_reloader":    "prometheus-config-reload-image",
			"kube_state_metrics":            "quay.io/kube/kube-state-metrics",
			"node_exporter":                 "quay.io/kube/node-exporter",
			"prometheus":                    "quay.io/prometheus/prometheus",
			"endpoint_monitoring_operator":  "endpoint-monitoring-operator-image",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(clusterVersion, platformAgent, cmao, rawScrapeConfig, caSecret, certSecret, accessorSecret, addonCM).
		Build()

	mcAddon := &addonapiv1beta1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-addon",
			Namespace: spokeName,
		},
		Status: addonapiv1beta1.ManagedClusterAddOnStatus{
			ConfigReferences: []addonapiv1beta1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
						Group:    "",
						Resource: "configmaps",
					},
					DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
						ConfigReferent: addonapiv1beta1.ConfigReferent{
							Name:      config.ImagesConfigMapObjKey.Name,
							Namespace: config.ImagesConfigMapObjKey.Namespace,
						},
					},
				},
				{
					ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
						Group:    "monitoring.rhobs",
						Resource: "prometheusagents",
					},
					DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
						ConfigReferent: addonapiv1beta1.ConfigReferent{
							Name:      "test-prometheus-agent",
							Namespace: hubNamespace,
						},
					},
				},
				{
					ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
						Group:    "monitoring.rhobs",
						Resource: "scrapeconfigs",
					},
					DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
						ConfigReferent: addonapiv1beta1.ConfigReferent{
							Name:      "raw-scrape-config",
							Namespace: hubNamespace,
						},
					},
				},
			},
		},
	}

	managedCluster := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: spokeName,
			Labels: map[string]string{
				"vendor": "OpenShift",
			},
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
		Registries: []addonapiv1beta1.ImageMirror{},
	}

	builder := &OptionsBuilder{
		Client: fakeClient,
		Logger: logr.Discard(),
	}

	retOpts, err := builder.Build(context.Background(), mcAddon, managedCluster, opts)
	require.NoError(t, err)

	// ScrapeConfig targeted to a COO stack should NOT be exported to managed cluster
	assert.Empty(t, retOpts.Platform.ScrapeConfigs)

	// Secrets should be distributed to the COO stack namespace
	assert.NotEmpty(t, retOpts.Secrets)
	caFound := false
	certFound := false
	for _, s := range retOpts.Secrets {
		if s.Namespace == "stack-namespace" {
			if s.Name == config.GetHubMtlsCASecretName(config.GetTrimmedClusterID(clusterID)) {
				caFound = true
			}
			if s.Name == config.GetHubMtlsCertSecretName(config.GetTrimmedClusterID(clusterID)) {
				certFound = true
			}
		}
	}
	assert.True(t, caFound, "COO ca secret should be copied")
	assert.True(t, certFound, "COO cert secret should be copied")

	// Transpiled patch should exist
	require.Len(t, retOpts.MonitoringStackPatches, 1)
	patch := retOpts.MonitoringStackPatches[0]
	assert.Equal(t, "stack-namespace", patch.Namespace)
	assert.Equal(t, "stack-name", patch.Name)
	require.Len(t, patch.RemoteWriteSpecs, 1)
	assert.NotNil(t, patch.RemoteWriteSpecs[0])
}

func TestBuildOptionsRawMetricsNonOCP(t *testing.T) {
	const (
		hubNamespace = "test-hub"
		spokeName    = "test-spoke"
		clusterID    = "test-cluster-id"
	)

	scheme := runtime.NewScheme()
	require.NoError(t, kubescheme.AddToScheme(scheme))
	require.NoError(t, configv1.AddToScheme(scheme))
	require.NoError(t, cooprometheusv1alpha1.AddToScheme(scheme))
	require.NoError(t, cooprometheusv1.AddToScheme(scheme))
	require.NoError(t, prometheusv1.AddToScheme(scheme))
	require.NoError(t, clusterv1.Install(scheme))
	require.NoError(t, addonapiv1beta1.Install(scheme))
	require.NoError(t, workv1.Install(scheme))

	clusterVersion := &configv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name: "version",
		},
		Spec: configv1.ClusterVersionSpec{
			ClusterID: configv1.ClusterID(clusterID),
		},
	}

	platformAgent := &cooprometheusv1alpha1.PrometheusAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-prometheus-agent",
			Namespace: hubNamespace,
			Labels:    config.PlatformPrometheusMatchLabels,
		},
		Spec: cooprometheusv1alpha1.PrometheusAgentSpec{
			CommonPrometheusFields: cooprometheusv1.CommonPrometheusFields{
				RemoteWrite: []cooprometheusv1.RemoteWriteSpec{
					{
						Name: ptr.To(config.RemoteWriteCfgName),
					},
				},
			},
		},
	}

	cmao := &addonapiv1beta1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name: "multicluster-observability-addon",
			UID:  types.UID("test-cmao-uid"),
		},
	}
	require.NoError(t, controllerutil.SetOwnerReference(cmao, platformAgent, scheme))

	rawScrapeConfig := &cooprometheusv1alpha1.ScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "raw-scrape-config",
			Namespace: hubNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/component": "platform-metrics-collector",
			},
			Annotations: map[string]string{
				config.RawResolutionAnnotation: config.RawResolutionValue,
			},
		},
		Spec: cooprometheusv1alpha1.ScrapeConfigSpec{
			Params: map[string][]string{
				"match[]": {
					"up",
				},
			},
		},
	}

	// mTLS Secrets on Hub
	caSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.HubCASecretName,
			Namespace: config.HubInstallNamespace,
		},
		Data: map[string][]byte{"ca.crt": []byte("test-ca")},
	}
	certSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ClientCertSecretName,
			Namespace: config.HubInstallNamespace,
		},
		Data: map[string][]byte{"tls.crt": []byte("test-cert"), "tls.key": []byte("test-key")},
	}
	accessorSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.AlertmanagerAccessorSecretName,
			Namespace: config.HubInstallNamespace,
		},
		Data: map[string][]byte{
			"key":  []byte("data"),
			"pass": []byte("data"),
		},
	}

	addonCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ImagesConfigMapObjKey.Name,
			Namespace: config.ImagesConfigMapObjKey.Namespace,
		},
		Data: map[string]string{
			"obo_prometheus_rhel9_operator": "obo-prom-operator-image",
			"kube_rbac_proxy":               "kube-rbac-proxy-image",
			"prometheus_config_reloader":    "prometheus-config-reload-image",
			"kube_state_metrics":            "quay.io/kube/kube-state-metrics",
			"node_exporter":                 "quay.io/kube/node-exporter",
			"prometheus":                    "quay.io/prometheus/prometheus",
			"endpoint_monitoring_operator":  "endpoint-monitoring-operator-image",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(clusterVersion, platformAgent, cmao, rawScrapeConfig, caSecret, certSecret, accessorSecret, addonCM).
		Build()

	mcAddon := &addonapiv1beta1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-addon",
			Namespace: spokeName,
		},
		Status: addonapiv1beta1.ManagedClusterAddOnStatus{
			ConfigReferences: []addonapiv1beta1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
						Group:    "",
						Resource: "configmaps",
					},
					DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
						ConfigReferent: addonapiv1beta1.ConfigReferent{
							Name:      config.ImagesConfigMapObjKey.Name,
							Namespace: config.ImagesConfigMapObjKey.Namespace,
						},
					},
				},
				{
					ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
						Group:    "monitoring.rhobs",
						Resource: "prometheusagents",
					},
					DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
						ConfigReferent: addonapiv1beta1.ConfigReferent{
							Name:      "test-prometheus-agent",
							Namespace: hubNamespace,
						},
					},
				},
				{
					ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
						Group:    "monitoring.rhobs",
						Resource: "scrapeconfigs",
					},
					DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
						ConfigReferent: addonapiv1beta1.ConfigReferent{
							Name:      "raw-scrape-config",
							Namespace: hubNamespace,
						},
					},
				},
			},
		},
	}

	// Managed Cluster with Non-OCP Vendor Label (eg: vendor=other)
	managedCluster := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: spokeName,
			Labels: map[string]string{
				"vendor": "other",
			},
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
		Registries: []addonapiv1beta1.ImageMirror{},
	}

	builder := &OptionsBuilder{
		Client: fakeClient,
		Logger: logr.Discard(),
	}

	retOpts, err := builder.Build(context.Background(), mcAddon, managedCluster, opts)
	require.NoError(t, err)

	// ScrapeConfig targeted for Raw metrics should NOT be exported directly
	assert.Empty(t, retOpts.Platform.ScrapeConfigs)

	// No MonitoringStack patches on non-OCP since there's no COO there
	assert.Empty(t, retOpts.MonitoringStackPatches)

	// Target RemoteWrite specifications should be transpiled directly on the Prometheus Server
	require.Len(t, retOpts.PrometheusServerRemoteWrite, 1)
	rwSpec := retOpts.PrometheusServerRemoteWrite[0]
	assert.NotNil(t, rwSpec)
	assert.Equal(t, config.GetHubMtlsCASecretName(config.GetTrimmedClusterID(clusterID)), rwSpec.TLSConfig.CA.Secret.Name)
	assert.Equal(t, config.GetHubMtlsCertSecretName(config.GetTrimmedClusterID(clusterID)), rwSpec.TLSConfig.Cert.Secret.Name)
}

func TestProcessScrapeConfigs(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, cooprometheusv1alpha1.AddToScheme(scheme))
	require.NoError(t, cooprometheusv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	caSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.HubCASecretName,
			Namespace: config.HubInstallNamespace,
		},
		Data: map[string][]byte{"ca.crt": []byte("test-ca")},
	}
	certSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ClientCertSecretName,
			Namespace: config.HubInstallNamespace,
		},
		Data: map[string][]byte{"tls.crt": []byte("test-cert"), "tls.key": []byte("test-key")},
	}

	builder := &OptionsBuilder{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(caSecret, certSecret).Build(),
		Logger: logr.Discard(),
	}

	agent := &cooprometheusv1alpha1.PrometheusAgent{
		Spec: cooprometheusv1alpha1.PrometheusAgentSpec{
			CommonPrometheusFields: cooprometheusv1.CommonPrometheusFields{
				RemoteWrite: []cooprometheusv1.RemoteWriteSpec{
					{
						Name: ptr.To(config.RemoteWriteCfgName),
					},
				},
			},
		},
	}

	scRaw := &cooprometheusv1alpha1.ScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "raw-sc",
			Annotations: map[string]string{
				config.RawResolutionAnnotation:       config.RawResolutionValue,
				config.COOMonitoringStacksAnnotation: "ns1/stack1",
			},
		},
		Spec: cooprometheusv1alpha1.ScrapeConfigSpec{
			Params: map[string][]string{
				"match[]": {"up"},
			},
		},
	}

	scStandard := &cooprometheusv1alpha1.ScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "standard-sc",
		},
		Spec: cooprometheusv1alpha1.ScrapeConfigSpec{
			Params: map[string][]string{
				"match[]": {"up"},
			},
		},
	}

	t.Run("OCP - Raw Metrics ScrapeConfig with COO stack should be transpiled and filtered out", func(t *testing.T) {
		secrets := &[]*corev1.Secret{}
		input := []*cooprometheusv1alpha1.ScrapeConfig{scRaw, scStandard}

		filtered, patches, _, err := builder.processScrapeConfigs(
			context.Background(),
			input,
			agent,
			"ca-secret",
			"cert-secret",
			secrets,
			true, // isOCP
			"default-ns",
		)
		require.NoError(t, err)

		// Standard scrapeConfig remains as-is, Raw scrapeConfig is filtered out
		assert.Len(t, filtered, 1)
		assert.Equal(t, "standard-sc", filtered[0].Name)

		// Raw scrapeConfig is transpiled to a MonitoringStack patch
		assert.Len(t, patches, 1)
		assert.Equal(t, "ns1", patches[0].Namespace)
		assert.Equal(t, "stack1", patches[0].Name)
		require.Len(t, patches[0].RemoteWriteSpecs, 1)
		assert.NotNil(t, patches[0].RemoteWriteSpecs[0])
	})

	t.Run("OCP - Raw Metrics ScrapeConfig without COO stack (CMO target) should deploy secrets in default-ns and fall through as ScrapeConfig", func(t *testing.T) {
		secrets := &[]*corev1.Secret{}
		// ScrapeConfig with Raw resolution but no COO target stack annotation
		scCmo := &cooprometheusv1alpha1.ScrapeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "raw-cmo-sc",
				Annotations: map[string]string{
					config.RawResolutionAnnotation: config.RawResolutionValue,
				},
			},
			Spec: cooprometheusv1alpha1.ScrapeConfigSpec{
				Params: map[string][]string{
					"match[]": {"up"},
				},
			},
		}

		input := []*cooprometheusv1alpha1.ScrapeConfig{scCmo}

		filtered, patches, _, err := builder.processScrapeConfigs(
			context.Background(),
			input,
			agent,
			"ca-secret",
			"cert-secret",
			secrets,
			true, // isOCP
			"default-ns",
		)
		require.NoError(t, err)

		// Since it's standard platform/user workload CMO target, it should fall through to filtered (exported to managed cluster)
		assert.Len(t, filtered, 1)
		assert.Equal(t, "raw-cmo-sc", filtered[0].Name)

		// No MonitoringStack patches should be generated
		assert.Empty(t, patches)

		// Secrets should be deployed directly to default-ns so default Prometheus instances can authenticate
		assert.NotEmpty(t, *secrets)
		caFound := false
		certFound := false
		for _, s := range *secrets {
			if s.Namespace == "default-ns" {
				if s.Name == "ca-secret" {
					caFound = true
				}
				if s.Name == "cert-secret" {
					certFound = true
				}
			}
		}
		assert.True(t, caFound)
		assert.True(t, certFound)
	})

	t.Run("Non-OCP - Raw Metrics ScrapeConfig should be transpiled for Prometheus Server directly and filtered out", func(t *testing.T) {
		secrets := &[]*corev1.Secret{}
		input := []*cooprometheusv1alpha1.ScrapeConfig{scRaw, scStandard}

		filtered, patches, serverRemoteWrites, err := builder.processScrapeConfigs(
			context.Background(),
			input,
			agent,
			"ca-secret",
			"cert-secret",
			secrets,
			false, // isOCP
			"default-ns",
		)
		require.NoError(t, err)

		// Standard remains as-is, Raw is filtered out
		assert.Len(t, filtered, 1)
		assert.Equal(t, "standard-sc", filtered[0].Name)

		// No MonitoringStack patches on non-OCP since there's no COO
		assert.Empty(t, patches)

		// Raw is transpiled with direct credentials target on serverRemoteWrites
		require.Len(t, serverRemoteWrites, 1)
		assert.Equal(t, "ca-secret", serverRemoteWrites[0].TLSConfig.CA.Secret.Name)
		assert.Equal(t, "cert-secret", serverRemoteWrites[0].TLSConfig.Cert.Secret.Name)
	})
}
