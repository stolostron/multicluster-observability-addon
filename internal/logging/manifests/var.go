package manifests

import "errors"

const (
	subscriptionChannelValueKey     = "openshiftLoggingChannel"
	defaultLoggingVersion           = "stable-6.3"
	CloSubscriptionInstallName      = "cluster-logging"
	CloSubscriptionInstallNamespace = "openshift-logging"
	LoggingNamespace                = "openshift-logging"

	DefaultCollectionCLFName        = "mcoa-logging-managed-collection"
	DefaultCollectionCertCommonName = "mcoa-logging-managed-collection"
	DefaultCollectionMTLSSecretName = "mcoa-logging-managed-collection-tls"

	DefaultStorageLSName               = "mcoa-logging-managed-storage"
	DefaultStorageCertCommonName       = "mcoa-logging-managed-storage"
	DefaultStorageMTLSSecretName       = "mcoa-logging-managed-storage-tls"
	DefaultStorageObjStorageSecretName = "mcoa-logging-managed-storage-objstorage"

	// ObsAPIServerMTLSSecretName is the cert-manager secret obs-api serves with
	// when managed logging is on. The deployment healthcheck flag
	// --tls.healthchecks.server-name must stay equal to ObsAPIServerCertCommonName.
	ObsAPIServerMTLSSecretName = "mcoa-observatorium-api-tls"
	ObsAPIServerCertCommonName = "observability-server-certificate"
	ObsAPIServiceName          = "mcoa-observability-observatorium-api"
)

var errInvalidSubscriptionChannel = errors.New("current version of the cluster-logging installed doesn't match the supported MCOA version")
