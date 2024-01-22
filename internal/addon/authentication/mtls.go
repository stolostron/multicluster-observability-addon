package authentication

import (
	"context"
	"fmt"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MTLSConfig struct {
	CommonName string
	Subject    *certmanagerv1.X509Subject
	DNSNames   []string
	IssuerRef  cmmetav1.ObjectReference
}

// createMTLSSecret generates a Kubernetes secret for mTLS authentication. This is
// done using Cert-Manager CR.
func (k *secretsProvider) createMTLSSecret(ctx context.Context, key client.ObjectKey) error {
	certKey := client.ObjectKey{Name: fmt.Sprintf("%s-cert", key.Name), Namespace: key.Namespace}
	certManagerCert := &certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      certKey.Name,
			Namespace: certKey.Namespace,
		},
		Spec: certmanagerv1.CertificateSpec{
			SecretName: key.Namespace,
			CommonName: k.MTLSConfig.CommonName, // Signal specific
			Subject:    k.MTLSConfig.Subject,    // Signal specific
			DNSNames:   k.MTLSConfig.DNSNames,   // Signal specific
			IssuerRef:  k.MTLSConfig.IssuerRef,  // Signal specific (possibly)
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

	return k.createOrUpdate(ctx, certManagerCert)
}
