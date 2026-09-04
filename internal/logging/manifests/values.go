package manifests

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
)

type LoggingValues struct {
	Enabled                 bool            `json:"enabled"`
	InstallCLO              bool            `json:"installCLO"`
	OpenshiftLoggingChannel string          `json:"openshiftLoggingChannel"`
	Unmanaged               UnmanagedValues `json:"unmanaged"`
	Managed                 ManagedValues   `json:"managed"`
}

type UnmanagedValues struct {
	Collection CollectionValues `json:"collection"`
}

type ManagedValues struct {
	Collection CollectionValues `json:"collection"`
	Storage    StorageValues    `json:"storage"`
}

type CollectionValues struct {
	Enabled        bool            `json:"enabled"`
	CLFAnnotations string          `json:"clfAnnotations"`
	CLFSpec        string          `json:"clfSpec"`
	Secrets        []ResourceValue `json:"secrets"`
	ConfigMaps     []ResourceValue `json:"configmaps"`
}

type StorageValues struct {
	Enabled bool            `json:"enabled"`
	Secrets []ResourceValue `json:"secrets"`
	LSSpec  string          `json:"lsSpec"`
}

type ResourceValue struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

func secretsToResourceValues(secrets []corev1.Secret) ([]ResourceValue, error) {
	values := make([]ResourceValue, 0, len(secrets))
	for _, s := range secrets {
		dataJSON, err := json.Marshal(s.Data)
		if err != nil {
			return values, err
		}
		values = append(values, ResourceValue{Name: s.Name, Data: string(dataJSON)})
	}
	return values, nil
}

func configMapsToResourceValues(configMaps []corev1.ConfigMap) ([]ResourceValue, error) {
	values := make([]ResourceValue, 0, len(configMaps))
	for _, cm := range configMaps {
		dataJSON, err := json.Marshal(cm.Data)
		if err != nil {
			return values, err
		}
		values = append(values, ResourceValue{Name: cm.Name, Data: string(dataJSON)})
	}
	return values, nil
}

func BuildValues(opts Options) (*LoggingValues, error) {
	subChannel := buildSubscriptionChannel(opts)

	installCLO, err := shouldInstallCLO(opts, subChannel)
	if err != nil {
		return nil, err
	}

	uValues, err := buildUnmanagedValues(opts)
	if err != nil {
		return nil, err
	}

	mValues, err := buildManagedValues(opts)
	if err != nil {
		return nil, err
	}

	return &LoggingValues{
		Enabled:                 enabledLogging(opts),
		OpenshiftLoggingChannel: subChannel,
		InstallCLO:              installCLO,
		Unmanaged:               uValues,
		Managed:                 mValues,
	}, nil
}

func enabledLogging(opts Options) bool {
	return opts.UnmanagedCollectionEnabled() || opts.DefaultStackEnabled()
}

func buildUnmanagedValues(opts Options) (UnmanagedValues, error) {
	if !opts.UnmanagedCollectionEnabled() {
		return UnmanagedValues{}, nil
	}

	uValues := UnmanagedValues{
		Collection: CollectionValues{
			Enabled: true,
		},
	}

	configmaps, err := configMapsToResourceValues(opts.Unmanaged.Collection.ConfigMaps)
	if err != nil {
		return uValues, err
	}
	uValues.Collection.ConfigMaps = configmaps

	secrets, err := secretsToResourceValues(opts.Unmanaged.Collection.Secrets)
	if err != nil {
		return uValues, err
	}
	uValues.Collection.Secrets = secrets

	clfSpec, err := buildClusterLogForwarderSpec(opts)
	if err != nil {
		return uValues, err
	}

	// CLO uses annotations to signal feature flags so users must be able to set
	// them. Marshal after buildClusterLogForwarderSpec so the SSA managed-fields
	// annotation is included.
	clfAnnotations := opts.Unmanaged.Collection.ClusterLogForwarder.GetAnnotations()
	clfAnnotationsJson, err := json.Marshal(clfAnnotations)
	if err != nil {
		return uValues, err
	}
	uValues.Collection.CLFAnnotations = string(clfAnnotationsJson)

	b, err := json.Marshal(clfSpec)
	if err != nil {
		return uValues, err
	}
	uValues.Collection.CLFSpec = string(b)

	return uValues, nil
}

func buildManagedValues(opts Options) (ManagedValues, error) {
	if !opts.DefaultStackEnabled() {
		return ManagedValues{}, nil
	}

	collection, err := buildManagedCollectionValues(opts)
	if err != nil {
		return ManagedValues{}, err
	}

	storage, err := buildManagedStorageValues(opts)
	if err != nil {
		return ManagedValues{}, err
	}

	return ManagedValues{
		Collection: collection,
		Storage:    storage,
	}, nil
}

func buildManagedCollectionValues(opts Options) (CollectionValues, error) {
	cValues := CollectionValues{
		Enabled: true,
	}
	configmaps, err := configMapsToResourceValues(opts.DefaultStack.Collection.ConfigMaps)
	if err != nil {
		return cValues, err
	}
	cValues.ConfigMaps = configmaps

	secrets, err := secretsToResourceValues(opts.DefaultStack.Collection.Secrets)
	if err != nil {
		return cValues, err
	}
	cValues.Secrets = secrets

	clfSpec, err := buildManagedCLFSpec(opts)
	if err != nil {
		return cValues, err
	}

	clfMarshaled, err := json.Marshal(clfSpec)
	if err != nil {
		return cValues, err
	}
	cValues.CLFSpec = string(clfMarshaled)

	return cValues, nil
}

func buildManagedStorageValues(opts Options) (StorageValues, error) {
	if opts.DefaultStack.Storage.LokiStack == nil {
		return StorageValues{}, nil
	}

	sValues := StorageValues{
		Enabled: true,
	}
	secrets, err := secretsToResourceValues([]corev1.Secret{
		opts.DefaultStack.Storage.ObjStorageSecret,
		opts.DefaultStack.Storage.MTLSSecret,
	})
	if err != nil {
		return sValues, err
	}
	sValues.Secrets = secrets

	lsSpec, err := buildManagedLokistackSpec(opts)
	if err != nil {
		return sValues, err
	}

	lsMarshaled, err := json.Marshal(lsSpec)
	if err != nil {
		return sValues, err
	}
	sValues.LSSpec = string(lsMarshaled)

	return sValues, nil
}

func shouldInstallCLO(opts Options, channel string) (bool, error) {
	if opts.ClusterLoggingSubscription == nil || opts.ClusterLoggingSubscription.Name == "" {
		return true, nil
	}

	if opts.ClusterLoggingSubscription.Spec.Channel != channel {
		return false, errInvalidSubscriptionChannel
	}

	if value, exists := opts.ClusterLoggingSubscription.Labels["release"]; exists && value == "multicluster-observability-addon" {
		return true, nil
	}

	return false, nil
}
