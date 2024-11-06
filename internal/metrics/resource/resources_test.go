package resource

import (
	"context"
	"testing"

	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/stretchr/testify/assert"
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
	_ = addonapiv1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = prometheusalpha1.AddToScheme(scheme)

	cmAddon := &addonapiv1alpha1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addon.Name,
			Namespace: "test-ns",
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
			namespace: "test-ns",
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
			namespace: "test-ns",
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
			namespace:     "test-ns",
			existingObjs:  []client.Object{}, // No existing objects
			expectedError: &errors.StatusError{},
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
			err := DeployDefaultResourcesOnce(context.Background(), fakeClient, tc.namespace)

			// Verify error matches expected
			if tc.expectedError != nil {
				assert.Error(t, err)
				assert.IsType(t, tc.expectedError, err)
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

func TestCreateOrUpdateResource(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))

	defaultOwner := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "controller-pod",
			Namespace: "default",
			UID:       "test-uid",
		},
	}

	tests := []struct {
		name          string
		existingObjs  []client.Object
		newResource   *corev1.ConfigMap
		owner         *corev1.Pod
		expectedError error
		validate      func(*testing.T, client.Client, *corev1.ConfigMap)
	}{
		{
			name: "Successfully create new resource",
			newResource: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
				Data: map[string]string{"key": "value"},
			},
			owner:         defaultOwner,
			expectedError: nil,
			validate: func(t *testing.T, c client.Client, cm *corev1.ConfigMap) {
				var created corev1.ConfigMap
				err := c.Get(context.Background(), types.NamespacedName{
					Name:      cm.Name,
					Namespace: cm.Namespace,
				}, &created)

				assert.NoError(t, err)
				assert.Equal(t, "value", created.Data["key"])
				assert.Len(t, created.OwnerReferences, 1)
				assert.Equal(t, string(defaultOwner.ObjectMeta.UID), string(created.OwnerReferences[0].UID))
			},
		},
		{
			name: "Update existing resource with correct owner",
			existingObjs: []client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cm",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								UID: "test-uid",
							},
						},
					},
					Data: map[string]string{"key": "value"},
				},
			},
			newResource: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
				Data: map[string]string{"key": "new-value"},
			},
			owner:         defaultOwner,
			expectedError: nil,
			validate: func(t *testing.T, c client.Client, cm *corev1.ConfigMap) {
				var updated corev1.ConfigMap
				err := c.Get(context.Background(), types.NamespacedName{
					Name:      cm.Name,
					Namespace: cm.Namespace,
				}, &updated)

				assert.NoError(t, err)
				assert.Equal(t, "new-value", updated.Data["key"])
				assert.Len(t, updated.OwnerReferences, 1)
			},
		},
		{
			name: "Fail to update - not owner",
			existingObjs: []client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cm",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								UID: "different-uid",
							},
						},
					},
				},
			},
			newResource: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
			},
			owner:         defaultOwner,
			expectedError: ErrNotOwner,
			validate: func(t *testing.T, c client.Client, cm *corev1.ConfigMap) {
				var existing corev1.ConfigMap
				err := c.Get(context.Background(), types.NamespacedName{
					Name:      cm.Name,
					Namespace: cm.Namespace,
				}, &existing)

				assert.NoError(t, err)
				assert.Equal(t, "different-uid", string(existing.OwnerReferences[0].UID))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new fake client for each test
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tc.existingObjs...).
				Build()

			err := CreateOrUpdateResource(context.Background(), fakeClient, tc.newResource, tc.owner)

			if tc.expectedError != nil {
				assert.Equal(t, tc.expectedError, err)
			} else {
				assert.NoError(t, err)
			}

			if tc.validate != nil {
				tc.validate(t, fakeClient, tc.newResource)
			}
		})
	}
}
