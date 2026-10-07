package mcoagateway

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"testing"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuildCollectionCertificate(t *testing.T) {
	cert, err := BuildCollectionCertificate("spoke-1")
	require.NoError(t, err)
	assert.Equal(t, DefaultCollectionMTLSSecretName, cert.Name)
	assert.Equal(t, "spoke-1", cert.Namespace)
	assert.Equal(t, []string{"spoke-1"}, cert.Spec.Subject.OrganizationalUnits)
	assert.Equal(t, certmanagerv1.ClusterIssuerKind, cert.Spec.IssuerRef.Kind)
}

func TestBuildServerCertificate(t *testing.T) {
	cert, err := BuildServerCertificate("mcoa-gateway.example.com")
	require.NoError(t, err)
	assert.Equal(t, DefaultStorageMTLSSecretName, cert.Name)
	assert.Equal(t, addoncfg.InstallNamespace, cert.Namespace)
	assert.Contains(t, cert.Spec.DNSNames, "mcoa-gateway.example.com")
	assert.Contains(t, cert.Spec.DNSNames, fmt.Sprintf("%s.%s.svc", DefaultStorageCertCommonName, addoncfg.InstallNamespace))
	assert.Contains(t, cert.Spec.Usages, certmanagerv1.UsageServerAuth)
}

func TestBuildLokiClientCertificate(t *testing.T) {
	cert, err := BuildLokiClientCertificate()
	require.NoError(t, err)
	assert.Equal(t, DefaultLokiClientMTLSSecretName, cert.Name)
	assert.Equal(t, addoncfg.InstallNamespace, cert.Namespace)
	assert.Contains(t, cert.Spec.Usages, certmanagerv1.UsageClientAuth)
	assert.NotContains(t, cert.Spec.Usages, certmanagerv1.UsageServerAuth)
}

func TestServerCertIncludesHost(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	const host = "mcoa-gateway.example.com"

	t.Run("accepts an issued cert that already lists the host", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: DefaultStorageMTLSSecretName, Namespace: addoncfg.InstallNamespace},
			Data:       map[string][]byte{corev1.TLSCertKey: testServerCertPEM(t, host)},
		}
		k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
		require.NoError(t, ServerCertIncludesHost(t.Context(), k8s, host))
	})

	t.Run("rejects a cert that is missing the host", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: DefaultStorageMTLSSecretName, Namespace: addoncfg.InstallNamespace},
			Data:       map[string][]byte{corev1.TLSCertKey: testServerCertPEM(t, "other.example.com")},
		}
		k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
		require.ErrorIs(t, ServerCertIncludesHost(t.Context(), k8s, host), ErrGatewayServerCertHost)
	})
}

func testServerCertPEM(t *testing.T, dnsNames ...string) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: DefaultStorageCertCommonName},
		DNSNames:     dnsNames,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
