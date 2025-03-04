package addon

import (
	"context"

	"github.com/ViaQ/logerr/v2/log"
	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	addonhelm "github.com/rhobs/multicluster-observability-addon/internal/addon/helm"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	"open-cluster-management.io/addon-framework/pkg/utils"
	addonv1alpha1client "open-cluster-management.io/api/client/addon/clientset/versioned"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func NewAddonManager(ctx context.Context, kubeConfig *rest.Config, scheme *runtime.Scheme) (addonmanager.AddonManager, error) {
	logger := log.NewLogger("mcoa")

	addonClient, err := addonv1alpha1client.NewForConfig(kubeConfig)
	if err != nil {
		return nil, err
	}

	mgr, err := addonmanager.New(kubeConfig)
	if err != nil {
		klog.Errorf("failed to new addon manager %v", err)
		return nil, err
	}

	registrationOption := addon.NewRegistrationOption(utilrand.String(5))

	httpClient, err := rest.HTTPClientFor(kubeConfig)
	if err != nil {
		return nil, err
	}

	mapper, err := apiutil.NewDynamicRESTMapper(kubeConfig, httpClient)
	if err != nil {
		return nil, err
	}

	opts := client.Options{
		Scheme:     scheme,
		Mapper:     mapper,
		HTTPClient: httpClient,
	}

	k8sClient, err := client.New(kubeConfig, opts)
	if err != nil {
		return nil, err
	}

	addonConfigValuesFn := addonfactory.GetAddOnDeploymentConfigValues(
		addonfactory.NewAddOnDeploymentConfigGetter(addonClient),
		addonfactory.ToAddOnCustomizedVariableValues,
	)

	mcoaAgentAddon, err := addonfactory.NewAgentAddonFactory(addon.Name, addon.FS, "manifests/charts/mcoa").
		WithConfigGVRs(
			schema.GroupVersionResource{Version: loggingv1.GroupVersion.Version, Group: loggingv1.GroupVersion.Group, Resource: addon.ClusterLogForwardersResource},
			schema.GroupVersionResource{Version: otelv1beta1.GroupVersion.Version, Group: otelv1beta1.GroupVersion.Group, Resource: addon.OpenTelemetryCollectorsResource},
			schema.GroupVersionResource{Version: otelv1alpha1.GroupVersion.Version, Group: otelv1alpha1.GroupVersion.Group, Resource: addon.InstrumentationResource},
			schema.GroupVersionResource{Version: monitoringv1alpha1.SchemeGroupVersion.Version, Group: monitoringv1alpha1.SchemeGroupVersion.Group, Resource: monitoringv1alpha1.PrometheusAgentName},
			schema.GroupVersionResource{Version: monitoringv1alpha1.SchemeGroupVersion.Version, Group: monitoringv1alpha1.SchemeGroupVersion.Group, Resource: monitoringv1alpha1.ScrapeConfigName},
			schema.GroupVersionResource{Version: monitoringv1.SchemeGroupVersion.Version, Group: monitoringv1.SchemeGroupVersion.Group, Resource: monitoringv1.PrometheusRuleName},
			utils.AddOnDeploymentConfigGVR,
		).
		WithGetValuesFuncs(addonConfigValuesFn, addonhelm.GetValuesFunc(ctx, k8sClient, logger)).
		WithAgentHealthProber(addon.AgentHealthProber()).
		WithAgentRegistrationOption(registrationOption).
		WithScheme(scheme).
		BuildHelmAgentAddon()
	if err != nil {
		logger.Error(err, "failed to build agent")
		return nil, err
	}

	err = mgr.AddAgent(mcoaAgentAddon)
	if err != nil {
		logger.Error(err, "unable to add mcoa agent addon")
		return nil, err
	}

	return mgr, nil
}
