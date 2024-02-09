package manifests

import (
	"encoding/json"
)

type LoggingValues struct {
	Enabled                    bool          `json:"enabled"`
	CLFSpec                    string        `json:"clfSpec"`
	LoggingSubscriptionChannel string        `json:"loggingSubscriptionChannel"`
	Secrets                    []SecretValue `json:"secrets"`
}
type SecretValue struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

func BuildValues(opts Options) (*LoggingValues, error) {
	values := &LoggingValues{
		Enabled: true,
	}

	values.LoggingSubscriptionChannel = buildSubscriptionChannel(opts)

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

	return values, nil
}
