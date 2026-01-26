#!/usr/bin/env bash
# Copyright Contributors to the Open Cluster Management project
# Copyright (c) 2026 Red Hat, Inc.

set -euo pipefail

# Verifies that RHEL versions are consistent across FROM statement, name label, and cpe label
# in Dockerfile.Konflux. This prevents mismatches when updating RHEL versions.
#
# Usage: ./hack/verify-dockerfile-labels.sh
# Exit code: 0 if all checks pass, 1 if any mismatch detected

echo "Verifying Dockerfile.Konflux label consistency..."
echo ""

DOCKERFILE="Dockerfile.Konflux"

if [ ! -f "$DOCKERFILE" ]; then
  echo "❌ ERROR: $DOCKERFILE not found"
  exit 1
fi

# Extract RHEL version from FROM statement (e.g., "FROM ubi9/ubi-minimal" -> "9")
from_rhel=$(grep "^FROM.*ubi" "$DOCKERFILE" | sed -E 's/.*ubi([0-9]+).*/\1/' || echo "")

# Extract RHEL version from name label (e.g., "name=rhacm2/foo-rhel9" -> "9")
name_rhel=$(grep 'name="rhacm2/.*-rhel' "$DOCKERFILE" | sed -E 's/.*-rhel([0-9]+).*/\1/' || echo "")

# Extract RHEL version from cpe label (e.g., "cpe=cpe:/a:redhat:acm:2.16::el9" -> "9")
cpe_rhel=$(grep 'cpe="cpe:/a:redhat:acm' "$DOCKERFILE" | sed -E 's/.*::el([0-9]+).*/\1/' || echo "")

# Validate all versions were extracted
if [ -z "$from_rhel" ]; then
  echo "❌ ERROR: Could not extract RHEL version from FROM statement"
  exit 1
fi

if [ -z "$name_rhel" ]; then
  echo "❌ ERROR: Could not extract RHEL version from name label"
  exit 1
fi

if [ -z "$cpe_rhel" ]; then
  echo "❌ ERROR: Could not extract RHEL version from cpe label"
  exit 1
fi

# Check for consistency
if [ "$from_rhel" != "$name_rhel" ] || [ "$from_rhel" != "$cpe_rhel" ]; then
  echo "❌ MISMATCH: FROM uses ubi${from_rhel}, name uses rhel${name_rhel}, cpe uses el${cpe_rhel}"
  echo ""
  echo "When updating RHEL versions, you must update:"
  echo '  1. FROM registry.access.redhat.com/ubi{VERSION}/ubi-minimal:latest'
  echo '  2. name="rhacm2/multicluster-observability-addon-rhel{VERSION}"'
  echo '  3. cpe="cpe:/a:redhat:acm:{ACM_VER}::el{VERSION}"'
  exit 1
fi

# Verify name label format (must not use $IMAGE_NAME variable)
if grep -q 'name="\$IMAGE_NAME"' "$DOCKERFILE"; then
  echo "❌ ERROR: name label uses \$IMAGE_NAME variable instead of hardcoded value"
  exit 1
fi

# Verify cpe label exists and has correct format
if ! grep -q 'cpe="cpe:/a:redhat:acm:' "$DOCKERFILE"; then
  echo "❌ ERROR: cpe label missing or has incorrect format"
  exit 1
fi

echo "✅ All labels consistent with RHEL ${from_rhel}"
echo ""
exit 0
