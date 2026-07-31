// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package util

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"strconv"
	"strings"

	ocinfrav1 "github.com/openshift/api/config/v1"
	tlsutil "github.com/openshift/controller-runtime-common/pkg/tls"
	libgocrypto "github.com/openshift/library-go/pkg/crypto"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("util")

var (
	tlsProfileSpec  *ocinfrav1.TLSProfileSpec
	tlsConfig       func(*tls.Config)
	ocpConfigClient client.Client

	// Override in tests to inject a fake client.
	tlsClientFunc = func() (client.Client, error) {
		return getOrCreateOCPConfigCRClient()
	}
)

// GetOrCreateTLSProfileSpec retrieves spec.tlsSecurityProfile
// from a OCP Cluster API server: apiservers.config.openshift.io/cluster resource
// and applies it based on the adherence policy.
func GetOrCreateTLSProfileSpec(ctx context.Context) (*ocinfrav1.TLSProfileSpec, error) {
	if tlsProfileSpec != nil {
		return tlsProfileSpec, nil
	}

	c, err := tlsClientFunc()
	if err != nil {
		log.Error(err, "unable to create client for API server")
		return nil, err
	}

	tap, err := tlsutil.FetchAPIServerTLSAdherencePolicy(ctx, c)
	if err != nil {
		log.Error(err, "unable to get TLS adherence policy from API server")
		tap = ""
	}

	defaultSpec := ocinfrav1.TLSProfiles[libgocrypto.DefaultTLSProfileType]

	if libgocrypto.ShouldHonorClusterTLSProfile(tap) {
		tps, err := tlsutil.FetchAPIServerTLSProfile(ctx, c)
		if err != nil {
			log.Error(err, "unable to get TLS profile from API server")
			tlsProfileSpec = defaultSpec
		} else {
			tlsProfileSpec = &tps
		}
	} else {
		tlsProfileSpec = defaultSpec
	}

	return tlsProfileSpec, nil
}

// GetOrCreateTLSConfig returns a function that configures a tls.Config
// based on the OCP Cluster API server TLSProfileSpec.
func GetOrCreateTLSConfig(ctx context.Context) (func(*tls.Config), error) {
	if tlsConfig != nil {
		return tlsConfig, nil
	}

	profileSpec, err := GetOrCreateTLSProfileSpec(ctx)
	if err != nil {
		return nil, err
	}

	var unsupportedCiphers []string
	tlsConfig, unsupportedCiphers = tlsutil.NewTLSConfigFromProfile(*profileSpec)
	if len(unsupportedCiphers) > 0 {
		log.Info("TLS configuration contains unsupported ciphers that will be ignored", "ciphers", unsupportedCiphers)
	}

	return tlsConfig, nil
}

// GetTLSVersionAndCiphers returns the TLS min version
// and comma-separated IANA cipher suites from the cluster TLS profile.
func GetTLSVersionAndCiphers(ctx context.Context) (minVersion string, cipherSuites string, err error) {
	profileSpec, err := GetOrCreateTLSProfileSpec(ctx)
	if err != nil {
		if isTest, parseErr := strconv.ParseBool(os.Getenv("UNIT_TEST")); parseErr == nil && isTest {
			log.Info("running unit test, skipping TLS profile adherence")
			return "", "", nil
		}
		return "", "", err
	}

	minVersion = string(profileSpec.MinTLSVersion)
	cipherSuites = strings.Join(libgocrypto.OpenSSLToIANACipherSuites(profileSpec.Ciphers), ",")
	return minVersion, cipherSuites, nil
}

// SetTLSSecurityConfiguration appends or overwrites --tls-cipher-suites and --tls-min-version
// flags on a container args slice based on the cluster TLS profile.
func SetTLSSecurityConfiguration(ctx context.Context, args []string, tlsCipherSuitesArg string, minTLSversionArg string) ([]string, error) {
	profileSpec, err := GetOrCreateTLSProfileSpec(ctx)
	if err != nil {
		if isTest, parseErr := strconv.ParseBool(os.Getenv("UNIT_TEST")); parseErr == nil && isTest {
			log.Info("running unit test, skipping TLS profile adherence")
			return args, nil
		}
		log.Error(err, "unable to get TLS security configuration")
		return nil, err
	}

	cipherSuites := strings.Join(libgocrypto.OpenSSLToIANACipherSuites(profileSpec.Ciphers), ",")
	args = setArg(args, tlsCipherSuitesArg, cipherSuites)
	args = setArg(args, minTLSversionArg, string(profileSpec.MinTLSVersion))
	return args, nil
}

func setArg(args []string, argName string, argValue string) []string {
	found := false
	for i, arg := range args {
		if arg == argName || (argName[len(argName)-1] == '=' && strings.HasPrefix(arg, argName)) {
			args[i] = argName + argValue
			found = true
		}
	}
	if !found {
		args = append(args, argName+argValue)
	}
	return args
}

func getOrCreateOCPConfigCRClient() (client.Client, error) {
	if ocpConfigClient != nil {
		return ocpConfigClient, nil
	}

	config, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
	}

	scheme := runtime.NewScheme()
	if err = ocinfrav1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add config.openshift.io/v1 to scheme: %w", err)
	}

	ocpConfigClient, err = client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create OCP config client: %w", err)
	}

	return ocpConfigClient, nil
}
