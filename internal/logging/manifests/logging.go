package manifests

import (
	"encoding/json"
	"errors"

	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
)

var (
	errPlatformLogsNotDefined     = errors.New("Platform logs not defined")
	errUserWorkloadLogsNotDefined = errors.New("User Workloads logs not defined")
)

func buildSubscriptionChannel(resources Options) string {
	if resources.SubscriptionChannel != "" {
		return resources.SubscriptionChannel
	}
	return defaultLoggingVersion
}

func buildConfigMaps(resources Options) ([]ResourceValue, error) {
	configmapsValue := []ResourceValue{}
	for _, configmap := range resources.ConfigMaps {
		dataJSON, err := json.Marshal(configmap.Data)
		if err != nil {
			return configmapsValue, err
		}
		configmapValue := ResourceValue{
			Name: configmap.Name,
			Data: string(dataJSON),
		}
		configmapsValue = append(configmapsValue, configmapValue)
	}
	return configmapsValue, nil
}

func buildSecrets(resources Options) ([]ResourceValue, error) {
	secretsValue := []ResourceValue{}
	for _, secret := range resources.Secrets {
		dataJSON, err := json.Marshal(secret.Data)
		if err != nil {
			return secretsValue, err
		}
		secretValue := ResourceValue{
			Name: secret.Name,
			Data: string(dataJSON),
		}
		secretsValue = append(secretsValue, secretValue)
	}
	return secretsValue, nil
}

func buildClusterLogForwarderSpec(opts Options) (*loggingv1.ClusterLogForwarderSpec, error) {
	clf := opts.ClusterLogForwarder
	clf.Spec.ManagementState = loggingv1.ManagementStateManaged

	// Validate Platform Logs enabled
	var (
		platformInputRefs []string
		platformDetected  bool

		userWorkloadInputRefs []string
		userWorkloadsDetected bool
	)

	for _, input := range clf.Spec.Inputs {
		if input.Application != nil {
			userWorkloadInputRefs = append(userWorkloadInputRefs, input.Name)
		}
		if input.Infrastructure != nil || input.Audit != nil {
			platformInputRefs = append(platformInputRefs, input.Name)
		}
	}

	for _, pipeline := range clf.Spec.Pipelines {
		// Consider pipelines without outputs invalid
		if pipeline.OutputRefs == nil {
			continue
		}

	outer:
		for _, ref := range pipeline.InputRefs {
			for _, input := range platformInputRefs {
				if input == ref {
					platformDetected = true
					continue outer
				}
			}

			for _, input := range userWorkloadInputRefs {
				if input == ref {
					userWorkloadsDetected = true
					continue outer
				}
			}

			if ref == string(loggingv1.InputTypeInfrastructure) || ref == string(loggingv1.InputTypeAudit) {
				platformDetected = true
			}
			if ref == string(loggingv1.InputTypeApplication) {
				userWorkloadsDetected = true
			}
		}
	}

	if opts.Platform.CollectionEnabled && !platformDetected {
		return nil, errPlatformLogsNotDefined
	}

	if opts.UserWorkloads.CollectionEnabled && !userWorkloadsDetected {
		return nil, errUserWorkloadLogsNotDefined
	}

	return &clf.Spec, nil
}
