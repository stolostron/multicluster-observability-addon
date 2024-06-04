package manifests

import (
	"encoding/json"
	"errors"
	"slices"

	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
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

func buildSecrets(resources Options) ([]SecretValue, error) {
	secretsValue := []SecretValue{}
	// Always go through map in order
	keys := make([]string, 0, len(resources.Secrets))
	for t := range resources.Secrets {
		keys = append(keys, string(t))
	}
	slices.Sort(keys)

	for _, key := range keys {
		secret := resources.Secrets[addon.Endpoint(key)]
		dataJSON, err := json.Marshal(secret.Data)
		if err != nil {
			return secretsValue, err
		}
		secretValue := SecretValue{
			Name: secret.Name,
			Data: string(dataJSON),
		}
		secretsValue = append(secretsValue, secretValue)
	}
	return secretsValue, nil
}

func buildClusterLogForwarderSpec(opts Options) (*loggingv1.ClusterLogForwarderSpec, error) {
	clf := opts.ClusterLogForwarder
	clf.Spec.ServiceAccountName = "mcoa-logcollector"

	// Validate Platform Logs enabled
	var (
		platformDetected      bool
		userWorkloadsDetected bool
	)
	for _, pipeline := range clf.Spec.Pipelines {
		// Consider pipelines without outputs invalid
		if pipeline.OutputRefs == nil {
			continue
		}

		for _, ref := range pipeline.InputRefs {
			if ref == loggingv1.InputNameInfrastructure || ref == loggingv1.InputNameAudit {
				platformDetected = true
			}
			if ref == loggingv1.InputNameApplication {
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
