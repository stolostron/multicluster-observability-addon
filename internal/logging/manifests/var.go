package manifests

import "errors"

const (
	subscriptionChannelValueKey = "openshiftLoggingChannel"
	defaultLoggingVersion       = "stable-6.2"

	DefaultCollectionCertCommonName = "mcoa-logging-managed-collection"
	DefaultCollectionMTLSSecretName = "mcoa-logging-managed-collection-tls"

	DefaultStorageCertCommonName       = "mcoa-logging-managed-storage"
	DefaultStorageMTLSSecretName       = "mcoa-logging-managed-storage-tls"
	DefaultStorageObjStorageSecretName = "mcoa-logging-managed-storage-objstorage"

	CloSubscriptionInstallName      = "cluster-logging"
	CloSubscriptionInstallNamespace = "openshift-logging"
)

var errInvalidSubscriptionChannel = errors.New("current version of the cluster-logging installed doesn't match the supported MCOA version")
