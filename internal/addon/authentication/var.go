package authentication

const (
	// ExistingSecret represents a pre-existing secret in the cluster.
	ExistingSecret AuthenticationType = "ExistingSecret"

	AnnotationAuthOutput = "authentication.mcoa.openshift.io/"
)
