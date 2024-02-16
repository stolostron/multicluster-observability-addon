package manifests

import (
	"fmt"
	"testing"

	nfv1beta2 "github.com/netobserv/network-observability-operator/apis/flowcollector/v1beta2"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"open-cluster-management.io/addon-framework/pkg/addonmanager/addontesting"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
)

func Test_BuildFlowCollectorSpec(t *testing.T) {
	var (
		// Addon envinronment and registration
		managedClusterAddOn *addonapiv1alpha1.ManagedClusterAddOn

		// Addon configuration
		fc *nfv1beta2.FlowCollector

		clusterName = "cluster-1"
	)

	// Register the addon for the managed cluster
	managedClusterAddOn = addontesting.NewAddon("test", "cluster-1")
	managedClusterAddOn.Status.ConfigReferences = []addonapiv1alpha1.ConfigReference{
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "logging.openshift.io",
				Resource: "clusterlogforwarders",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: "open-cluster-management",
				Name:      "instance",
			},
		},
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "",
				Resource: "secrets",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: clusterName,
				Name:      fmt.Sprintf("%s-app-logs", clusterName),
			},
		},
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "",
				Resource: "secrets",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: clusterName,
				Name:      fmt.Sprintf("%s-cluster-logs", clusterName),
			},
		},
	}

	// Setup configuration resources: ClusterLogForwarder, AddOnDeploymentConfig, Secrets, ConfigMaps
	fc = &nfv1beta2.FlowCollector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "instance",
			Namespace: "open-cluster-management",
		},
		Spec: nfv1beta2.FlowCollectorSpec{},
	}

	// Setup the fake k8s client
	resources := Options{
		FlowCollector: fc,
	}
	fcSpec, err := buildFlowCollectorSpec(resources)
	require.NoError(t, err)
	require.NotNil(t, fcSpec)
}
