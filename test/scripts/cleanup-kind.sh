#!/usr/bin/env bash
set -euo pipefail

if [ -z "${CONTAINER_ENGINE:-}" ]; then
    if command -v podman >/dev/null 2>&1; then
        CONTAINER_ENGINE="podman"
    elif command -v docker >/dev/null 2>&1; then
        CONTAINER_ENGINE="docker"
    fi
fi
if [ "${CONTAINER_ENGINE:-}" = "podman" ]; then
    export KIND_EXPERIMENTAL_PROVIDER=podman
fi
CLUSTER_NAME="${CLUSTER_NAME:-mcoa-e2e}"
KUBECONFIG="${KUBECONFIG:-/tmp/mcoa-e2e.kubeconfig}"

if [ -z "${KIND_BIN:-}" ] || ! command -v "${KIND_BIN}" >/dev/null 2>&1; then
    for candidate in "$(which kind 2>/dev/null)" "/usr/local/bin/kind" "$HOME/go/bin/kind"; do
        if [ -x "${candidate}" ]; then
            KIND_BIN="${candidate}"
            break
        fi
    done
fi
KIND_BIN="${KIND_BIN:-kind}"

echo "=== Deleting KinD Cluster '${CLUSTER_NAME}' ==="
"${KIND_BIN}" delete cluster --name "${CLUSTER_NAME}" || true

echo "=== Removing temporary kubeconfig and join files ==="
rm -f "${KUBECONFIG}" /tmp/mcoa-join.sh

echo "=== Cleanup Complete! ==="
