package manifests

const (
	subscriptionChannelValueKey = "openshiftLoggingChannel"
	defaultLoggingVersion       = "stable-6.1"

	ManagedCollectionCertCommonName = "mcoa-logging-managed-collection"
	ManagedCollectionSecretName     = "mcoa-logging-managed-collection-tls"

	ManagedStorageCertCommonName       = "mcoa-logging-managed-storage"
	ManagedStorageMTLSSecretName       = "mcoa-logging-managed-storage-tls"
	ManagedStorageObjStorageSecretName = "mcoa-logging-managed-storage-objstorage"
)
