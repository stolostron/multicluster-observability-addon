package addon

import (
	"errors"
	"fmt"

	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"open-cluster-management.io/addon-framework/pkg/agent"
	"open-cluster-management.io/addon-framework/pkg/utils"
	"open-cluster-management.io/api/addon/v1alpha1"
	v1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
)

var (
	errMissingFields              = errors.New("no fields found in health checker")
	errProbeConditionNotSatisfied = errors.New("probe condition is not satisfied")
	errProbeValueIsNil            = errors.New("probe value is nil")
	errUnknownProbeKey            = errors.New("probe has key that doesn't match the key defined")
	errUnknownResource            = errors.New("undefined health check for resource")
)

func NewRegistrationOption(agentName string) *agent.RegistrationOption {
	return &agent.RegistrationOption{
		CSRConfigurations: agent.KubeClientSignerConfigurations(addoncfg.Name, agentName),
		CSRApproveCheck:   utils.DefaultCSRApprover(agentName),
	}
}

// AgentHealthProber returns a HealthProber struct that contains the necessary
// information to assert if an addon deployment is ready or not.
func AgentHealthProber() *agent.HealthProber {
	return &agent.HealthProber{
		Type: agent.HealthProberTypeWork,
		WorkProber: &agent.WorkHealthProber{
			ProbeFields: []agent.ProbeField{
				{
					ResourceIdentifier: workv1.ResourceIdentifier{
						Group:     prometheusalpha1.SchemeGroupVersion.Group,
						Resource:  prometheusalpha1.PrometheusAgentName,
						Name:      mconfig.PlatformMetricsCollectorApp,
						Namespace: addoncfg.InstallNamespace,
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
				if len(fields) == 0 {
					return errMissingFields
				}
				for _, field := range fields {
					if len(field.FeedbackResult.Values) == 0 {
						// If a probe didn't get any values maybe the resources were not deployed
						continue
					}
					identifier := field.ResourceIdentifier
					switch identifier.Resource {
					case prometheusalpha1.PrometheusAgentName:
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
					default:
						// If a resource is being probed it should have a health check defined
						return fmt.Errorf("%w: %s with key %s/%s", errUnknownResource, identifier.Resource, identifier.Namespace, identifier.Name)
					}
				}
				return nil
			},
		},
	}
}
