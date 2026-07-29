package common

import (
	"context"
	"testing"

	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestCleanOrphanResources(t *testing.T) {
	// Setup common test variables
	testNamespace := addoncfg.InstallNamespace
	placementName := "test-placement"
	placementNs := testNamespace
	cmaoName := "test-cmao"

	// Create a new scheme and add the types we need
	scheme := runtime.NewScheme()
	require.NoError(t, addonapiv1beta1.Install(scheme))
	require.NoError(t, cooprometheusv1alpha1.AddToScheme(scheme))

	tests := []struct {
		name               string
		cmao               *addonapiv1beta1.ClusterManagementAddOn
		cmaoOwnedResources []*cooprometheusv1alpha1.PrometheusAgent
		extraResources     []client.Object
		expectDeleted      map[string]bool
	}{
		{
			name: "No placement exists but resources exist not owned by CMAO",
			cmao: &addonapiv1beta1.ClusterManagementAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name: cmaoName,
				},
				Spec: addonapiv1beta1.ClusterManagementAddOnSpec{
					// No placements
				},
			},
			extraResources: []client.Object{
				&cooprometheusv1alpha1.PrometheusAgent{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-1",
						Namespace: testNamespace,
						Annotations: map[string]string{
							addoncfg.PlacementAnnotationKey: placementNs + "/" + placementName,
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
			name: "user-defined resource (part-of label, not owned) is deleted when none of its placements exist",
			cmao: &addonapiv1beta1.ClusterManagementAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name: cmaoName,
				},
				Spec: addonapiv1beta1.ClusterManagementAddOnSpec{
					// No placements
				},
			},
			extraResources: []client.Object{
				&cooprometheusv1alpha1.PrometheusAgent{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user-agent-orphan",
						Namespace: testNamespace,
						Labels: map[string]string{
							addoncfg.PartOfK8sLabelKey: addoncfg.Name,
						},
						Annotations: map[string]string{
							addoncfg.PlacementAnnotationKey: placementNs + "/" + placementName,
						},
						// Not owned by CMAO, opted in via part-of label instead
					},
				},
			},
			expectDeleted: map[string]bool{
				"user-agent-orphan": true, // User-defined and none of its placements exist, should be deleted
			},
		},
		{
			name: "user-defined resource (part-of label, not owned) is kept while its placement still exists",
			cmao: &addonapiv1beta1.ClusterManagementAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name: cmaoName,
				},
				Spec: addonapiv1beta1.ClusterManagementAddOnSpec{
					InstallStrategy: addonapiv1beta1.InstallStrategy{
						Placements: []addonapiv1beta1.PlacementStrategy{
							{
								PlacementRef: addonapiv1beta1.PlacementRef{
									Name:      placementName,
									Namespace: placementNs,
								},
							},
						},
					},
				},
			},
			extraResources: []client.Object{
				&cooprometheusv1alpha1.PrometheusAgent{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user-agent-covered",
						Namespace: testNamespace,
						Labels: map[string]string{
							addoncfg.PartOfK8sLabelKey: addoncfg.Name,
						},
						Annotations: map[string]string{
							addoncfg.PlacementAnnotationKey: placementNs + "/" + placementName,
						},
						// Not owned by CMAO, opted in via part-of label instead
					},
				},
			},
			expectDeleted: map[string]bool{
				"user-agent-covered": false, // Its placement still exists, shouldn't be deleted
			},
		},
		{
			name: "resource with neither CMAO ownership nor part-of label is never touched",
			cmao: &addonapiv1beta1.ClusterManagementAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name: cmaoName,
				},
				Spec: addonapiv1beta1.ClusterManagementAddOnSpec{
					// No placements
				},
			},
			extraResources: []client.Object{
				&cooprometheusv1alpha1.PrometheusAgent{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rogue-agent",
						Namespace: testNamespace,
						Annotations: map[string]string{
							addoncfg.PlacementAnnotationKey: placementNs + "/" + placementName,
						},
						// Neither owned by CMAO nor opted in via part-of label
					},
				},
			},
			expectDeleted: map[string]bool{
				"rogue-agent": false, // Not recognized by MCOA at all, must never be touched
			},
		},
		{
			name: "No placement but exist resources owned by CMAO",
			cmao: &addonapiv1beta1.ClusterManagementAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name: cmaoName,
				},
				Spec: addonapiv1beta1.ClusterManagementAddOnSpec{
					// No placements
				},
			},
			cmaoOwnedResources: []*cooprometheusv1alpha1.PrometheusAgent{
				// Will be deleted because it's owned by CMAO but no placement
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-2",
						Namespace: testNamespace,
						Annotations: map[string]string{
							addoncfg.PlacementAnnotationKey: placementNs + "/" + placementName,
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
			cmao: &addonapiv1beta1.ClusterManagementAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name: cmaoName,
				},
				Spec: addonapiv1beta1.ClusterManagementAddOnSpec{
					InstallStrategy: addonapiv1beta1.InstallStrategy{
						Placements: []addonapiv1beta1.PlacementStrategy{
							{
								PlacementRef: addonapiv1beta1.PlacementRef{
									Name:      placementName,
									Namespace: placementNs,
								},
							},
						},
					},
				},
			},
			cmaoOwnedResources: []*cooprometheusv1alpha1.PrometheusAgent{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-3",
						Namespace: testNamespace,
						Annotations: map[string]string{
							addoncfg.PlacementAnnotationKey: placementNs + "/" + placementName,
						},
						// Not owned by CMAO
					},
				},
			},
			extraResources: []client.Object{
				&cooprometheusv1alpha1.PrometheusAgent{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-4",
						Namespace: testNamespace,
						Annotations: map[string]string{
							addoncfg.PlacementAnnotationKey: placementNs + "/" + placementName,
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
			cmao: &addonapiv1beta1.ClusterManagementAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name: cmaoName,
				},
				Spec: addonapiv1beta1.ClusterManagementAddOnSpec{
					InstallStrategy: addonapiv1beta1.InstallStrategy{
						Placements: []addonapiv1beta1.PlacementStrategy{
							{
								PlacementRef: addonapiv1beta1.PlacementRef{
									Name:      placementName,
									Namespace: placementNs,
								},
							},
						},
					},
				},
			},
			cmaoOwnedResources: []*cooprometheusv1alpha1.PrometheusAgent{
				// Will not be deleted because it matches a placement
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-5",
						Namespace: testNamespace,
						Annotations: map[string]string{
							addoncfg.PlacementAnnotationKey: placementNs + "/" + placementName,
						},
						// Will be set as owned by CMAO in the test
					},
				},
				// Will be deleted because it doesn't match any placement
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-6",
						Namespace: testNamespace,
						Annotations: map[string]string{
							addoncfg.PlacementAnnotationKey: placementNs + "/other-placement",
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
		{
			name: "resource referencing multiple placements is kept while at least one still exists",
			cmao: &addonapiv1beta1.ClusterManagementAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name: cmaoName,
				},
				Spec: addonapiv1beta1.ClusterManagementAddOnSpec{
					InstallStrategy: addonapiv1beta1.InstallStrategy{
						Placements: []addonapiv1beta1.PlacementStrategy{
							{
								PlacementRef: addonapiv1beta1.PlacementRef{
									Name:      placementName,
									Namespace: placementNs,
								},
							},
						},
					},
				},
			},
			cmaoOwnedResources: []*cooprometheusv1alpha1.PrometheusAgent{
				// Will not be deleted because one of its referenced placements still exists
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-7",
						Namespace: testNamespace,
						Annotations: map[string]string{
							addoncfg.PlacementAnnotationKey: placementNs + "/other-placement," + placementNs + "/" + placementName,
						},
						// Will be set as owned by CMAO in the test
					},
				},
			},
			expectDeleted: map[string]bool{
				"agent-7": false, // At least one referenced placement still exists, shouldn't be deleted
			},
		},
		{
			name: "resource referencing multiple placements is deleted once none of them exist",
			cmao: &addonapiv1beta1.ClusterManagementAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name: cmaoName,
				},
				Spec: addonapiv1beta1.ClusterManagementAddOnSpec{
					// No placements
				},
			},
			cmaoOwnedResources: []*cooprometheusv1alpha1.PrometheusAgent{
				// Will be deleted because none of its referenced placements exist anymore
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-8",
						Namespace: testNamespace,
						Annotations: map[string]string{
							addoncfg.PlacementAnnotationKey: placementNs + "/other-placement," + placementNs + "/" + placementName,
						},
						// Will be set as owned by CMAO in the test
					},
				},
			},
			expectDeleted: map[string]bool{
				"agent-8": true, // None of the referenced placements exist, should be deleted
			},
		},
		{
			name: "resource referencing the dummy sentinel is never deleted, even with no placements",
			cmao: &addonapiv1beta1.ClusterManagementAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name: cmaoName,
				},
				Spec: addonapiv1beta1.ClusterManagementAddOnSpec{
					// No placements
				},
			},
			cmaoOwnedResources: []*cooprometheusv1alpha1.PrometheusAgent{
				// The "dummy" sentinel never corresponds to a real placement by design, so it must
				// never be treated as orphaned (otherwise CreateDefaultAgent and DeleteOrphanResources
				// would thrash: create the dummy agent, immediately delete it, repeat).
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-dummy",
						Namespace: testNamespace,
						Annotations: map[string]string{
							addoncfg.PlacementAnnotationKey: testNamespace + "/dummy",
						},
						// Will be set as owned by CMAO in the test
					},
				},
			},
			expectDeleted: map[string]bool{
				"agent-dummy": false, // dummy is a sentinel, never orphaned
			},
		},
		{
			name: "resource with no placement-ref annotation owned by CMAO is deleted",
			cmao: &addonapiv1beta1.ClusterManagementAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name: cmaoName,
				},
				Spec: addonapiv1beta1.ClusterManagementAddOnSpec{
					InstallStrategy: addonapiv1beta1.InstallStrategy{
						Placements: []addonapiv1beta1.PlacementStrategy{
							{
								PlacementRef: addonapiv1beta1.PlacementRef{
									Name:      placementName,
									Namespace: placementNs,
								},
							},
						},
					},
				},
			},
			cmaoOwnedResources: []*cooprometheusv1alpha1.PrometheusAgent{
				// Will be deleted because it has no placement-ref annotation at all
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "agent-9",
						Namespace: testNamespace,
						// Will be set as owned by CMAO in the test
					},
				},
			},
			expectDeleted: map[string]bool{
				"agent-9": true, // No placement-ref annotation, should be deleted
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
			err := DeleteOrphanResources(context.Background(), klog.Background(), fakeClient, tc.cmao, &cooprometheusv1alpha1.PrometheusAgentList{})
			require.NoError(t, err, "CleanOrphanResources should not return an error")

			// Check that resources were deleted or not as expected
			for name, shouldBeDeleted := range tc.expectDeleted {
				agent := &cooprometheusv1alpha1.PrometheusAgent{}
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
