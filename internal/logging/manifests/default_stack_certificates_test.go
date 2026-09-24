package manifests

import (
	"testing"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSSAObsAPIServerCertificate(t *testing.T) {
	cert, err := BuildSSAObsAPIServerCertificate("obs-api.apps.example.com")
	require.NoError(t, err)
	assert.Equal(t, ObsAPIServerMTLSSecretName, cert.Name)
	assert.Equal(t, addoncfg.InstallNamespace, cert.Namespace)
	assert.Equal(t, ObsAPIServerCertCommonName, cert.Spec.CommonName)
	assert.Equal(t, certmanagerv1.UsageServerAuth, cert.Spec.Usages[0])
	assert.Contains(t, cert.Spec.DNSNames, "obs-api.apps.example.com")
	assert.Contains(t, cert.Spec.DNSNames, ObsAPIServiceName+"."+addoncfg.InstallNamespace+".svc")
}
