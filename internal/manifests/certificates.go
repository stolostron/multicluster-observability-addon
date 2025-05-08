package manifests

import (
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const clusterIssuerName = "mcoa-root-issuer"

type CertificateConfig struct {
	CommonName string
	Subject    *certmanagerv1.X509Subject
	DNSNames   []string
}

// BuildClientCertificate builds a client certificate with the given key and mTLSConfig
func BuildClientCertificate(key client.ObjectKey, mTLSConfig CertificateConfig) (*certmanagerv1.Certificate, error) {
	certManagerCert := &certmanagerv1.Certificate{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Certificate",
			APIVersion: certmanagerv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
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
				Kind: certmanagerv1.ClusterIssuerKind,
				Name: clusterIssuerName,
			},
		},
	}
	return certManagerCert, nil
}

// BuildServerCertificate builds a server certificate with the given key and mTLSConfig
func BuildServerCertificate(key client.ObjectKey, mTLSConfig CertificateConfig) (*certmanagerv1.Certificate, error) {
	certManagerCert := &certmanagerv1.Certificate{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Certificate",
			APIVersion: certmanagerv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
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
				certmanagerv1.UsageServerAuth,
				certmanagerv1.UsageKeyEncipherment,
				certmanagerv1.UsageDigitalSignature,
			},
			IssuerRef: cmmetav1.ObjectReference{
				Kind: certmanagerv1.ClusterIssuerKind,
				Name: clusterIssuerName,
			},
		},
	}
	return certManagerCert, nil
}
