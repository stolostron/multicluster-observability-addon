#!/usr/bin/env bash
# test-rs-placement.sh — Comprehensive placement filtering test for right-sizing.
#
# Placement ConfigMaps (constant for parts 1-5):
#   rs-namespace-placement: targets env=prod
#   rs-virt-placement:      targets env=dev
#
# Prerequisites:
#   - oc logged in to the hub cluster
#   - MCOA deployed with in-memory placement filtering
#   - At least two managed clusters joined and available
#
# Usage:
#   ./hack/test-rs-placement.sh <cluster-a> <cluster-b>

set -euo pipefail

NAMESPACE="open-cluster-management-observability"
MANIFESTWORK_NAME="addon-multicluster-observability-addon-deploy-0"
MAX_WAIT=180
POLL=10

if [ $# -lt 2 ]; then
  echo "Usage: $0 <cluster-a> <cluster-b>"
  echo "  Runs 5 placement scenarios using two managed clusters."
  exit 1
fi

CLUSTER_A="$1"
CLUSTER_B="$2"
FAILURES=0
TOTAL_PARTS=0
PASSED_PARTS=0

# ── Helpers ──────────────────────────────────────────────────────────────────

get_rs_rules() {
  oc get manifestwork "${MANIFESTWORK_NAME}" -n "$1" -o json 2>/dev/null | \
    python3 -c "
import json, sys
def scan(obj):
    if isinstance(obj, dict):
        if obj.get('kind') == 'PrometheusRule' and 'acm-rs' in obj.get('metadata',{}).get('name',''):
            name = obj['metadata']['name']
            has_rules = len(obj.get('spec',{}).get('groups',[])) > 0
            print(f'{name} has_rules={has_rules}')
        for v in obj.values(): scan(v)
    elif isinstance(obj, list):
        for v in obj: scan(v)
scan(json.load(sys.stdin))
" 2>/dev/null
}

check() {
  local cluster="$1" rule="$2" expect="$3"
  local rules
  rules=$(get_rs_rules "${cluster}")
  if [ "${expect}" = "True" ]; then
    if echo "${rules}" | grep -q "${rule} has_rules=True"; then
      echo "  OK: ${cluster}: ${rule} has recording rules"
    else
      echo "  FAIL: ${cluster}: ${rule} should have recording rules"
      FAILURES=$((FAILURES + 1))
    fi
  else
    if echo "${rules}" | grep -q "${rule} has_rules=True"; then
      echo "  FAIL: ${cluster}: ${rule} should be empty or absent"
      FAILURES=$((FAILURES + 1))
    elif echo "${rules}" | grep -q "${rule} has_rules=False"; then
      echo "  OK: ${cluster}: ${rule} is empty (no recording rules)"
    else
      echo "  OK: ${cluster}: ${rule} is absent"
    fi
  fi
}

wait_for_state() {
  local ca="$1" ns_a="$2" virt_a="$3" cb="$4" ns_b="$5" virt_b="$6"
  local elapsed=0

  echo "  Waiting for ManifestWorks (max ${MAX_WAIT}s)..."

  while [ "$elapsed" -lt "$MAX_WAIT" ]; do
    sleep "$POLL"
    elapsed=$((elapsed + POLL))

    local a_rules b_rules ok=true
    a_rules=$(get_rs_rules "${ca}" || echo "")
    b_rules=$(get_rs_rules "${cb}" || echo "")

    # Check cluster A namespace
    if [ "${ns_a}" = "True" ]; then
      echo "${a_rules}" | grep -q "acm-rs-namespace-prometheus-rules has_rules=True" || ok=false
    else
      echo "${a_rules}" | grep -q "acm-rs-namespace-prometheus-rules has_rules=True" && ok=false
    fi
    # Check cluster A virt
    if [ "${virt_a}" = "True" ]; then
      echo "${a_rules}" | grep -q "acm-rs-virt-prometheus-rules has_rules=True" || ok=false
    else
      echo "${a_rules}" | grep -q "acm-rs-virt-prometheus-rules has_rules=True" && ok=false
    fi
    # Check cluster B namespace
    if [ "${ns_b}" = "True" ]; then
      echo "${b_rules}" | grep -q "acm-rs-namespace-prometheus-rules has_rules=True" || ok=false
    else
      echo "${b_rules}" | grep -q "acm-rs-namespace-prometheus-rules has_rules=True" && ok=false
    fi
    # Check cluster B virt
    if [ "${virt_b}" = "True" ]; then
      echo "${b_rules}" | grep -q "acm-rs-virt-prometheus-rules has_rules=True" || ok=false
    else
      echo "${b_rules}" | grep -q "acm-rs-virt-prometheus-rules has_rules=True" && ok=false
    fi

    if $ok; then
      echo "  ManifestWorks reached expected state after ${elapsed}s"
      return 0
    fi
    echo "    waiting... (${elapsed}s / ${MAX_WAIT}s)"
  done

  echo "  TIMEOUT: ManifestWorks did not update within ${MAX_WAIT}s"
  FAILURES=$((FAILURES + 1))
  return 1
}

run_part() {
  local num="$1" desc="$2" ns_a="$3" virt_a="$4" ns_b="$5" virt_b="$6"
  local before=$FAILURES
  TOTAL_PARTS=$((TOTAL_PARTS + 1))

  echo ""
  echo "════════════════════════════════════════════════════════════"
  echo "  Part ${num}: ${desc}"
  echo "════════════════════════════════════════════════════════════"
  echo ""
  echo "  Expected:"
  echo "    ${CLUSTER_A}: namespace=${ns_a}, virt=${virt_a}"
  echo "    ${CLUSTER_B}: namespace=${ns_b}, virt=${virt_b}"
  echo ""

  if wait_for_state "${CLUSTER_A}" "${ns_a}" "${virt_a}" "${CLUSTER_B}" "${ns_b}" "${virt_b}"; then
    check "${CLUSTER_A}" "acm-rs-namespace-prometheus-rules" "${ns_a}"
    check "${CLUSTER_A}" "acm-rs-virt-prometheus-rules" "${virt_a}"
    check "${CLUSTER_B}" "acm-rs-namespace-prometheus-rules" "${ns_b}"
    check "${CLUSTER_B}" "acm-rs-virt-prometheus-rules" "${virt_b}"
  fi

  if [ "$FAILURES" -eq "$before" ]; then
    PASSED_PARTS=$((PASSED_PARTS + 1))
    echo -e "\n  Part ${num} PASSED"
  else
    echo -e "\n  Part ${num} FAILED"
  fi
}

# ── Step 0: Verify clusters ──────────────────────────────────────────────────

echo "=== Verifying managed clusters ==="
for cluster in "${CLUSTER_A}" "${CLUSTER_B}"; do
  status=$(oc get managedcluster "${cluster}" -o jsonpath='{.status.conditions[?(@.type=="ManagedClusterConditionAvailable")].status}' 2>/dev/null || echo "")
  if [ "${status}" = "True" ]; then
    echo "  OK: ${cluster} is available"
  else
    echo "  FAIL: ${cluster} is not available"
    exit 1
  fi
done

# ── Step 1: Apply placement ConfigMaps ────────────────────────────────────────

echo ""
echo "=== Applying placement ConfigMaps: namespace→env=prod, virt→env=dev ==="

oc apply -f - <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: rs-namespace-placement
  namespace: open-cluster-management-observability
  labels:
    observability.open-cluster-management.io/managed-by: analytics-rightsizing
data:
  placementConfiguration: '{"spec":{"predicates":[{"requiredClusterSelector":{"labelSelector":{"matchLabels":{"env":"prod"}}}}]}}'
EOF

oc apply -f - <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: rs-virt-placement
  namespace: open-cluster-management-observability
  labels:
    observability.open-cluster-management.io/managed-by: analytics-rightsizing
data:
  placementConfiguration: '{"spec":{"predicates":[{"requiredClusterSelector":{"labelSelector":{"matchLabels":{"env":"dev"}}}}]}}'
EOF

# ══════════════════════════════════════════════════════════════════════════════
# Part 1: A=prod, B=dev → A gets namespace, B gets virt
# ══════════════════════════════════════════════════════════════════════════════

echo ""
echo ">>> Labeling: ${CLUSTER_A}=env:prod, ${CLUSTER_B}=env:dev"
oc label managedcluster "${CLUSTER_A}" env=prod --overwrite
oc label managedcluster "${CLUSTER_B}" env=dev --overwrite

run_part 1 "${CLUSTER_A}=env:prod, ${CLUSTER_B}=env:dev" \
  "True" "False" "False" "True"

# ══════════════════════════════════════════════════════════════════════════════
# Part 2: Swap — A=dev, B=prod → A gets virt, B gets namespace
# ══════════════════════════════════════════════════════════════════════════════

echo ""
echo ">>> Labeling: ${CLUSTER_A}=env:dev, ${CLUSTER_B}=env:prod"
oc label managedcluster "${CLUSTER_A}" env=dev --overwrite
oc label managedcluster "${CLUSTER_B}" env=prod --overwrite

run_part 2 "${CLUSTER_A}=env:dev, ${CLUSTER_B}=env:prod" \
  "False" "True" "True" "False"

# ══════════════════════════════════════════════════════════════════════════════
# Part 3: Both prod → both get namespace only
# ══════════════════════════════════════════════════════════════════════════════

echo ""
echo ">>> Labeling: ${CLUSTER_A}=env:prod, ${CLUSTER_B}=env:prod"
oc label managedcluster "${CLUSTER_A}" env=prod --overwrite
oc label managedcluster "${CLUSTER_B}" env=prod --overwrite

run_part 3 "${CLUSTER_A}=env:prod, ${CLUSTER_B}=env:prod (both namespace)" \
  "True" "False" "True" "False"

# ══════════════════════════════════════════════════════════════════════════════
# Part 4: Both dev → both get virt only
# ══════════════════════════════════════════════════════════════════════════════

echo ""
echo ">>> Labeling: ${CLUSTER_A}=env:dev, ${CLUSTER_B}=env:dev"
oc label managedcluster "${CLUSTER_A}" env=dev --overwrite
oc label managedcluster "${CLUSTER_B}" env=dev --overwrite

run_part 4 "${CLUSTER_A}=env:dev, ${CLUSTER_B}=env:dev (both virt)" \
  "False" "True" "False" "True"

# ══════════════════════════════════════════════════════════════════════════════
# Part 5: Remove labels → neither matches → no active rules
# ══════════════════════════════════════════════════════════════════════════════

echo ""
echo ">>> Removing env label from both clusters"
oc label managedcluster "${CLUSTER_A}" env- 2>/dev/null || true
oc label managedcluster "${CLUSTER_B}" env- 2>/dev/null || true

run_part 5 "No labels (neither matches any placement)" \
  "False" "False" "False" "False"

# ── Final Summary ────────────────────────────────────────────────────────────

echo ""
echo "════════════════════════════════════════════════════════════"
echo "  Final Summary"
echo "════════════════════════════════════════════════════════════"
echo ""
echo "  Parts passed: ${PASSED_PARTS} / ${TOTAL_PARTS}"
echo ""
if [ "${FAILURES}" -eq 0 ]; then
  echo "  ALL CHECKS PASSED"
else
  echo "  ${FAILURES} check(s) failed"
  exit 1
fi
