package manifests

import (
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
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

// BuildSSAObsAPIServerCertificate issues the hub obs-api server certificate from
// mcoa-root-issuer. Spoke collectors trust that issuer, so the route host has to
// be a SAN before they can verify the server. The same secret's ca.crt is the
// client CA obs-api uses to accept those collector certificates.
func BuildSSAObsAPIServerCertificate(routeHost string) (*certmanagerv1.Certificate, error) {
	dnsNames := []string{
		ObsAPIServerCertCommonName,
		ObsAPIServiceName,
		ObsAPIServiceName + "." + addoncfg.InstallNamespace,
		ObsAPIServiceName + "." + addoncfg.InstallNamespace + ".svc",
		ObsAPIServiceName + "." + addoncfg.InstallNamespace + ".svc.cluster.local",
	}
	if routeHost != "" {
		dnsNames = append(dnsNames, routeHost)
	}
	certConfig := manifests.CertificateConfig{
		CommonName: ObsAPIServerCertCommonName,
		Subject:    &certmanagerv1.X509Subject{},
		DNSNames:   dnsNames,
	}
	key := client.ObjectKey{Name: ObsAPIServerMTLSSecretName, Namespace: addoncfg.InstallNamespace}
	return manifests.BuildServerCertificate(key, certConfig)
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
