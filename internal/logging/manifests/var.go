package manifests

import "errors"

const (
	subscriptionChannelValueKey = "openshiftLoggingChannel"
	defaultLoggingVersion       = "stable-6.2"

	CloSubscriptionInstallName = "cluster-logging"
	LoggingNamespace           = "openshift-logging"

	DefaultCollectionCLFName        = "mcoa-logging-managed-collection"
	DefaultCollectionCertCommonName = "mcoa-logging-managed-collection"
	DefaultCollectionMTLSSecretName = "mcoa-logging-managed-collection-tls"

	DefaultStorageLSName               = "mcoa-logging-managed-storage"
	DefaultStorageCertCommonName       = "mcoa-logging-managed-storage"
	DefaultStorageMTLSSecretName       = "mcoa-logging-managed-storage-tls"
	DefaultStorageObjStorageSecretName = "mcoa-logging-managed-storage-objstorage"
)

var errInvalidSubscriptionChannel = errors.New("current version of the cluster-logging installed doesn't match the supported MCOA version")
