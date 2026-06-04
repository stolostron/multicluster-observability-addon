#!/usr/bin/env bash
# Deploy test workloads with CPU/memory requests, limits, and ResourceQuotas
# so that right-sizing recording rules produce data for all metric dimensions.
#
# Usage:
#   ./deploy-test-workloads.sh          # create
#   ./deploy-test-workloads.sh cleanup  # delete

set -euo pipefail

NS="rs-test-workloads"

cleanup() {
  echo "Cleaning up namespace ${NS}..."
  kubectl delete namespace "${NS}" --ignore-not-found --wait=false
  echo "Done."
}

if [[ "${1:-}" == "cleanup" ]]; then
  cleanup
  exit 0
fi

echo "Creating namespace ${NS}..."
kubectl create namespace "${NS}" --dry-run=client -o yaml | kubectl apply -f -

echo "Creating ResourceQuota (provides request_hard metrics)..."
kubectl apply -f - <<EOF
apiVersion: v1
kind: ResourceQuota
metadata:
  name: rs-test-quota
  namespace: ${NS}
spec:
  hard:
    requests.cpu: "8"
    requests.memory: 16Gi
    limits.cpu: "16"
    limits.memory: 32Gi
EOF

echo "Deploying test workloads..."

# Workload 1: nginx with moderate CPU/memory
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-frontend
  namespace: ${NS}
spec:
  replicas: 3
  selector:
    matchLabels:
      app: web-frontend
  template:
    metadata:
      labels:
        app: web-frontend
    spec:
      containers:
      - name: nginx
        image: registry.access.redhat.com/ubi9/nginx-124:latest
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
        ports:
        - containerPort: 8080
EOF

# Workload 2: stress container with higher usage
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-backend
  namespace: ${NS}
spec:
  replicas: 2
  selector:
    matchLabels:
      app: api-backend
  template:
    metadata:
      labels:
        app: api-backend
    spec:
      containers:
      - name: ubi
        image: registry.access.redhat.com/ubi9/ubi-minimal:latest
        command: ["sleep", "infinity"]
        resources:
          requests:
            cpu: 250m
            memory: 256Mi
          limits:
            cpu: "1"
            memory: 1Gi
EOF

# Workload 3: batch job style
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: batch-worker
  namespace: ${NS}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: batch-worker
  template:
    metadata:
      labels:
        app: batch-worker
    spec:
      containers:
      - name: worker
        image: registry.access.redhat.com/ubi9/ubi-minimal:latest
        command: ["sleep", "infinity"]
        resources:
          requests:
            cpu: 500m
            memory: 512Mi
          limits:
            cpu: "2"
            memory: 2Gi
EOF

echo ""
echo "Waiting for pods to be ready..."
kubectl wait --for=condition=available deployment/web-frontend deployment/api-backend deployment/batch-worker \
  -n "${NS}" --timeout=120s 2>/dev/null || true

echo ""
echo "=== Deployed Resources ==="
kubectl get resourcequota -n "${NS}"
echo ""
kubectl get deployments -n "${NS}"
echo ""
kubectl get pods -n "${NS}"
echo ""
echo "Recording rules will start producing data within 5 minutes."
echo "Check with: kubectl exec -n openshift-monitoring prometheus-k8s-0 -c prometheus -- \\"
echo "  curl -s --data-urlencode 'query=acm_rs:namespace:cpu_limit:5m{namespace=\"${NS}\"}' http://localhost:9090/api/v1/query"
