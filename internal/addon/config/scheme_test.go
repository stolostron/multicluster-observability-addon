package config

import (
	"testing"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	"github.com/stretchr/testify/require"
)

func TestNewSchemeIncludesGatewayResources(t *testing.T) {
	s := NewScheme()

	_, _, err := s.ObjectKinds(&certmanagerv1.Certificate{})
	require.NoError(t, err)
	_, _, err = s.ObjectKinds(&lokiv1.LokiStack{})
	require.NoError(t, err)
}
