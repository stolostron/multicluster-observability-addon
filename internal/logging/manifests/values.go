package manifests

import "encoding/json"

type LoggingValues struct {
	Enabled                    bool          `json:"enabled"`
	CLFs                       []CLFValue    `json:"clfs"`
	LoggingSubscriptionChannel string        `json:"loggingSubscriptionChannel"`
	Secrets                    []SecretValue `json:"secrets"`
}

type CLFValue struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Spec      string `json:"spec"`
}

type SecretValue struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Data      string `json:"data"`
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

	clfs, err := buildClusterLogForwarders(opts)
	if err != nil {
		return nil, err
	}

	clfsValue := []CLFValue{}
	for _, clf := range clfs {
		specJSON, err := json.Marshal(clf.Spec)
		if err != nil {
			return nil, err
		}
		clfValue := CLFValue{
			Name:      clf.Name,
			Namespace: clf.Namespace,
			Spec:      string(specJSON),
		}
		clfsValue = append(clfsValue, clfValue)
	}

	values.CLFs = clfsValue

	return values, nil
}
