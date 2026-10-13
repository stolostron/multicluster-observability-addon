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
KUBECONFIG="${KUBECONFIG:-/tmp/mcoa-e2e.kubeconfig}"
export KUBECONFIG

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

echo "=== 1. Checking KinD Cluster '${CLUSTER_NAME}' ==="
if "${CONTAINER_ENGINE}" ps -a --filter "name=^${CLUSTER_NAME}-control-plane\$" -q 2>/dev/null | grep -q .; then
    echo "Cluster '${CLUSTER_NAME}' already exists. Deleting to start clean..."
    "${KIND_BIN}" delete cluster --name "${CLUSTER_NAME}"
fi

echo "=== 2. Creating KinD Cluster '${CLUSTER_NAME}' ==="
"${KIND_BIN}" create cluster \
    --name "${CLUSTER_NAME}" \
    --config "${SCRIPT_DIR}/kind-config.yaml" \
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

if ! image_exists localhost/origin-service-ca-operator:kind-e2e; then
    echo "Building native localhost/origin-service-ca-operator:kind-e2e..."
    cat <<'EOF' | "${CONTAINER_ENGINE}" build --platform "linux/${PLATFORM}" -t localhost/origin-service-ca-operator:kind-e2e -
FROM registry.access.redhat.com/ubi9/go-toolset:latest AS builder
USER 0
RUN git clone --depth 1 -b release-4.16 https://github.com/openshift/service-ca-operator.git /src
WORKDIR /src
ENV GOTOOLCHAIN=local
RUN CGO_ENABLED=0 go build -mod=vendor -o /bin/service-ca-operator ./cmd/service-ca-operator && \
    chmod 0755 /bin/service-ca-operator

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
COPY --from=builder /bin/service-ca-operator /usr/bin/service-ca-operator
RUN chmod 0755 /usr/bin/service-ca-operator && \
    ln -sf /usr/bin/service-ca-operator /usr/local/bin/service-ca-operator
USER 1001
ENTRYPOINT ["/usr/bin/service-ca-operator"]
EOF
fi

if ! image_exists localhost/prometheus:v3.0.0; then
    echo "Building localhost/prometheus:v3.0.0 (non-root)..."
    if ! image_exists quay.io/prometheus/prometheus:v3.0.0; then
        "${CONTAINER_ENGINE}" pull quay.io/prometheus/prometheus:v3.0.0
    fi
    printf "FROM quay.io/prometheus/prometheus:v3.0.0\nUSER 65534\n" | "${CONTAINER_ENGINE}" build --platform "linux/${PLATFORM}" -t localhost/prometheus:v3.0.0 -
fi

if ! image_exists localhost/obo-prometheus-operator:v4.18; then
    echo "Building localhost/obo-prometheus-operator:v4.18..."
    OBO_SRC="quay.io/prometheus-operator/prometheus-operator:v0.83.0"
    if ! image_exists "${OBO_SRC}"; then
        "${CONTAINER_ENGINE}" pull "${OBO_SRC}"
    fi
    printf "FROM %s\nLABEL kind.e2e=true\n" "${OBO_SRC}" | "${CONTAINER_ENGINE}" build --platform "linux/${PLATFORM}" -t localhost/obo-prometheus-operator:v4.18 -
fi

if ! image_exists localhost/prometheus-config-reloader:v4.18; then
    if ! image_exists quay.io/prometheus-operator/prometheus-config-reloader:v0.83.0; then
        "${CONTAINER_ENGINE}" pull quay.io/prometheus-operator/prometheus-config-reloader:v0.83.0
    fi
    printf "FROM quay.io/prometheus-operator/prometheus-config-reloader:v0.83.0\nUSER 65534\n" | "${CONTAINER_ENGINE}" build --platform "linux/${PLATFORM}" -t localhost/prometheus-config-reloader:v4.18 -
fi

if ! image_exists localhost/endpoint-monitoring-operator:kind-e2e; then
    EMO_SRC="quay.io/stolostron/endpoint-monitoring-operator:5.1.0-SNAPSHOT-2026-10-12-16-50-23"
    if ! image_exists "${EMO_SRC}"; then
        "${CONTAINER_ENGINE}" pull "${EMO_SRC}"
    fi
    printf "FROM %s\nLABEL kind.e2e=true\n" "${EMO_SRC}" | "${CONTAINER_ENGINE}" build --platform "linux/${PLATFORM}" -t localhost/endpoint-monitoring-operator:kind-e2e -
fi

echo "=== 4. Loading Images into KinD ==="
IMAGES_TO_LOAD=(
    localhost/mcoa:e2e
    localhost/origin-service-ca-operator:kind-e2e
    localhost/obo-prometheus-operator:v4.18
    localhost/prometheus-config-reloader:v4.18
    localhost/prometheus:v3.0.0
    localhost/endpoint-monitoring-operator:kind-e2e
    quay.io/prometheus/prometheus:v2.53.0
    docker.io/library/nginx:alpine
    quay.io/brancz/kube-rbac-proxy:v0.15.0
)

for img in "${IMAGES_TO_LOAD[@]}"; do
    if ! image_exists "${img}"; then
        echo "Pulling ${img}..."
        "${CONTAINER_ENGINE}" pull "${img}"
    fi
done

echo "Streaming ${#IMAGES_TO_LOAD[@]} images directly into KinD control plane..."
"${CONTAINER_ENGINE}" save "${IMAGES_TO_LOAD[@]}" | "${CONTAINER_ENGINE}" exec -i "${CLUSTER_NAME}-control-plane" ctr -n k8s.io images import -

echo "=== 5. Initializing OCM Hub via clusteradm ==="
JOIN_FILE="/tmp/mcoa-join.sh"
rm -f "${JOIN_FILE}"

"${CLUSTERADM_BIN}" init --wait --output-join-command-file="${JOIN_FILE}" --kubeconfig "${KUBECONFIG}"

echo "=== 6. Joining local-cluster spoke to Hub ==="
JOIN_CMD=$(cat "${JOIN_FILE}")
MODIFIED_JOIN_CMD=$(echo "${JOIN_CMD}" | sed -E "s|^clusteradm|${CLUSTERADM_BIN}|" | sed -E 's|--cluster-name.*$|--cluster-name local-cluster|')
MODIFIED_JOIN_CMD="${MODIFIED_JOIN_CMD} --force-internal-endpoint-lookup=true --wait --kubeconfig=${KUBECONFIG}"

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
if [ ! -d "${REPO_ROOT}/deploy/crds" ] || [ -z "$(ls -A "${REPO_ROOT}/deploy/crds" 2>/dev/null)" ]; then
    echo "Downloading CRDs..."
    make -C "${REPO_ROOT}" download-crds
fi
kubectl apply --server-side -f "${REPO_ROOT}/deploy/crds/"
kubectl apply --server-side -f "${SCRIPT_DIR}/../manifests/crds/"
kubectl wait --for condition=established --timeout=30s \
    crd/clusterversions.config.openshift.io \
    crd/servicecas.operator.openshift.io \
    crd/scrapeconfigs.monitoring.rhobs \
    crd/prometheusagents.monitoring.rhobs

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
kubectl apply -f "${SCRIPT_DIR}/../manifests/04-mcoa.yaml"
echo "Waiting for multicluster-observability-addon-manager deployment..."
kubectl wait -n open-cluster-management-observability --for=condition=Available deployment/multicluster-observability-addon-manager --timeout=120s

echo "=== KinD E2E Environment Setup Complete! ==="
