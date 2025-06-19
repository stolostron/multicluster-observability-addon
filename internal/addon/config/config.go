package config

import (
	"errors"

	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
)

const (
	Name              = "multicluster-observability-addon"
	LabelOCMAddonName = "open-cluster-management.io/addon-name"
	InstallNamespace  = "open-cluster-management-observability"
	HubNamespace      = "local-cluster"

	McoaChartDir    = "manifests/charts/mcoa"
	MetricsChartDir = "manifests/charts/mcoa/charts/metrics"
	LoggingChartDir = "manifests/charts/mcoa/charts/logging"
	TracingChartDir = "manifests/charts/mcoa/charts/tracing"
	COOChartDir     = "manifests/charts/mcoa/charts/coo"

	AddonDeploymentConfigResource = "addondeploymentconfigs"

	CooSubscriptionName      = "cluster-observability-operator"
	CooSubscriptionNamespace = "openshift-cluster-observability-operator"
	CooSubscriptionChannel   = "stable"

	PaProbeKey  = "isAvailable"
	PaProbePath = ".status.conditions[?(@.type==\"Available\")].status"

	ClusterLogForwardersResource = "clusterlogforwarders"
	LokiStacksResource           = "lokistacks"
	UnmanagedCLFName             = "mcoa-instance"
	LoggingNamespace             = "openshift-logging"
	ClfProbeKey                  = "isReady"
	ClfProbePath                 = ".status.conditions[?(@.type==\"Ready\")].status"

	OpenTelemetryCollectorsResource = "opentelemetrycollectors"
	InstrumentationResource         = "instrumentations"
	SpokeOTELColName                = "mcoa-instance"
	SpokeInstrumentationName        = "mcoa-instance"
	IDetectionUIPluginName          = "monitoring"
	SpokeOTELColNamespace           = "mcoa-opentelemetry"
	OtelColProbeKey                 = "replicas"
	OtelColProbePath                = ".spec.replicas"

	UiPluginsResource = "uiplugins"
	UipProbeKey       = "isAvailable"
	UipProbePath      = ".status.conditions[?(@.type==\"Available\")].status"

	DefaultStackPrefix            = "default-stack-instance"
	PlacementRefNameLabelKey      = "placement-ref-name"
	PlacementRefNamespaceLabelKey = "placement-ref-namespace"

	ComponentK8sLabelKey = "app.kubernetes.io/component"
	ManagedByK8sLabelKey = "app.kubernetes.io/managed-by"
	PartOfK8sLabelKey    = "app.kubernetes.io/part-of"

	GlobalPlacementName      = "global"
	GlobalPlacementNamespace = "open-cluster-management-global-set"
)

var (
	ErrInvalidMetricsHubHostname  = errors.New("invalid metrics hub hostname")
	ErrInvalidSubscriptionChannel = errors.New("current version of the cluster-observability-operator installed doesn't match the supported MCOA version")

	GlobalPlacementRef = addonv1alpha1.PlacementRef{
		Name:      GlobalPlacementName,
		Namespace: GlobalPlacementNamespace,
	}
)
