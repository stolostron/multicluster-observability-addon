package authentication

const (
	// ExistingSecret represents a pre-existing secret in the cluster.
	ExistingSecret AuthenticationType = "ExistingSecret"
	// Static represents static authentication type.
	Static AuthenticationType = "StaticAuthentication"
	// Managed represents managed authentication type.
	Managed AuthenticationType = "ManagedAuthentication"
	// MTLS represents mTLS authentication type.
	MTLS AuthenticationType = "mTLS"
	// MCO represents an authentication type that will re-use the MCO provided credentials
	MCO AuthenticationType = "MCO"

	AnnotationAuthOutput = "authentication.mcoa.openshift.io/"
	AnnotationCAToInject = "authentication.mcoa.openshift.io/ca"
)

var certManagerCRDs = []string{"certificates.cert-manager.io", "issuers.cert-manager.io", "clusterissuers.cert-manager.io"}
