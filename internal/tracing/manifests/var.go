package manifests

import (
	v1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon/authentication"
	"github.com/rhobs/multicluster-observability-addon/internal/manifests"
)

const (
	AnnotationTargetOutputName = "tracing.mcoa.openshift.io/target-output-name"
)

var AuthDefaultConfig = &authentication.Config{
	// TODO(iblancasa)
	MTLSConfig: manifests.MTLSConfig{
		CommonName: "",
		Subject: &v1.X509Subject{
			OrganizationalUnits: []string{
				"tracing-ocm-addon",
			},
		},
		DNSNames: []string{
			"otelcol.observability.svc",
		},
	},
}
