// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License, Version 2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"slices"
	"time"

	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"
)

const (
	mcoGroup  = "observability.open-cluster-management.io"
	mcoKind   = "MultiClusterObservability"
	mcoCRName = "observability"
)

// errUnexpectedKind is returned when the offline input file is not a
// MultiClusterObservability CR.
var errUnexpectedKind = errors.New("unexpected kind")

// prometheusDuration matches durations accepted by Prometheus model.ParseDuration,
// which is what the thanos-operator Duration type uses. MCO's CRD allows Go-style
// values (e.g. fractional "1.5h") that Prometheus rejects, so warn on those.
var prometheusDuration = regexp.MustCompile(`^([0-9]+(ms|s|m|h|d|w|y))+$`)

func runCapture(args []string) error {
	fs := flag.NewFlagSet("capture", flag.ExitOnError)
	inFile := fs.String("f", "", "path to a MultiClusterObservability CR YAML (offline mode)")
	out := fs.String("o", "thanos-migration.yaml", "output migration file")
	mcoName := fs.String("mco-name", mcoCRName, "name of the MultiClusterObservability CR")
	mcoVersion := fs.String("mco-api-version", "v1beta2", "API version of the MultiClusterObservability CR")
	namespace := fs.String("namespace", mconfig.HubInstallNamespace, "namespace of the MultiClusterObservability CR")
	if err := fs.Parse(args); err != nil {
		return err
	}

	mco, err := loadMCOCR(*inFile, *mcoName, *mcoVersion, *namespace)
	if err != nil {
		return err
	}

	cfg := extract(mco)
	cfg.APIVersion = "mcoa.redhat.com/v1alpha1"
	cfg.Kind = "ThanosConfigMigration"
	cfg.Source.Name = mco.GetName()
	cfg.Source.APIVersion = mco.GetAPIVersion()
	cfg.Source.CapturedAt = time.Now().UTC().Format(time.RFC3339)

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal migration config: %w", err)
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", *out, err)
	}

	fmt.Printf("wrote %s\n\n", *out)
	printSummary(cfg)
	return nil
}

// loadMCOCR loads the MultiClusterObservability CR either from a YAML file
// (offline capture) or from the cluster referenced by the current kubeconfig.
// The CR is read as unstructured data on purpose: this repo must not depend on
// the multicluster-observability-operator module.
func loadMCOCR(inFile, name, version, namespace string) (*unstructured.Unstructured, error) {
	if inFile != "" {
		data, err := os.ReadFile(inFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", inFile, err)
		}
		mco := &unstructured.Unstructured{}
		if err := yaml.Unmarshal(data, &mco.Object); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", inFile, err)
		}
		if mco.GetKind() != mcoKind {
			return nil, fmt.Errorf("%w: expected kind %s in %s, got %q", errUnexpectedKind, mcoKind, inFile, mco.GetKind())
		}
		return mco, nil
	}

	restCfg, err := ctrlconfig.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}
	cl, err := client.New(restCfg, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	mco := &unstructured.Unstructured{}
	mco.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   mcoGroup,
		Version: version,
		Kind:    mcoKind,
	})
	if err := cl.Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, mco); err != nil {
		return nil, fmt.Errorf("failed to get %s %s/%s (use --mco-name/--mco-api-version or -f to adjust): %w",
			mcoKind, namespace, name, err)
	}
	return mco, nil
}

// extract maps user-declared fields of the MultiClusterObservability CR onto
// the MigrationConfig. Only fields the user explicitly set are extracted; MCO
// defaults and instanceSize-derived sizing are intentionally not carried over.
func extract(mco *unstructured.Unstructured) *MigrationConfig {
	cfg := &MigrationConfig{}
	spec, _ := mco.Object["spec"].(map[string]any)
	if spec == nil {
		return cfg
	}
	advanced, _ := spec["advanced"].(map[string]any)
	storage, _ := spec["storageConfig"].(map[string]any)

	retention := mapOf(advanced, "retentionConfig")
	warn := cfg.warn

	// Retention (compact + local).
	setDuration(&cfg.Compact.RetentionRaw, str(retention, "retentionResolutionRaw"), "spec.advanced.retentionConfig.retentionResolutionRaw", warn)
	setDuration(&cfg.Compact.Retention5m, str(retention, "retentionResolution5m"), "spec.advanced.retentionConfig.retentionResolution5m", warn)
	setDuration(&cfg.Compact.Retention1h, str(retention, "retentionResolution1h"), "spec.advanced.retentionConfig.retentionResolution1h", warn)
	// retentionInLocal applied to both receive and rule in MCO; keep that split.
	setDuration(&cfg.Receive.Retention, str(retention, "retentionInLocal"), "spec.advanced.retentionConfig.retentionInLocal", warn)
	setDuration(&cfg.Ruler.Retention, str(retention, "retentionInLocal"), "spec.advanced.retentionConfig.retentionInLocal", warn)
	if v := str(retention, "deleteDelay"); v != "" {
		warn("spec.advanced.retentionConfig.deleteDelay=%q: the Thanos CRs do not expose a delete-delay setting; not migrated", v)
	}
	if v := str(retention, "blockDuration"); v != "" {
		warn("spec.advanced.retentionConfig.blockDuration=%q: ThanosRuler does not expose a TSDB block duration; not migrated", v)
	}

	// Storage sizes and class.
	cfg.Ruler.StorageSize = str(storage, "ruleStorageSize")
	cfg.Compact.StorageSize = str(storage, "compactStorageSize")
	cfg.Receive.StorageSize = str(storage, "receiveStorageSize")
	cfg.Store.StorageSize = str(storage, "storeStorageSize")
	if sc := str(storage, "storageClass"); sc != "" {
		cfg.Ruler.StorageClass = sc
		cfg.Compact.StorageClass = sc
		cfg.Receive.StorageClass = sc
		cfg.Store.StorageClass = sc
	}

	// Replicas.
	cfg.Query.Replicas = int32Of(advanced, "query", "replicas")
	cfg.Query.QueryFrontendReplicas = int32Of(advanced, "queryFrontend", "replicas")
	cfg.Receive.Replicas = int32Of(advanced, "receive", "replicas")
	cfg.Ruler.Replicas = int32Of(advanced, "rule", "replicas")
	cfg.Store.Replicas = int32Of(advanced, "store", "replicas")

	// Rule evaluation interval.
	setDuration(&cfg.Ruler.EvalInterval, str(advanced, "rule", "evalInterval"), "spec.advanced.rule.evalInterval", warn)

	// Compact debug tuning.
	compactDebug := mapOf(advanced, "compact", "debug")
	cfg.Compact.DownsampleConcurrency = int32Val(compactDebug["downsampleConcurrency"])
	cfg.Compact.BlockMetaFetchConcurrency = int32Val(compactDebug["blockMetaFetchConcurrency"])
	if v := strVal(compactDebug["waitInterval"]); v != "" {
		// MCO used waitInterval to also drive --compact.cleanup-interval;
		// only the cleanup cadence has a Thanos CR equivalent.
		cfg.Compact.CleanupInterval = v
		warn("spec.advanced.compact.debug.waitInterval=%q: mapped approximately to ThanosCompact.spec.compactConfig.cleanupInterval (compaction cycle wait itself has no equivalent)", v)
	}
	setLogLevel(&cfg.Compact.LogLevel, str(compactDebug, "logLevel"), "spec.advanced.compact.debug.logLevel", warn)
	setLogLevel(&cfg.Receive.LogLevel, str(advanced, "receive", "debug", "logLevel"), "spec.advanced.receive.debug.logLevel", warn)
	if b, ok := nested(advanced, "query", "usePrometheusEngine"); ok {
		if use, _ := b.(bool); use {
			warn("spec.advanced.query.usePrometheusEngine=true: ThanosQuery does not expose a query engine selector; not migrated")
		}
	}

	// Sidecar containers (applied only with --include-sidecars).
	cfg.Query.Containers = containersOf(advanced, "query", "containers")
	cfg.Query.QueryFrontendContainers = containersOf(advanced, "queryFrontend", "containers")
	cfg.Receive.Containers = containersOf(advanced, "receive", "containers")
	cfg.Ruler.Containers = containersOf(advanced, "rule", "containers")
	cfg.Compact.Containers = containersOf(advanced, "compact", "containers")
	cfg.Store.Containers = containersOf(advanced, "store", "containers")

	// serviceAccountAnnotations map to the CommonFields annotations of the
	// CRs; the thanos-operator stamps them onto the component's
	// ServiceAccount (and pod template). MCO ran one receive component where
	// MCOA runs a router and an ingester per hashring, so receive's
	// annotations go to both, mirroring the log level split.
	cfg.Query.Annotations = strMapOf(advanced, "query", "serviceAccountAnnotations")
	cfg.Query.QueryFrontendAnnotations = strMapOf(advanced, "queryFrontend", "serviceAccountAnnotations")
	cfg.Receive.Annotations = strMapOf(advanced, "receive", "serviceAccountAnnotations")
	cfg.Ruler.Annotations = strMapOf(advanced, "rule", "serviceAccountAnnotations")
	cfg.Compact.Annotations = strMapOf(advanced, "compact", "serviceAccountAnnotations")
	cfg.Store.Annotations = strMapOf(advanced, "store", "serviceAccountAnnotations")

	// Memcached tuning is applied to the memcached-store and
	// memcached-query-frontend Deployments. It persists only on MCOA versions
	// that manage those Deployments with SSA + OnSpokePresent.
	cfg.StoreMemcached = extractMemcached(advanced, "storeMemcached")
	cfg.QueryFrontendMemcached = extractMemcached(advanced, "queryFrontendMemcached")

	// Node placement and per-component resources belong on the AddOnDeploymentConfig.
	collectADDCSuggestion(cfg, spec, advanced)

	return cfg
}

func collectADDCSuggestion(cfg *MigrationConfig, spec, advanced map[string]any) {
	sugg := &ADDCSuggestion{}

	if ns, ok := spec["nodeSelector"].(map[string]any); ok && len(ns) > 0 {
		sugg.NodeSelector = map[string]string{}
		for k, v := range ns {
			if s, ok := v.(string); ok {
				sugg.NodeSelector[k] = s
			}
		}
	}
	if tols := containersLike[corev1.Toleration](spec["tolerations"]); len(tols) > 0 {
		sugg.Tolerations = tols
	}

	// MCO component -> MCOA ADC containerID.
	for _, m := range []struct {
		component string
		container string
	}{
		{"query", mconfig.ThanosQueryContainerID},
		{"queryFrontend", mconfig.ThanosQueryFrontendContainerID},
		{"receive", mconfig.ThanosReceiveIngesterContainerID},
		{"rule", mconfig.ThanosRulerContainerID},
		{"compact", mconfig.ThanosCompactContainerID},
		{"store", mconfig.ThanosStoreContainerID},
		{"storeMemcached", mconfig.MemcachedStoreContainerID},
		{"queryFrontendMemcached", mconfig.MemcachedQueryFrontendContainerID},
	} {
		resources := resourcesOf(advanced, m.component, "resources")
		if resources == nil {
			continue
		}
		sugg.ResourceRequirements = append(sugg.ResourceRequirements, ADDCContainerResource{
			ContainerID: m.container,
			Resources:   *resources,
		})
	}

	if len(sugg.NodeSelector) > 0 || len(sugg.Tolerations) > 0 || len(sugg.ResourceRequirements) > 0 {
		cfg.SuggestedAddOnDeploymentConfig = sugg
		cfg.warn("node placement and per-component resources are managed by the AddOnDeploymentConfig on the MCOA side; " +
			"a suggested patch is included in suggestedAddOnDeploymentConfig and must be applied to the ADC manually")
	}
}

func (c *MigrationConfig) warn(format string, args ...any) {
	c.Warnings = append(c.Warnings, fmt.Sprintf(format, args...))
}

// extractMemcached reads one MCO CacheConfig section (spec.advanced.<name>).
func extractMemcached(advanced map[string]any, name string) MemcachedConfig {
	section := mapOf(advanced, name)
	if section == nil {
		return MemcachedConfig{}
	}
	return MemcachedConfig{
		Replicas:        int32Of(section, "replicas"),
		MemoryLimitMB:   int32Val(section["memoryLimitMb"]),
		MaxItemSize:     str(section, "maxItemSize"),
		ConnectionLimit: int32Val(section["connectionLimit"]),
	}
}

// --- unstructured field helpers -------------------------------------------

func nested(obj map[string]any, path ...string) (any, bool) {
	if obj == nil {
		return nil, false
	}
	cur := obj
	for i, key := range path {
		v, ok := cur[key]
		if !ok || v == nil {
			return nil, false
		}
		if i == len(path)-1 {
			return v, true
		}
		if cur, ok = v.(map[string]any); !ok {
			return nil, false
		}
	}
	return nil, false
}

func mapOf(obj map[string]any, path ...string) map[string]any {
	v, ok := nested(obj, path...)
	if !ok {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

// strMapOf reads a map[string]string field from unstructured data, returning
// nil when absent or when no entry has a string value.
func strMapOf(obj map[string]any, path ...string) map[string]string {
	m := mapOf(obj, path...)
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func str(obj map[string]any, path ...string) string {
	v, ok := nested(obj, path...)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func strVal(v any) string {
	s, _ := v.(string)
	return s
}

func int32Of(obj map[string]any, path ...string) *int32 {
	v, ok := nested(obj, path...)
	if !ok {
		return nil
	}
	return int32Val(v)
}

func int32Val(v any) *int32 {
	switch n := v.(type) {
	case float64:
		i := int32(n)
		return &i
	case int64:
		i := int32(n)
		return &i
	case int:
		i := int32(n)
		return &i
	}
	return nil
}

// containersLike converts an unstructured list of objects into a typed slice
// via a JSON round trip.
func containersLike[T any](v any) []T {
	if v == nil {
		return nil
	}
	list, ok := v.([]any)
	if !ok || len(list) == 0 {
		return nil
	}
	data, err := json.Marshal(list)
	if err != nil {
		return nil
	}
	var out []T
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}

func containersOf(obj map[string]any, path ...string) []corev1.Container {
	v, ok := nested(obj, path...)
	if !ok {
		return nil
	}
	return containersLike[corev1.Container](v)
}

func resourcesOf(obj map[string]any, path ...string) *corev1.ResourceRequirements {
	v, ok := nested(obj, path...)
	if !ok {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out corev1.ResourceRequirements
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return &out
}

// setDuration records the value if set, and warns and skips values that use
// syntax Prometheus (and therefore thanos-operator) rejects.
func setDuration(dst *string, val, field string, warn func(string, ...any)) {
	if val == "" {
		return
	}
	if !prometheusDuration.MatchString(val) {
		warn("%s=%q: not a Prometheus-style duration (e.g. 24h, 365d, 2w); not migrated", field, val)
		return
	}
	*dst = val
}

// validLogLevels mirrors the CommonFields.logLevel enum of the Thanos CRs.
var validLogLevels = []string{"debug", "info", "warn", "error"}

// setLogLevel records the value if it matches the logLevel enum the Thanos CRs
// accept, and warns and skips anything else.
func setLogLevel(dst *string, val, field string, warn func(string, ...any)) {
	if val == "" {
		return
	}
	if !slices.Contains(validLogLevels, val) {
		warn("%s=%q: not one of debug|info|warn|error; not migrated", field, val)
		return
	}
	*dst = val
}

func printSummary(cfg *MigrationConfig) {
	if cfg.isEmpty() {
		fmt.Println("no user-defined Thanos settings found in the MCO CR; nothing to migrate")
		return
	}
	fmt.Println("captured settings:")
	if b, err := yaml.Marshal(struct {
		Compact                CompactConfig   `json:"compact,omitzero"`
		Receive                ReceiveConfig   `json:"receive,omitzero"`
		Ruler                  RulerConfig     `json:"ruler,omitzero"`
		Query                  QueryConfig     `json:"query,omitzero"`
		Store                  StoreConfig     `json:"store,omitzero"`
		StoreMemcached         MemcachedConfig `json:"storeMemcached,omitzero"`
		QueryFrontendMemcached MemcachedConfig `json:"queryFrontendMemcached,omitzero"`
	}{cfg.Compact, cfg.Receive, cfg.Ruler, cfg.Query, cfg.Store, cfg.StoreMemcached, cfg.QueryFrontendMemcached}); err == nil {
		_, _ = os.Stdout.Write(b)
	}
	printWarnings(cfg)
}

func printWarnings(cfg *MigrationConfig) {
	if len(cfg.Warnings) == 0 {
		return
	}
	fmt.Println("\nwarnings (settings NOT migrated):")
	for _, w := range cfg.Warnings {
		fmt.Printf("  - %s\n", w)
	}
}
