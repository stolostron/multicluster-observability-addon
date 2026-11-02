# MCO → MCOA Thanos config migration

`mco-thanos-migrate` is a one-time migration tool for
[ACM-35788](https://redhat.atlassian.net/browse/ACM-35788). It carries the
user-defined Thanos configuration from a `MultiClusterObservability` CR (MCO)
to the Thanos CRs managed by the multicluster-observability-addon (MCOA):
`ThanosCompact`, `ThanosReceive`, `ThanosQuery`, `ThanosRuler` and
`ThanosStore`, all named `mcoa` in the `open-cluster-management-observability`
namespace.

The tool does **not** import the MCO module: the MCO CR is read as
unstructured data, so no dependency on multicluster-observability-operator is
introduced.

## Why edits to the Thanos CRs stick

MCOA deploys the five Thanos CRs through a ManifestWork with a server-side
apply update strategy that ignores `.spec` when it changes on the cluster
(`OnSpokeChange`). Editing `.spec` of those CRs on the hub cluster is the
supported escape hatch for user-defined configuration: the addon keeps
rendering its defaults on the hub side, but the work agent stops overwriting
`.spec` once it detects a cluster-side change.

## Usage

Two steps, typically around an upgrade:

```sh
# 1. capture — BEFORE the MCO Thanos stack is removed.
#    Either from the cluster (current kubeconfig):
go run ./hack/migrate-thanos-config capture -o thanos-migration.yaml
#    or offline from a saved CR:
kubectl get mco observability -o yaml > mco.yaml
go run ./hack/migrate-thanos-config capture -f mco.yaml -o thanos-migration.yaml

# 2. apply — AFTER the MCOA Thanos CRs exist (thanos-operator stack enabled).
go run ./hack/migrate-thanos-config apply -f thanos-migration.yaml --dry-run
go run ./hack/migrate-thanos-config apply -f thanos-migration.yaml
```

Flags: see `go run ./hack/migrate-thanos-config help`.

Only fields the user explicitly set in the MCO CR (`spec.advanced` and
`spec.storageConfig`) are captured. MCO defaults and `instanceSize`-derived
sizing are deliberately not carried over — MCOA has its own defaults and its
own `instanceSize` handling via the AddOnDeploymentConfig.

## Field mapping

| MCO (`MultiClusterObservability`) | MCOA Thanos CR |
|---|---|
| `spec.advanced.retentionConfig.retentionResolutionRaw` | `ThanosCompact.spec.retentionConfig.raw` |
| `spec.advanced.retentionConfig.retentionResolution5m` | `ThanosCompact.spec.retentionConfig.fiveMinutes` |
| `spec.advanced.retentionConfig.retentionResolution1h` | `ThanosCompact.spec.retentionConfig.oneHour` |
| `spec.advanced.retentionConfig.retentionInLocal` | `ThanosReceive.spec.ingester.hashrings[0].tsdbConfig.retention` **and** `ThanosRuler.spec.rulerMode.stateful.retention` (MCO used one knob for both) |
| `spec.storageConfig.compactStorageSize` | `ThanosCompact.spec.storage.size` |
| `spec.storageConfig.receiveStorageSize` | `ThanosReceive.spec.ingester.hashrings[0].storage.size` |
| `spec.storageConfig.ruleStorageSize` | `ThanosRuler.spec.storage.size` |
| `spec.storageConfig.storeStorageSize` | `ThanosStore.spec.storage.size` |
| `spec.storageConfig.storageClass` | `.spec.storage.storageClass` on all four (immutable, see notes) |
| `spec.advanced.query.replicas` | `ThanosQuery.spec.replicas` |
| `spec.advanced.queryFrontend.replicas` | `ThanosQuery.spec.queryFrontend.replicas` |
| `spec.advanced.receive.replicas` | `ThanosReceive.spec.ingester.hashrings[0].replicas` |
| `spec.advanced.rule.replicas` | `ThanosRuler.spec.replicas` |
| `spec.advanced.store.replicas` | `ThanosStore.spec.replicas` |
| `spec.advanced.rule.evalInterval` | `ThanosRuler.spec.evaluationInterval` |
| `spec.advanced.compact.debug.downsampleConcurrency` | `ThanosCompact.spec.downsamplingConfig.concurrency` |
| `spec.advanced.compact.debug.blockMetaFetchConcurrency` | `ThanosCompact.spec.blockConfig.blockMetaFetchConcurrency` |
| `spec.advanced.compact.debug.waitInterval` | `ThanosCompact.spec.compactConfig.cleanupInterval` (approximate, warned) |
| `spec.advanced.<component>.containers` | `.spec.additionalContainers` (only with `--include-sidecars`) |
| `spec.advanced.<component>.resources` | **not written to CRs** — emitted as a suggested `AddOnDeploymentConfig` patch (`suggestedAddOnDeploymentConfig` in the migration file) |
| `spec.nodeSelector` / `spec.tolerations` | same — suggested ADC patch only |
| `spec.advanced.storeMemcached.replicas` | `Deployment/memcached-store` `.spec.replicas` |
| `spec.advanced.storeMemcached.memoryLimitMb` | `Deployment/memcached-store` memcached container arg `-m` |
| `spec.advanced.storeMemcached.maxItemSize` | `Deployment/memcached-store` memcached container arg `-I` |
| `spec.advanced.storeMemcached.connectionLimit` | `Deployment/memcached-store` memcached container arg `-c` |
| `spec.advanced.queryFrontendMemcached.{...}` | same knobs on `Deployment/memcached-query-frontend` |

Resources, node selectors and tolerations are managed on the MCOA side
through the AddOnDeploymentConfig (`nodePlacement`, `resourceRequirements` per
container ID) and must not be written into the Thanos CRs, so `apply` ignores
them and `capture` records them as an advisory snippet instead. Apply that
snippet manually to the ADC that deploys MCOA.

## Known gaps (warned, not migrated)

| MCO setting | Reason |
|---|---|
| `retentionConfig.deleteDelay` | Thanos CRs expose no delete-delay |
| `retentionConfig.blockDuration` | ThanosRuler exposes no TSDB block duration |
| `<component>.debug.logLevel` | Thanos CRs expose no log level |
| `query.usePrometheusEngine` | ThanosQuery exposes no query engine selector |
| `<component>.serviceAccountAnnotations` | no equivalent |

Memcached tuning is applied directly to the `memcached-store` and
`memcached-query-frontend` Deployments (replica count and the `-m`/`-I`/`-c`
container args). These edits persist only while MCOA manages those Deployments
with server-side apply + `OnSpokePresent` on exactly those paths — on older
MCOA versions the default update strategy reverts cluster-side patches on the
next reconcile, so run `apply` after upgrading to an MCOA build that includes
the memcached manifest configs.

## Notes

- **Storage sizes do not resize existing PVCs.** Changing
  `storage.size` on the CR only affects newly created volumes.
  After applying, verify with `kubectl get pvc -n open-cluster-management-observability`
  and expand volumes manually if the storage class allows expansion
  (`kubectl patch pvc <name> -p '{"spec":{"resources":{"requests":{"storage":"<size>"}}}}'`).
- **storageClass is immutable** on the Thanos CRs (CEL `self == oldSelf`).
  `apply` only sets it when the CR has none yet and notes this otherwise.
- **Durations** must be Prometheus-style (`24h`, `365d`, `2w`); values that
  only parse as Go durations (e.g. fractional `1.5h`) are warned about.
- The `mcoa` CRs must already exist when running `apply`; the tool does not
  create them. If they are missing, the thanos-operator stack is probably not
  enabled (annotation `mcoa-thanos-operator: "true"` on the
  AddOnDeploymentConfig).
- Once `apply` writes `.spec`, the whole `.spec` of that CR stops receiving
  hub-side default updates (`OnSpokeChange` ignores it as a unit). That is
  the intended trade-off of user-defined config on these CRs.
