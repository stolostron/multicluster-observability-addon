package mcoagateway

import (
	"fmt"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/manifests"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultCollectionCertCommonName = "mcoa-observability-observatorium-api"
	DefaultCollectionMTLSSecretName = "mcoa-gateway-tls"

	DefaultStorageCertCommonName    = "mcoa-observability-observatorium-api"
	DefaultStorageMTLSSecretName    = "mcoa-gateway-server-tls"
	DefaultLokiClientMTLSSecretName = "mcoa-gateway-loki-client-tls"
)

func BuildGatewayCertificates(cluster, gatewayHost string) ([]client.Object, error) {
	objects := []client.Object{}

	clientCertConfig := manifests.CertificateConfig{
		CommonName: DefaultCollectionCertCommonName,
		Subject: &certmanagerv1.X509Subject{
			OrganizationalUnits: []string{cluster},
		},
		DNSNames: []string{DefaultCollectionCertCommonName},
	}
	clientKey := client.ObjectKey{Name: DefaultCollectionMTLSSecretName, Namespace: cluster}
	clientCert, err := manifests.BuildClientCertificate(clientKey, clientCertConfig)
	if err != nil {
		return nil, err
	}
	objects = append(objects, clientCert)

	if cluster == "local-cluster" {
		svcName := DefaultStorageCertCommonName
		dnsNames := []string{
			svcName,
			fmt.Sprintf("%s.%s.svc", svcName, addoncfg.InstallNamespace),
			fmt.Sprintf("%s.%s.svc.cluster.local", svcName, addoncfg.InstallNamespace),
		}

		if gatewayHost != "" {
			dnsNames = append(dnsNames, gatewayHost)
		}
		serverCertConfig := manifests.CertificateConfig{
			CommonName: DefaultStorageCertCommonName,
			Subject:    &certmanagerv1.X509Subject{},
			DNSNames:   dnsNames,
		}
		serverKey := client.ObjectKey{Name: DefaultStorageMTLSSecretName, Namespace: addoncfg.InstallNamespace}
		serverCert, err := manifests.BuildServerCertificate(serverKey, serverCertConfig)
		if err != nil {
			return nil, err
		}
		objects = append(objects, serverCert)

		// New client cert for observatorium-api → LokiStack hop (hub only)
		lokiClientCertConfig := manifests.CertificateConfig{
			CommonName: "mcoa-gateway-loki-client",
			Subject:    &certmanagerv1.X509Subject{},
			DNSNames:   []string{"mcoa-gateway-loki-client"},
		}
		lokiClientKey := client.ObjectKey{Name: DefaultLokiClientMTLSSecretName, Namespace: addoncfg.InstallNamespace}
		lokiClientCert, err := manifests.BuildClientCertificate(lokiClientKey, lokiClientCertConfig)
		if err != nil {
			return nil, err
		}
		objects = append(objects, lokiClientCert)
	}

	return objects, nil
}
