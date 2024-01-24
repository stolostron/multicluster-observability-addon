package manifests

import (
	"context"
	"fmt"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StaticAuthenticationConfig struct {
	ExistingSecret client.ObjectKey
}

type MTLSConfig struct {
	CommonName string
	Subject    *certmanagerv1.X509Subject
	DNSNames   []string
	IssuerRef  cmmetav1.ObjectReference
}

// BuildStaticSecret creates a Kubernetes secret for static authentication
// TODO (JoaoBraveCoding) In the future we will want to deprecate this
// authentication method as it's not ideal for multicluster authentication
func BuildStaticSecret(ctx context.Context, k client.Client, key client.ObjectKey, saConfig StaticAuthenticationConfig) (*corev1.Secret, error) {
	staticAuth := &corev1.Secret{}
	err := k.Get(ctx, saConfig.ExistingSecret, staticAuth, &client.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get existing secret: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Data: staticAuth.Data, // Signal specific
	}

	return secret, nil
}

// BuildCertificate generates a Kubernetes secret for mTLS authentication. This is
// done using Cert-Manager CR.
func BuildCertificate(key client.ObjectKey, mTLSConfig MTLSConfig) (*certmanagerv1.Certificate, error) {
	certKey := client.ObjectKey{Name: fmt.Sprintf("%s-cert", key.Name), Namespace: key.Namespace}
	certManagerCert := &certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      certKey.Name,
			Namespace: certKey.Namespace,
		},
		Spec: certmanagerv1.CertificateSpec{
			SecretName: key.Namespace,
			CommonName: mTLSConfig.CommonName, // Signal specific
			Subject:    mTLSConfig.Subject,    // Signal specific
			DNSNames:   mTLSConfig.DNSNames,   // Signal specific
			IssuerRef:  mTLSConfig.IssuerRef,  // Signal specific (possibly)
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

	return certManagerCert, nil
}

// createMCOSecret creates a Kubernetes secret for authentication using the
// credentials provided by MCO
// TODO (JoaoBraveCoding) Not implemented
func BuildMCOSecret(key client.ObjectKey) (*corev1.Secret, error) {
	return nil, nil
}

// createManagedSecret generates a Kubernetes secret for managed authentication
// such as workload identity federation.
// TODO (JoaoBraveCoding) Currently not implemented, this should only work on
// STS/WIF enabeld clusters
func BuildManagedSecret(key client.ObjectKey) (*corev1.Secret, error) {
	// Set additional keys for managed secret
	data := map[string][]byte{
		"roleARN":          []byte("foo"),
		"webIdentityToken": []byte("foo"),
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Data: data,
		Type: corev1.SecretTypeOpaque,
	}

	return secret, nil
}

func BuildAllRootCertificate() []client.Object {
	issuerName := "mcoa-bootstrap-issuer"
	issuer := certmanagerv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      issuerName,
			Namespace: "cert-manager",
		},
		Spec: certmanagerv1.IssuerSpec{
			IssuerConfig: certmanagerv1.IssuerConfig{
				SelfSigned: &certmanagerv1.SelfSignedIssuer{},
			},
		},
	}

	certSecretName := "mcoa-root-certificate"
	cert := certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mcoa-root-certificate",
			Namespace: "cert-manager",
		},
		Spec: certmanagerv1.CertificateSpec{
			IsCA:       true,
			SecretName: certSecretName,
			CommonName: "MCOA Root Certificate",
			PrivateKey: &certmanagerv1.CertificatePrivateKey{
				Algorithm: certmanagerv1.RSAKeyAlgorithm,
				Size:      4096,
				Encoding:  certmanagerv1.PKCS8,
			},
			IssuerRef: cmmetav1.ObjectReference{
				Kind: "Issuer",
				Name: issuerName,
			},
		},
	}

	cIssuer := certmanagerv1.ClusterIssuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mcoa-root-issuer",
			Namespace: "cert-manager",
		},
		Spec: certmanagerv1.IssuerSpec{
			IssuerConfig: certmanagerv1.IssuerConfig{
				CA: &certmanagerv1.CAIssuer{
					SecretName: certSecretName,
				},
			},
		},
	}
	return []client.Object{&issuer, &cert, &cIssuer}
}
