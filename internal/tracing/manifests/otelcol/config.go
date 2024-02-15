package otelcol

import (
	"github.com/ViaQ/logerr/v2/kverrors"
	"gopkg.in/yaml.v3"
)

func ConfigFromString(configStr string) (map[string]interface{}, error) {
	config := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(configStr), &config); err != nil {
		return nil, kverrors.New("couldn't parse the opentelemetry-collector configuration")
	}

	return config, nil
}
