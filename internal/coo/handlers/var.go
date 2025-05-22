package handlers

import "errors"

var (
	cooSubscriptionName      = "cluster-observability-operator"
	cooSubscriptionNamespace = "openshift-cluster-observability-operator"
	cooSubscriptionChannel   = "stable"

	errInvalidSubscriptionChannel = errors.New("current version of the cluster-observability-operator installed doesn't match the supported MCOA version")
)
