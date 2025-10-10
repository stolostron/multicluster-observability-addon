package common

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetConfigMaps(t *testing.T) {
	var (
		defaultNamespace = "open-cluster-management"
		clusterConfigMap = &corev1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      "foo",
				Namespace: "cluster",
			},
			Data: map[string]string{
				"foo": "bar",
			},
		}
		defaultConfigMap = &corev1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      "foo",
				Namespace: defaultNamespace,
			},
			Data: map[string]string{
				"bar": "baz",
			},
		}
	)

	for _, tc := range []struct {
		name                    string
		configMapName           string
		configResourceNamespace string
		mcAddonNamespace        string
		expectedError           bool
		expectedConfigMap       *corev1.ConfigMap
	}{
		{
			name:                    "configMap in cluster namespace",
			mcAddonNamespace:        "cluster",
			configResourceNamespace: defaultNamespace,
			configMapName:           "foo",
			expectedConfigMap:       clusterConfigMap,
		},
		{
			name:                    "default configMap used",
			mcAddonNamespace:        "cluster-no-configMap",
			configResourceNamespace: defaultNamespace,
			configMapName:           "foo",
			expectedConfigMap:       defaultConfigMap,
		},
		{
			name:                    "default namespace not defined",
			configResourceNamespace: "",
			expectedError:           true,
		},
		{
			name:                    "no configMap found",
			mcAddonNamespace:        "cluster",
			configResourceNamespace: defaultNamespace,
			configMapName:           "bar",
			expectedError:           true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeKubeClient := fake.NewClientBuilder().
				WithObjects(clusterConfigMap, defaultConfigMap).
				Build()

			targetConfigMaps := []string{
				tc.configMapName,
			}
			configMaps, err := GetConfigMaps(t.Context(), fakeKubeClient, tc.configResourceNamespace, tc.mcAddonNamespace, targetConfigMaps)
			if tc.expectedError {
				require.Error(t, err)
				return
			}
			require.Len(t, configMaps, 1)
			configMap := configMaps[0]
			require.NoError(t, err)
			require.Equal(t, tc.expectedConfigMap.Name, configMap.Name)
			require.Equal(t, tc.expectedConfigMap.Namespace, configMap.Namespace)
			require.Equal(t, tc.expectedConfigMap.Data, configMap.Data)
		})
	}
}
