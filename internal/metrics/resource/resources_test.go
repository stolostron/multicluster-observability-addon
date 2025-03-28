package resource

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDeployDefaultResourcesOnce(t *testing.T) {
	// Create a scheme and add required types
	scheme := runtime.NewScheme()
	require.NoError(t, addonapiv1alpha1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, prometheusalpha1.AddToScheme(scheme))
	require.NoError(t, prometheusv1.AddToScheme(scheme))

	cmAddon := &addonapiv1alpha1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addon.Name,
			Namespace: addon.InstallNamespace,
		},
	}

	tests := []struct {
		name          string
		namespace     string
		existingObjs  []client.Object
		isInitialized bool
		expectedError error
		validate      func(*testing.T, client.Client, string)
	}{
		{
			name:      "Successfully deploy resources first time",
			namespace: addon.InstallNamespace,
			existingObjs: []client.Object{
				cmAddon,
			},
			expectedError: nil,
			validate: func(t *testing.T, c client.Client, ns string) {
				// Verify resources were created, with owner reference
				resources := DefaultPlaftformAgentResources(ns)
				for _, resource := range resources {
					key := types.NamespacedName{
						Name:      resource.GetName(),
						Namespace: resource.GetNamespace(),
					}

					existing := resource.DeepCopyObject().(client.Object)
					err := c.Get(context.Background(), key, existing)

					assert.NoError(t, err, "Resource should exist: %s", resource.GetName())
					assert.NotEmpty(t, existing.GetOwnerReferences(), "Resource should have owner reference")
					assert.Equal(t, addon.Name, existing.GetOwnerReferences()[0].Name)
				}

				assert.True(t, initialized)
			},
		},
		{
			name:      "Skip deployment when already initialized",
			namespace: addon.InstallNamespace,
			existingObjs: []client.Object{
				cmAddon,
			},
			isInitialized: true,
			expectedError: nil,
			validate: func(t *testing.T, c client.Client, ns string) {
				// Verify no new resources were created
				resources := DefaultPlaftformAgentResources(ns)
				for _, resource := range resources {
					key := types.NamespacedName{
						Name:      resource.GetName(),
						Namespace: resource.GetNamespace(),
					}

					existing := resource.DeepCopyObject().(client.Object)
					err := c.Get(context.Background(), key, existing)

					assert.True(t, errors.IsNotFound(err), "Resource should not exist: %s", resource.GetName())
				}
			},
		},
		{
			name:          "Fail when owner resource not found",
			namespace:     addon.InstallNamespace,
			existingObjs:  []client.Object{}, // No existing objects
			expectedError: fmt.Errorf("failed to deploy default monitoring resources: %s", "clustermanagementaddons.addon.open-cluster-management.io \"multicluster-observability-addon\" not found"),
			validate: func(t *testing.T, c client.Client, ns string) {
				assert.False(t, initialized)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Reset initialized flag before each test
			initialized = tc.isInitialized

			// Create a new fake client for each test
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tc.existingObjs...).
				Build()

			// Run the function
			err := DeployDefaultResourcesOnce(context.Background(), fakeClient, logr.Logger{}, tc.namespace)

			// Verify error matches expected
			if tc.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}

			// Run validation if provided
			if tc.validate != nil {
				tc.validate(t, fakeClient, tc.namespace)
			}
		})
	}
}
