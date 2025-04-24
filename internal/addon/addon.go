package addon

import (
	"errors"
	"fmt"

	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
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
		CSRConfigurations: agent.KubeClientSignerConfigurations(Name, agentName),
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
						Resource:  PrometheusAgentResource,
						Name:      PPAName,
						Namespace: InstallNamespace,
					},
					ProbeRules: []workv1.FeedbackRule{
						{
							Type: workv1.JSONPathsType,
							JsonPaths: []workv1.JsonPath{
								{
									Name: paProbeKey,
									Path: paProbePath,
								},
							},
						},
					},
				},
				{
					ResourceIdentifier: workv1.ResourceIdentifier{
						Group:     loggingv1.GroupVersion.Group,
						Resource:  ClusterLogForwardersResource,
						Name:      SpokeUnmanagedCLFName,
						Namespace: LoggingNamespace,
					},
					ProbeRules: []workv1.FeedbackRule{
						{
							Type: workv1.JSONPathsType,
							JsonPaths: []workv1.JsonPath{
								{
									Name: clfProbeKey,
									Path: clfProbePath,
								},
							},
						},
					},
				},
				{
					ResourceIdentifier: workv1.ResourceIdentifier{
						Group:     otelv1alpha1.GroupVersion.Group,
						Resource:  OpenTelemetryCollectorsResource,
						Name:      SpokeOTELColName,
						Namespace: SpokeOTELColNamespace,
					},
					ProbeRules: []workv1.FeedbackRule{
						{
							Type: workv1.JSONPathsType,
							JsonPaths: []workv1.JsonPath{
								{
									Name: otelColProbeKey,
									Path: otelColProbePath,
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
					case PrometheusAgentResource:
						for _, value := range field.FeedbackResult.Values {
							if value.Name != paProbeKey {
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
					case ClusterLogForwardersResource:
						for _, value := range field.FeedbackResult.Values {
							if value.Name != clfProbeKey {
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
					case OpenTelemetryCollectorsResource:
						for _, value := range field.FeedbackResult.Values {
							if value.Name != otelColProbeKey {
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

func GenerateDefaultStackName(placementName string) string {
	return fmt.Sprintf("%s-%s", DefaultStackPrefix, placementName)
}
