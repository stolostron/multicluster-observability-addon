package authentication

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

var certManagerCRDs = []string{"certificates.cert-manager.io", "issuers.cert-manager.io", "clusterissuers.cert-manager.io"}
