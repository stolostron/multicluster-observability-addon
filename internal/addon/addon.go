package addon

import (
	"errors"
	"fmt"

	"open-cluster-management.io/addon-framework/pkg/agent"
	"open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	workapiv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errUnavailable    = errors.New("probe condition is not satisfied")
	errValueNotProbed = errors.New("value not probed")
)

func NewRegistrationOption(agentName string) *agent.RegistrationOption {
	return &agent.RegistrationOption{
		CSRConfigurations: agent.KubeClientSignerConfigurations(Name, agentName),
		CSRApproveCheck:   utils.DefaultCSRApprover(agentName),
	}
}

func GetObjectKey(configRef []addonapiv1alpha1.ConfigReference, group, resource string) client.ObjectKey {
	var key client.ObjectKey
	for _, config := range configRef {
		if config.ConfigGroupResource.Group != group {
			continue
		}
		if config.ConfigGroupResource.Resource != resource {
			continue
		}

		key.Name = config.Name
		key.Namespace = config.Namespace
		break
	}
	return key
}

func AgentHealthProber() *agent.HealthProber {
	return &agent.HealthProber{
		Type: agent.HealthProberTypeWork,
		WorkProber: &agent.WorkHealthProber{
			ProbeFields: []agent.ProbeField{
				{
					ResourceIdentifier: workapiv1.ResourceIdentifier{
						Group:     OtelcolGroup,
						Resource:  OtelcolResource,
						Name:      OtelcolName,
						Namespace: OtelcolNS,
					},
					ProbeRules: []workapiv1.FeedbackRule{
						{
							Type: workapiv1.JSONPathsType,
							JsonPaths: []workapiv1.JsonPath{
								{
									Name: "replicas",
									Path: ".spec.replicas",
								},
							},
						},
					},
				},
				{
					ResourceIdentifier: workapiv1.ResourceIdentifier{
						Group:     ClfGroup,
						Resource:  ClfResource,
						Name:      ClfName,
						Namespace: ClusterLoggingNS,
					},
					ProbeRules: []workapiv1.FeedbackRule{
						{
							Type: workapiv1.JSONPathsType,
							JsonPaths: []workapiv1.JsonPath{
								{
									Name: "isReady",
									Path: ".status.conditions[?(@.type==\"Ready\")].status",
								},
							},
						},
					},
				},
			},
			HealthCheck: func(identifier workapiv1.ResourceIdentifier, result workapiv1.StatusFeedbackResult) error {
				for _, value := range result.Values {
					switch {
					case identifier.Resource == ClfResource:
						if value.Name != "isReady" {
							continue
						}

						if *value.Value.String != "True" {
							return fmt.Errorf("%w: status condition type is %s for %s/%s", errUnavailable, *value.Value.String, identifier.Namespace, identifier.Name)
						}

						return nil
					case identifier.Resource == OtelcolResource:
						if value.Name != "replicas" {
							continue
						}

						if *value.Value.Integer < 1 {
							return fmt.Errorf("%w: replicas is %d for %s/%s", errUnavailable, *value.Value.Integer, identifier.Namespace, identifier.Name)
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
