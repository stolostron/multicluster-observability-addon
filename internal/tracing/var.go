package tracing

const (
	subscriptionChannel = "stable"

	otelColResource = "opentelemetrycollectors"
)

type TracingValues struct {
	Enabled                 bool   `json:"enabled"`
	OtelSubscriptionChannel string `json:"otelSubscriptionChannel"`
	OTELColSpec             string `json:"otelColSpec"`
}
