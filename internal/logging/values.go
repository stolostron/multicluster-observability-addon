package logging

import (
	"encoding/json"

	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetValuesFunc(k8s client.Client, _ *clusterv1.ManagedCluster, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, adoc *addonapiv1alpha1.AddOnDeploymentConfig) (LoggingValues, error) {
	values := LoggingValues{
		Enabled: true,
	}

	values.LoggingSubscriptionChannel = buildSubscriptionChannel(adoc)

	clfSpec, err := buildClusterLogForwarderSpec(k8s, mcAddon)
	if err != nil {
		return values, err
	}

	b, err := json.Marshal(clfSpec)
	if err != nil {
		return values, err
	}

	values.CLFSpec = string(b)

	return values, nil
}
