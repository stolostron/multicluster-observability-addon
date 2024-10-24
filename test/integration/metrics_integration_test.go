package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	prometheusapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	addonctrl "github.com/rhobs/multicluster-observability-addon/internal/controllers/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/metrics/config"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	workv1 "open-cluster-management.io/api/work/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestIntegration_Metrics(t *testing.T) {
	scheme := newScheme()

	// Set up the test environment
	spokeName := "managed-cluster-1"
	obsNamespace := "open-cluster-management-observability"

	promAgentConfig := types.NamespacedName{Namespace: obsNamespace, Name: "default-agent"}
	envoyConfig := types.NamespacedName{Namespace: obsNamespace, Name: "default-envoy-config"}
	remoteWriteSecrets := newRemoteWriteSecrets(obsNamespace)
	resources := []client.Object{
		newManagedCluster(spokeName),
		newNamespace(spokeName),
		newManagedClusterAddon(spokeName, promAgentConfig, envoyConfig),
		newNamespace(obsNamespace),
		newClusterManagementAddon(obsNamespace, "foo"),
		newAddonDeploymentConfig(obsNamespace, "foo").WithPlatformMetricsCollection().Build(),
		mewImagesListConfigMap(obsNamespace),
		newPrometheusAgent(promAgentConfig.Namespace, promAgentConfig.Name),
		newEnvoyConfigMap(envoyConfig.Namespace, envoyConfig.Name),
		remoteWriteSecrets[0],
		remoteWriteSecrets[1],
	}

	k8sClient, err := client.New(restCfgHub, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}

	for _, resource := range resources {
		if err := k8sClient.Create(context.Background(), resource); err != nil {
			t.Fatal(err, resource.GetName(), resource.GetNamespace())
		}
	}

	// Restrict the test to the agent controller
	os.Setenv("DISABLE_WATCHER_CONTROLLER", "true")

	// Start the controller
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgr, err := addonctrl.NewAddonManager(ctx, restCfgHub, scheme)
	if err != nil {
		t.Fatal(err)
	}

	if err := mgr.Start(ctx); err != nil {
		t.Fatal(err)
	}

	if err := waitForController(ctx, k8sClient); err != nil {
		t.Fatalf("failed to wait for controller to start: %v", err)
	}

	// Validate that the manifest work for the managed cluster is created and contains the prometheus agent
	manifestWork := &workv1.ManifestWork{}
	err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 15*time.Second, false, func(ctx context.Context) (bool, error) {
		manifestWorkList := &workv1.ManifestWorkList{}
		err := k8sClient.List(ctx, manifestWorkList, client.InNamespace(spokeName))
		if err != nil {
			return false, err
		}

		if len(manifestWorkList.Items) == 0 {
			return false, nil
		}

		if len(manifestWorkList.Items) > 1 {
			return false, fmt.Errorf("expected 1 manifestwork, got %d", len(manifestWorkList.Items))
		}

		manifestWork = &manifestWorkList.Items[0]

		return true, nil
	})
	assert.NoError(t, err)

	dec := serializer.NewCodecFactory(scheme).UniversalDeserializer()
	var found bool
	for _, resource := range manifestWork.Spec.Workload.Manifests {
		obj, _, err := dec.Decode(resource.Raw, nil, nil)
		assert.NoError(t, err)

		if obj.GetObjectKind().GroupVersionKind().Group == prometheusapi.GroupName && obj.GetObjectKind().GroupVersionKind().Kind == prometheusalpha1.PrometheusAgentsKind {
			found = true
			break
		}
	}
	assert.Truef(t, found, "expected prometheus agent in manifest work")
}

func newManagedClusterAddon(ns string, promAgent, envoyConfig types.NamespacedName) *addonapiv1alpha1.ManagedClusterAddOn {
	return &addonapiv1alpha1.ManagedClusterAddOn{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      addon.Name,
			Namespace: ns,
		},
		Spec: addonapiv1alpha1.ManagedClusterAddOnSpec{
			InstallNamespace: "foo",
			Configs: []addonapiv1alpha1.AddOnConfig{
				{
					ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
						Group:    prometheusapi.GroupName,
						Resource: prometheusalpha1.PrometheusAgentName,
					},
					ConfigReferent: addonapiv1alpha1.ConfigReferent{
						Namespace: promAgent.Namespace,
						Name:      promAgent.Name,
					},
				},
				{
					ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
						Group:    "",
						Resource: "configmaps",
					},
					ConfigReferent: addonapiv1alpha1.ConfigReferent{
						Namespace: envoyConfig.Namespace,
						Name:      envoyConfig.Name,
					},
				},
			},
		},
	}
}

// func newAddonDeploymentConfig(ns, name string) *addonapiv1alpha1.AddOnDeploymentConfig {
// 	return &addonapiv1alpha1.AddOnDeploymentConfig{
// 		ObjectMeta: ctrl.ObjectMeta{
// 			Namespace: ns,
// 			Name:      name,
// 		},
// 		Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
// 			AgentInstallNamespace: "foo",
// 			CustomizedVariables: []addonapiv1alpha1.CustomizedVariable{
// 				{
// 					Name:  addon.KeyPlatformMetricsCollection,
// 					Value: string(addon.PrometheusAgentMetricsCollectorV1alpha1),
// 				},
// 			},
// 		},
// 	}
// }

func mewImagesListConfigMap(ns string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "images-list",
			Namespace: ns,
		},
		Data: map[string]string{
			"prometheus_operator":        "operator-image",
			"prometheus_config_reloader": "reloader-image",
			"kube_rbac_proxy":            "proxy-image",
		},
	}
}

func newPrometheusAgent(ns, name string) *prometheusalpha1.PrometheusAgent {
	return &prometheusalpha1.PrometheusAgent{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
			Labels:    config.PlatformPrometheusMatchLabels,
		},
	}
}

func newEnvoyConfigMap(ns, name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
			Labels:    config.PlatformPrometheusMatchLabels,
		},
	}
}

func newRemoteWriteSecrets(ns string) []*corev1.Secret {
	return []*corev1.Secret{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      config.ClientCertSecretName,
				Namespace: ns,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      config.HubCASecretName,
				Namespace: ns,
			},
		},
	}
}
