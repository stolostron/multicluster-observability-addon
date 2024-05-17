package authentication

import (
	"context"
	"testing"

	"github.com/rhobs/multicluster-observability-addon/internal/manifests"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	spConfig := &Config{}
	sp, err := NewSecretsProvider(fakeKubeClient, "test", "logging", spConfig)
	require.NoError(t, err)
	keys := map[Target]SecretKey{
		"target-1": {Name: "foo", Namespace: "bar"},
	}
	secrets, err := sp.FetchSecrets(context.TODO(), keys)
	require.NoError(t, err)
	require.Len(t, secrets, 1)
	require.Equal(t, sFoo, secrets["target-1"])
}

func Test_FetchSecretsAndAnnotate(t *testing.T) {
	var (
		annotation     = "foo-annotation"
		target         = "target-1"
		expAnnotations = map[string]string{
			annotation: target,
		}
	)

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

	spConfig := &Config{}
	sp, err := NewSecretsProvider(fakeKubeClient, "test", "logging", spConfig)
	require.NoError(t, err)
	keys := map[Target]SecretKey{
		"target-1": {Name: "foo", Namespace: "bar"},
	}
	secrets, err := sp.FetchSecretsAndAnnotate(context.TODO(), keys, "foo-annotation")
	require.NoError(t, err)
	require.Len(t, secrets, 1)
	require.Equal(t, sFoo.Name, secrets[0].Name)
	require.Equal(t, expAnnotations, secrets[0].Annotations)
}

func Test_InjectCA(t *testing.T) {
	var (
		key = client.ObjectKey{
			Name:      "foo",
			Namespace: "bar",
		}
		ca = "foo-ca"
	)
	sFoo := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Data: map[string][]byte{
			"foo": []byte("bar"),
		},
	}

	fakeKubeClient := fake.NewClientBuilder().
		WithObjects(sFoo).
		Build()

	spConfig := &Config{MTLSConfig: manifests.MTLSConfig{
		CAToInject: ca,
	}}
	sp, err := NewSecretsProvider(fakeKubeClient, "test", "logging", spConfig)
	require.NoError(t, err)
	targetAuth := map[Target]AuthenticationType{
		"target-1": "mTLS",
	}
	targetKeys := map[Target]SecretKey{
		"target-1": SecretKey(key),
	}
	err = sp.injectCA(context.TODO(), targetAuth, targetKeys)
	require.NoError(t, err)
	secret := &corev1.Secret{}
	err = fakeKubeClient.Get(context.TODO(), key, secret, &client.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, sFoo.Name, secret.Name)
	require.Equal(t, sFoo.Data["foo"], secret.Data["foo"])
	require.Equal(t, []byte(ca), secret.Data["ca-bundle.crt"])
}

func Test_BuildAuthenticationFromAnnotations(t *testing.T) {
	for _, tc := range []struct {
		name            string
		annotations     map[string]string
		expecedError    bool
		expectedAuthMap map[Target]AuthenticationType
	}{
		{
			name: "valid annotation",
			annotations: map[string]string{
				"authentication.mcoa.openshift.io/foo": "ExistingSecret",
			},
			expectedAuthMap: map[Target]AuthenticationType{
				Target("foo"): ExistingSecret,
			},
		},
		{
			name: "invalid annotation",
			annotations: map[string]string{
				"authentication.mcoa.openshift.io/foo/bar": "ExistingSecret",
			},
			expecedError: true,
		},
		{
			name: "invalid no target specifed",
			annotations: map[string]string{
				"authentication.mcoa.openshift.io/": "ExistingSecret",
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
				"foo": "ExistingSecret",
			},
			expectedAuthMap: map[Target]AuthenticationType{},
		},
	} {
		authMap, err := BuildAuthenticationFromAnnotations(tc.annotations)
		if tc.expecedError {
			require.Error(t, err)
			return
		}
		require.NoError(t, err)
		require.Equal(t, tc.expectedAuthMap, authMap)
	}
}

func Test_GetExistingSecret(t *testing.T) {
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
		sp, err := NewSecretsProvider(fakeKubeClient, tc.clusterNamespace, "logging", &Config{
			DefaultNamespace: tc.defaultNamespace,
		})
		require.NoError(t, err)
		gotSecret, err := sp.getExistingSecret(context.TODO(), fakeKubeClient, tc.secretName)
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
