package addon

import (
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
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/agent"
	"open-cluster-management.io/addon-framework/pkg/utils"
	"open-cluster-management.io/api/addon/v1alpha1"
	v1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
)

const (
	minPrometheusOperatorVersion = "0.79.0"
)

var (
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

// AgentHealthProber returns a HealthProber struct that contains the necessary
// information to assert if an addon deployment is ready or not.
func AgentHealthProber(logger logr.Logger) *agent.HealthProber {
	return &agent.HealthProber{
		Type: agent.HealthProberTypeWork,
		WorkProber: &agent.WorkHealthProber{
			ProbeFields: []agent.ProbeField{
				{
					ResourceIdentifier: workv1.ResourceIdentifier{
						Group:     cooprometheusv1alpha1.SchemeGroupVersion.Group,
						Resource:  cooprometheusv1alpha1.PrometheusAgentName,
						Name:      mconfig.PlatformMetricsCollectorApp,
						Namespace: addonfactory.AddonDefaultInstallNamespace,
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
						Resource: "customresourcedefinitions",
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
						Resource: "customresourcedefinitions",
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
			},
			HealthChecker: func(fields []agent.FieldResult, mc *v1.ManagedCluster, mcao *v1alpha1.ManagedClusterAddOn) error {
				if err := healthChecker(logger, fields, mc, mcao); err != nil {
					logger.V(1).Info("Health check failed for managed cluster %s: %v", mc.Name, err)
					return err
				}
				return nil
			},
		},
	}
}

func Updaters() []agent.Updater {
	ssaWithoutForce := workv1.UpdateStrategy{
		Type: workv1.UpdateStrategyTypeServerSideApply,
		ServerSideApply: &workv1.ServerSideApplyConfig{
			Force: false,
		},
	}
	return []agent.Updater{
		{
			UpdateStrategy: ssaWithoutForce,
			ResourceIdentifier: workv1.ResourceIdentifier{
				Group:    apiextensionsv1.GroupName,
				Resource: "customresourcedefinitions",
				Name:     scrapeConfigCRDName,
			},
		},
		{
			UpdateStrategy: ssaWithoutForce,
			ResourceIdentifier: workv1.ResourceIdentifier{
				Group:    apiextensionsv1.GroupName,
				Resource: "customresourcedefinitions",
				Name:     prometheusAgentCRDName,
			},
		},
		{
			UpdateStrategy: ssaWithoutForce,
			ResourceIdentifier: workv1.ResourceIdentifier{
				Group:    apiextensionsv1.GroupName,
				Resource: "customresourcedefinitions",
				Name:     serviceMonitorCRDName,
			},
		},
		{
			UpdateStrategy: ssaWithoutForce,
			ResourceIdentifier: workv1.ResourceIdentifier{
				Group:    apiextensionsv1.GroupName,
				Resource: "customresourcedefinitions",
				Name:     podMonitorCRDName,
			},
		},
		{
			UpdateStrategy: ssaWithoutForce,
			ResourceIdentifier: workv1.ResourceIdentifier{
				Group:    apiextensionsv1.GroupName,
				Resource: "customresourcedefinitions",
				Name:     probeCRDName,
			},
		},
	}
}

func healthChecker(logger logr.Logger, fields []agent.FieldResult, mc *v1.ManagedCluster, mcao *v1alpha1.ManagedClusterAddOn) error {
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
				return fmt.Errorf("%w: %s with key %s/%s", errMissingFields, identifier.Resource, identifier.Namespace, identifier.Name)
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
				// If a probe didn't get any values maybe the resources were not deployed
				continue
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
				// If a probe didn't get any values maybe the resources were not deployed
				continue
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
				// If a probe didn't get any values maybe the resources were not deployed
				continue
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
		case "customresourcedefinitions":
			switch addoncfg.Name {
			case "scrapeconfigs.monitoring.rhobs":
				if err := checkScrapeConfigCRD(logger, field.FeedbackResult.Values, mc); err != nil {
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

func checkScrapeConfigCRD(logger logr.Logger, feedbackValues []workv1.FeedbackValue, mc *v1.ManagedCluster) error {
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
		logger.Info("failed to parse prometheus operator version", "error", err.Error(), "managedCluster", mc.Name)
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
