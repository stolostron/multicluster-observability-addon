package otelcol

import (
	"github.com/ViaQ/logerr/v2/kverrors"
	"gopkg.in/yaml.v2"
)

func ConfigFromString(configStr string) (map[interface{}]interface{}, error) {
	config := make(map[interface{}]interface{})
	if err := yaml.Unmarshal([]byte(configStr), &config); err != nil {
		return nil, kverrors.New("couldn't parse the opentelemetry-collector configuration")
	}

	return config, nil
}
