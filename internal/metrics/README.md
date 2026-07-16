# Metrics Package

## CRD Management for Cluster Observability Operator (COO)

The Metrics package in MCOA leverages the **Cluster Observability Operator (COO)** CRDs when they are present on a managed cluster. MCOA must dynamically detect COO's presence and adapt its configuration at runtime.

### CRD Ownership and Footprint Minimization

When COO is missing on a managed cluster, we do not fully install COO itself because we only need its Prometheus Operator capabilities, and fully installing COO would spawn other workloads that we do not need. To limit our footprint and conserve resources on managed clusters (where CPU and memory can be scarce), we only install the necessary CRDs and run our own lightweight Prometheus Operator instance on the managed cluster to reconcile the metrics collection resources.

Furthermore, as most of these CRDs are large, we avoid storing them in the hub's etcd through ManifestWorks by having the endpoint operator apply them directly on the managed cluster. This avoids storing gigabytes of schema data in the hub's etcd when managing many clusters.

CRDs are split into two categories:

**CRDs managed by the Endpoint Operator (on the spoke)**

The following CRDs have large schemas. They are owned and applied directly by the endpoint operator on the managed cluster:

- `prometheusagents.monitoring.rhobs`
- `scrapeconfigs.monitoring.rhobs`
- `servicemonitors.monitoring.rhobs`
- `podmonitors.monitoring.rhobs`
- `probes.monitoring.rhobs`
- `prometheuses.monitoring.rhobs`
- `prometheusrules.monitoring.rhobs`

**CRDs managed by ManifestWork (on the hub)**

The addon manager still makes use of the following "dummy" CRD to detect the state of the managed cluster and apply relevant configuration. This CRD is small and does not contain the full schema. They are used to leverage the `feedbackRules` API from ManifestWorks to detect COO's presence and the establishment of the CRDs.

| CRD | Strategy | Purpose |
|-----|----------|---------|
| `monitoringstacks.monitoring.rhobs` | `CreateOnly` | COO detection anchor — OLM takes it over when COO is installed |
| `prometheusagents.monitoring.rhobs` | `ReadOnly` | Feedback only — hub reads `isEstablished` and timestamps to trigger prometheus-operator restart |
| `scrapeconfigs.monitoring.rhobs` | `ReadOnly` | Feedback only — same as above, also carries `prometheusOperatorVersion` |

`prometheusagents` and `scrapeconfigs` use `ReadOnly`: the Work Agent never creates or modifies them — it only reads their status to report feedback back to the hub. The endpoint operator is the sole owner of their content.

### COO Detection Strategy

To detect whether COO is installed, MCOA leverages the `feedbackRules` API from `ManifestWorks`.

#### Choosing the Detection Resource
A key challenge is selecting a stable resource for detection that does not negatively impact addon health:
- **Constraints**: `feedbackRules` can only be used on objects already present in the `ManifestWork`'s manifest list.
- **Why not use COO-only resources?**: If we include a resource that only exists when COO is installed, the addon will be marked as **Degraded** when COO is missing (as the resource won't be "Available").
- **Why not always include the full CRDs?**: If MCOA provides the CRDs and OLM (via COO) also tries to manage them, it leads to reconciliation conflicts and a degraded health status.

#### The "Dummy" CRD Solution
The chosen solution is to create a "dummy" `monitoringstacks` CRD with the minimum required fields to be accepted by the API server.
- **Update Strategy**: Set to `CreateOnly`. This ensures that when OLM installs COO and takes over the CRD, the OCM Work Agent does not try to revert OLM's changes, preventing conflicts and keeping the resource clean.
- **Continuous Feedback**: This dummy resource allows the OLM presence `feedbackRule` to work regardless of whether COO is currently installed.
- **Conditional Deletion-Orphan Annotation**: The dummy `monitoringstacks` CRD conditionally applies the deletion-orphan annotation:
  ```yaml
  {{- if not .Values.deployCOOResources }}
  addon.open-cluster-management.io/deletion-orphan: ""
  {{- end }}
  ```
  This is critical for proper uninstallation behavior. When COO is installed (`.Values.deployCOOResources` is `false`), MCOA does not manage the CRDs, so the OCM Work Agent should not delete the `monitoringstacks` CRD at uninstallation time (preserving the COO-managed resource). However, when COO is not installed (`.Values.deployCOOResources` is `true`), MCOA manages the resources and does not apply this annotation, allowing the OCM Work Agent to clean up the dummy `monitoringstacks` CRD along with the other resources listed in the `ManifestWork` during uninstallation.

### Adaptation Logic

1.  **COO Detected**: When `feedbackRules` indicate COO is present, MCOA disables the deployment of its own Prometheus Operator manifests (relying on COO to provide/manage the operator) to avoid reconciliation conflicts.
2.  **COO Uninstalled**: If a user uninstalls COO, MCOA detects its absence and re-installs its own Prometheus Operator manifests on the managed cluster to ensure continuous metrics collection.

### Operator Synchronization and Restarts

The OCM `WorkAgent` does not guarantee the order in which resources within a `ManifestWork` are deployed. This creates a potential race condition where the Prometheus Operator pod may start before its dependent CRDs (such as `PrometheusAgent` or `ScrapeConfig`) are fully established on the managed cluster.

To prevent locked states or synchronization issues:
- **Establishment Detection**: MCOA uses `feedbackRules` on the `ReadOnly` CRD stubs to monitor the `Established` condition and transition timestamps of the deployed CRDs. Because the stubs are `ReadOnly`, the Work Agent reports feedback from whichever entity created the real CRD (endpoint operator, COO, or MCO).
- **Forced Restart**: Once the CRDs are established, MCOA injects a special annotation containing these timestamps into the Prometheus Operator's Deployment template.
- **Triggered Rollout**: This change triggers a standard Kubernetes rolling update, ensuring the operator restarts and correctly discovers the now-available CRDs.

### Ownership and Deletion Handling

To prevent accidental deletion of CRDs during transitions:
- **`deletion-orphan` annotation**: The `addon.open-cluster-management.io/deletion-orphan` annotation is conditionally set on the `monitoringstacks` CRD based on `.Values.deployCOOResources`.
- **Reasoning**: Since OLM does not override the existing ownership on this specific CRD (while it does on the others), MCOA would normally delete the CRD upon its own uninstallation. The annotation removes the ownership claim, ensuring the resource is not deleted by the `WorkAgent` when COO is managing the CRDs. If COO is not installed, the annotation is omitted so that MCOA cleans up the CRD upon uninstallation. For the endpoint-operator-managed CRDs, the endpoint operator is their sole SSA owner. When COO installs and takes over, it displaces that ownership and the endpoint operator stops reconciling them.
- **`ReadOnly` CRDs**: Resources with `ReadOnly` update strategy are never deleted by the Work Agent, regardless of whether they are present in the manifest list.

### Hub-side CRD Dependencies
Note that `prometheusagents` and `scrapeconfigs` CRDs are not deployed on the hub by the endpoint operator. These are installed by the **MultiCluster Observability (MCO)** operator as they are direct dependencies of the Addon Manager (MCOA). The `ReadOnly` feedback stubs still work on hub because MCO's CRDs satisfy the existence check.

## Lifecycle Sequence Diagrams

The following diagrams illustrate how MCOA manages the lifecycle of COO CRDs on managed clusters.

### 1. Initial Startup (COO Missing)

When MCOA starts and detects that COO is not present on the managed cluster, the endpoint operator takes responsibility for deploying the necessary CRDs directly on the spoke.

```mermaid
sequenceDiagram
    autonumber
    participant AddonManager as Addon Manager (Hub)
    participant ManifestWork as ManifestWork (Hub)
    participant WorkAgent as Work Agent (Spoke)
    participant ManagedCluster as Managed Cluster API
    participant EndpointOp as Endpoint Operator (Spoke)
    participant PromOperator as Prometheus Operator (Spoke)

    AddonManager->>ManifestWork: Adds "dummy" MonitoringStack CRD (CreateOnly)
    AddonManager->>ManifestWork: Adds ReadOnly stubs for PrometheusAgent & ScrapeConfig CRDs
    AddonManager->>ManifestWork: Sets feedbackRules for OLM detection & CRD establishment
    AddonManager->>ManifestWork: Adds Endpoint Operator + Prometheus Operator manifests
    WorkAgent->>ManifestWork: Watches ManifestWork, detects new revision
    WorkAgent->>ManifestWork: Reads manifest list
    WorkAgent->>ManagedCluster: Deploys all resources (CRD stubs, Endpoint Op, Prometheus Op, ...)
    ManagedCluster-->>WorkAgent: Returns status (MonitoringStack CRD conditions)
    WorkAgent->>ManifestWork: Updates feedback: COO not detected
    EndpointOp->>ManagedCluster: Applies full CRD schemas (PrometheusAgent, ScrapeConfig, etc.)
    ManagedCluster-->>WorkAgent: CRDs become Established (detected via ReadOnly stubs)
    WorkAgent->>ManifestWork: Updates feedback: CRDs Established & Timestamps
    ManifestWork-->>AddonManager: Status update trigger
    AddonManager->>ManifestWork: Adds restart annotation (timestamp) to Prometheus Operator Deployment
    WorkAgent->>ManifestWork: Watches ManifestWork, detects updated Deployment
    WorkAgent->>ManifestWork: Reads updated manifest
    WorkAgent->>PromOperator: Applies updated Deployment (triggering restart)
    PromOperator->>ManagedCluster: Restarts and discovers new CRDs
```

### 2. COO Installation (Dynamic Transition)

When a user or OLM installs COO on the managed cluster, MCOA detects the transition and steps back to avoid management conflicts.

```mermaid
sequenceDiagram
    autonumber
    participant AddonManager as Addon Manager (Hub)
    participant ManifestWork as ManifestWork (Hub)
    participant WorkAgent as Work Agent (Spoke)
    participant OLM
    participant EndpointOp as Endpoint Operator (Spoke)
    participant User

    User->>OLM: Installs COO
    OLM->>OLM: Takes over all COO CRDs (adds OLM label)
    WorkAgent->>OLM: Detects OLM label on MonitoringStack (via feedbackRule)
    WorkAgent->>ManifestWork: Updates feedback: COOIsInstalled=true
    ManifestWork-->>AddonManager: Status update trigger
    AddonManager->>ManifestWork: Adds 'deletion-orphan' to MonitoringStack
    AddonManager->>ManifestWork: Sets deployCOOResources=false (removes prometheus-operator)
    EndpointOp->>EndpointOp: Detects COO, stops reconciling CRDs
    Note over WorkAgent: ReadOnly CRD stubs remain in ManifestWork<br/>but Work Agent never deletes ReadOnly resources
```

### 3. COO Uninstallation

If COO is removed, MCOA detects the deletion and the endpoint operator restores its managed versions.

```mermaid
sequenceDiagram
    autonumber
    participant AddonManager as Addon Manager (Hub)
    participant ManifestWork as ManifestWork (Hub)
    participant WorkAgent as Work Agent (Spoke)
    participant ManagedCluster as Managed Cluster API
    participant EndpointOp as Endpoint Operator (Spoke)
    participant User

    User->>ManagedCluster: Uninstalls COO & deletes MonitoringStack CRD
    WorkAgent->>ManagedCluster: Detects MonitoringStack is missing and recreates it (CreateOnly)
    WorkAgent->>ManifestWork: Updates feedback: COO not detected
    ManifestWork-->>AddonManager: Status update trigger
    AddonManager->>ManifestWork: Re-enables deployCOOResources (restores prometheus-operator)
    EndpointOp->>ManagedCluster: Re-applies full CRD schemas
```
