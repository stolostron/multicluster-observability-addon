package common

import (
	"context"
	"testing"

	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestCleanOrphanResources(t *testing.T) {
	// Setup common test variables
	testNamespace := addon.InstallNamespace
	placementName := "test-placement"
	placementNs := testNamespace
	cmaoName := "test-cmao"

	// Create a new scheme and add the types we need
	scheme := runtime.NewScheme()
	require.NoError(t, addonapiv1alpha1.AddToScheme(scheme))
	require.NoError(t, prometheusalpha1.AddToScheme(scheme))

	tests := []struct {
		name               string
		cmao               *addonapiv1alpha1.ClusterManagementAddOn
		cmaoOwnedResources []*prometheusalpha1.PrometheusAgent
		extraResources     []client.Object
		expectDeleted      map[string]bool
	}{
		{
			name: "No placement exists but resources exist not owned by CMAO",
			cmao: &addonapiv1alpha1.ClusterManagementAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name: cmaoName,
				},
				Spec: addonapiv1alpha1.ClusterManagementAddOnSpec{
					// No placements
				},
			},
			extraResources: []client.Object{
				&prometheusalpha1.PrometheusAgent{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-1",
						Namespace: testNamespace,
						Labels: map[string]string{
							addon.PlacementRefNameLabelKey:      placementName,
							addon.PlacementRefNamespaceLabelKey: placementNs,
						},
						// Not owned by CMAO
					},
				},
			},
			expectDeleted: map[string]bool{
				"agent-1": false, // Resource not owned by CMAO, shouldn't be deleted
			},
		},
		{
			name: "No placement but exist resources owned by CMAO",
			cmao: &addonapiv1alpha1.ClusterManagementAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name: cmaoName,
				},
				Spec: addonapiv1alpha1.ClusterManagementAddOnSpec{
					// No placements
				},
			},
			cmaoOwnedResources: []*prometheusalpha1.PrometheusAgent{
				// Will be deleted because it's owned by CMAO but no placement
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-2",
						Namespace: testNamespace,
						Labels: map[string]string{
							addon.PlacementRefNameLabelKey:      placementName,
							addon.PlacementRefNamespaceLabelKey: placementNs,
						},
						// Will be set as owned by CMAO in the test
					},
				},
			},
			expectDeleted: map[string]bool{
				"agent-2": true, // Should be deleted as it's owned by CMAO and no placement exists
			},
		},
		{
			name: "Placement exists but also exists some resources not owned by CMAO",
			cmao: &addonapiv1alpha1.ClusterManagementAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name: cmaoName,
				},
				Spec: addonapiv1alpha1.ClusterManagementAddOnSpec{
					InstallStrategy: addonapiv1alpha1.InstallStrategy{
						Placements: []addonapiv1alpha1.PlacementStrategy{
							{
								PlacementRef: addonapiv1alpha1.PlacementRef{
									Name:      placementName,
									Namespace: placementNs,
								},
							},
						},
					},
				},
			},
			cmaoOwnedResources: []*prometheusalpha1.PrometheusAgent{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-3",
						Namespace: testNamespace,
						Labels: map[string]string{
							addon.PlacementRefNameLabelKey:      placementName,
							addon.PlacementRefNamespaceLabelKey: placementNs,
						},
						// Not owned by CMAO
					},
				},
			},
			extraResources: []client.Object{
				&prometheusalpha1.PrometheusAgent{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-4",
						Namespace: testNamespace,
						Labels: map[string]string{
							addon.PlacementRefNameLabelKey:      placementName,
							addon.PlacementRefNamespaceLabelKey: placementNs,
						},
						// Not owned by CMAO
					},
				},
			},
			expectDeleted: map[string]bool{
				"agent-3": false, // Not owned by CMAO, shouldn't be deleted
				"agent-4": false, // Not owned by CMAO, shouldn't be deleted
			},
		},
		{
			name: "Placement exists but also exists some resources owned by CMAO",
			cmao: &addonapiv1alpha1.ClusterManagementAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name: cmaoName,
				},
				Spec: addonapiv1alpha1.ClusterManagementAddOnSpec{
					InstallStrategy: addonapiv1alpha1.InstallStrategy{
						Placements: []addonapiv1alpha1.PlacementStrategy{
							{
								PlacementRef: addonapiv1alpha1.PlacementRef{
									Name:      placementName,
									Namespace: placementNs,
								},
							},
						},
					},
				},
			},
			cmaoOwnedResources: []*prometheusalpha1.PrometheusAgent{
				// Will not be deleted because it matches a placement
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-5",
						Namespace: testNamespace,
						Labels: map[string]string{
							addon.PlacementRefNameLabelKey:      placementName,
							addon.PlacementRefNamespaceLabelKey: placementNs,
						},
						// Will be set as owned by CMAO in the test
					},
				},
				// Will be deleted because it doesn't match any placement
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-6",
						Namespace: testNamespace,
						Labels: map[string]string{
							addon.PlacementRefNameLabelKey:      "other-placement",
							addon.PlacementRefNamespaceLabelKey: placementNs,
						},
						// Will be set as owned by CMAO in the test
					},
				},
			},
			expectDeleted: map[string]bool{
				"agent-5": false, // Matches placement, shouldn't be deleted
				"agent-6": true,  // Owned by CMAO but doesn't match any placement, should be deleted
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a fresh fake client for each test case
			existingResources := []client.Object{tc.cmao}
			existingResources = append(existingResources, tc.extraResources...)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingResources...).Build()

			// Set up existing resources with ownership as needed
			for _, agent := range tc.cmaoOwnedResources {
				err := controllerutil.SetControllerReference(tc.cmao, agent, scheme)
				require.NoError(t, err, "Failed to set controller reference")

				// Create the resource in the fake client
				err = fakeClient.Create(context.Background(), agent)
				require.NoError(t, err, "Failed to create resource")
			}

			// Run the function under test
			err := DeleteOrphanResources(context.Background(), klog.Background(), fakeClient, tc.cmao, &prometheusalpha1.PrometheusAgentList{})
			require.NoError(t, err, "CleanOrphanResources should not return an error")

			// Check that resources were deleted or not as expected
			for name, shouldBeDeleted := range tc.expectDeleted {
				agent := &prometheusalpha1.PrometheusAgent{}
				err := fakeClient.Get(context.Background(), types.NamespacedName{
					Name:      name,
					Namespace: testNamespace,
				}, agent)

				if shouldBeDeleted {
					assert.Error(t, err, "Resource %s should have been deleted", name)
				} else {
					assert.NoError(t, err, "Resource %s should not have been deleted", name)
				}
			}
		})
	}
}
