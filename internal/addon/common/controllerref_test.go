package common

import (
	"context"
	"fmt"
	"testing"

	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	_ = corev1.AddToScheme(scheme.Scheme)
	_ = addonapiv1alpha1.AddToScheme(scheme.Scheme)
)

func TestGetOwnedResource(t *testing.T) {
	const (
		testGroup     = ""
		testResource  = "configmaps"
		cmaoName      = addon.Name
		testName      = "test-resource"
		testNamespace = "test-namespace"
	)

	createManagedClusterAddOn := func(name, namespace string, configRefs []addonapiv1alpha1.ConfigReference) *addonapiv1alpha1.ManagedClusterAddOn {
		return &addonapiv1alpha1.ManagedClusterAddOn{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Status: addonapiv1alpha1.ManagedClusterAddOnStatus{
				ConfigReferences: configRefs,
			},
		}
	}

	createConfigMap := func(name, namespace string, ownedByCMAO bool) *corev1.ConfigMap {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Data: map[string]string{
				"test-key": "test-value",
			},
		}

		if ownedByCMAO {
			cmao := &addonapiv1alpha1.ClusterManagementAddOn{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ClusterManagementAddOn",
					APIVersion: addonapiv1alpha1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: cmaoName,
				},
			}
			err := controllerutil.SetOwnerReference(cmao, cm, scheme.Scheme)
			if err != nil {
				t.Fatalf("failed to set owner reference: %v", err)
			}
		}

		return cm
	}

	// Test cases
	testCases := []struct {
		name          string
		mcAddon       *addonapiv1alpha1.ManagedClusterAddOn
		objects       []client.Object
		expectedError error
	}{
		{
			name: "no config references for group/resource",
			mcAddon: createManagedClusterAddOn("addon1", "cluster1", []addonapiv1alpha1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
						Group:    "another.group",
						Resource: "otherresources",
					},
					ConfigReferent: addonapiv1alpha1.ConfigReferent{
						Name:      "other-config",
						Namespace: testNamespace,
					},
				},
			}),
			objects:       []client.Object{},
			expectedError: fmt.Errorf("%w: %s/%s", errMissingResourceRefs, testGroup, testResource),
		},
		{
			name: "referenced resource not found",
			mcAddon: createManagedClusterAddOn("addon1", "cluster1", []addonapiv1alpha1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
						Group:    testGroup,
						Resource: testResource,
					},
					ConfigReferent: addonapiv1alpha1.ConfigReferent{
						Name:      testName,
						Namespace: testNamespace,
					},
				},
			}),
			objects:       []client.Object{},
			expectedError: fmt.Errorf("%w: %s/%s %s/%s", errMissingResource, testGroup, testResource, testNamespace, testName),
		},
		{
			name: "no resource with owner reference",
			mcAddon: createManagedClusterAddOn("addon1", "cluster1", []addonapiv1alpha1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
						Group:    testGroup,
						Resource: testResource,
					},
					ConfigReferent: addonapiv1alpha1.ConfigReferent{
						Name:      testName,
						Namespace: testNamespace,
					},
				},
			}),
			objects:       []client.Object{createConfigMap(testName, testNamespace, false)},
			expectedError: fmt.Errorf("%w: group=%s, resource=%s", errMissingOwnerRef, testGroup, testResource),
		},
		{
			name: "success - resource found with correct owner reference",
			mcAddon: createManagedClusterAddOn("addon1", "cluster1", []addonapiv1alpha1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
						Group:    testGroup,
						Resource: testResource,
					},
					ConfigReferent: addonapiv1alpha1.ConfigReferent{
						Name:      testName,
						Namespace: testNamespace,
					},
				},
			}),
			objects: []client.Object{createConfigMap(testName, testNamespace, true)},
		},
		{
			name: "success - multiple references but only one with correct owner",
			mcAddon: createManagedClusterAddOn("addon1", "cluster1", []addonapiv1alpha1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
						Group:    testGroup,
						Resource: testResource,
					},
					ConfigReferent: addonapiv1alpha1.ConfigReferent{
						Name:      "no-owner-cm",
						Namespace: testNamespace,
					},
				},
				{
					ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
						Group:    testGroup,
						Resource: testResource,
					},
					ConfigReferent: addonapiv1alpha1.ConfigReferent{
						Name:      testName,
						Namespace: testNamespace,
					},
				},
			}),
			objects: []client.Object{
				createConfigMap("no-owner-cm", testNamespace, false),
				createConfigMap(testName, testNamespace, true),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(tc.objects...).
				Build()

			// Call function
			resultCM, err := GetResourceWithOwnerRef(context.TODO(), client, tc.mcAddon, testGroup, testResource, &corev1.ConfigMap{})

			// Validate error
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError, err)
				return
			}

			// Validate success case
			require.NoError(t, err)
			require.NotNil(t, resultCM)
			assert.Equal(t, testName, resultCM.Name)
			assert.Equal(t, testNamespace, resultCM.Namespace)
			assert.Equal(t, "test-value", resultCM.Data["test-key"])
		})
	}
}
