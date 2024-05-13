package manifests

import (
	"fmt"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	rootIssuerName       = "mcoa-bootstrap-issuer"
	rootCertName         = "mcoa-root-certificate"
	clusterIssuerName    = "mcoa-cluster-issuer"
	certManagerNamespace = "cert-manager"
	caKey                = "ca-bundle.crt"
)

type StaticAuthenticationConfig struct {
	ExistingSecret client.ObjectKey
}

type MTLSConfig struct {
	CAToInject string
	CommonName string
	Subject    *certmanagerv1.X509Subject
	DNSNames   []string
}

// BuildStaticSecret creates a Kubernetes secret for static authentication
// TODO (JoaoBraveCoding) In the future we will want to deprecate this
// authentication method as it's not ideal for multicluster authentication
func BuildStaticSecret(key client.ObjectKey, secret *corev1.Secret) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Data: secret.Data, // Signal specific
	}
}

// BuildCertificate generates a Kubernetes secret for mTLS authentication. This is
// done using Cert-Manager CR.
func BuildCertificate(key client.ObjectKey, mTLSConfig MTLSConfig) *certmanagerv1.Certificate {
	certKey := client.ObjectKey{Name: fmt.Sprintf("%s-cert", key.Name), Namespace: key.Namespace}
	return &certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      certKey.Name,
			Namespace: certKey.Namespace,
		},
		Spec: certmanagerv1.CertificateSpec{
			SecretName: key.Name,
			CommonName: mTLSConfig.CommonName, // Signal specific
			Subject:    mTLSConfig.Subject,    // Signal specific
			DNSNames:   mTLSConfig.DNSNames,   // Signal specific
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
			IssuerRef: cmmetav1.ObjectReference{
				Kind: "ClusterIssuer",
				Name: clusterIssuerName,
			},
		},
	}

}

// createMCOSecret creates a Kubernetes secret for authentication using the
// credentials provided by MCO
// TODO (JoaoBraveCoding) Not implemented
func BuildMCOSecret(key client.ObjectKey) *corev1.Secret {
	return nil
}

// createManagedSecret generates a Kubernetes secret for managed authentication
// such as workload identity federation.
// TODO (JoaoBraveCoding) Currently not implemented, this should only work on
// STS/WIF enabeld clusters
func BuildManagedSecret(key client.ObjectKey) *corev1.Secret {
	// Set additional keys for managed secret
	data := map[string][]byte{
		"roleARN":          []byte("foo"),
		"webIdentityToken": []byte("foo"),
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Data: data,
		Type: corev1.SecretTypeOpaque,
	}
}

func BuildAllRootCertificate() []client.Object {
	issuer := &certmanagerv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rootIssuerName,
			Namespace: certManagerNamespace,
		},
		Spec: certmanagerv1.IssuerSpec{
			IssuerConfig: certmanagerv1.IssuerConfig{
				SelfSigned: &certmanagerv1.SelfSignedIssuer{},
			},
		},
	}

	cert := &certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rootCertName,
			Namespace: certManagerNamespace,
		},
		Spec: certmanagerv1.CertificateSpec{
			IsCA:       true,
			SecretName: rootCertName,
			CommonName: "MCOA Root Certificate",
			PrivateKey: &certmanagerv1.CertificatePrivateKey{
				Algorithm: certmanagerv1.RSAKeyAlgorithm,
				Size:      4096,
				Encoding:  certmanagerv1.PKCS8,
			},
			IssuerRef: cmmetav1.ObjectReference{
				Kind: "Issuer",
				Name: rootIssuerName,
			},
		},
	}

	cIssuer := &certmanagerv1.ClusterIssuer{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterIssuerName,
		},
		Spec: certmanagerv1.IssuerSpec{
			IssuerConfig: certmanagerv1.IssuerConfig{
				CA: &certmanagerv1.CAIssuer{
					SecretName: rootCertName,
				},
			},
		},
	}
	return []client.Object{issuer, cert, cIssuer}
}

func InjectCA(secret *corev1.Secret, ca string) {
	secret.Data[caKey] = []byte(ca)
}
