package config

import (
	"errors"
	"time"

	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"k8s.io/apimachinery/pkg/fields"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	Name               = "multicluster-observability-addon"
	LabelOCMAddonName  = "open-cluster-management.io/addon-name"
	InstallNamespace   = "open-cluster-management-observability"
	AnalyticsNamespace = "observability-analytics"

	DefaultContextTimeout = 10 * time.Second

	McoaChartDir    = "manifests/charts/mcoa"
	MetricsChartDir = "manifests/charts/mcoa/charts/metrics"
	LoggingChartDir = "manifests/charts/mcoa/charts/logging"
	TracingChartDir = "manifests/charts/mcoa/charts/tracing"
	COOChartDir     = "manifests/charts/mcoa/charts/coo"

	AddonDeploymentConfigResource = "addondeploymentconfigs"

	CooSubscriptionName      = "cluster-observability-operator"
	CooSubscriptionNamespace = "openshift-cluster-observability-operator"
	CooSubscriptionChannel   = "stable"

	CooStatusClaimName = "coo.observability.open-cluster-management.io"

	DefaultCooCatalogSource          = "redhat-operators"
	DefaultCooCatalogSourceNamespace = "openshift-marketplace"

	PaProbeKey                           = "isAvailable"
	PaProbePath                          = ".status.conditions[?(@.type==\"Available\")].status"
	EndpointMonitoringOperatorDeployment = "endpoint-monitoring-operator"
	EndpointMonitoringOperatorProbeKey   = "availableReplicas"
	EndpointMonitoringOperatorProbePath  = ".status.availableReplicas"

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

	ThanosProbeKey  = "isReady"
	ThanosProbePath = ".status.conditions[?(@.type==\"Ready\")].status"

	DefaultStackPrefix = "mcoa-default"

	// Label keys
	ReleaseLabelKey               = "release"
	PlacementRefNameLabelKey      = "placement-ref-name"
	PlacementRefNamespaceLabelKey = "placement-ref-namespace"
	ComponentK8sLabelKey          = "app.kubernetes.io/component"
	ManagedByK8sLabelKey          = "app.kubernetes.io/managed-by"
	PartOfK8sLabelKey             = "app.kubernetes.io/part-of"
	BackupLabelKey                = "cluster.open-cluster-management.io/backup"
	BackupLabelValue              = ""
	PlacementAnnotationKey        = "observability.open-cluster-management.io/placements"
	// SSAManagedFieldsAnnotationKey is set on resources where MCOA enforces fields
	// (hub server-side apply, or spoke manifests). The value is a newline-separated
	// list of JSON paths for the fields MCOA will override if changed by the user.
	SSAManagedFieldsAnnotationKey = "observability.open-cluster-management.io/ssa-managed-fields"

	ClusterClaimClusterID        = "id.k8s.io"
	ManagedClusterLabelClusterID = "clusterID"

	// Feedback rule names
	IsEstablishedFeedbackName             = "isEstablished"
	IsEstablishedFeedbackPath             = ".status.conditions[?(@.type==\"Established\")].status"
	PrometheusOperatorVersionFeedbackName = "prometheusOperatorVersion"
	PrometheusOperatorVersionFeedbackPath = `.metadata.annotations.operator\.prometheus\.io/version`
	LastTransitionTimeFeedbackName        = "lastTransitionTime"
	LastTransitionTimeFeedbackPath        = ".status.conditions[?(@.type==\"Established\")].lastTransitionTime"
	// TLS profile feedback
	TLSProfileConfigMapNamespace = "open-cluster-management-agent"
	TLSProfileConfigMapName      = "ocm-tls-profile"
	TLSProfileTypeFeedbackName   = "profileType"
	TLSProfileTypeFeedbackPath   = ".data.profileType"
	TLSMinVersionFeedbackName    = "minTLSVersion"
	TLSMinVersionFeedbackPath    = ".data.minTLSVersion"
	TLSCipherSuitesFeedbackName  = "cipherSuites"
	TLSCipherSuitesFeedbackPath  = ".data.cipherSuites"
	TLSDefaultMinVersion         = "VersionTLS12"

	PrometheusRulesResource = "prometheusrules"
	RsProbeKey              = "ruleName"
	RsProbePath             = ".metadata.name"

	VendorOverrideAnnotationKey = "mcoa-override-vendor"
	AnnotationOriginalResource  = "mcoa.openshift.io/original-resource"
)

var (
	ErrInvalidMetricsHubHostname  = errors.New("invalid metrics hub hostname")
	ErrInvalidProxyURL            = errors.New("invalid proxy URL")
	ErrInvalidSubscriptionChannel = errors.New("current version of the cluster-observability-operator installed doesn't match the supported MCOA version")
	ErrInvalidPort                = errors.New("invalid port")
)

// GetCacheOptions configures the in-memory cache for the controller-runtime Manager.
// It applies TransformStripManagedFields by default to strip metadata.managedFields,
// significantly reducing memory consumption in large fleets.
// PrometheusAgent is exempted via an identity transform because MCOA inspects
// metadata.managedFields to determine whether an agent is exclusively owned by MCOA
// or partially owned by a user before deleting orphaned agents.
// It also scopes AddOnDeploymentConfig to the install namespace and
// ClusterManagementAddOn to the MCOA instance name to prevent unbounded cache growth.
func GetCacheOptions() cache.Options {
	return cache.Options{
		DefaultTransform: cache.TransformStripManagedFields(),
		ByObject: map[client.Object]cache.ByObject{
			&addonv1beta1.AddOnDeploymentConfig{}: {
				Namespaces: map[string]cache.Config{
					InstallNamespace: {},
				},
			},
			&addonv1beta1.ClusterManagementAddOn{}: {
				Field: fields.SelectorFromSet(fields.Set{"metadata.name": Name}),
			},
			&cooprometheusv1alpha1.PrometheusAgent{}: {
				Namespaces: map[string]cache.Config{
					InstallNamespace: {},
				},
				Transform: func(in any) (any, error) {
					return in, nil
				},
			},
		},
	}
}
