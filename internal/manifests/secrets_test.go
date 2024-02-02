package manifests

import (
	"context"
	"testing"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_BuildStaticSecret(t *testing.T) {
	key := client.ObjectKey{Name: "foo", Namespace: "foo"}
	saConfig := StaticAuthenticationConfig{
		ExistingSecret: client.ObjectKey{
			Name:      "bar",
			Namespace: "bar",
		},
	}

	existingSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "bar",
		},
		Data: map[string][]byte{
			"user": []byte("data"),
			"pass": []byte("secret"),
		},
	}

	fakeKubeClient := fake.NewClientBuilder().
		WithObjects(&existingSecret).
		Build()

	s, err := BuildStaticSecret(context.TODO(), fakeKubeClient, key, saConfig)
	require.NoError(t, err)
	require.Equal(t, existingSecret.Data, s.Data)
}

func Test_BuildMTLSSecret(t *testing.T) {
	key := client.ObjectKey{Name: "foo", Namespace: "foo"}
	mTLSConfig := MTLSConfig{
		CommonName: "foo",
		Subject: &certmanagerv1.X509Subject{
			OrganizationalUnits: []string{
				"foo",
			},
		},
		DNSNames: []string{
			"foo",
		},
	}

	c, err := BuildCertificate(key, mTLSConfig)
	require.NoError(t, err)
	require.Equal(t, "foo", c.Spec.SecretName)
	require.Equal(t, mTLSConfig.CommonName, c.Spec.CommonName)
	require.Equal(t, mTLSConfig.Subject, c.Spec.Subject)
	require.Equal(t, "mcoa-cluster-issuer", c.Spec.IssuerRef.Name)
	require.ElementsMatch(t, mTLSConfig.DNSNames, c.Spec.DNSNames)
}

func Test_InjectCA(t *testing.T) {
	secret := &corev1.Secret{
		Data: map[string][]byte{
			"foo": []byte("bar"),
		},
	}
	ca := "test"
	InjectCA(secret, ca)
	require.Equal(t, []byte("bar"), secret.Data["foo"])
	require.Equal(t, []byte("test"), secret.Data["ca-bundle.crt"])
}
