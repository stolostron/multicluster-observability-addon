package manifests

import (
	v1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon/authentication"
	"github.com/rhobs/multicluster-observability-addon/internal/manifests"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AnnotationTargetOutputName = "logging.mcoa.openshift.io/target-output-name"
	AnnotationCAToInject       = "logging.mcoa.openshift.io/ca"

	subscriptionChannelValueKey = "loggingSubscriptionChannel"
	defaultLoggingVersion       = "stable-5.9"

	certOrganizatonalUnit = "multicluster-observability-addon"
	certDNSNameCollector  = "collector.openshift-logging.svc"

	staticSecretName      = "static-authentication"
	staticSecretNamespace = "open-cluster-management"
)

var AuthDefaultConfig = &authentication.Config{
	StaticAuthConfig: manifests.StaticAuthenticationConfig{
		ExistingSecret: client.ObjectKey{
			Name:      staticSecretName,
			Namespace: staticSecretNamespace,
		},
	},
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
