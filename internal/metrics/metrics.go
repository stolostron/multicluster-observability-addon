package metrics

import (
	"context"
	"fmt"

	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/types"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MetricsValues struct {
	Enabled bool `json:"enabled"`
	// TODO: revert this hack to the official way as recommended by the docs.
	// See https://open-cluster-management.io/developer-guides/addon/#values-definition.
	AddonInstallNamespace string `json:"addonInstallNamespace"`
	DestinationEndpoint   string `json:"destinationEndpoint"`
}

func GetValuesFunc(
	k8sClient client.Client,
	_ *clusterv1.ManagedCluster,
	mca *addonapiv1alpha1.ManagedClusterAddOn,
	adoc *addonapiv1alpha1.AddOnDeploymentConfig,
) (MetricsValues, error) {
	endpoint, err := getDestinationEndpoint(k8sClient, adoc)
	if err != nil {
		return MetricsValues{}, fmt.Errorf("failed to get metrics destination endpoint: %w", err)
	}
	values := MetricsValues{
		Enabled:               true,
		AddonInstallNamespace: mca.Spec.InstallNamespace,
		// TODO: grab Location from the `open-cluster-management-observability/observatorium-api` Route in the Hub
		DestinationEndpoint: endpoint,
	}
	return values, nil
}

func getDestinationEndpoint(k8sClient client.Client, adoc *addonapiv1alpha1.AddOnDeploymentConfig) (string, error) {
	if adoc == nil {
		return "", nil
	}

	for _, customVar := range adoc.Spec.CustomizedVariables {
		if customVar.Name == "metricsDestinationEndpoint" {
			return customVar.Value, nil
		}
	}

	route := &routev1.Route{}
	err := k8sClient.Get(context.TODO(), types.NamespacedName{
		Namespace: "open-cluster-management-observability",
		Name:      "observatorium-api",
	}, route)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://%s/api/metrics/v1/default/api/v1/receive", route.Spec.Host), nil
}
