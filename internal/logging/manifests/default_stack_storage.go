package manifests

import (
	"encoding/json"
	"fmt"

	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const mcoaAdmin = "mcoa-logs-admin"

func buildManagedLokistackSpec(opts Options) (lokiv1.LokiStackSpec, error) {
	// Tenants definition
	tenantsAuthentication := []lokiv1.AuthenticationSpec{}
	for _, tenant := range opts.DefaultStack.Storage.Tenants {
		tenantAuth := lokiv1.AuthenticationSpec{
			TenantName: tenant,
			TenantID:   tenant,
			MTLS: &lokiv1.MTLSSpec{
				CA: &lokiv1.CASpec{
					// Since all tenants share the same CA, we can use the same CA for
					// all of them
					CAKey: "ca.crt",
					// TODO (JoaoBraveCoding): This for now this will have to be manually
					// created, since LokiStack doesn't allow for secrets to be referenced
					CA: DefaultStorageMTLSSecretName,
				},
			},
		}
		tenantsAuthentication = append(tenantsAuthentication, tenantAuth)
	}

	// Tenants Read & Write RBAC
	roles := []lokiv1.RoleSpec{}
	rolesBinding := []lokiv1.RoleBindingsSpec{}
	for _, tenant := range opts.DefaultStack.Storage.Tenants {
		role := lokiv1.RoleSpec{
			Name:        fmt.Sprintf("%s-logs", tenant),
			Resources:   []string{"logs"},
			Permissions: []lokiv1.PermissionType{"read", "write"},
			Tenants:     []string{tenant},
		}
		roles = append(roles, role)

		roleBinding := lokiv1.RoleBindingsSpec{
			Name:  fmt.Sprintf("%s-logs", tenant),
			Roles: []string{role.Name},
			Subjects: []lokiv1.Subject{{
				Kind: "group",
				Name: tenant,
			}},
		}
		rolesBinding = append(rolesBinding, roleBinding)
	}
	// Admin Read RBAC
	adminRole := lokiv1.RoleSpec{
		Name:        "cluster-reader",
		Resources:   []string{"logs"},
		Permissions: []lokiv1.PermissionType{"read"},
		Tenants:     opts.DefaultStack.Storage.Tenants,
	}
	roles = append(roles, adminRole)
	adminRoleBinding := lokiv1.RoleBindingsSpec{
		Name:  "cluster-reader",
		Roles: []string{adminRole.Name},
		Subjects: []lokiv1.Subject{{
			Kind: "group",
			Name: mcoaAdmin,
		}},
	}
	rolesBinding = append(rolesBinding, adminRoleBinding)

	tenants := &lokiv1.TenantsSpec{
		Mode:           lokiv1.Static,
		Authentication: tenantsAuthentication,
		Authorization: &lokiv1.AuthorizationSpec{
			Roles:        roles,
			RoleBindings: rolesBinding,
			OPA:          nil,
		},
	}

	lsSpec := opts.DefaultStack.Storage.LokiStack.Spec
	lsSpec.ManagementState = lokiv1.ManagementStateManaged
	lsSpec.Tenants = tenants
	return lsSpec, nil
}

func buildManagedStorageSecrets(resources Options) ([]ResourceValue, error) {
	secretsValue := []ResourceValue{}

	dataJSON, err := json.Marshal(resources.DefaultStack.Storage.ObjStorageSecret.Data)
	if err != nil {
		return secretsValue, err
	}

	rv := ResourceValue{
		Name: resources.DefaultStack.Storage.ObjStorageSecret.Name,
		Data: string(dataJSON),
	}
	secretsValue = append(secretsValue, rv)

	dataJSON, err = json.Marshal(resources.DefaultStack.Storage.MTLSSecret.Data)
	if err != nil {
		return secretsValue, err
	}

	rv = ResourceValue{
		Name: resources.DefaultStack.Storage.MTLSSecret.Name,
		Data: string(dataJSON),
	}
	secretsValue = append(secretsValue, rv)

	return secretsValue, nil
}

func BuildSSALokiStack(opts Options, lsName string) (*lokiv1.LokiStack, error) {
	existingLS := opts.DefaultStack.Storage.LokiStack

	lokistackSpec, err := buildManagedLokistackSpec(opts)
	if err != nil {
		return nil, err
	}
	lokistackSpec.ManagementState = lokiv1.ManagementStateUnmanaged

	// SSA requires us to provide all the required fields, so if the resource is not yet created
	// we default otherwise we use the existing resource, since we are using SSA we only need to
	// copy the required fields
	if existingLS.Name == "" {
		lokistackSpec.Size = lokiv1.SizeOneXDemo
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
		lokistackSpec.Size = existingLS.Spec.Size
		lokistackSpec.StorageClassName = existingLS.Spec.StorageClassName
		lokistackSpec.Storage = existingLS.Spec.Storage
	}
	// TODO (JoaoBraveCoding): This is a hack for us not being able to create LS without OPA
	lokistackSpec.Tenants.Authorization.OPA = &lokiv1.OPASpec{}

	return &lokiv1.LokiStack{
		TypeMeta: metav1.TypeMeta{
			Kind:       "LokiStack",
			APIVersion: lokiv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      lsName,
			Namespace: addon.InstallNamespace,
		},
		Spec: lokistackSpec,
	}, nil
}
