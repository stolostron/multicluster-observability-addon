package authentication

import (
	v1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
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

	Metrics Signal = "Metrics"
	Logging Signal = "Logging"
	Tracing Signal = "Tracing"
)

var (
	metricsDefaults = ProviderConfig{}

	loggingDefaults = ProviderConfig{
		StaticAuthConfig: StaticAuthenticationConfig{
			existingSecret: client.ObjectKey{
				Name:      "static-authentication",
				Namespace: "open-cluster-management",
			},
		},
		// TODO(JoaoBraveCoding) Implement when support for LokiStack is added
		MTLSConfig: MTLSConfig{
			CommonName: "",
			Subject:    &v1.X509Subject{},
			DNSNames:   []string{},
			IssuerRef:  cmmetav1.ObjectReference{},
		},
	}

	tracingDefaults = ProviderConfig{}
)
