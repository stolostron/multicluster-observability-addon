#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

if [ -z "${CONTAINER_ENGINE:-}" ]; then
    if command -v podman >/dev/null 2>&1; then
        CONTAINER_ENGINE="podman"
    elif command -v docker >/dev/null 2>&1; then
        CONTAINER_ENGINE="docker"
    else
        echo "Error: Neither podman nor docker found." >&2
        exit 1
    fi
fi
if [ "${CONTAINER_ENGINE}" = "podman" ]; then
    export KIND_EXPERIMENTAL_PROVIDER=podman
fi

CLUSTER_NAME="${CLUSTER_NAME:-mcoa-e2e}"
KIND_NODE_IMAGE="${KIND_NODE_IMAGE:-kindest/node:v1.37.0@sha256:a1ed56cfb0e7b93589bdf97c8cd566405a265939e3620fc4f5de89adff580ae5}"
SERVICE_CA_OPERATOR_BRANCH="${SERVICE_CA_OPERATOR_BRANCH:-release-4.18}"
PROMETHEUS_VERSION="${PROMETHEUS_VERSION:-$(go list -m -f '{{.Version}}' github.com/prometheus/prometheus 2>/dev/null | sed -E 's/^v0\.3/v3./; s/^v0\./v2./')}"
PROMETHEUS_VERSION="${PROMETHEUS_VERSION:-v3.15.1}"
RHOBS_OPERATOR_VERSION="${RHOBS_OPERATOR_VERSION:-$(go list -m -f '{{.Version}}' github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring 2>/dev/null || echo "v0.90.1-rhobs1")}"
ENDPOINT_MONITORING_OPERATOR_VERSION="${ENDPOINT_MONITORING_OPERATOR_VERSION:-5.1.0-SNAPSHOT-2026-10-12-16-50-23}"
KUBE_RBAC_PROXY_VERSION="${KUBE_RBAC_PROXY_VERSION:-v0.23.1}"
E2E_KUBECONFIG="${E2E_KUBECONFIG:-/tmp/mcoa-e2e.kubeconfig}"
export KUBECONFIG="${E2E_KUBECONFIG}"

if [ -z "${KIND_BIN:-}" ] || ! command -v "${KIND_BIN}" >/dev/null 2>&1; then
    for candidate in "$(which kind 2>/dev/null)" "/usr/local/bin/kind" "$HOME/go/bin/kind"; do
        if [ -x "${candidate}" ]; then
            KIND_BIN="${candidate}"
            break
        fi
    done
fi

if [ -z "${CLUSTERADM_BIN:-}" ] || ! command -v "${CLUSTERADM_BIN}" >/dev/null 2>&1; then
    for candidate in "$(which clusteradm 2>/dev/null)" "/usr/local/bin/clusteradm" "$HOME/go/bin/clusteradm"; do
        if [ -x "${candidate}" ]; then
            CLUSTERADM_BIN="${candidate}"
            break
        fi
    done
fi

if ! command -v "${KIND_BIN}" >/dev/null 2>&1; then
    echo "Error: kind CLI not found. Please install kind or set KIND_BIN." >&2
    exit 1
fi
if ! command -v "${CLUSTERADM_BIN}" >/dev/null 2>&1; then
    echo "Error: clusteradm CLI not found. Please install clusteradm or set CLUSTERADM_BIN." >&2
    exit 1
fi
if ! command -v openssl >/dev/null 2>&1; then
    echo "Error: openssl CLI not found." >&2
    exit 1
fi

image_exists() {
    "${CONTAINER_ENGINE}" image inspect "$1" >/dev/null 2>&1
}

image_matches_version() {
    local img="$1"
    local expected="$2"
    local actual
    actual="$("${CONTAINER_ENGINE}" image inspect --format '{{index .Config.Labels "kind.e2e.source-version"}}' "${img}" 2>/dev/null || true)"
    [ "${actual}" = "${expected}" ]
}

echo "=== 1. Checking KinD Cluster '${CLUSTER_NAME}' ==="
if "${CONTAINER_ENGINE}" ps -a --filter "name=^${CLUSTER_NAME}-control-plane\$" -q 2>/dev/null | grep -q .; then
    echo "Cluster '${CLUSTER_NAME}' already exists. Deleting to start clean..."
    "${KIND_BIN}" delete cluster --name "${CLUSTER_NAME}"
fi

echo "=== 2. Creating KinD Cluster '${CLUSTER_NAME}' ==="
"${KIND_BIN}" create cluster \
    --name "${CLUSTER_NAME}" \
    --config "${SCRIPT_DIR}/kind-config.yaml" \
    --image "${KIND_NODE_IMAGE}" \
    --kubeconfig "${KUBECONFIG}"

echo "=== 2b. Labeling Node for master-role compatibility ==="
kubectl label node --all node-role.kubernetes.io/master="" --overwrite

echo "=== 3. Preparing Container Images ==="
if [ -z "${PLATFORM:-}" ]; then
    ARCH="$(uname -m)"
    case "${ARCH}" in
        x86_64) PLATFORM="amd64" ;;
        aarch64|arm64) PLATFORM="arm64" ;;
        *) PLATFORM="${ARCH}" ;;
    esac
fi

echo "Building local MCOA image (localhost/mcoa:e2e)..."
"${CONTAINER_ENGINE}" build --platform "linux/${PLATFORM}" -t localhost/mcoa:e2e "${REPO_ROOT}"

if ! image_exists localhost/origin-service-ca-operator:kind-e2e || ! image_matches_version localhost/origin-service-ca-operator:kind-e2e "${SERVICE_CA_OPERATOR_BRANCH}"; then
    echo "Building native localhost/origin-service-ca-operator:kind-e2e (branch: ${SERVICE_CA_OPERATOR_BRANCH})..."
    cat <<'EOF' | "${CONTAINER_ENGINE}" build --platform "linux/${PLATFORM}" --build-arg SERVICE_CA_OPERATOR_BRANCH="${SERVICE_CA_OPERATOR_BRANCH}" -t localhost/origin-service-ca-operator:kind-e2e -
ARG SERVICE_CA_OPERATOR_BRANCH=release-4.18
FROM registry.access.redhat.com/ubi9/go-toolset:latest AS builder
ARG SERVICE_CA_OPERATOR_BRANCH
USER 0
RUN git clone --depth 1 -b ${SERVICE_CA_OPERATOR_BRANCH} https://github.com/openshift/service-ca-operator.git /src
WORKDIR /src
ENV GOTOOLCHAIN=local
RUN CGO_ENABLED=0 go build -mod=vendor -o /bin/service-ca-operator ./cmd/service-ca-operator && \
    chmod 0755 /bin/service-ca-operator

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
ARG SERVICE_CA_OPERATOR_BRANCH
LABEL kind.e2e=true kind.e2e.source-version="${SERVICE_CA_OPERATOR_BRANCH}"
COPY --from=builder /bin/service-ca-operator /usr/bin/service-ca-operator
RUN chmod 0755 /usr/bin/service-ca-operator && \
    ln -sf /usr/bin/service-ca-operator /usr/local/bin/service-ca-operator
USER 1001
ENTRYPOINT ["/usr/bin/service-ca-operator"]
EOF
fi

if ! image_exists localhost/prometheus:kind-e2e || ! image_matches_version localhost/prometheus:kind-e2e "${PROMETHEUS_VERSION}"; then
    echo "Building localhost/prometheus:kind-e2e (${PROMETHEUS_VERSION}, non-root)..."
    if ! image_exists "quay.io/prometheus/prometheus:${PROMETHEUS_VERSION}"; then
        "${CONTAINER_ENGINE}" pull "quay.io/prometheus/prometheus:${PROMETHEUS_VERSION}"
    fi
    printf "FROM quay.io/prometheus/prometheus:%s\nLABEL kind.e2e=true kind.e2e.source-version=%s\nUSER 65534\n" "${PROMETHEUS_VERSION}" "${PROMETHEUS_VERSION}" | "${CONTAINER_ENGINE}" build --platform "linux/${PLATFORM}" -t localhost/prometheus:kind-e2e -
fi

if ! image_exists localhost/obo-prometheus-operator:kind-e2e || ! image_matches_version localhost/obo-prometheus-operator:kind-e2e "${RHOBS_OPERATOR_VERSION}"; then
    echo "Building localhost/obo-prometheus-operator:kind-e2e (${RHOBS_OPERATOR_VERSION})..."
    OBO_SRC="${OBO_SRC:-quay.io/rhobs/obo-prometheus-operator:${RHOBS_OPERATOR_VERSION}}"
    if ! image_exists "${OBO_SRC}"; then
        # Quay only publishes linux/amd64 manifests for RHOBS images.
        # On arm64 hosts, omitting --platform causes Docker/Podman to fail with:
        # "no image found in manifest list for architecture arm64".
        # We try the host platform first, falling back to linux/amd64 (run via Rosetta/QEMU emulation).
        "${CONTAINER_ENGINE}" pull --platform "linux/${PLATFORM}" "${OBO_SRC}" 2>/dev/null || \
            "${CONTAINER_ENGINE}" pull --platform linux/amd64 "${OBO_SRC}"
    fi
    printf "FROM %s\nLABEL kind.e2e=true kind.e2e.source-version=%s\n" "${OBO_SRC}" "${RHOBS_OPERATOR_VERSION}" | "${CONTAINER_ENGINE}" build --platform linux/amd64 -t localhost/obo-prometheus-operator:kind-e2e -
fi

if ! image_exists localhost/prometheus-config-reloader:kind-e2e || ! image_matches_version localhost/prometheus-config-reloader:kind-e2e "${RHOBS_OPERATOR_VERSION}"; then
    echo "Building localhost/prometheus-config-reloader:kind-e2e (${RHOBS_OPERATOR_VERSION})..."
    RELOADER_SRC="${RELOADER_SRC:-quay.io/rhobs/obo-prometheus-config-reloader:${RHOBS_OPERATOR_VERSION}}"
    if ! image_exists "${RELOADER_SRC}"; then
        # Quay only publishes linux/amd64 manifests for RHOBS images.
        # On arm64 hosts, omitting --platform causes Docker/Podman to fail with:
        # "no image found in manifest list for architecture arm64".
        # We try the host platform first, falling back to linux/amd64 (run via Rosetta/QEMU emulation).
        "${CONTAINER_ENGINE}" pull --platform "linux/${PLATFORM}" "${RELOADER_SRC}" 2>/dev/null || \
            "${CONTAINER_ENGINE}" pull --platform linux/amd64 "${RELOADER_SRC}"
    fi
    printf "FROM %s\nLABEL kind.e2e=true kind.e2e.source-version=%s\nUSER 65534\n" "${RELOADER_SRC}" "${RHOBS_OPERATOR_VERSION}" | "${CONTAINER_ENGINE}" build --platform linux/amd64 -t localhost/prometheus-config-reloader:kind-e2e -
fi

if ! image_exists localhost/endpoint-monitoring-operator:kind-e2e || ! image_matches_version localhost/endpoint-monitoring-operator:kind-e2e "${ENDPOINT_MONITORING_OPERATOR_VERSION}"; then
    echo "Building localhost/endpoint-monitoring-operator:kind-e2e (${ENDPOINT_MONITORING_OPERATOR_VERSION})..."
    ENDPOINT_MONITORING_OPERATOR_SRC="${ENDPOINT_MONITORING_OPERATOR_SRC:-quay.io/stolostron/endpoint-monitoring-operator:${ENDPOINT_MONITORING_OPERATOR_VERSION}}"
    if ! image_exists "${ENDPOINT_MONITORING_OPERATOR_SRC}"; then
        "${CONTAINER_ENGINE}" pull --platform "linux/${PLATFORM}" "${ENDPOINT_MONITORING_OPERATOR_SRC}" 2>/dev/null || \
            "${CONTAINER_ENGINE}" pull --platform linux/amd64 "${ENDPOINT_MONITORING_OPERATOR_SRC}"
    fi
    printf "FROM %s\nLABEL kind.e2e=true kind.e2e.source-version=%s\n" "${ENDPOINT_MONITORING_OPERATOR_SRC}" "${ENDPOINT_MONITORING_OPERATOR_VERSION}" | "${CONTAINER_ENGINE}" build --platform linux/amd64 -t localhost/endpoint-monitoring-operator:kind-e2e -
fi

echo "=== 4. Loading Images into KinD ==="
IMAGES_TO_LOAD=(
    localhost/mcoa:e2e
    localhost/origin-service-ca-operator:kind-e2e
    localhost/obo-prometheus-operator:kind-e2e
    localhost/prometheus-config-reloader:kind-e2e
    localhost/prometheus:kind-e2e
    localhost/endpoint-monitoring-operator:kind-e2e
    docker.io/library/nginx:alpine
    "quay.io/brancz/kube-rbac-proxy:${KUBE_RBAC_PROXY_VERSION}"
)

for img in "${IMAGES_TO_LOAD[@]}"; do
    if ! image_exists "${img}"; then
        echo "Pulling ${img}..."
        "${CONTAINER_ENGINE}" pull "${img}"
    fi
done

echo "Loading ${#IMAGES_TO_LOAD[@]} images individually into KinD control plane..."
for img in "${IMAGES_TO_LOAD[@]}"; do
    echo "  Loading ${img}..."
    "${CONTAINER_ENGINE}" save "${img}" | "${CONTAINER_ENGINE}" exec -i "${CLUSTER_NAME}-control-plane" ctr -n k8s.io images import --all-platforms -
done

echo "Verifying loaded images in KinD node:"
"${CONTAINER_ENGINE}" exec "${CLUSTER_NAME}-control-plane" crictl images

echo "=== 5. Initializing OCM Hub via clusteradm ==="
JOIN_FILE="/tmp/mcoa-join.sh"
rm -f "${JOIN_FILE}"

"${CLUSTERADM_BIN}" init --wait --output-join-command-file="${JOIN_FILE}" --kubeconfig "${KUBECONFIG}"

echo "Waiting for OCM webhooks in open-cluster-management-hub..."
kubectl wait -n open-cluster-management-hub --for=condition=Available deployment --all --timeout=120s

echo "=== 6. Joining local-cluster spoke to Hub ==="
JOIN_CMD=$(cat "${JOIN_FILE}")
MODIFIED_JOIN_CMD=$(echo "${JOIN_CMD}" | sed -E "s|^clusteradm|${CLUSTERADM_BIN}|" | sed -E 's|--cluster-name.*$|--cluster-name local-cluster|')
MODIFIED_JOIN_CMD="${MODIFIED_JOIN_CMD} --force-internal-endpoint-lookup=true --wait --kubeconfig='${KUBECONFIG}'"

echo "Executing join command: ${MODIFIED_JOIN_CMD}"
eval "${MODIFIED_JOIN_CMD}"

echo "=== 7. Accepting local-cluster join request ==="
"${CLUSTERADM_BIN}" accept --clusters local-cluster --wait --kubeconfig "${KUBECONFIG}"

echo "=== 8. Labeling ManagedCluster local-cluster as OpenShift ==="
kubectl label managedcluster local-cluster \
    vendor=OpenShift \
    openshiftVersion=5.0.0-ec.3 \
    clusterID=cc983d7a-2cd2-4171-b55c-d978e587a978 \
    local-cluster=true \
    --overwrite

echo "=== 9. Installing CRDs ==="
kubectl apply --server-side --force-conflicts -f "${REPO_ROOT}/test/crds/"
kubectl wait --for condition=established --timeout=30s \
    crd/clusterversions.config.openshift.io \
    crd/servicecas.operator.openshift.io \
    crd/scrapeconfigs.monitoring.rhobs \
    crd/prometheusagents.monitoring.rhobs \
    crd/servicemonitors.monitoring.rhobs \
    crd/podmonitors.monitoring.rhobs \
    crd/probes.monitoring.rhobs \
    crd/prometheuses.monitoring.rhobs

echo "=== 10. Deploying OpenShift service-ca Operator & Service ==="
kubectl apply -f "${SCRIPT_DIR}/../manifests/01-service-ca.yaml"
echo "Waiting for service-ca-operator deployment..."
kubectl wait -n openshift-service-ca-operator --for=condition=Available deployment/service-ca-operator --timeout=120s

echo "Waiting for service-ca deployment in openshift-service-ca..."
for _ in {1..60}; do
    if kubectl get deployment -n openshift-service-ca service-ca &>/dev/null; then
        break
    fi
    sleep 2
done
kubectl wait -n openshift-service-ca --for=condition=Available deployment/service-ca --timeout=120s

echo "=== 11. Deploying Hub Setup (Prometheus receiver + TLS secrets + ScrapeConfig) ==="
kubectl apply -f "${SCRIPT_DIR}/../manifests/02-hub-setup.yaml"

echo "Generating dynamic test certificates and Hub secrets..."
CERT_DIR="$(mktemp -d)"
openssl req -x509 -newkey rsa:2048 -nodes -keyout "${CERT_DIR}/ca.key" -out "${CERT_DIR}/ca.crt" -days 365 -subj "/CN=observability-ca" 2>/dev/null
openssl req -newkey rsa:2048 -nodes -keyout "${CERT_DIR}/tls.key" -out "${CERT_DIR}/tls.csr" -subj "/CN=observability-client" 2>/dev/null
openssl x509 -req -in "${CERT_DIR}/tls.csr" -CA "${CERT_DIR}/ca.crt" -CAkey "${CERT_DIR}/ca.key" -CAcreateserial -out "${CERT_DIR}/tls.crt" -days 365 2>/dev/null

kubectl create secret generic observability-managed-cluster-certs \
    -n open-cluster-management-observability \
    --from-file=ca.crt="${CERT_DIR}/ca.crt" \
    --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic observability-controller-open-cluster-management.io-observability-signer-client-cert \
    -n open-cluster-management-observability \
    --from-file=tls.crt="${CERT_DIR}/tls.crt" \
    --from-file=tls.key="${CERT_DIR}/tls.key" \
    --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic observability-alertmanager-accessor \
    -n open-cluster-management-observability \
    --from-literal=token="dummy-token" \
    --dry-run=client -o yaml | kubectl apply -f -

rm -rf "${CERT_DIR}"

echo "Waiting for hub-prometheus deployment..."
kubectl wait -n open-cluster-management-observability --for=condition=Available deployment/hub-prometheus --timeout=120s

echo "=== 12. Deploying Spoke CMO (prometheus-k8s with dynamic serving cert) ==="
kubectl apply -f "${SCRIPT_DIR}/../manifests/03-spoke-cmo.yaml"
echo "Waiting for prometheus-k8s deployment..."
kubectl wait -n openshift-monitoring --for=condition=Available deployment/prometheus-k8s --timeout=120s

echo "=== 13. Deploying MCOA Operator ==="
# Applying 04-mcoa.yaml creates the ClusterManagementAddOn (CMAO) resource, which triggers
# OCM's admission/conversion webhook (cluster-manager-addon-webhook in open-cluster-management-hub).
# Even after the webhook deployment reports Available, kube-apiserver service routing,
# TLS handshakes, or endpoint propagation can briefly reject connections with 'connection refused'.
# We retry up to 10 times with backoff to absorb this transient webhook startup latency.
APPLIED=0
for i in {1..10}; do
    if kubectl apply -f "${SCRIPT_DIR}/../manifests/04-mcoa.yaml"; then
        APPLIED=1
        break
    fi
    echo "Retrying kubectl apply for 04-mcoa.yaml in 3s (attempt ${i}/10)..."
    sleep 3
done
if [ "${APPLIED}" -eq 0 ]; then
    echo "Error: Failed to apply 04-mcoa.yaml after 10 attempts" >&2
    exit 1
fi
echo "Waiting for multicluster-observability-addon-manager deployment..."
kubectl wait -n open-cluster-management-observability --for=condition=Available deployment/multicluster-observability-addon-manager --timeout=120s

echo "=== KinD E2E Environment Setup Complete! ==="
