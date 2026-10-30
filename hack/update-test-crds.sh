#!/usr/bin/env bash

# Copyright (c) 2026 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET_DIR="${REPO_ROOT}/test/crds"
TMP_DIR="$(mktemp -d)"

cleanup() {
    rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

mkdir -p "${TARGET_DIR}"

# Pinned Upstream Versions for remote operands
# Prometheus operators default dynamically to matching versions in go.mod
RHOBS_OPERATOR_VERSION="${RHOBS_OPERATOR_VERSION:-$(go list -m -f '{{.Version}}' github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring 2>/dev/null || echo "v0.90.1-rhobs1")}"
PROMETHEUS_OPERATOR_VERSION="${PROMETHEUS_OPERATOR_VERSION:-$(go list -m -f '{{.Version}}' github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring 2>/dev/null || echo "v0.90.1")}"
OTEL_OPERATOR_VERSION="${OTEL_OPERATOR_VERSION:-v0.100.1}"
CLO_VERSION="${CLO_VERSION:-release-6.0}"

download_crd() {
    local url="$1"
    local dest="$2"
    local filename
    filename="$(basename "${dest}")"
    echo "  -> Fetching ${filename}..."
    if ! curl -sSfL "${url}" -o "${TMP_DIR}/${filename}"; then
        echo "Error: Failed to download ${url}" >&2
        return 1
    fi
    if [ ! -s "${TMP_DIR}/${filename}" ]; then
        echo "Error: Downloaded file ${filename} is empty" >&2
        return 1
    fi
    cp "${TMP_DIR}/${filename}" "${dest}"
}

echo "=== 1. Syncing CRDs from Go module dependencies ==="

# 1.1 Open Cluster Management (OCM) Core APIs
OCM_API_DIR="$(go list -m -f '{{.Dir}}' open-cluster-management.io/api 2>/dev/null || true)"
if [ -n "${OCM_API_DIR}" ] && [ -d "${OCM_API_DIR}" ]; then
    echo "Syncing OCM API CRDs from ${OCM_API_DIR}..."
    cp "${OCM_API_DIR}/addon/v1beta1/0000_02_addon.open-cluster-management.io_addondeploymentconfigs.crd.yaml" \
       "${TARGET_DIR}/addondeploymentconfigs.addon.open-cluster-management.io.yaml"
    cp "${OCM_API_DIR}/addon/v1beta1/0000_00_addon.open-cluster-management.io_clustermanagementaddons.crd.yaml" \
       "${TARGET_DIR}/clustermanagementaddons.addon.open-cluster-management.io.yaml"
    cp "${OCM_API_DIR}/addon/v1beta1/0000_01_addon.open-cluster-management.io_managedclusteraddons.crd.yaml" \
       "${TARGET_DIR}/managedclusteraddons.addon.open-cluster-management.io.yaml"
    cp "${OCM_API_DIR}/cluster/v1/0000_00_clusters.open-cluster-management.io_managedclusters.crd.yaml" \
       "${TARGET_DIR}/managedclusters.cluster.open-cluster-management.io.yaml"
    cp "${OCM_API_DIR}/work/v1/0000_00_work.open-cluster-management.io_manifestworks.crd.yaml" \
       "${TARGET_DIR}/manifestworks.work.open-cluster-management.io.yaml"
else
    echo "Warning: open-cluster-management.io/api module not found in Go cache, skipping OCM CRD sync."
fi

# 1.2 HyperShift
HYPERSHIFT_DIR="$(go list -m -f '{{.Dir}}' github.com/openshift/hypershift/api 2>/dev/null || true)"
if [ -n "${HYPERSHIFT_DIR}" ] && [ -d "${HYPERSHIFT_DIR}" ]; then
    echo "Syncing HyperShift CRDs from ${HYPERSHIFT_DIR}..."
    HC_CRD="${HYPERSHIFT_DIR}/hypershift/v1beta1/zz_generated.featuregated-crd-manifests/hostedclusters.hypershift.openshift.io/AAA_ungated.yaml"
    if [ -f "${HC_CRD}" ]; then
        cp "${HC_CRD}" "${TARGET_DIR}/hostedclusters.hypershift.openshift.io.yaml"
        # Strip invalid trailing-slash annotation from HyperShift CRD
        # (Kubernetes >= 1.30 apiserver rejects "feature-gate.release.openshift.io/: true")
        if command -v yq >/dev/null 2>&1; then
            yq eval 'del(.metadata.annotations["feature-gate.release.openshift.io/"]) | del(.metadata.annotations | select(length == 0))' -i "${TARGET_DIR}/hostedclusters.hypershift.openshift.io.yaml"
        elif command -v python3 >/dev/null 2>&1; then
            python3 -c "
import sys, re
with open(sys.argv[1], 'r') as f:
    c = f.read()
c = re.sub(r'  annotations:\n    feature-gate\.release\.openshift\.io/: [^\n]+\n', '', c)
with open(sys.argv[1], 'w') as f:
    f.write(c)
" "${TARGET_DIR}/hostedclusters.hypershift.openshift.io.yaml"
        fi
    fi
else
    echo "Warning: github.com/openshift/hypershift/api module not found in Go cache, skipping HyperShift CRD sync."
fi

echo "=== 2. Downloading remote operand CRDs ==="

# 2.1 OpenTelemetry Operator
echo "OpenTelemetry Operator (${OTEL_OPERATOR_VERSION}):"
download_crd \
    "https://raw.githubusercontent.com/open-telemetry/opentelemetry-operator/${OTEL_OPERATOR_VERSION}/bundle/manifests/opentelemetry.io_opentelemetrycollectors.yaml" \
    "${TARGET_DIR}/opentelemetrycollectors.opentelemetry.io.yaml"
download_crd \
    "https://raw.githubusercontent.com/open-telemetry/opentelemetry-operator/${OTEL_OPERATOR_VERSION}/bundle/manifests/opentelemetry.io_instrumentations.yaml" \
    "${TARGET_DIR}/instrumentations.opentelemetry.io.yaml"

# 2.2 OpenShift Cluster Logging Operator
echo "OpenShift Cluster Logging Operator (${CLO_VERSION}):"
download_crd \
    "https://raw.githubusercontent.com/openshift/cluster-logging-operator/${CLO_VERSION}/bundle/manifests/observability.openshift.io_clusterlogforwarders.yaml" \
    "${TARGET_DIR}/clusterlogforwarders.observability.openshift.io.yaml"

# 2.3 RHOBS Prometheus Operator
echo "RHOBS Prometheus Operator (${RHOBS_OPERATOR_VERSION}):"
for crd in prometheusagents prometheuses podmonitors probes prometheusrules scrapeconfigs servicemonitors; do
    download_crd \
        "https://raw.githubusercontent.com/rhobs/obo-prometheus-operator/refs/tags/${RHOBS_OPERATOR_VERSION}/example/prometheus-operator-crd/monitoring.rhobs_${crd}.yaml" \
        "${TARGET_DIR}/${crd}.monitoring.rhobs.yaml"
done

# 2.4 CoreOS Prometheus Operator
echo "CoreOS Prometheus Operator (${PROMETHEUS_OPERATOR_VERSION}):"
for crd in prometheusrules servicemonitors; do
    download_crd \
        "https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/${PROMETHEUS_OPERATOR_VERSION}/example/prometheus-operator-crd/monitoring.coreos.com_${crd}.yaml" \
        "${TARGET_DIR}/${crd}.monitoring.coreos.com.yaml"
done

echo "=== 3. Validating CRD directory ==="
TOTAL_CRDS="$(find "${TARGET_DIR}" -maxdepth 1 -name "*.yaml" | wc -l | tr -d ' ')"
echo "Total CRDs in ${TARGET_DIR}: ${TOTAL_CRDS}"

echo "CRD sync complete."
