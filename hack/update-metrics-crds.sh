#!/usr/bin/env bash

set -euo pipefail

# This script updates the Prometheus Operator CRDs from the rhobs/obo-prometheus-operator repository.

# The version is pinned to what is defined in go.mod.
VERSION=$(grep 'github.com/rhobs/obo-prometheus-operator' go.mod | awk '{print $2}')
if [ -z "$VERSION" ]; then
    echo "Error: Could not find obo-prometheus-operator version in go.mod"
    exit 1
fi

echo "Using obo-prometheus-operator version: ${VERSION}"

# Corrected base URL for the raw CRD files on GitHub.
BASE_URL="https://raw.githubusercontent.com/rhobs/obo-prometheus-operator/refs/tags/${VERSION}/example/prometheus-operator-crd"

# Destination directory for the CRDs in the Helm chart.
DEST_DIR="internal/addon/manifests/charts/mcoa/charts/metrics/templates/coo/crds"

if [ ! -d "${DEST_DIR}" ]; then
    echo "Error: Destination directory ${DEST_DIR} not found."
    exit 1
fi

# Loop through the yaml files in the destination directory.
for dest_path in "${DEST_DIR}"/*.yaml; do
    if [ ! -f "${dest_path}" ]; then
        continue
    fi

    local_file=$(basename "${dest_path}")
    # The remote repository uses 'rhobs' instead of 'coreos.com' in the filenames.
    remote_file=${local_file/coreos.com/rhobs}
    url="${BASE_URL}/${remote_file}"

    echo "Updating ${local_file} from ${url}..."

    # Download the new CRD content.
    crd_content=$(curl -sL "${url}")
    if [ -z "$crd_content" ] || [[ "$crd_content" == "404: Not Found" ]]; then
        echo "Error: Failed to download CRD from ${url}"
        exit 1
    fi

    # Preserve the first and last lines (Helm directives).
    first_line=$(head -n 1 "${dest_path}")
    last_line=$(tail -n 1 "${dest_path}")

    # Write the new content, preserving the Helm directives.
    {
        echo "${first_line}"
        echo "${crd_content}"
        echo "${last_line}"
    } > "${dest_path}"

    echo "Successfully updated ${dest_path}"
done

echo "All CRDs updated successfully."