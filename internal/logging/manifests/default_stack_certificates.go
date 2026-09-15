package manifests

import (
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/manifests"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func BuildSSACollectionCertificates(cluster string) ([]client.Object, error) {
	certConfig := manifests.CertificateConfig{
		CommonName: DefaultCollectionCertCommonName,
		Subject: &certmanagerv1.X509Subject{
			OrganizationalUnits: []string{cluster},
		},
		DNSNames: []string{DefaultCollectionCertCommonName},
	}
	key := client.ObjectKey{Name: DefaultCollectionMTLSSecretName, Namespace: cluster}
	cert, err := manifests.BuildClientCertificate(key, certConfig)
	if err != nil {
		return nil, err
	}
	return []client.Object{cert}, nil
}

// BuildSSAStorageCertificate creates the storage mTLS certificate in the namespace
// of the cluster whose ManagedClusterAddOn will deploy LokiStack (the hub today).
func BuildSSAStorageCertificate(cluster string) ([]client.Object, error) {
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
	return []client.Object{cert}, nil
}
