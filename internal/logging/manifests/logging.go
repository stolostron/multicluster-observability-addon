package manifests

import (
	"encoding/json"
	"errors"

	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
)

var (
	errPlatformLogsNotDefined          = errors.New("no ClusterLogForwarder provided defined Platform logs")
	errUserWorkloadLogsNotDefined      = errors.New("no ClusterLogForwarder provided defined User Workloads logs")
	errCLFNamespaceNotOpenshiftLogging = errors.New("to handle platform logs the ClusterLogForwarder must be in the openshift-logging")
)

func buildSubscriptionChannel(resources Options) string {
	if resources.SubscriptionChannel != "" {
		return resources.SubscriptionChannel
	}
	return defaultLoggingVersion
}

func buildSecrets(resources Options) ([]SecretValue, error) {
	secretsValue := []SecretValue{}
	for namespace, secrets := range resources.Secrets {
		for _, secret := range secrets {
			dataJSON, err := json.Marshal(secret.Data)
			if err != nil {
				return secretsValue, err
			}
			secretValue := SecretValue{
				Name:      secret.Name,
				Namespace: namespace,
				Data:      string(dataJSON),
			}
			secretsValue = append(secretsValue, secretValue)
		}
	}
	return secretsValue, nil
}

func buildClusterLogForwarders(resources Options) ([]loggingv1.ClusterLogForwarder, error) {
	var (
		platformDetected      bool
		userWorkloadsDetected bool
	)
	clfs := []loggingv1.ClusterLogForwarder{}
	for _, clf := range resources.ClusterLogForwarders {
		// TODO @JoaoBraveCoding this would need to be fixed we would need to analize the spec
		// of each clf and determine which RBAC we would need
		clf.Spec.ServiceAccountName = "mcoa-logcollector"

		// By default, the namespace is openshift-logging unless users specify
		// a different namespace
		clf.Namespace = "openshift-logging"
		if val, ok := clf.Annotations["mcoa/namespace"]; ok {
			clf.Namespace = val
		}
		pd, uwd, err := validateClusterLogForwarder(clf)
		if err != nil {
			return clfs, err
		}
		platformDetected = platformDetected || pd
		userWorkloadsDetected = userWorkloadsDetected || uwd

		clfs = append(clfs, clf)
	}

	if resources.Platform.CollectionEnabled && !platformDetected {
		return nil, errPlatformLogsNotDefined
	}

	if resources.UserWorkloads.CollectionEnabled && !userWorkloadsDetected {
		return nil, errUserWorkloadLogsNotDefined
	}

	return clfs, nil
}

func validateClusterLogForwarder(clf loggingv1.ClusterLogForwarder) (bool, bool, error) {
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

	if len(platformInputRefs) > 0 && clf.Namespace != "openshift-logging" {
		return false, false, errCLFNamespaceNotOpenshiftLogging
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

			if ref == loggingv1.InputNameInfrastructure || ref == loggingv1.InputNameAudit {
				platformDetected = true
			}
			if ref == loggingv1.InputNameApplication {
				userWorkloadsDetected = true
			}
		}
	}

	return platformDetected, userWorkloadsDetected, nil
}
