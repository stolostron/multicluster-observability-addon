package manifests

import (
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const managedCollectionServiceAccount = "mcoa-logging-managed-collector"

func buildManagedCLFSpec(opts Options) (loggingv1.ClusterLogForwarderSpec, error) {
	sa := loggingv1.ServiceAccount{
		Name: managedCollectionServiceAccount,
	}
	outputs := []loggingv1.OutputSpec{
		{
			Name: "hub-lokistack",
			Type: loggingv1.OutputTypeOTLP,
			OTLP: &loggingv1.OTLP{
				URL: opts.DefaultStack.LokiURL,
			},
			TLS: &loggingv1.OutputTLSSpec{
				InsecureSkipVerify: true,
				TLSSpec: loggingv1.TLSSpec{
					CA: &loggingv1.ValueReference{
						Key:        "ca.crt",
						SecretName: DefaultCollectionMTLSSecretName,
					},
					Certificate: &loggingv1.ValueReference{
						Key:        corev1.TLSCertKey,
						SecretName: DefaultCollectionMTLSSecretName,
					},
					Key: &loggingv1.SecretReference{
						Key:        corev1.TLSPrivateKeyKey,
						SecretName: DefaultCollectionMTLSSecretName,
					},
				},
			},
		},
	}
	pipelines := []loggingv1.PipelineSpec{
		{
			Name:       "infra-hub-lokistack",
			InputRefs:  []string{"infrastructure"},
			OutputRefs: []string{"hub-lokistack"},
		},
	}

	clfSpec := opts.DefaultStack.Collection.ClusterLogForwarder.Spec
	clfSpec.ManagementState = loggingv1.ManagementStateManaged
	clfSpec.ServiceAccount = sa
	clfSpec.Outputs = outputs
	clfSpec.Pipelines = pipelines

	return clfSpec, nil
}

// BuildSSAClusterLogForwarder builds a ClusterLogForwarder object for the SSA.
// A key concept of this function is that uses the same function buildManagedCLFSpec to build
// the ClusterLogForwarder spec as buildManagedCollectionValues does. This ensure that the version users configure (this template) ends
// up being the same as the version MCOA generate but with the MCOA specific values.
func BuildSSAClusterLogForwarder(opts Options, clfName, placementNamespace, placementName string) (*loggingv1.ClusterLogForwarder, error) {
	clfSpec, err := buildManagedCLFSpec(opts)
	if err != nil {
		return nil, err
	}
	// We set to unmanaged to avoid the ClusterLogForwarder being reconciled by the operator as this is only a template.
	clfSpec.ManagementState = loggingv1.ManagementStateUnmanaged

	return &loggingv1.ClusterLogForwarder{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterLogForwarder",
			APIVersion: loggingv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clfName,
			Namespace: addoncfg.InstallNamespace,
			Labels: map[string]string{
				addoncfg.PlacementRefNameLabelKey:      placementName,
				addoncfg.PlacementRefNamespaceLabelKey: placementNamespace,
			},
			Annotations: map[string]string{
				addoncfg.PlacementAnnotationKey: placementNamespace + "/" + placementName,
			},
		},
		Spec: clfSpec,
	}, nil
}
