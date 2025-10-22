package addon

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	coomonitoringv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	addonhelm "github.com/stolostron/multicluster-observability-addon/internal/addon/helm"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/rest"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	"open-cluster-management.io/addon-framework/pkg/agent"
	"open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonv1alpha1client "open-cluster-management.io/api/client/addon/clientset/versioned"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func NewAddonManager(ctx context.Context, kubeConfig *rest.Config, scheme *runtime.Scheme, logger logr.Logger) (addonmanager.AddonManager, error) {
	logger = logger.WithName("addon")

	addonClient, err := addonv1alpha1client.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create addonv1alpha1 client: %w", err)
	}

	mgr, err := addonmanager.New(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create addon manager: %w", err)
	}

	registrationOption := addon.NewRegistrationOption(utilrand.String(5))

	httpClient, err := rest.HTTPClientFor(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client for kubeConfig: %w", err)
	}

	mapper, err := apiutil.NewDynamicRESTMapper(kubeConfig, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic REST mapper: %w", err)
	}

	opts := client.Options{
		Scheme:     scheme,
		Mapper:     mapper,
		HTTPClient: httpClient,
	}

	k8sClient, err := client.New(kubeConfig, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create new Kubernetes client: %w", err)
	}

	addonConfigValuesFn := addonfactory.GetAddOnDeploymentConfigValues(
		addonfactory.NewAddOnDeploymentConfigGetter(addonClient),
		addonfactory.ToAddOnCustomizedVariableValues,
	)

	agentLogger := logger.WithName("agent")
	mcoaAgentAddon, err := addonfactory.NewAgentAddonFactory(addoncfg.Name, addon.FS, "manifests/charts/mcoa").
		WithConfigGVRs(
			schema.GroupVersionResource{Version: loggingv1.GroupVersion.Version, Group: loggingv1.GroupVersion.Group, Resource: addoncfg.ClusterLogForwardersResource},
			schema.GroupVersionResource{Version: otelv1beta1.GroupVersion.Version, Group: otelv1beta1.GroupVersion.Group, Resource: addoncfg.OpenTelemetryCollectorsResource},
			schema.GroupVersionResource{Version: otelv1alpha1.GroupVersion.Version, Group: otelv1alpha1.GroupVersion.Group, Resource: addoncfg.InstrumentationResource},
			schema.GroupVersionResource{Version: coomonitoringv1alpha1.SchemeGroupVersion.Version, Group: coomonitoringv1alpha1.SchemeGroupVersion.Group, Resource: coomonitoringv1alpha1.PrometheusAgentName},
			schema.GroupVersionResource{Version: coomonitoringv1alpha1.SchemeGroupVersion.Version, Group: coomonitoringv1alpha1.SchemeGroupVersion.Group, Resource: coomonitoringv1alpha1.ScrapeConfigName},
			schema.GroupVersionResource{Version: monitoringv1.SchemeGroupVersion.Version, Group: monitoringv1.SchemeGroupVersion.Group, Resource: monitoringv1.PrometheusRuleName},
			utils.AddOnDeploymentConfigGVR,
		).
		WithGetValuesFuncs(addonConfigValuesFn, addonhelm.GetValuesFunc(ctx, k8sClient, agentLogger)).
		WithUpdaters(addon.Updaters()).
		WithAgentRegistrationOption(registrationOption).
		WithAgentInstallNamespace(
			// Set agent install namespace from addon deployment config if it exists
			utils.AgentInstallNamespaceFromDeploymentConfigFunc(
				utils.NewAddOnDeploymentConfigGetter(addonClient),
			),
		).WithScheme(scheme).
		BuildHelmAgentAddon()
	if err != nil {
		return nil, fmt.Errorf("failed to build helm agent addon: %w", err)
	}

	err = mgr.AddAgent(&AgentAddonWithDynamicHealthProber{
		agent:  mcoaAgentAddon,
		logger: agentLogger,
		client: k8sClient,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add mcoa agent to manager: %w", err)
	}

	return mgr, nil
}

type AgentAddonWithDynamicHealthProber struct {
	agent  agent.AgentAddon
	logger logr.Logger
	client client.Client
}

func (a *AgentAddonWithDynamicHealthProber) Manifests(cluster *clusterv1.ManagedCluster, addon *addonapiv1alpha1.ManagedClusterAddOn) ([]runtime.Object, error) {
	return a.agent.Manifests(cluster, addon)
}

func (a *AgentAddonWithDynamicHealthProber) GetAgentAddonOptions() agent.AgentAddonOptions {
	options := a.agent.GetAgentAddonOptions()
	options.HealthProber = addon.DynamicAgentHealthProber(a.client, a.logger)

	return options
}
