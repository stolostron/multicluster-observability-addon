package main

import (
	"testing"
	"time"

	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

func TestGetCacheOptions_Structure(t *testing.T) {
	opts := getCacheOptions()

	require.NotNil(t, opts.DefaultTransform, "DefaultTransform must be configured")
	require.NotNil(t, opts.ByObject, "ByObject must be configured")
	assert.Len(t, opts.ByObject, 2, "ByObject should contain exactly 2 object types")

	var aodcByObj *cache.ByObject
	var cmaoByObj *cache.ByObject

	for obj, cfg := range opts.ByObject {
		switch obj.(type) {
		case *addonv1beta1.AddOnDeploymentConfig:
			cfgCopy := cfg
			aodcByObj = &cfgCopy
		case *addonv1beta1.ClusterManagementAddOn:
			cfgCopy := cfg
			cmaoByObj = &cfgCopy
		}
	}

	// Verify AddOnDeploymentConfig scoping
	require.NotNil(t, aodcByObj, "ByObject must contain AddOnDeploymentConfig")
	require.NotNil(t, aodcByObj.Namespaces, "AddOnDeploymentConfig should be namespace-scoped")
	assert.Contains(t, aodcByObj.Namespaces, addoncfg.InstallNamespace,
		"AddOnDeploymentConfig cache should be restricted to %s", addoncfg.InstallNamespace)
	assert.Len(t, aodcByObj.Namespaces, 1,
		"AddOnDeploymentConfig cache should only watch a single install namespace")

	// Verify ClusterManagementAddOn scoping
	require.NotNil(t, cmaoByObj, "ByObject must contain ClusterManagementAddOn")
	require.NotNil(t, cmaoByObj.Field, "ClusterManagementAddOn should have a field selector")

	// Field selector must match addon name and reject any other addon
	assert.True(t, cmaoByObj.Field.Matches(fields.Set{"metadata.name": addoncfg.Name}),
		"Field selector should match addon name %s", addoncfg.Name)
	assert.False(t, cmaoByObj.Field.Matches(fields.Set{"metadata.name": "other-addon"}),
		"Field selector should not match different addon names")
	assert.Empty(t, cmaoByObj.Namespaces,
		"ClusterManagementAddOn is cluster-scoped and must not set Namespaces")
}

func TestTransformStripManagedFields(t *testing.T) {
	opts := getCacheOptions()
	require.NotNil(t, opts.DefaultTransform)

	testObj := &addonv1beta1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addoncfg.Name,
			Namespace: addoncfg.InstallNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/name": "mcoa",
			},
			Annotations: map[string]string{
				"test.annotation": "true",
			},
			ManagedFields: []metav1.ManagedFieldsEntry{
				{
					Manager:    "test-manager",
					Operation:  metav1.ManagedFieldsOperationApply,
					APIVersion: "addon.open-cluster-management.io/v1beta1",
					Time:       &metav1.Time{Time: time.Now()},
					FieldsType: "FieldsV1",
				},
				{
					Manager:    "other-controller",
					Operation:  metav1.ManagedFieldsOperationUpdate,
					APIVersion: "addon.open-cluster-management.io/v1beta1",
				},
			},
		},
		Spec: addonv1beta1.AddOnDeploymentConfigSpec{
			AgentInstallNamespace: "open-cluster-management-agent-addon",
		},
	}

	transformed, err := opts.DefaultTransform(testObj)
	require.NoError(t, err)

	resultObj, ok := transformed.(*addonv1beta1.AddOnDeploymentConfig)
	require.True(t, ok, "Transformed object should retain original type")

	// Assert managedFields was stripped
	assert.Empty(t, resultObj.GetManagedFields(), "ManagedFields must be stripped to reduce cache memory")

	// Assert all other metadata and spec are preserved
	assert.Equal(t, addoncfg.Name, resultObj.GetName())
	assert.Equal(t, addoncfg.InstallNamespace, resultObj.GetNamespace())
	assert.Equal(t, map[string]string{"app.kubernetes.io/name": "mcoa"}, resultObj.GetLabels())
	assert.Equal(t, map[string]string{"test.annotation": "true"}, resultObj.GetAnnotations())
	assert.Equal(t, "open-cluster-management-agent-addon", resultObj.Spec.AgentInstallNamespace)
}
