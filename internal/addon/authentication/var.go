package authentication

import (
	v1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/manifests"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Static represents static authentication type.
	Static AuthenticationType = "StaticAuthentication"
	// Managed represents managed authentication type.
	Managed AuthenticationType = "ManagedAuthentication"
	// MTLS represents mTLS authentication type.
	MTLS AuthenticationType = "mTLS"
	// MCO represents an authentication type that will re-use the MCO provided credentials
	MCO AuthenticationType = "MCO"
)

var (
	metricsDefaults = ProviderConfig{}

	loggingDefaults = ProviderConfig{
		StaticAuthConfig: manifests.StaticAuthenticationConfig{
			ExistingSecret: client.ObjectKey{
				Name:      "static-authentication",
				Namespace: "open-cluster-management",
			},
		},
		// TODO(JoaoBraveCoding) Implement when support for LokiStack is added
		MTLSConfig: manifests.MTLSConfig{
			CommonName: "",
			Subject:    &v1.X509Subject{},
			DNSNames:   []string{},
		},
	}

	tracingDefaults = ProviderConfig{}

	certManagerCRDs = []string{"certificates.cert-manager.io", "issuers.cert-manager.io", "clusterissuers.cert-manager.io"}
)