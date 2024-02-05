package tracing

const (
	subscriptionChannel = "stable"

	otelColResource = "opentelemetrycollectors"
)

type TracingValues struct {
	Enabled                 bool   `json:"enabled"`
	OtelSubscriptionChannel string `json:"otelSubscriptionChannel"`
	OTELColSpec             string `json:"otelColSpec"`
	// TODO: revert this hack to the official way as recommended by the docs.
	// See https://open-cluster-management.io/developer-guides/addon/#values-definition.
	AddonInstallNamespace string `json:"addonInstallNamespace"`
}
