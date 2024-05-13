package manifests

import (
	v1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon/authentication"
	"github.com/rhobs/multicluster-observability-addon/internal/manifests"
)

const (
	LabelCLFRef                = "mcoa.openshift.io/clf-ref"
	AnnotationTargetOutputName = "logging.mcoa.openshift.io/target-output-name"

	subscriptionChannelValueKey = "loggingSubscriptionChannel"
	defaultLoggingVersion       = "stable-5.9"

	certOrganizatonalUnit = "multicluster-observability-addon"
	certDNSNameCollector  = "collector.openshift-logging.svc"
)

var AuthDefaultConfig = &authentication.Config{
	MTLSConfig: manifests.MTLSConfig{
		CommonName: "", // Should be set when using these defaults
		Subject: &v1.X509Subject{
			OrganizationalUnits: []string{
				certOrganizatonalUnit,
			},
		},
		DNSNames: []string{
			certDNSNameCollector,
		},
	},
}
