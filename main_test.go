// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewControllerCommandFlags(t *testing.T) {
	cmd := newControllerCommand()
	require.NotNil(t, cmd)

	flags := cmd.Flags()

	// Verify metrics-bind-address flag
	bindAddrFlag := flags.Lookup("metrics-bind-address")
	require.NotNil(t, bindAddrFlag)
	assert.Equal(t, "127.0.0.1:8080", bindAddrFlag.DefValue)

	// Test flag override
	err := cmd.ParseFlags([]string{
		"--metrics-bind-address=0.0.0.0:8084",
	})
	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0:8084", metricsBindAddress)
}
