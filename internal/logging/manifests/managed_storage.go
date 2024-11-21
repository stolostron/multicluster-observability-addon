package manifests

import (
	"encoding/json"
	"fmt"

	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
)

const mcoaAdmin = "mcoa-logs-admin"

func buildManagedLokistackSpec(opts Options) (lokiv1.LokiStackSpec, error) {
	// Tenants definition
	tenantsAuthentication := []lokiv1.AuthenticationSpec{}
	for _, tenant := range opts.Managed.Storage.Tenants {
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
					CA: "mcoa-managed-storage-tls",
				},
			},
		}
		tenantsAuthentication = append(tenantsAuthentication, tenantAuth)
	}

	// Tenants Read & Write RBAC
	roles := []lokiv1.RoleSpec{}
	rolesBinding := []lokiv1.RoleBindingsSpec{}
	for _, tenant := range opts.Managed.Storage.Tenants {
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
				Kind: "Group",
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
		Tenants:     opts.Managed.Storage.Tenants,
	}
	roles = append(roles, adminRole)
	adminRoleBinding := lokiv1.RoleBindingsSpec{
		Name:  "cluster-reader",
		Roles: []string{adminRole.Name},
		Subjects: []lokiv1.Subject{{
			Kind: "Group",
			Name: mcoaAdmin,
		}},
	}
	rolesBinding = append(rolesBinding, adminRoleBinding)

	return lokiv1.LokiStackSpec{
		// TODO (JoaoBraveCoding): This should be dynamic
		Size: lokiv1.SizeOneXDemo,
		// TODO (JoaoBraveCoding): This should be be user defined
		StorageClassName: "gp3-csi",
		// TODO (JoaoBraveCoding): This should be be user defined
		Storage: lokiv1.ObjectStorageSpec{
			Secret: lokiv1.ObjectStorageSecretSpec{
				Type: "s3",
				Name: "mcoa-logging-managed-storage",
			},
			Schemas: []lokiv1.ObjectStorageSchema{
				{
					Version:       lokiv1.ObjectStorageSchemaV13,
					EffectiveDate: "2024-11-18",
				},
			},
		},
		Tenants: &lokiv1.TenantsSpec{
			Mode:           lokiv1.Static,
			Authentication: tenantsAuthentication,
			Authorization: &lokiv1.AuthorizationSpec{
				Roles:        roles,
				RoleBindings: rolesBinding,
			},
		},
	}, nil
}

func buildManagedStorageSecrets(resources Options) ([]ResourceValue, error) {
	secretsValue := []ResourceValue{}

	dataJSON, err := json.Marshal(resources.Managed.Storage.ObjStorageSecret.Data)
	if err != nil {
		return secretsValue, err
	}

	rv := ResourceValue{
		Name: resources.Managed.Storage.ObjStorageSecret.Name,
		Data: string(dataJSON),
	}
	secretsValue = append(secretsValue, rv)

	dataJSON, err = json.Marshal(resources.Managed.Storage.MTLSSecret.Data)
	if err != nil {
		return secretsValue, err
	}

	rv = ResourceValue{
		Name: resources.Managed.Storage.MTLSSecret.Name,
		Data: string(dataJSON),
	}
	secretsValue = append(secretsValue, rv)

	return secretsValue, nil
}
