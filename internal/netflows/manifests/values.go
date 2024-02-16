package manifests

import (
	"encoding/json"
)

type NetflowValues struct {
	Enabled           bool   `json:"enabled"`
	FlowCollectorSpec string `json:"flowCollectorSpec"`
}

func BuildValues(opts Options) (*NetflowValues, error) {
	values := NetflowValues{
		Enabled: true,
	}

	clfSpec, err := buildFlowCollectorSpec(opts)
	if err != nil {
		return nil, err
	}

	b, err := json.Marshal(clfSpec)
	if err != nil {
		return nil, err
	}
	values.FlowCollectorSpec = string(b)

	return &values, nil
}
