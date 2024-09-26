package manifests

import (
	"encoding/json"
)

type LoggingValues struct {
	Enabled                    bool            `json:"enabled"`
	CLFSpec                    string          `json:"clfSpec"`
	ServiceAccountName         string          `json:"serviceAccountName"`
	LoggingSubscriptionChannel string          `json:"loggingSubscriptionChannel"`
	Secrets                    []ResourceValue `json:"secrets"`
	ConfigMaps                 []ResourceValue `json:"configmaps"`
}
type ResourceValue struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

func BuildValues(opts Options) (*LoggingValues, error) {
	values := &LoggingValues{
		Enabled: true,
	}

	values.LoggingSubscriptionChannel = buildSubscriptionChannel(opts)

	configmaps, err := buildConfigMaps(opts)
	if err != nil {
		return nil, err
	}
	values.ConfigMaps = configmaps

	secrets, err := buildSecrets(opts)
	if err != nil {
		return nil, err
	}
	values.Secrets = secrets

	clfSpec, err := buildClusterLogForwarderSpec(opts)
	if err != nil {
		return nil, err
	}

	b, err := json.Marshal(clfSpec)
	if err != nil {
		return nil, err
	}
	values.CLFSpec = string(b)
	values.ServiceAccountName = opts.ClusterLogForwarder.Spec.ServiceAccount.Name

	return values, nil
}
