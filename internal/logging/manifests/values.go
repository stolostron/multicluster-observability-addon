package manifests

import (
	"encoding/json"
)

type LoggingValues struct {
	Enabled                    bool      `json:"enabled"`
	LoggingSubscriptionChannel string    `json:"loggingSubscriptionChannel"`
	Unmanaged                  Unmanaged `json:"unmanaged"`
	Managed                    Managed   `json:"managed"`
}

// Unmanaged is a struct that holds configuration for resources managed by
// the user.
type Unmanaged struct {
	Collection Collection `json:"collection"`
}

// Managed is a struct that holds configuration for resources managed by
// MCOA.
type Managed struct {
	Collection Collection `json:"collection"`
	Storage    Storage    `json:"storage"`
}

type Collection struct {
	Enabled    bool            `json:"enabled"`
	CLFSpec    string          `json:"clfSpec"`
	Secrets    []ResourceValue `json:"secrets"`
	ConfigMaps []ResourceValue `json:"configmaps"`
}

type Storage struct {
	Enabled bool `json:"enabled"`
}

type ResourceValue struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

func BuildValues(opts Options) (*LoggingValues, error) {
	values := &LoggingValues{
		Enabled: true,
		Managed: Managed{
			Collection: Collection{
				Enabled: false,
			},
			Storage: Storage{
				Enabled: false,
			},
		},
		Unmanaged: Unmanaged{
			Collection: Collection{
				Enabled: true,
			},
		},
	}

	values.LoggingSubscriptionChannel = buildSubscriptionChannel(opts)

	configmaps, err := buildConfigMaps(opts)
	if err != nil {
		return nil, err
	}
	values.Unmanaged.Collection.ConfigMaps = configmaps

	secrets, err := buildSecrets(opts)
	if err != nil {
		return nil, err
	}
	values.Unmanaged.Collection.Secrets = secrets

	clfSpec, err := buildClusterLogForwarderSpec(opts)
	if err != nil {
		return nil, err
	}

	b, err := json.Marshal(clfSpec)
	if err != nil {
		return nil, err
	}
	values.Unmanaged.Collection.CLFSpec = string(b)

	return values, nil
}
