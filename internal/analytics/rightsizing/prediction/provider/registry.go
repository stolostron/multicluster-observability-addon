package provider

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

type externalProviderConfig struct {
	APIKey       string `json:"apiKey"`
	ConsentGiven bool   `json:"consentGiven"`
}

type customProviderConfig struct {
	EndpointURL  string `json:"endpointURL"`
	ConsentGiven bool   `json:"consentGiven"`
}

// Create returns a PredictionProvider from a typed configuration envelope.
func Create(pc prediction.ProviderConfig) (PredictionProvider, error) {
	t := strings.ToLower(strings.TrimSpace(pc.Type))
	switch t {
	case "", string(ProviderBuiltin):
		var mc prediction.ModelConfig
		if len(pc.Config) == 0 {
			mc = prediction.DefaultModelConfig()
		} else if err := json.Unmarshal(pc.Config, &mc); err != nil {
			return nil, fmt.Errorf("prediction provider builtin config: %w", err)
		}
		return NewBuiltinProvider(mc), nil

	case string(ProviderONNX):
		// Integration task will supply model bytes via ConfigMap; registry passes nil for now.
		return NewONNXProvider(nil), nil

	case string(ProviderExternal):
		var ec externalProviderConfig
		if len(pc.Config) > 0 {
			if err := json.Unmarshal(pc.Config, &ec); err != nil {
				return nil, fmt.Errorf("prediction provider external config: %w", err)
			}
		}
		return NewExternalProvider(ec.APIKey, ec.ConsentGiven), nil

	case string(ProviderCustom):
		var cc customProviderConfig
		if len(pc.Config) > 0 {
			if err := json.Unmarshal(pc.Config, &cc); err != nil {
				return nil, fmt.Errorf("prediction provider custom config: %w", err)
			}
		}
		return NewCustomProvider(cc.EndpointURL, cc.ConsentGiven), nil

	default:
		return nil, fmt.Errorf("prediction provider: unknown type %q", pc.Type)
	}
}
