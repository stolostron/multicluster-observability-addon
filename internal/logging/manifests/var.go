package manifests

import "errors"

const (
	subscriptionChannelValueKey     = "openshiftLoggingChannel"
	defaultLoggingVersion           = "stable-6.2"
	CloSubscriptionInstallName      = "cluster-logging"
	CloSubscriptionInstallNamespace = "openshift-logging"
)

var errInvalidSubscriptionChannel = errors.New("current version of the cluster-logging installed doesn't match the supported MCOA version")
