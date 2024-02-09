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
	secrets, err := sp.FetchSecrets(context.TODO(), keys, "foo-annotation")
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
