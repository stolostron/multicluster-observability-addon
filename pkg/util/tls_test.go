// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package util

import (
	"context"
	"crypto/tls"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = configv1.AddToScheme(s)
	return s
}

func newAPIServerWithProfile(profile *configv1.TLSSecurityProfile, adherence configv1.TLSAdherencePolicy) *configv1.APIServer {
	return &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: configv1.APIServerSpec{
			TLSSecurityProfile: profile,
			TLSAdherence:       adherence,
		},
	}
}

func setFakeClient(objs ...client.Object) {
	c := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(objs...).Build()
	tlsClientFunc = func() (client.Client, error) {
		return c, nil
	}
}

func TestGetOrCreateTLSProfileSpec(t *testing.T) {
	strict := configv1.TLSAdherencePolicyStrictAllComponents
	intermediateSpec := configv1.TLSProfiles[configv1.TLSProfileIntermediateType]

	tests := []struct {
		name       string
		profile    *configv1.TLSSecurityProfile
		adherence  configv1.TLSAdherencePolicy
		wantedSpec *configv1.TLSProfileSpec
	}{
		{
			name: "strict + intermediate profile",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileIntermediateType,
			},
			adherence:  strict,
			wantedSpec: configv1.TLSProfiles[configv1.TLSProfileIntermediateType],
		},
		{
			name: "strict + modern profile",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileModernType,
			},
			adherence:  strict,
			wantedSpec: configv1.TLSProfiles[configv1.TLSProfileModernType],
		},
		{
			name: "strict + old profile",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileOldType,
			},
			adherence:  strict,
			wantedSpec: configv1.TLSProfiles[configv1.TLSProfileOldType],
		},
		{
			name:       "strict + nil profile defaults to intermediate",
			profile:    nil,
			adherence:  strict,
			wantedSpec: intermediateSpec,
		},
		{
			name: "NoOpinion adherence defaults to intermediate",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileModernType,
			},
			adherence:  configv1.TLSAdherencePolicyNoOpinion,
			wantedSpec: intermediateSpec,
		},
		{
			name: "empty adherence defaults to intermediate",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileModernType,
			},
			adherence:  "",
			wantedSpec: intermediateSpec,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer resetTLSState()
			setFakeClient(newAPIServerWithProfile(tt.profile, tt.adherence))

			spec, err := GetOrCreateTLSProfileSpec(context.Background())
			require.NoError(t, err)
			assert.Equal(t, tt.wantedSpec.MinTLSVersion, spec.MinTLSVersion)
		})
	}
}

func TestGetOrCreateTLSProfileSpec_NotFound(t *testing.T) {
	defer resetTLSState()
	// No APIServer object
	setFakeClient()

	spec, err := GetOrCreateTLSProfileSpec(context.Background())
	require.NoError(t, err)

	intermediateSpec := configv1.TLSProfiles[configv1.TLSProfileIntermediateType]
	assert.Equal(t, intermediateSpec.MinTLSVersion, spec.MinTLSVersion)
}

func TestGetOrCreateTLSProfileSpec_Caching(t *testing.T) {
	defer resetTLSState()

	setFakeClient(newAPIServerWithProfile(&configv1.TLSSecurityProfile{
		Type: configv1.TLSProfileModernType,
	}, configv1.TLSAdherencePolicyStrictAllComponents))

	spec1, err := GetOrCreateTLSProfileSpec(context.Background())
	require.NoError(t, err)

	// Break the client and verify the second still returns cached value
	tlsClientFunc = func() (client.Client, error) {
		return nil, assert.AnError
	}

	spec2, err := GetOrCreateTLSProfileSpec(context.Background())
	require.NoError(t, err)
	assert.Equal(t, spec1, spec2)
}

func TestGetOrCreateTLSConfig(t *testing.T) {
	tests := []struct {
		name           string
		profile        *configv1.TLSSecurityProfile
		adherence      configv1.TLSAdherencePolicy
		wantMinVersion uint16
	}{
		{
			name: "strict + intermediate",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileIntermediateType,
			},
			adherence:      configv1.TLSAdherencePolicyStrictAllComponents,
			wantMinVersion: tls.VersionTLS12,
		},
		{
			name: "strict + modern",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileModernType,
			},
			adherence:      configv1.TLSAdherencePolicyStrictAllComponents,
			wantMinVersion: tls.VersionTLS13,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer resetTLSState()
			setFakeClient(newAPIServerWithProfile(tt.profile, tt.adherence))

			tlsConfigFn, err := GetOrCreateTLSConfig(context.Background())
			require.NoError(t, err)

			cfg := &tls.Config{}
			tlsConfigFn(cfg)
			assert.Equal(t, tt.wantMinVersion, cfg.MinVersion)
		})
	}
}

func TestGetTLSVersionAndCiphers(t *testing.T) {
	defer resetTLSState()

	setFakeClient(newAPIServerWithProfile(&configv1.TLSSecurityProfile{
		Type: configv1.TLSProfileIntermediateType,
	}, configv1.TLSAdherencePolicyStrictAllComponents))

	minVer, ciphers, err := GetTLSVersionAndCiphers(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "VersionTLS12", minVer)
	assert.NotEmpty(t, ciphers)
	assert.Contains(t, ciphers, "TLS_")
}

func TestSetTLSSecurityConfiguration(t *testing.T) {
	defer resetTLSState()

	setFakeClient(newAPIServerWithProfile(&configv1.TLSSecurityProfile{
		Type: configv1.TLSProfileIntermediateType,
	}, configv1.TLSAdherencePolicyStrictAllComponents))

	args := []string{"--upstream=http://127.0.0.1:9090"}
	args, err := SetTLSSecurityConfiguration(context.Background(), args, "--tls-cipher-suites=", "--tls-min-version=")
	require.NoError(t, err)

	var hasCiphers, hasVersion bool
	for _, arg := range args {
		if len(arg) > len("--tls-cipher-suites=") && arg[:len("--tls-cipher-suites=")] == "--tls-cipher-suites=" {
			hasCiphers = true
		}
		if len(arg) > len("--tls-min-version=") && arg[:len("--tls-min-version=")] == "--tls-min-version=" {
			hasVersion = true
		}
	}
	assert.True(t, hasCiphers, "expected --tls-cipher-suites arg")
	assert.True(t, hasVersion, "expected --tls-min-version arg")
}

func TestSetTLSSecurityConfiguration_OverwriteExisting(t *testing.T) {
	defer resetTLSState()

	setFakeClient(newAPIServerWithProfile(&configv1.TLSSecurityProfile{
		Type: configv1.TLSProfileModernType,
	}, configv1.TLSAdherencePolicyStrictAllComponents))

	args := []string{
		"--upstream=http://127.0.0.1:9090",
		"--tls-min-version=VersionTLS10",
		"--tls-cipher-suites=OLD_CIPHER",
	}
	args, err := SetTLSSecurityConfiguration(context.Background(), args, "--tls-cipher-suites=", "--tls-min-version=")
	require.NoError(t, err)

	// Should have overwritten, not duplicated
	var versionCount int
	for _, arg := range args {
		if len(arg) > len("--tls-min-version=") && arg[:len("--tls-min-version=")] == "--tls-min-version=" {
			versionCount++
		}
	}
	assert.Equal(t, 1, versionCount, "should overwrite not duplicate --tls-min-version")
}

func resetTLSState() {
	tlsProfileSpec = nil
	tlsConfig = nil
	ocpConfigClient = nil
	tlsClientFunc = func() (client.Client, error) {
		return getOrCreateOCPConfigCRClient()
	}
}
