package integration

import (
	"context"
	"time"

	telv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func newScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(loggingv1.AddToScheme(scheme))
	utilruntime.Must(telv1alpha1.AddToScheme(scheme))
	utilruntime.Must(otelv1beta1.AddToScheme(scheme))
	utilruntime.Must(clusterv1.Install(scheme))
	utilruntime.Must(workv1.Install(scheme))
	utilruntime.Must(addonapiv1alpha1.Install(scheme))
	utilruntime.Must(prometheusalpha1.AddToScheme(scheme))
	utilruntime.Must(prometheusv1.AddToScheme(scheme))

	return scheme
}

func waitForController(ctx context.Context, k8sClient client.Client) error {
	err := wait.PollUntilContextTimeout(ctx, 1*time.Second, 20*time.Second, false, func(ctx context.Context) (bool, error) {
		// list managedClusterAddons and check if one has the status.conditions populated, if so the controller is running
		managedClustersList := &addonapiv1alpha1.ManagedClusterAddOnList{}
		err := k8sClient.List(ctx, managedClustersList)
		if err != nil {
			return false, err
		}

		for _, managedClusterAddon := range managedClustersList.Items {
			if len(managedClusterAddon.Status.Conditions) > 0 {
				return true, nil
			}
		}

		return false, nil
	})

	return err
}

func applyResources(ctx context.Context, k8sClient client.Client, resources []client.Object) error {
	for _, resource := range resources {
		if err := k8sClient.Create(ctx, resource); err != nil {
			return err
		}
	}

	return nil
}

func newNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: ctrl.ObjectMeta{
			Name: name,
		},
	}
}

func newManagedCluster(name string) *clusterv1.ManagedCluster {
	return &clusterv1.ManagedCluster{
		ObjectMeta: ctrl.ObjectMeta{
			Name: name,
		},
	}
}

func newClusterManagementAddon() *addonapiv1alpha1.ClusterManagementAddOn {
	return &addonapiv1alpha1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name: addon.Name,
		},
		Spec: addonapiv1alpha1.ClusterManagementAddOnSpec{
			InstallStrategy: addonapiv1alpha1.InstallStrategy{
				Type: addonapiv1alpha1.AddonInstallStrategyPlacements,
			},
		},
	}
}

type addonDeploymentConfigBuilder addonapiv1alpha1.AddOnDeploymentConfig

func newAddonDeploymentConfig(ns, name string) *addonDeploymentConfigBuilder {
	return &addonDeploymentConfigBuilder{
		ObjectMeta: ctrl.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}
}

func (a *addonDeploymentConfigBuilder) WithPlatformMetricsCollection() *addonDeploymentConfigBuilder {
	a.Spec.CustomizedVariables = append(a.Spec.CustomizedVariables, addonapiv1alpha1.CustomizedVariable{
		Name:  addon.KeyPlatformMetricsCollection,
		Value: string(addon.PrometheusAgentMetricsCollectorV1alpha1),
	})

	return a
}

func (a *addonDeploymentConfigBuilder) WithUserWorkloadsMetricsCollection() *addonDeploymentConfigBuilder {
	a.Spec.CustomizedVariables = append(a.Spec.CustomizedVariables, addonapiv1alpha1.CustomizedVariable{
		Name:  addon.KeyUserWorkloadMetricsCollection,
		Value: string(addon.PrometheusAgentMetricsCollectorV1alpha1),
	})

	return a
}

func (a *addonDeploymentConfigBuilder) WithPlatformHubEndpoint(endpoint string) *addonDeploymentConfigBuilder {
	a.Spec.CustomizedVariables = append(a.Spec.CustomizedVariables, addonapiv1alpha1.CustomizedVariable{
		Name:  addon.KeyPlatformSignalsHubEndpoint,
		Value: endpoint,
	})

	return a
}

func (a *addonDeploymentConfigBuilder) Build() *addonapiv1alpha1.AddOnDeploymentConfig {
	return (*addonapiv1alpha1.AddOnDeploymentConfig)(a)
}

func setOwner(client client.Client, resource, owner client.Object) error {
	if err := client.Get(context.Background(), types.NamespacedName{Name: owner.GetName(), Namespace: owner.GetNamespace()}, owner); err != nil {
		return err
	}

	return controllerutil.SetControllerReference(owner, resource, client.Scheme())
}