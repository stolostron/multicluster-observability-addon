package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testClusterName = "test-cluster"
	testAddonName   = "test-addon"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	scheme := runtime.NewScheme()
	assert.NoError(t, workv1.AddToScheme(scheme))
	return scheme
}

func TestFilterFeedbackValuesByName(t *testing.T) {
	values := []workv1.FeedbackValue{
		{Name: "foo", Value: workv1.FieldValue{Type: workv1.String, String: ptr.To("bar")}},
		{Name: "baz", Value: workv1.FieldValue{Type: workv1.Boolean, Boolean: ptr.To(true)}},
		{Name: "foo", Value: workv1.FieldValue{Type: workv1.String, String: ptr.To("qux")}},
	}

	expected := []workv1.FeedbackValue{
		{Name: "foo", Value: workv1.FieldValue{Type: workv1.String, String: ptr.To("bar")}},
		{Name: "foo", Value: workv1.FieldValue{Type: workv1.String, String: ptr.To("qux")}},
	}

	filtered := FilterFeedbackValuesByName(values, "foo")
	assert.Equal(t, expected, filtered)

	filteredEmpty := FilterFeedbackValuesByName(values, "nonexistent")
	assert.Empty(t, filteredEmpty)
}

func TestListAddonManifestWorks(t *testing.T) {
	scheme := newTestScheme(t)
	works := []client.Object{
		&workv1.ManifestWork{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "addon-work-1",
				Namespace: testClusterName,
				Labels:    map[string]string{"open-cluster-management.io/addon-name": testAddonName},
			},
		},
		&workv1.ManifestWork{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "other-addon-work",
				Namespace: testClusterName,
				Labels:    map[string]string{"open-cluster-management.io/addon-name": "other-addon"},
			},
		},
		&workv1.ManifestWork{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "addon-work-2",
				Namespace: "other-cluster",
				Labels:    map[string]string{"open-cluster-management.io/addon-name": testAddonName},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(works...).Build()

	workList, err := ListAddonManifestWorks(context.TODO(), fakeClient, testClusterName, testAddonName)
	assert.NoError(t, err)
	assert.Len(t, workList.Items, 1)
	assert.Equal(t, "addon-work-1", workList.Items[0].Name)
}

func TestGetFeedbackValuesForResources(t *testing.T) {
	resID1 := workv1.ResourceIdentifier{Group: "g1", Resource: "r1", Name: "n1"}
	resID2 := workv1.ResourceIdentifier{Group: "g2", Resource: "r2", Name: "n2"}
	resID3 := workv1.ResourceIdentifier{Group: "g3", Resource: "r3", Name: "n3"} // Not in works

	scheme := newTestScheme(t)
	works := []client.Object{
		&workv1.ManifestWork{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "work-1",
				Namespace: testClusterName,
				Labels:    map[string]string{"open-cluster-management.io/addon-name": testAddonName},
			},
			Status: workv1.ManifestWorkStatus{
				ResourceStatus: workv1.ManifestResourceStatus{
					Manifests: []workv1.ManifestCondition{
						{
							ResourceMeta: workv1.ManifestResourceMeta{
								Group: "g1", Resource: "r1", Name: "n1",
							},
							StatusFeedbacks: workv1.StatusFeedbackResult{
								Values: []workv1.FeedbackValue{
									{Name: "status", Value: workv1.FieldValue{Type: workv1.String, String: ptr.To("ok")}},
								},
							},
						},
						{
							ResourceMeta: workv1.ManifestResourceMeta{
								Group: "unrelated", Resource: "unrelated", Name: "unrelated",
							},
						},
					},
				},
			},
		},
		&workv1.ManifestWork{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "work-2",
				Namespace: testClusterName,
				Labels:    map[string]string{"open-cluster-management.io/addon-name": testAddonName},
			},
			Status: workv1.ManifestWorkStatus{
				ResourceStatus: workv1.ManifestResourceStatus{
					Manifests: []workv1.ManifestCondition{
						{
							ResourceMeta: workv1.ManifestResourceMeta{
								Group: "g2", Resource: "r2", Name: "n2",
							},
							StatusFeedbacks: workv1.StatusFeedbackResult{
								Values: []workv1.FeedbackValue{
									{Name: "isReady", Value: workv1.FieldValue{Type: workv1.Boolean, Boolean: ptr.To(true)}},
								},
							},
						},
					},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(works...).Build()

	results, err := GetFeedbackValuesForResources(context.TODO(), fakeClient, testClusterName, testAddonName, resID1, resID2, resID3)
	assert.NoError(t, err)
	assert.Len(t, results, 3)

	// Check resID1
	val1, ok1 := results[resID1]
	assert.True(t, ok1)
	assert.Len(t, val1, 1)
	assert.Equal(t, "status", val1[0].Name)

	// Check resID2
	val2, ok2 := results[resID2]
	assert.True(t, ok2)
	assert.Len(t, val2, 1)
	assert.Equal(t, "isReady", val2[0].Name)

	// Check resID3 (not found)
	val3, ok3 := results[resID3]
	assert.True(t, ok3)
	assert.Empty(t, val3)
}
