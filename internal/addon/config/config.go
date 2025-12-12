package config

import (
	"errors"
)

const (
	Name              = "multicluster-observability-addon"
	LabelOCMAddonName = "open-cluster-management.io/addon-name"
	InstallNamespace  = "open-cluster-management-observability"

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
	SpokeCLFName                 = "mcoa-instance"
	SpokeCLFNamespace            = "openshift-logging"
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

	DefaultStackPrefix = "mcoa-default"

	// Label keys
	PlacementRefNameLabelKey      = "placement-ref-name"
	PlacementRefNamespaceLabelKey = "placement-ref-namespace"
	ComponentK8sLabelKey          = "app.kubernetes.io/component"
	ManagedByK8sLabelKey          = "app.kubernetes.io/managed-by"
	PartOfK8sLabelKey             = "app.kubernetes.io/part-of"
	BackupLabelKey                = "cluster.open-cluster-management.io/backup"
	BackupLabelValue              = ""

	ClusterClaimClusterID        = "id.k8s.io"
	ManagedClusterLabelClusterID = "clusterID"

	// Feedback rule names
	IsEstablishedFeedbackName             = "isEstablished"
	PrometheusOperatorVersionFeedbackName = "prometheusOperatorVersion"
	LastTransitionTimeFeedbackName        = "lastTransitionTime"
	IsOLMManagedFeedbackName              = "isOLMManaged"

	VendorOverrideAnnotationKey = "mcoa-override-vendor"
	AnnotationOriginalResource  = "mcoa.openshift.io/original-resource"
)

var (
	ErrInvalidMetricsHubHostname          = errors.New("invalid metrics hub hostname")
	ErrInvalidMetricsAlertManagerHostname = errors.New("invalid metrics alert manager hostname")
	ErrInvalidProxyURL                    = errors.New("invalid proxy URL")
	ErrInvalidSubscriptionChannel         = errors.New("current version of the cluster-observability-operator installed doesn't match the supported MCOA version")
)
