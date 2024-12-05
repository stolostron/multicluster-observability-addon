package manifests

import (
	"testing"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/stretchr/testify/require"
)

func TestGetMutateFunc_MutateCertificate(t *testing.T) {
	got := &certmanagerv1.Certificate{
		Spec: certmanagerv1.CertificateSpec{
			SecretName: "foo",
			CommonName: "foo",
			Subject: &certmanagerv1.X509Subject{
				OrganizationalUnits: []string{
					"foo",
				},
			},
			DNSNames: []string{
				"foo",
			},
			IssuerRef: cmmetav1.ObjectReference{
				Kind: "ClusterIssuer",
				Name: "foo",
			},
			PrivateKey: &certmanagerv1.CertificatePrivateKey{
				Algorithm: certmanagerv1.RSAKeyAlgorithm,
				Encoding:  certmanagerv1.PKCS8,
				Size:      4096,
			},
			Usages: []certmanagerv1.KeyUsage{
				certmanagerv1.UsageClientAuth,
				certmanagerv1.UsageKeyEncipherment,
				certmanagerv1.UsageDigitalSignature,
			},
		},
	}
	want := &certmanagerv1.Certificate{
		Spec: certmanagerv1.CertificateSpec{
			SecretName: "bar",
			CommonName: "bar",
			Subject: &certmanagerv1.X509Subject{
				OrganizationalUnits: []string{
					"foo",
					"bar",
				},
			},
			DNSNames: []string{
				"foo",
				"bar",
			},
			IssuerRef: cmmetav1.ObjectReference{
				Kind: "ClusterIssuer",
				Name: "bar",
			},
			PrivateKey: &certmanagerv1.CertificatePrivateKey{
				Algorithm: certmanagerv1.RSAKeyAlgorithm,
				Encoding:  certmanagerv1.PKCS8,
				Size:      4096,
			},
			Usages: []certmanagerv1.KeyUsage{
				certmanagerv1.UsageClientAuth,
				certmanagerv1.UsageKeyEncipherment,
				certmanagerv1.UsageDigitalSignature,
			},
		},
	}
	f := MutateFuncFor(got, want, nil)
	err := f()
	require.NoError(t, err)

	// Ensure partial mutation applied
	require.Equal(t, want.Spec.SecretName, got.Spec.SecretName)
	require.Equal(t, want.Spec.CommonName, got.Spec.CommonName)
	require.Equal(t, want.Spec.IssuerRef, got.Spec.IssuerRef)
	require.ElementsMatch(t, want.Spec.DNSNames, got.Spec.DNSNames)
}
