package addon

import (
	"fmt"

	"open-cluster-management.io/addon-framework/pkg/agent"
	"open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	workapiv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		Type: agent.HealthProberTypeDeploymentAvailability,
		WorkProber: &agent.WorkHealthProber{
			ProbeFields: []agent.ProbeField{
				{
					ResourceIdentifier: workapiv1.ResourceIdentifier{
						Group:     "apps",
						Resource:  "deployments",
						Name:      "cluster-logging-operator",
						Namespace: ClusterLoggingNS,
					},
					ProbeRules: []workapiv1.FeedbackRule{
						{
							Type: workapiv1.WellKnownStatusType,
						},
					},
				},
				{
					ResourceIdentifier: workapiv1.ResourceIdentifier{
						Group:     "apps",
						Resource:  "deployments",
						Name:      "spoke-otelcol-collector",
						Namespace: CollectorNS,
					},
					ProbeRules: []workapiv1.FeedbackRule{
						{
							Type: workapiv1.WellKnownStatusType,
						},
					},
				},
			},
			HealthCheck: func(identifier workapiv1.ResourceIdentifier, result workapiv1.StatusFeedbackResult) error {
				if identifier.Resource != "deployments" {
					return fmt.Errorf("unsupported resource type %s", identifier.Resource)
				}
				if identifier.Group != "apps" {
					return fmt.Errorf("unsupported resource group %s", identifier.Group)
				}
				if len(result.Values) == 0 {
					return fmt.Errorf("no values are probed for deployment %s/%s", identifier.Namespace, identifier.Name)
				}
				for _, value := range result.Values {
					if value.Name != "ReadyReplicas" {
						continue
					}

					if *value.Value.Integer >= 0 {
						return nil
					}

					return fmt.Errorf("readyReplicas is %d for deployement %s/%s", *value.Value.Integer, identifier.Namespace, identifier.Name)
				}
				return fmt.Errorf("readyReplicas is not probed")
			},
		},
	}
}
