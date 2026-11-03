// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License, Version 2.0

// mco-thanos-migrate is a one-time migration tool that carries user-defined
// Thanos configuration from a MultiClusterObservability CR (MCO) to the Thanos
// CRs managed by the multicluster-observability-addon (MCOA).
//
// It runs in two steps, typically around an upgrade:
//
//  1. capture: read the MultiClusterObservability CR (from the cluster or a
//     saved YAML file) and extract the user-declared Thanos settings into an
//     intermediate migration file. Run this BEFORE the MCO Thanos stack is
//     removed.
//  2. apply: read the migration file and patch the MCOA Thanos CRs
//     (ThanosCompact/Receive/Query/Ruler/Store named "mcoa"). Run this AFTER
//     the MCOA Thanos CRs have been created. Edits made this way persist
//     because the addon deploys those CRs with the OnSpokeChange update
//     strategy on .spec.
//
// See README.md in this directory for the full field mapping and known gaps.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "capture":
		err = runCapture(os.Args[2:])
	case "apply":
		err = runApply(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `mco-thanos-migrate: migrate user-defined Thanos config from MCO to MCOA

Usage:
  mco-thanos-migrate capture [flags]        extract MCO Thanos settings to a migration file
  mco-thanos-migrate apply [flags]          apply a migration file to the MCOA Thanos CRs

Capture flags:
  -f string            path to a MultiClusterObservability CR YAML (offline mode;
                       omit to read the CR from the current cluster)
  -o string            output migration file (default "thanos-migration.yaml")
  --mco-name string    name of the MultiClusterObservability CR (default "observability")
  --mco-api-version    API version of the MultiClusterObservability CR (default "v1beta2")
  --namespace string   namespace of the MultiClusterObservability CR
                       (default "open-cluster-management-observability")

Apply flags:
  -f string            path to the migration file produced by capture (required)
  --namespace string   namespace of the MCOA Thanos CRs
                       (default "open-cluster-management-observability")
  --dry-run            compute and print the changes without writing to the cluster
  --include-sidecars   also copy MCO sidecar containers into the Thanos CRs
                       (additionalContainers); skipped by default
`)
}
