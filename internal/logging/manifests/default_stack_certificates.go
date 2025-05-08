package manifests

import (
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/manifests"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func BuildSSAClusterCertificates(cluster string) ([]client.Object, error) {
	objects := []client.Object{}
	certConfig := manifests.CertificateConfig{
		CommonName: DefaultCollectionCertCommonName,
		Subject: &certmanagerv1.X509Subject{
			// Observatorium API uses OrganizationalUnits to authorize access to
			// the tenant
			OrganizationalUnits: []string{cluster},
		},
		DNSNames: []string{DefaultCollectionCertCommonName},
	}
	key := client.ObjectKey{Name: DefaultCollectionMTLSSecretName, Namespace: cluster}
	cert, err := manifests.BuildClientCertificate(key, certConfig)
	if err != nil {
		return nil, err
	}
	objects = append(objects, cert)

	if cluster == "local-cluster" {
		certConfig := manifests.CertificateConfig{
			CommonName: DefaultStorageCertCommonName,
			Subject:    &certmanagerv1.X509Subject{},
			DNSNames:   []string{DefaultStorageCertCommonName},
		}
		key := client.ObjectKey{Name: DefaultStorageMTLSSecretName, Namespace: cluster}
		cert, err := manifests.BuildServerCertificate(key, certConfig)
		if err != nil {
			return nil, err
		}
		objects = append(objects, cert)
	}

	return objects, nil
}
