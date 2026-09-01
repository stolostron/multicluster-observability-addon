package common_test

import (
	"testing"

	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestIsCRDEstablished(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, apiextensionsv1.AddToScheme(scheme))

	const crdName = "lokistacks.loki.grafana.com"

	tests := []struct {
		name    string
		objects []runtime.Object
		want    bool
	}{
		{
			name: "CRD does not exist",
			want: false,
		},
		{
			name: "CRD exists but not yet Established",
			objects: []runtime.Object{
				&apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{Name: crdName},
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
							{Type: apiextensionsv1.NamesAccepted, Status: apiextensionsv1.ConditionTrue},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "CRD exists and Established",
			objects: []runtime.Object{
				&apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{Name: crdName},
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
							{Type: apiextensionsv1.NamesAccepted, Status: apiextensionsv1.ConditionTrue},
							{Type: apiextensionsv1.Established, Status: apiextensionsv1.ConditionTrue},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "CRD exists but Established is False",
			objects: []runtime.Object{
				&apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{Name: crdName},
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
							{Type: apiextensionsv1.Established, Status: apiextensionsv1.ConditionFalse},
						},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.objects...).Build()

			got, err := common.IsCRDEstablished(t.Context(), fakeClient, crdName)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
