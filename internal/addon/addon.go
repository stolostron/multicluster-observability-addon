package addon

import (
	"errors"
	"fmt"

	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	"open-cluster-management.io/addon-framework/pkg/agent"
	"open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errProbeConditionNotSatisfied = errors.New("probe condition is not satisfied")
	errProbeValueIsNil            = errors.New("probe value is nil")
	errValueNotProbed             = errors.New("value not probed")
)

func NewRegistrationOption(agentName string) *agent.RegistrationOption {
	return &agent.RegistrationOption{
		CSRConfigurations: agent.KubeClientSignerConfigurations(Name, agentName),
		CSRApproveCheck:   utils.DefaultCSRApprover(agentName),
	}
}

func GetObjectKeys(configRef []addonapiv1alpha1.ConfigReference, group, resource string) []client.ObjectKey {
	var keys []client.ObjectKey
	for _, config := range configRef {
		if config.ConfigGroupResource.Group != group {
			continue
		}
		if config.ConfigGroupResource.Resource != resource {
			continue
		}

		keys = append(keys, client.ObjectKey{
			Name:      config.Name,
			Namespace: config.Namespace,
		})
	}
	return keys
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
						Group:     loggingv1.GroupVersion.Group,
						Resource:  ClusterLogForwardersResource,
						Name:      SpokeCLFName,
						Namespace: SpokeCLFNamespace,
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
			HealthCheck: func(identifier workv1.ResourceIdentifier, result workv1.StatusFeedbackResult) error {
				for _, value := range result.Values {
					switch {
					case identifier.Resource == ClusterLogForwardersResource:
						if value.Name != clfProbeKey {
							continue
						}

						if value.Value.String == nil {
							return fmt.Errorf("%w: clusterlogforwarder with key %s/%s", errProbeValueIsNil, identifier.Namespace, identifier.Name)
						}

						if *value.Value.String != "True" {
							return fmt.Errorf("%w: clusterlogforwarder status condition type is %s for %s/%s", errProbeConditionNotSatisfied, *value.Value.String, identifier.Namespace, identifier.Name)
						}

						return nil
					case identifier.Resource == OpenTelemetryCollectorsResource:
						if value.Name != otelColProbeKey {
							continue
						}

						if value.Value.Integer == nil {
							return fmt.Errorf("%w: opentelemetrycollector with key %s/%s", errProbeValueIsNil, identifier.Namespace, identifier.Name)
						}

						if *value.Value.Integer < 1 {
							return fmt.Errorf("%w: opentelemetrycollector replicas is %d for %s/%s", errProbeConditionNotSatisfied, *value.Value.Integer, identifier.Namespace, identifier.Name)
						}

						return nil
					default:
						continue
					}
				}
				return fmt.Errorf("%w: for resource %s with key %s/%s", errValueNotProbed, identifier.Resource, identifier.Namespace, identifier.Name)
			},
		},
	}
}
