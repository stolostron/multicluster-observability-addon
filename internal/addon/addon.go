package addon

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"open-cluster-management.io/addon-framework/pkg/agent"
	"open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	v1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	minPrometheusOperatorVersion = "0.79.0"
	crdResourceName              = "customresourcedefinitions"
)

var (
	errMissingFeedbackValues      = errors.New("missing feedback values")
	errMissingFields              = errors.New("no fields found in health checker")
	errProbeConditionNotSatisfied = errors.New("probe condition is not satisfied")
	errProbeValueIsNil            = errors.New("probe value is nil")
	errUnknownProbeKey            = errors.New("probe has key that doesn't match the key defined")
	errUnknownResource            = errors.New("undefined health check for resource")
	errInvalidVersionString       = errors.New("invalid version string")

	prometheusAgentCRDName = fmt.Sprintf("%s.%s", cooprometheusv1alpha1.PrometheusAgentName, cooprometheusv1alpha1.SchemeGroupVersion.Group)
	scrapeConfigCRDName    = fmt.Sprintf("%s.%s", cooprometheusv1alpha1.ScrapeConfigName, cooprometheusv1alpha1.SchemeGroupVersion.Group)
	serviceMonitorCRDName  = fmt.Sprintf("%s.%s", cooprometheusv1.ServiceMonitorName, cooprometheusv1.SchemeGroupVersion.Group)
	podMonitorCRDName      = fmt.Sprintf("%s.%s", cooprometheusv1.PodMonitorName, cooprometheusv1.SchemeGroupVersion.Group)
	probeCRDName           = fmt.Sprintf("%s.%s", cooprometheusv1.ProbeName, cooprometheusv1.SchemeGroupVersion.Group)

	isEstablishedFeedbackPath      = ".status.conditions[?(@.type==\"Established\")].status"
	lastTransitionTimeFeedbackPath = ".status.conditions[?(@.type==\"Established\")].lastTransitionTime"
)

func NewRegistrationOption(agentName string) *agent.RegistrationOption {
	return &agent.RegistrationOption{
		CSRConfigurations: agent.KubeClientSignerConfigurations(addoncfg.Name, agentName),
		CSRApproveCheck:   utils.DefaultCSRApprover(agentName),
	}
}

// DynamicAgentHealthProber returns a HealthProber struct that contains the necessary
// information to assert if an addon deployment is ready or not, taking into account
// the enabled features.
func DynamicAgentHealthProber(k8s client.Client, logger logr.Logger) *agent.HealthProber {
	// Get addonDeploymentConfig and generate Options
	aodc := &addonapiv1alpha1.AddOnDeploymentConfig{}
	key := client.ObjectKey{
		Namespace: addoncfg.InstallNamespace,
		Name:      addoncfg.Name,
	}
	if err := k8s.Get(context.Background(), key, aodc, &client.GetOptions{}); err != nil {
		logger.Error(err, "failed to get the AddOnDeploymentConfig")
	}
	opts, err := BuildOptions(aodc)
	if err != nil {
		logger.Error(err, "failed to build options")
	}

	// Depending on enabled options, set appropriate probefields
	probeFields := []agent.ProbeField{}
	if opts.Platform.Metrics.CollectionEnabled {
		probeFields = append(probeFields, getMetricsProbeFields(aodc.Spec.AgentInstallNamespace)...)
	}
	if opts.Platform.Logs.CollectionEnabled {
		probeFields = append(probeFields, getLogsProbeFields()...)
	}
	if opts.UserWorkloads.Traces.CollectionEnabled {
		probeFields = append(probeFields, getTracesProbeFields()...)
	}

	if opts.Platform.AnalyticsOptions.IncidentDetection.Enabled {
		probeFields = append(probeFields, getAnalyticsProbeFields()...)
	}
	return &agent.HealthProber{
		Type: agent.HealthProberTypeWork,
		WorkProber: &agent.WorkHealthProber{
			ProbeFields: probeFields,
			HealthChecker: func(fields []agent.FieldResult, mc *v1.ManagedCluster, mcao *addonapiv1alpha1.ManagedClusterAddOn) error {
				if err := healthChecker(fields); err != nil {
					logger.V(1).Info("Health check failed for managed cluster", "clusterName", mc.Name, "error", err.Error())
					return err
				}
				return nil
			},
		},
	}
}

func getMetricsProbeFields(ns string) []agent.ProbeField {
	return []agent.ProbeField{
		{
			ResourceIdentifier: workv1.ResourceIdentifier{
				Group:     cooprometheusv1alpha1.SchemeGroupVersion.Group,
				Resource:  cooprometheusv1alpha1.PrometheusAgentName,
				Name:      mconfig.PlatformMetricsCollectorApp,
				Namespace: ns,
			},
			ProbeRules: []workv1.FeedbackRule{
				{
					Type: workv1.JSONPathsType,
					JsonPaths: []workv1.JsonPath{
						{
							Name: addoncfg.PaProbeKey,
							Path: addoncfg.PaProbePath,
						},
					},
				},
			},
		},
		{
			ResourceIdentifier: workv1.ResourceIdentifier{
				Group:    apiextensionsv1.GroupName,
				Resource: crdResourceName,
				Name:     scrapeConfigCRDName,
			},
			ProbeRules: []workv1.FeedbackRule{
				{
					Type: workv1.JSONPathsType,
					JsonPaths: []workv1.JsonPath{
						{
							Name: addoncfg.PrometheusOperatorVersionFeedbackName,
							Path: `.metadata.annotations.operator\.prometheus\.io/version`,
						},
						{
							Name: addoncfg.IsEstablishedFeedbackName,
							Path: isEstablishedFeedbackPath,
						},
						{
							Name: addoncfg.LastTransitionTimeFeedbackName,
							Path: lastTransitionTimeFeedbackPath,
						},
						{
							Name: addoncfg.IsOLMManagedFeedbackName,
							Path: `.metadata.labels.olm\.managed`,
						},
					},
				},
			},
		},
		{
			ResourceIdentifier: workv1.ResourceIdentifier{
				Group:    apiextensionsv1.GroupName,
				Resource: crdResourceName,
				Name:     prometheusAgentCRDName,
			},
			ProbeRules: []workv1.FeedbackRule{
				{
					Type: workv1.JSONPathsType,
					JsonPaths: []workv1.JsonPath{
						{
							Name: addoncfg.IsEstablishedFeedbackName, // needed for generating the sync annotation on the prometheus operator
							Path: isEstablishedFeedbackPath,
						},
						{
							Name: addoncfg.LastTransitionTimeFeedbackName,
							Path: lastTransitionTimeFeedbackPath,
						},
					},
				},
			},
		},
	}
}

func getLogsProbeFields() []agent.ProbeField {
	return []agent.ProbeField{
		{
			ResourceIdentifier: workv1.ResourceIdentifier{
				Group:     loggingv1.GroupVersion.Group,
				Resource:  addoncfg.ClusterLogForwardersResource,
				Name:      addoncfg.SpokeCLFName,
				Namespace: addoncfg.SpokeCLFNamespace,
			},
			ProbeRules: []workv1.FeedbackRule{
				{
					Type: workv1.JSONPathsType,
					JsonPaths: []workv1.JsonPath{
						{
							Name: addoncfg.ClfProbeKey,
							Path: addoncfg.ClfProbePath,
						},
					},
				},
			},
		},
	}
}

func getTracesProbeFields() []agent.ProbeField {
	return []agent.ProbeField{
		{
			ResourceIdentifier: workv1.ResourceIdentifier{
				Group:     otelv1alpha1.GroupVersion.Group,
				Resource:  addoncfg.OpenTelemetryCollectorsResource,
				Name:      addoncfg.SpokeOTELColName,
				Namespace: addoncfg.SpokeOTELColNamespace,
			},
			ProbeRules: []workv1.FeedbackRule{
				{
					Type: workv1.JSONPathsType,
					JsonPaths: []workv1.JsonPath{
						{
							Name: addoncfg.OtelColProbeKey,
							Path: addoncfg.OtelColProbePath,
						},
					},
				},
			},
		},
	}
}

func getAnalyticsProbeFields() []agent.ProbeField {
	return []agent.ProbeField{
		{
			ResourceIdentifier: workv1.ResourceIdentifier{
				Group:    uiplugin.GroupVersion.Group,
				Resource: addoncfg.UiPluginsResource,
				Name:     "monitoring",
			},
			ProbeRules: []workv1.FeedbackRule{
				{
					Type: workv1.JSONPathsType,
					JsonPaths: []workv1.JsonPath{
						{
							Name: addoncfg.UipProbeKey,
							Path: addoncfg.UipProbePath,
						},
					},
				},
			},
		},
	}
}

func Updaters() []agent.Updater {
	crdNames := []string{
		scrapeConfigCRDName,
		prometheusAgentCRDName,
		serviceMonitorCRDName,
		podMonitorCRDName,
		probeCRDName,
	}

	updaters := make([]agent.Updater, len(crdNames))
	for i, crdName := range crdNames {
		updaters[i] = agent.Updater{
			UpdateStrategy: workv1.UpdateStrategy{
				Type: workv1.UpdateStrategyTypeServerSideApply,
				ServerSideApply: &workv1.ServerSideApplyConfig{
					Force: false,
				},
			},
			ResourceIdentifier: workv1.ResourceIdentifier{
				Group:    apiextensionsv1.GroupName,
				Resource: crdResourceName,
				Name:     crdName,
			},
		}
	}
	return updaters
}

func healthChecker(fields []agent.FieldResult) error {
	if len(fields) == 0 {
		return errMissingFields
	}
	for _, field := range fields {
		identifier := field.ResourceIdentifier
		switch identifier.Resource {
		case cooprometheusv1alpha1.PrometheusAgentName:
			if len(field.FeedbackResult.Values) == 0 {
				// If the PrometheusAgent didn't get yet feedback values, it means it wasn't reconciled by the operator
				// It's in bad health.
				return fmt.Errorf("%w for %s with key %s/%s", errMissingFeedbackValues, identifier.Resource, identifier.Namespace, identifier.Name)
			}
			for _, value := range field.FeedbackResult.Values {
				if value.Name != addoncfg.PaProbeKey {
					return fmt.Errorf("%w: %s with key %s/%s unknown probe keys %s", errUnknownProbeKey, identifier.Resource, identifier.Namespace, identifier.Name, value.Name)
				}

				if value.Value.String == nil {
					return fmt.Errorf("%w: %s with key %s/%s", errProbeValueIsNil, identifier.Resource, identifier.Namespace, identifier.Name)
				}

				if *value.Value.String != "True" {
					return fmt.Errorf("%w: %s status condition type is %s for %s/%s", errProbeConditionNotSatisfied, identifier.Resource, *value.Value.String, identifier.Namespace, identifier.Name)
				}
				// pa passes the health check
			}
		case addoncfg.ClusterLogForwardersResource:
			if len(field.FeedbackResult.Values) == 0 {
				return fmt.Errorf("%w for %s with key %s/%s", errMissingFeedbackValues, identifier.Resource, identifier.Namespace, identifier.Name)
			}
			for _, value := range field.FeedbackResult.Values {
				if value.Name != addoncfg.ClfProbeKey {
					return fmt.Errorf("%w: %s with key %s/%s unknown probe keys %s", errUnknownProbeKey, identifier.Resource, identifier.Namespace, identifier.Name, value.Name)
				}

				if value.Value.String == nil {
					return fmt.Errorf("%w: %s with key %s/%s", errProbeValueIsNil, identifier.Resource, identifier.Namespace, identifier.Name)
				}

				if *value.Value.String != "True" {
					return fmt.Errorf("%w: %s status condition type is %s for %s/%s", errProbeConditionNotSatisfied, identifier.Resource, *value.Value.String, identifier.Namespace, identifier.Name)
				}
				// clf passes the health check
			}
		case addoncfg.OpenTelemetryCollectorsResource:
			if len(field.FeedbackResult.Values) == 0 {
				return fmt.Errorf("%w for %s with key %s/%s", errMissingFeedbackValues, identifier.Resource, identifier.Namespace, identifier.Name)
			}
			for _, value := range field.FeedbackResult.Values {
				if value.Name != addoncfg.OtelColProbeKey {
					return fmt.Errorf("%w: %s with key %s/%s unknown probe keys %s", errUnknownProbeKey, identifier.Resource, identifier.Namespace, identifier.Name, value.Name)
				}

				if value.Value.Integer == nil {
					return fmt.Errorf("%w: %s with key %s/%s", errProbeValueIsNil, identifier.Resource, identifier.Namespace, identifier.Name)
				}

				if *value.Value.Integer < 1 {
					return fmt.Errorf("%w: %s replicas is %d for %s/%s", errProbeConditionNotSatisfied, identifier.Resource, *value.Value.Integer, identifier.Namespace, identifier.Name)
				}
				// otel collector passes the health check
			}
		case addoncfg.UiPluginsResource:
			if len(field.FeedbackResult.Values) == 0 {
				return fmt.Errorf("%w for %s with key %s/%s", errMissingFeedbackValues, identifier.Resource, identifier.Namespace, identifier.Name)
			}
			for _, value := range field.FeedbackResult.Values {
				if value.Name != addoncfg.UipProbeKey {
					return fmt.Errorf("%w: %s with key %s unknown probe keys %s", errUnknownProbeKey, identifier.Resource, identifier.Name, value.Name)
				}

				if value.Value.String == nil {
					return fmt.Errorf("%w: %s with key %s", errProbeValueIsNil, identifier.Resource, identifier.Name)
				}

				if *value.Value.String != "True" {
					return fmt.Errorf("%w: %s status condition type is %s for %s", errProbeConditionNotSatisfied, identifier.Resource, *value.Value.String, identifier.Name)
				}
				// uiplugin passes the health check
			}
		case crdResourceName:
			if len(field.FeedbackResult.Values) == 0 {
				return fmt.Errorf("%w for %s with key %s/%s", errMissingFeedbackValues, identifier.Resource, identifier.Namespace, identifier.Name)
			}
			switch addoncfg.Name {
			case scrapeConfigCRDName:
				if err := checkScrapeConfigCRD(field.FeedbackResult.Values); err != nil {
					return fmt.Errorf("%w: %s with key %s", err, identifier.Resource, identifier.Name)
				}
			}
			continue
		default:
			// If a resource is being probed it should have a health check defined
			return fmt.Errorf("%w: %s with key %s/%s", errUnknownResource, identifier.Resource, identifier.Namespace, identifier.Name)
		}
	}
	return nil
}

func checkScrapeConfigCRD(feedbackValues []workv1.FeedbackValue) error {
	if len(feedbackValues) == 0 {
		return errProbeValueIsNil
	}

	var version, isEstablished string
	for _, value := range feedbackValues {
		switch value.Name {
		case addoncfg.PrometheusOperatorVersionFeedbackName:
			if value.Value.String != nil {
				version = *value.Value.String
			}
		case addoncfg.IsEstablishedFeedbackName:
			if value.Value.String != nil {
				isEstablished = *value.Value.String
			}
		}
	}

	if strings.ToLower(isEstablished) != "true" {
		return fmt.Errorf("%w: resource is not established", errProbeConditionNotSatisfied)
	}

	if version == "" {
		return fmt.Errorf("%w: prometheus operator version not found in scrapeconfigs.monitoring.rhobs CRD", errProbeConditionNotSatisfied)
	}

	isOlder, err := isVersionOlder(version, minPrometheusOperatorVersion)
	if err != nil {
		return fmt.Errorf("failed to parse prometheus operator version: %w", err)
	} else if isOlder {
		return fmt.Errorf("%w: incompatible prometheus operator version %s, requires %s or above", errProbeConditionNotSatisfied, version, minPrometheusOperatorVersion)
	}

	return nil
}

// isVersionOlder checks if v1 is older than v2.
// It handles versions like "0.80.1-rhobs1".
func isVersionOlder(v1, v2 string) (bool, error) {
	v1 = strings.Split(v1, "-")[0]
	v2 = strings.Split(v2, "-")[0]

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := max(len(parts1), len(parts2))

	for i := range maxLen {
		var num1, num2 int
		var err error

		if i < len(parts1) {
			num1, err = strconv.Atoi(parts1[i])
			if err != nil {
				return false, fmt.Errorf("%w: %s", errInvalidVersionString, v1)
			}
		}

		if i < len(parts2) {
			num2, err = strconv.Atoi(parts2[i])
			if err != nil {
				return false, fmt.Errorf("%w: %s", errInvalidVersionString, v2)
			}
		}

		if num1 < num2 {
			return true, nil
		}
		if num1 > num2 {
			return false, nil
		}
	}

	return false, nil // equal
}
