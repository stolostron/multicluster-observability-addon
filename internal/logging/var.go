package logging

const (
	annotationTargetOutputName = "logging.mcoa.openshift.io/target-output-name"

	subscriptionChannelValueKey = "loggingSubscriptionChannel"
	defaultLoggingVersion       = "stable-5.8"

	clusterLogForwarderResource = "clusterlogforwarders"
)

type LoggingValues struct {
	Enabled                    bool   `json:"enabled"`
	CLFSpec                    string `json:"clfSpec"`
	LoggingSubscriptionChannel string `json:"loggingSubscriptionChannel"`
}
