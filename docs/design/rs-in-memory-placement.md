# Right-Sizing: Replace Placement API with In-Memory Predicate Evaluation

## Review Concern

The following review comment was raised against `ensureRSPlacement()` and the surrounding Placement API usage:

> **`ensureRSPlacement` / `deleteRSPlacement` / `deleteOrphanRSPlacements` / `isClusterSelectedByRSPlacement`**
>
> "As discussed, I don't think this use of the placement API by RS for such a meta addon deploying several 'independent' stacks doesn't scale well. To be discussed in the arch call."

The concern is that **each feature stack in MCOA independently creating and managing its own Placement resources on the hub is architecturally wrong for a meta addon**. MCOA deploys several independent stacks — metrics collection, logging, tracing, incident detection, right-sizing (namespace + virtualization) — and if each one replicates the same pattern of creating Placement resources, waiting for PlacementDecisions, and handling lifecycle (create, update, delete, orphan cleanup, race conditions), it leads to:

1. **Hub resource sprawl** — every feature adds its own Placement + PlacementDecision resources to the hub
2. **Duplicated boilerplate** — each new feature needs ~160 lines of identical Placement CRUD code
3. **Unnecessary external dependency** — coupling to the OCM placement scheduler when the addon framework already handles cluster targeting via InstallStrategy
4. **Fragile race windows** — between Placement creation and PlacementDecision availability, with fail-open defaults that can deploy to wrong clusters

This change addresses all of these concerns by moving cluster selection from external Placement resources to in-memory predicate evaluation.

## Motivation

MCOA is a "meta addon" that deploys several independent stacks (metrics collection, logging, tracing, incident detection, right-sizing). The previous implementation had right-sizing (RS) creating its own `Placement` resources on the hub and relying on the OCM placement scheduler to produce `PlacementDecision` resources for cluster selection.

If every feature stack replicated the same pattern — creating Placement resources, waiting for PlacementDecisions, managing lifecycle (create, update, delete, orphan cleanup) — the hub would accumulate many feature-specific Placement resources and the codebase would carry significant boilerplate for each new feature.

## Problem — What the Old Implementation Did

### Hub-Side (ResourceCreator)

`ReconcileRSResources()` ran on every reconcile and performed:

1. **Orphan cleanup** — Listed all Placements with RS labels across all namespaces and deleted any outside `open-cluster-management-global-set` (to handle MCO mode-switch leftovers).
2. **Placement create/update** — For each enabled RS feature, read the ConfigMap, extracted `placementConfiguration`, and called `ensureRSPlacement()` which:
   - Fetched the Placement by name.
   - If not found, created it with the spec from ConfigMap.
   - If `AlreadyExists` (race condition), re-fetched and fell through to update.
   - If found, updated the spec.
3. **Placement delete** — For each disabled RS feature, deleted the corresponding Placement.
4. **ConfigMap delete** — For each disabled RS feature, deleted the ConfigMap.

### Per-Cluster (Build)

`Build()` called `isClusterSelectedByRSPlacement()` for each RS feature, which:

1. Listed all `PlacementDecision` resources in `open-cluster-management-global-set` matching the Placement name label.
2. If no PlacementDecisions existed yet (scheduler hasn't caught up), **defaulted to selected** (fail-open) to avoid blocking deployment.
3. Iterated all decisions checking if the cluster name appeared.

### Constants Required

```go
PlacementNamespace      = "open-cluster-management-global-set"
NamespacePlacementName  = "rs-placement"
VirtualizationPlacementName = "rs-virt-placement"
PlacementDecisionLabel  = "cluster.open-cluster-management.io/placement"
```

### Problems

| Problem | Description |
|---------|-------------|
| **Doesn't scale** | Every new feature stack would need the same Placement lifecycle management |
| **External dependency** | Relies on OCM placement scheduler running and producing PlacementDecisions |
| **Race window** | Between Placement creation and PlacementDecision availability, `Build()` defaulted to fail-open (selected=true), potentially deploying to wrong clusters briefly |
| **Namespace dependency** | Required `open-cluster-management-global-set` to have a `ManagedClusterSetBinding` |
| **Orphan management** | Needed cross-namespace listing and cleanup for mode switches (MCO <-> MCOA) |
| **Code volume** | ~160 lines of Placement CRUD, orphan cleanup, and PlacementDecision reading |

## Solution — In-Memory Predicate Evaluation

Replace all Placement API usage with a pure function that evaluates the placement predicates in-memory against the `ManagedCluster` object already available in `Build()`.

### Key Insight

The `ManagedCluster` object passed to `Build()` already contains everything needed for cluster selection:

- `cluster.Labels` — for label-based predicates
- `cluster.Status.ClusterClaims` — for claim-based predicates

There is no need to create external resources and wait for a scheduler when we can evaluate the same predicates locally.

### New Function: `clusterMatchesPlacement()`

```go
func clusterMatchesPlacement(cluster *clusterv1.ManagedCluster, placement clusterv1beta1.Placement) bool
```

Evaluates placement predicates following OCM semantics:

- **Empty predicates** → match all clusters (default behavior)
- **Multiple predicates** → ORed (any match selects the cluster)
- **Within a predicate** → `LabelSelector` and `ClaimSelector` are ANDed
- **LabelSelector** → uses `metav1.LabelSelectorAsSelector()` for both `matchLabels` and `matchExpressions`
- **ClaimSelector** → evaluates `matchExpressions` against `cluster.Status.ClusterClaims` using `In`, `NotIn`, `Exists`, `DoesNotExist` operators

## Concerns Addressed

| Concern from Review | How This Change Addresses It |
|---------------------|------------------------------|
| **"doesn't scale well"** — each feature creating its own Placements | Eliminated entirely. No Placement resources are created on the hub. Cluster selection is a pure function call during `Build()`. A new feature only needs to call `clusterMatchesPlacement()` — zero hub resources, zero lifecycle management. |
| **Hub resource sprawl** — RS alone created 2 Placements + PlacementDecisions | Zero hub Placement resources. The only hub resources RS manages are ConfigMaps (which store the user-facing configuration). |
| **Duplicated boilerplate** — ~160 lines of Placement CRUD per feature | Replaced with ~80 lines of reusable matching logic. Any feature can call `clusterMatchesPlacement(cluster, config)` — one line, no error handling needed, no context or client required. |
| **External scheduler dependency** — relied on OCM placement scheduler | Removed. Predicates are evaluated locally against the `ManagedCluster` object already available in `Build()`. No external controller needs to be running. |
| **Race window** — fail-open default between Placement creation and PlacementDecision | Eliminated. In-memory evaluation is synchronous — the result is immediately available, no window where wrong clusters could be selected. |
| **Namespace dependency** — required `open-cluster-management-global-set` with `ManagedClusterSetBinding` | Removed. No namespace dependency for Placement resources. |
| **Orphan cleanup** — cross-namespace listing and deletion for MCO mode switches | No longer needed. No Placement resources exist to become orphaned. |
| **"meta addon deploying independent stacks"** — pattern doesn't fit addon-framework model | Aligned with the addon-framework model: Level 1 (InstallStrategy) handles which clusters get the addon, Level 2 (in-memory predicates from ConfigMap) handles per-feature filtering within `Build()`. No feature bypasses or duplicates the framework's placement mechanism. |

## Changes Summary

### Files Changed (6 files, +302 / -336 lines)

#### `internal/analytics/rightsizing/handlers/handler.go`

**Added** `clusterMatchesPlacement()` and helpers (~80 lines):

- `clusterMatchesPlacement()` — entry point, handles empty predicates and OR semantics
- `clusterMatchesPredicate()` — evaluates a single `ClusterPredicate` (AND of label + claim)
- `clusterMatchesLabelSelector()` — uses `metav1.LabelSelectorAsSelector()` for standard label matching
- `clusterMatchesClaimSelector()` — maps cluster claims to a lookup table, evaluates `matchExpressions`

**Changed** `Build()` — replaced two `isClusterSelectedByRSPlacement()` calls (5 lines each, with error handling and fail-open default) with single-line `clusterMatchesPlacement()` calls:

```go
// Before (for each RS feature)
nsSelected, err := o.isClusterSelectedByRSPlacement(ctx, rightsizing.NamespacePlacementName, cluster.Name)
if err != nil {
    o.Logger.Error(err, "Failed to check namespace placement selection, defaulting to selected")
    nsSelected = true
}

// After
nsSelected := clusterMatchesPlacement(cluster, nsConfigData.PlacementConfiguration)
```

#### `internal/analytics/rightsizing/handlers/rs_resources.go`

**Removed** (~160 lines):

- `ensureRSPlacement()` — create/update Placement with AlreadyExists race handling
- `isClusterSelectedByRSPlacement()` — list PlacementDecisions and check for cluster
- `deleteRSPlacement()` — delete a Placement by name
- `deleteOrphanRSPlacements()` — cross-namespace orphan cleanup

**Simplified** `ReconcileRSResources()` — from ~55 lines (orphan cleanup + Placement lifecycle + ConfigMap lifecycle for each feature) to ~15 lines (ConfigMap cleanup only for disabled features):

```go
func (o *OptionsBuilder) ReconcileRSResources(ctx context.Context, opts addon.Options) error {
    if !opts.Platform.AnalyticsOptions.RightSizing.NamespaceEnabled {
        if err := o.deleteRSConfigMap(ctx, rightsizing.NamespaceConfigMapName); err != nil {
            return fmt.Errorf("failed to cleanup namespace configmap: %w", err)
        }
    }
    if !opts.Platform.AnalyticsOptions.RightSizing.VirtualizationEnabled {
        if err := o.deleteRSConfigMap(ctx, rightsizing.VirtualizationConfigMapName); err != nil {
            return fmt.Errorf("failed to cleanup virtualization configmap: %w", err)
        }
    }
    return nil
}
```

**Removed imports**: `metav1`, `clusterv1beta1`, `client` (no longer needed).

#### `internal/analytics/rightsizing/types.go`

**Removed constants** (no longer needed):

```go
PlacementNamespace         = "open-cluster-management-global-set"
NamespacePlacementName     = "rs-placement"
VirtualizationPlacementName = "rs-virt-placement"
PlacementDecisionLabel     = "cluster.open-cluster-management.io/placement"
```

#### `internal/analytics/rightsizing/handlers/rs_resources_test.go`

**Removed tests** for Placement lifecycle (no longer applicable):

- `TestReconcileRSResources_CleansOrphanPlacements`
- `TestReconcileRSResources_IgnoresNonRSPlacements`
- `TestReconcileRSResources_OrphanCleanupWithDisabledFeatures`
- `TestReconcileRSResources_PlatformDisabledCleansUp`

**Simplified** existing ConfigMap cleanup tests — removed Placement assertions, kept ConfigMap-only assertions.

**Added tests** for in-memory matching (10 test functions):

| Test | What It Verifies |
|------|------------------|
| `TestClusterMatchesPlacement_EmptyPredicates` | Default placement (empty predicates) matches all clusters |
| `TestClusterMatchesPlacement_LabelMatch` | `matchLabels` selects cluster with matching labels |
| `TestClusterMatchesPlacement_LabelNoMatch` | `matchLabels` rejects cluster without matching labels |
| `TestClusterMatchesPlacement_LabelExpressions` | `matchExpressions` with `In` operator works |
| `TestClusterMatchesPlacement_ClaimMatch` | Claim selector `In` matches cluster claim values |
| `TestClusterMatchesPlacement_ClaimNoMatch` | Claim selector `In` rejects non-matching claim values |
| `TestClusterMatchesPlacement_PredicatesORed` | Two predicates — second matches, cluster is selected |
| `TestClusterMatchesPlacement_ClaimDoesNotExist` | `DoesNotExist` operator matches absent claims |
| `TestClusterMatchesPlacement_CombinedLabelAndClaim` | Label AND claim within one predicate — both must match |

#### `internal/analytics/rightsizing/builder.go`

**Updated comment** on `GetDefaultRSPlacement()` — reflects in-memory evaluation instead of OCM scheduler.

#### `internal/controllers/resourcecreator/controller.go`

**Updated comment** on ConfigMap watch — removed "(for placement updates)" since Placement resources no longer exist.

## Non-Matching Resources

The OCM work agent reliably **updates** existing resources but does not reliably **delete** resources removed from a ManifestWork spec. To guarantee cleanup when a cluster no longer matches placement, `Build()` always includes all RS resources in the ManifestWork — using empty/no-op versions for non-matching features. This converts every potential delete into an update, which the work agent handles correctly.

- **PrometheusRules**: `emptyComponentOptions()` returns a PrometheusRule with `spec.groups: []`. The work agent overwrites the existing rule with the empty one, effectively disabling it.
- **ScrapeConfig**: `GenerateScrapeConfig()` always returns a valid ScrapeConfig. When no features match, the ScrapeConfig has no `match[]` params and no `staticConfigs` — it exists on the spoke but scrapes nothing.

## Backward Compatibility

| Aspect | Status |
|--------|--------|
| ConfigMap format (`placementConfiguration` field) | **Unchanged** — existing user customizations preserved |
| Default behavior (empty predicates = all clusters) | **Unchanged** |
| Predicate semantics (OR across predicates, AND within) | **Unchanged** — mirrors OCM placement scheduler behavior |
| Level 1 InstallStrategy | **Unchanged** — addon framework handles this independently |
| ConfigMap watch in ResourceCreator | **Retained** — config changes still trigger reconciliation |
| `RSConfigMapPredicate` | **Retained** — still filters ConfigMap events |

## Two-Level Architecture Preserved

The two-level placement architecture remains intact:

- **Level 1 (InstallStrategy):** Addon-framework built-in — determines which clusters get MCOA ManifestWork at all. **Unchanged.**
- **Level 2 (RS ConfigMap):** Determines which clusters get namespace vs virtualization RS within those ManifestWorks. **Same functional outcome, different implementation** — predicates evaluated in-memory instead of via Placement API + OCM scheduler.

## What Is No Longer Needed

- No `Placement` resources created on the hub
- No dependency on `open-cluster-management-global-set` namespace or `ManagedClusterSetBinding`
- No `PlacementDecision` reading or fail-open race window
- No cross-namespace orphan cleanup for mode switches
- No Placement-related constants (`PlacementNamespace`, `NamespacePlacementName`, etc.)

## Capability Impact Assessment

This change does **not** compromise any right-sizing capability. The `PlacementSpec` has 7 fields — here is the full audit of each.

### Fully Supported — Capabilities RS Uses

These are the features RS actually relies on for cluster selection. All produce identical results to the old Placement API approach.

| Capability | Old (Placement API) | New (In-Memory) | Verdict |
|-----------|-----|-----|---------|
| **Label predicates** — `matchLabels`, `matchExpressions` | OCM scheduler evaluates | `clusterMatchesLabelSelector()` via `metav1.LabelSelectorAsSelector()` | **Identical** |
| **Claim predicates** — `matchExpressions` on cluster claims | OCM scheduler evaluates | `clusterMatchesClaimSelector()` with `In`/`NotIn`/`Exists`/`DoesNotExist` | **Identical** |
| **Multiple predicates (ORed)** | OCM scheduler ORs them | `clusterMatchesPlacement()` loop with early return on first match | **Identical** |
| **LabelSelector + ClaimSelector (ANDed)** | OCM scheduler ANDs them | `clusterMatchesPredicate()` checks both, short-circuits on first failure | **Identical** |
| **Empty predicates = all clusters** | OCM scheduler selects all matching | Returns `true` immediately | **Identical** |

### Not Supported — Features RS Never Used

These are OCM scheduler-specific scheduling features. None were ever configured in RS ConfigMaps, and none are meaningful for deploying PrometheusRules to clusters.

| Feature | Why It's Not Relevant to RS |
|---------|----------------------------|
| **NumberOfClusters** | RS deploys recording rules to *all* matching clusters. Randomly picking N clusters makes no sense. Never configured in ConfigMap. |
| **PrioritizerPolicy** | Scoring/ranking clusters for when `NumberOfClusters` limits selection. Only relevant with `NumberOfClusters`, which RS doesn't use. |
| **SpreadPolicy** | Distributes decisions across topologies (regions, zones). RS doesn't do workload scheduling — it deploys the same rules everywhere. |
| **DecisionStrategy** | Groups PlacementDecisions for staged rollout. RS deploys the same rules to all selected clusters simultaneously. |
| **CelSelector** | CEL expressions on ManagedCluster fields. Never documented or used in RS ConfigMaps. Theoretical gap only. |

### Tolerations — Different Mechanism, Same Result

The old default placement had explicit tolerations for `unreachable` and `unavailable` taints so the OCM scheduler would still select tainted clusters. The new approach doesn't evaluate tolerations at all, but the effective behavior is **the same**:

- `Build()` is called for every cluster selected by Level 1 (InstallStrategy).
- If a cluster is unreachable, `Build()` is still invoked and in-memory evaluation matches it based on predicates — same as the old toleration-based approach.
- The ManifestWork will be pending for unreachable clusters and applied when they come back. This is the correct and expected behavior.

### One Thing That's Actually Better

The old fail-open race window is **eliminated**. Previously, between Placement creation and PlacementDecision availability (typically 10–30 seconds), `isClusterSelectedByRSPlacement()` returned `true` by default:

```go
// Old code in isClusterSelectedByRSPlacement:
if len(placementDecisionList.Items) == 0 {
    // No PlacementDecisions yet — Placement may be newly created.
    // Default to true (fail-open)
    return true, nil
}
```

This meant RS could briefly deploy to clusters it shouldn't have. With in-memory evaluation, the result is synchronous and deterministic — no window, no wrong deployments.

### ClusterSets — Also Better

The old code created Placements in `open-cluster-management-global-set`, scoping selection to clusters in that ManagedClusterSet. With in-memory evaluation, selection operates on whatever clusters `Build()` is called for — already filtered by Level 1 (InstallStrategy). This is **more flexible**: no longer limited to a specific ManagedClusterSet, and no dependency on `ManagedClusterSetBinding` existing in the right namespace.

### Summary

| Category | Count | Impact on RS |
|----------|-------|-------------|
| Fully supported (RS uses these) | 5 capabilities | **Zero** — identical results |
| Not supported (RS never used these) | 5 capabilities | **Zero** — scheduling features irrelevant to RS |
| Different mechanism, same result | 1 (tolerations) | **Zero** — same effective behavior |
| Actually improved | 2 (race window, ClusterSets) | **Positive** — eliminates fail-open window and namespace constraint |

## MCO-Safe Placement Configuration

### Problem

The MCO controller periodically syncs the RS ConfigMaps (`rs-namespace-config`, `rs-virt-config`), fully overwriting their `data` section. This resets any custom `placementConfiguration` the user has added. Since MCO writes default empty predicates (which match all clusters), custom placement filtering is lost after every MCO sync cycle.

### Solution: Dedicated Placement ConfigMaps

Placement configuration is stored in **separate, MCOA-owned ConfigMaps** that MCO does not know about and will never overwrite:

| ConfigMap | Purpose |
|-----------|---------|
| `rs-namespace-placement` | Placement predicates for namespace right-sizing |
| `rs-virt-placement` | Placement predicates for virtualization right-sizing |

These ConfigMaps use the same `placementConfiguration` key format as the RS ConfigMaps. MCOA checks for these first; if found, their placement takes priority. If not found, MCOA falls back to the placement from the RS ConfigMap (which MCO manages).

### Precedence

```
rs-namespace-placement exists?
  ├─ Yes → use its placementConfiguration (MCO cannot overwrite)
  └─ No  → use rs-namespace-config.placementConfiguration (MCO-managed, may be default)
```

### Example

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: rs-namespace-placement
  namespace: open-cluster-management-addon-observability
  labels:
    observability.open-cluster-management.io/managed-by: analytics-rightsizing
data:
  placementConfiguration: |
    {"spec":{"predicates":[{"requiredClusterSelector":{"labelSelector":{"matchLabels":{"env":"prod"}}}}]}}
```

This ensures `env=prod` clusters receive namespace right-sizing even when MCO syncs and overwrites `rs-namespace-config`.
