package authentication

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_FetchSecrets(t *testing.T) {
	sFoo := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
		Data: map[string][]byte{
			"foo": []byte("bar"),
		},
	}
	sBaz := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "baz",
			Namespace: "qux",
		},
		Data: map[string][]byte{
			"baz": []byte("qux"),
		},
	}

	fakeKubeClient := fake.NewClientBuilder().
		WithObjects(sFoo, sBaz).
		Build()

	sp := NewSecretsProvider(fakeKubeClient, "default", "cluster")
	keys := map[Target]SecretKey{
		"target-1": {Name: "foo", Namespace: "bar"},
	}
	secrets, err := sp.FetchSecrets(context.TODO(), keys)
	require.NoError(t, err)
	require.Len(t, secrets, 1)
	require.Equal(t, sFoo.Name, secrets["target-1"].Name)
	require.Equal(t, sFoo.Namespace, secrets["target-1"].Namespace)
	require.Equal(t, sFoo.Data, secrets["target-1"].Data)
}

func Test_buildAuthenticationFromAnnotations(t *testing.T) {
	for _, tc := range []struct {
		name            string
		annotations     map[string]string
		expecedError    bool
		expectedAuthMap map[Target]AuthenticationType
	}{
		{
			name: "valid annotation",
			annotations: map[string]string{
				"authentication.mcoa.openshift.io/foo": "SecretReference",
			},
			expectedAuthMap: map[Target]AuthenticationType{
				Target("foo"): SecretReference,
			},
		},
		{
			name: "invalid annotation",
			annotations: map[string]string{
				"authentication.mcoa.openshift.io/foo/bar": "SecretReference",
			},
			expecedError: true,
		},
		{
			name: "invalid no target specifed",
			annotations: map[string]string{
				"authentication.mcoa.openshift.io/": "SecretReference",
			},
			expecedError: true,
		},
		{
			name: "undefied authentication type",
			annotations: map[string]string{
				"authentication.mcoa.openshift.io/foo": "foo",
			},
			expectedAuthMap: map[Target]AuthenticationType{
				Target("foo"): AuthenticationType("foo"),
			},
		},
		{
			name: "regular annotation",
			annotations: map[string]string{
				"foo": "SecretReference",
			},
			expectedAuthMap: map[Target]AuthenticationType{},
		},
	} {
		authMap, err := buildAuthenticationFromAnnotations(tc.annotations)
		if tc.expecedError {
			require.Error(t, err)
			return
		}
		require.NoError(t, err)
		require.Equal(t, tc.expectedAuthMap, authMap)
	}
}

func Test_discoverSecretRef(t *testing.T) {
	var (
		defaultNamespace = "open-cluster-management"
		clusterSecret    = &corev1.Secret{
			ObjectMeta: v1.ObjectMeta{
				Name:      "foo",
				Namespace: "cluster",
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
		}
		defaultSecret = &corev1.Secret{
			ObjectMeta: v1.ObjectMeta{
				Name:      "foo",
				Namespace: defaultNamespace,
			},
			Data: map[string][]byte{
				"bar": []byte("baz"),
			},
		}
	)

	for _, tc := range []struct {
		name             string
		secretName       string
		defaultNamespace string
		clusterNamespace string
		expecedError     bool
		expectedSecret   *corev1.Secret
	}{
		{
			name:             "secret in cluster namespace",
			clusterNamespace: "cluster",
			defaultNamespace: defaultNamespace,
			secretName:       "foo",
			expectedSecret:   clusterSecret,
		},
		{
			name:             "default secret used",
			clusterNamespace: "cluster-no-secret",
			defaultNamespace: defaultNamespace,
			secretName:       "foo",
			expectedSecret:   defaultSecret,
		},
		{
			name:             "default namespace not defined",
			defaultNamespace: "",
			expecedError:     true,
		},
		{
			name:             "no secret found",
			clusterNamespace: "cluster",
			defaultNamespace: defaultNamespace,
			secretName:       "bar",
			expecedError:     true,
		},
	} {
		fakeKubeClient := fake.NewClientBuilder().
			WithObjects(clusterSecret, defaultSecret).
			Build()
		sp := NewSecretsProvider(fakeKubeClient, tc.defaultNamespace, tc.clusterNamespace)
		gotSecret, err := sp.discoverSecretRef(context.TODO(), tc.secretName)
		if tc.expecedError {
			require.Error(t, err)
			return
		}
		require.NoError(t, err)
		require.Equal(t, tc.expectedSecret.Name, gotSecret.Name)
		require.Equal(t, tc.expectedSecret.Namespace, gotSecret.Namespace)
		require.Equal(t, tc.expectedSecret.Data, gotSecret.Data)
	}
}
