package addon

import (
	"open-cluster-management.io/addon-framework/pkg/agent"
	"open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
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
