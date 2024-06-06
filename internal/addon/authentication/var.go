package authentication

const (
	// SecretReference represents a pre-existing secret in the cluster.
	SecretReference AuthenticationType = "SecretReference"

	AnnotationAuthOutput = "authentication.mcoa.openshift.io/"
)
