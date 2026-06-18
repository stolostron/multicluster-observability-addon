package manifests

import (
	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func buildManagedLokistackSpec(opts Options) (lokiv1.LokiStackSpec, error) {
	tenants := &lokiv1.TenantsSpec{
		Mode: lokiv1.Passthrough,
		Passthrough: &lokiv1.PassthroughTenantSpec{
			CA: &lokiv1.ValueReference{
				SecretName: DefaultStorageMTLSSecretName, // TODO(JoaoBraveCoding): Needs to be CA used by OBS-API
				Key:        "ca.crt",
			},
			// TODO(JoaoBraveCoding): For now we're gonna put everything in the infrastructure tenant, obs/api should
			// detect the tenant based on the cert and write to Loki using X-Scope-OrgID header.
			DefaultTenant: "infrastructure",
		},
	}

	lsSpec := opts.DefaultStack.Storage.LokiStack.Spec
	lsSpec.ManagementState = lokiv1.ManagementStateManaged
	lsSpec.Tenants = tenants
	return lsSpec, nil
}

func BuildSSALokiStack(opts Options, lsName, placementNamespace, placementName string) (*lokiv1.LokiStack, error) {
	existingLS := opts.DefaultStack.Storage.LokiStack

	lokistackSpec, err := buildManagedLokistackSpec(opts)
	if err != nil {
		return nil, err
	}
	lokistackSpec.ManagementState = lokiv1.ManagementStateUnmanaged

	if existingLS.Name == "" {
		// Default fields for SSA
		lokistackSpec.Size = lokiv1.SizeOneXPico
		// Default value can't be changed due to https://redhat.atlassian.net/browse/LOG-7390
		lokistackSpec.StorageClassName = "gp3-csi"
		lokistackSpec.Storage = lokiv1.ObjectStorageSpec{
			Secret: lokiv1.ObjectStorageSecretSpec{
				Type: "s3",
				Name: DefaultStorageObjStorageSecretName,
			},
			Schemas: []lokiv1.ObjectStorageSchema{
				{
					Version:       lokiv1.ObjectStorageSchemaV13,
					EffectiveDate: "2024-11-18",
				},
			},
		}
	} else {
		// Fields allowed to be managed by the user
		lokistackSpec.Size = existingLS.Spec.Size
		lokistackSpec.StorageClassName = existingLS.Spec.StorageClassName
		lokistackSpec.Storage = existingLS.Spec.Storage
	}

	return &lokiv1.LokiStack{
		TypeMeta: metav1.TypeMeta{
			Kind:       "LokiStack",
			APIVersion: lokiv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      lsName,
			Namespace: addoncfg.InstallNamespace,
			Labels: map[string]string{
				addoncfg.PlacementRefNameLabelKey:      placementName,
				addoncfg.PlacementRefNamespaceLabelKey: placementNamespace,
			},
		},
		Spec: lokistackSpec,
	}, nil
}
