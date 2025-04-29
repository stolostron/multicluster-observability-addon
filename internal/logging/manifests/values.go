package manifests

import (
	"encoding/json"
)

type LoggingValues struct {
	Enabled                 bool            `json:"enabled"`
	InstallCLO              bool            `json:"installCLO"`
	CLFAnnotations          string          `json:"clfAnnotations"`
	CLFSpec                 string          `json:"clfSpec"`
	ServiceAccountName      string          `json:"serviceAccountName"`
	OpenshiftLoggingChannel string          `json:"openshiftLoggingChannel"`
	Secrets                 []ResourceValue `json:"secrets"`
	ConfigMaps              []ResourceValue `json:"configmaps"`
}
type ResourceValue struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

func BuildValues(opts Options) (*LoggingValues, error) {
	values := &LoggingValues{
		Enabled: true,
	}

	values.OpenshiftLoggingChannel = buildSubscriptionChannel(opts)

	installCLO, err := shouldInstallCLO(opts, values.OpenshiftLoggingChannel)
	if err != nil {
		return nil, err
	}
	values.InstallCLO = installCLO

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

	// CLO uses annotations to signal feature flags so users must be able to set
	// them
	clfAnnotations := opts.ClusterLogForwarder.GetAnnotations()
	clfAnnotationsJson, err := json.Marshal(clfAnnotations)
	if err != nil {
		return nil, err
	}
	values.CLFAnnotations = string(clfAnnotationsJson)

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

func shouldInstallCLO(opts Options, channel string) (bool, error) {
	// If no subscription is provided, want to install CLO
	if opts.ClusterLoggingSubscription == nil || opts.ClusterLoggingSubscription.Name == "" {
		return true, nil
	}

	if opts.ClusterLoggingSubscription.Spec.Channel != channel {
		return false, errInvalidSubscriptionChannel
	}

	// If the subscription has our release label, install the operator
	if value, exists := opts.ClusterLoggingSubscription.Labels["release"]; exists && value == "multicluster-observability-addon" {
		return true, nil
	}

	return false, nil
}
