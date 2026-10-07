package mcoagateway

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"slices"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/manifests"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultCollectionCertCommonName = "mcoa-observability-observatorium-api"
	DefaultCollectionMTLSSecretName = "mcoa-gateway-tls"

	DefaultStorageCertCommonName    = "mcoa-observability-observatorium-api"
	DefaultStorageMTLSSecretName    = "mcoa-gateway-server-tls"
	DefaultLokiClientMTLSSecretName = "mcoa-gateway-loki-client-tls"
)

var (
	ErrGatewayServerCertIncomplete = errors.New("MCOA gateway server certificate secret has no tls.crt yet")
	ErrGatewayServerCertHost       = errors.New("MCOA gateway server certificate does not include route host yet")
)

func BuildCollectionCertificate(cluster string) (*certmanagerv1.Certificate, error) {
	clientCertConfig := manifests.CertificateConfig{
		CommonName: DefaultCollectionCertCommonName,
		Subject: &certmanagerv1.X509Subject{
			OrganizationalUnits: []string{cluster},
		},
		DNSNames: []string{DefaultCollectionCertCommonName},
	}
	clientKey := client.ObjectKey{Name: DefaultCollectionMTLSSecretName, Namespace: cluster}
	return manifests.BuildClientCertificate(clientKey, clientCertConfig)
}

func BuildServerCertificate(gatewayHost string) (*certmanagerv1.Certificate, error) {
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
	return manifests.BuildServerCertificate(serverKey, serverCertConfig)
}

func BuildLokiClientCertificate() (*certmanagerv1.Certificate, error) {
	lokiClientCertConfig := manifests.CertificateConfig{
		CommonName: "mcoa-gateway-loki-client",
		Subject:    &certmanagerv1.X509Subject{},
		DNSNames:   []string{"mcoa-gateway-loki-client"},
	}
	lokiClientKey := client.ObjectKey{Name: DefaultLokiClientMTLSSecretName, Namespace: addoncfg.InstallNamespace}
	return manifests.BuildClientCertificate(lokiClientKey, lokiClientCertConfig)
}

// ServerCertIncludesHost reports whether the issued gateway server certificate
// already lists host as a DNS SAN. Callers should requeue until this succeeds
// so collectors are not pointed at a Route the cert cannot yet serve.
func ServerCertIncludesHost(ctx context.Context, k8s client.Client, host string) error {
	secret := &corev1.Secret{}
	key := types.NamespacedName{Name: DefaultStorageMTLSSecretName, Namespace: addoncfg.InstallNamespace}
	if err := k8s.Get(ctx, key, secret); err != nil {
		return fmt.Errorf("failed to get MCOA gateway server certificate secret %s/%s: %w", key.Namespace, key.Name, err)
	}
	block, _ := pem.Decode(secret.Data[corev1.TLSCertKey])
	if block == nil {
		return fmt.Errorf("%w: %s/%s", ErrGatewayServerCertIncomplete, key.Namespace, key.Name)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse MCOA gateway server certificate %s/%s: %w", key.Namespace, key.Name, err)
	}
	if slices.Contains(cert.DNSNames, host) {
		return nil
	}
	return fmt.Errorf("%w: %s/%s host %s", ErrGatewayServerCertHost, key.Namespace, key.Name, host)
}
