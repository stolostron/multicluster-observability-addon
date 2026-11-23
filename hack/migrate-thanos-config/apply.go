// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License, Version 2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	thanosv1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"
)

// errMissingInput is returned when apply is invoked without -f.
var errMissingInput = errors.New("-f is required")

func runApply(args []string) error {
	fs := flag.NewFlagSet("apply", flag.ExitOnError)
	inFile := fs.String("f", "", "path to the migration file produced by capture (required)")
	namespace := fs.String("namespace", mconfig.HubInstallNamespace, "namespace of the MCOA Thanos CRs")
	dryRun := fs.Bool("dry-run", false, "compute and print the changes without writing to the cluster")
	includeSidecars := fs.Bool("include-sidecars", false, "also copy MCO sidecar containers into the Thanos CRs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *inFile == "" {
		return errMissingInput
	}

	data, err := os.ReadFile(*inFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", *inFile, err)
	}
	cfg := &MigrationConfig{}
	if err = yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("failed to parse %s: %w", *inFile, err)
	}
	if cfg.isEmpty() {
		fmt.Println("migration file contains no settings; nothing to apply")
		return nil
	}

	restCfg, err := ctrlconfig.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %w", err)
	}
	scheme := runtime.NewScheme()
	if err = thanosv1alpha1.AddToScheme(scheme); err != nil {
		return fmt.Errorf("failed to register thanos scheme: %w", err)
	}
	if err = corev1.AddToScheme(scheme); err != nil {
		return fmt.Errorf("failed to register core scheme: %w", err)
	}
	if err = appsv1.AddToScheme(scheme); err != nil {
		return fmt.Errorf("failed to register apps scheme: %w", err)
	}
	cl, err := client.New(restCfg, client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx := context.Background()
	key := types.NamespacedName{Name: mconfig.ThanosCRName, Namespace: *namespace}
	storageHints := []*changeLog{}

	type applyStep struct {
		name string
		fn   func(context.Context, client.Client, types.NamespacedName, *MigrationConfig, bool) (*changeLog, error)
		key  types.NamespacedName
	}
	steps := []applyStep{
		{"ThanosCompact", applyCompact, key},
		{"ThanosReceive", applyReceive, key},
		{"ThanosRuler", applyRuler, key},
		{"ThanosQuery", applyQuery, key},
		{"ThanosStore", applyStore, key},
	}
	if !cfg.StoreMemcached.isEmpty() {
		steps = append(steps, applyStep{
			"Deployment/memcached-store", applyMemcachedStore,
			types.NamespacedName{Name: mconfig.MemcachedStoreName, Namespace: *namespace},
		})
	}
	if !cfg.QueryFrontendMemcached.isEmpty() {
		steps = append(steps, applyStep{
			"Deployment/memcached-query-frontend", applyMemcachedQueryFrontend,
			types.NamespacedName{Name: mconfig.MemcachedQueryFrontendName, Namespace: *namespace},
		})
	}

	for _, step := range steps {
		log, err := step.fn(ctx, cl, step.key, cfg, *includeSidecars)
		if err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
		if len(log.lines) == 0 {
			fmt.Printf("%s: no changes\n", step.name)
			continue
		}
		fmt.Printf("%s changes:\n", step.name)
		for _, line := range log.lines {
			fmt.Printf("  %s\n", line)
		}
		if !*dryRun {
			// The object was mutated in place by the apply fn; persist it.
			if err := cl.Update(ctx, log.obj); err != nil {
				return fmt.Errorf("%s: failed to update: %w", step.name, err)
			}
		}
		storageHints = append(storageHints, log)
	}

	if *dryRun {
		fmt.Println("\ndry-run: no changes were written")
	}
	printWarnings(cfg)
	printPVCGuidance(storageHints, *namespace)
	return nil
}

type changeLog struct {
	obj   client.Object
	lines []string
}

func (c *changeLog) set(field, old, new string) {
	c.lines = append(c.lines, fmt.Sprintf("%s: %s -> %s", field, old, new))
}

func (c *changeLog) note(format string, args ...any) {
	c.lines = append(c.lines, fmt.Sprintf("NOTE "+format, args...))
}

func (c *changeLog) setDuration(field string, dst *thanosv1alpha1.Duration, val string) {
	if val == "" {
		return
	}
	if old := string(*dst); old != val {
		*dst = thanosv1alpha1.Duration(val)
		c.set(field, old, val)
	}
}

func (c *changeLog) setInt32(field string, dst *int32, val *int32) {
	if val == nil {
		return
	}
	if old := *dst; old != *val {
		*dst = *val
		c.set(field, fmt.Sprintf("%d", old), fmt.Sprintf("%d", *val))
	}
}

// setAnnotations merges the given annotations into a CR's spec.annotations
// key by key, without removing keys already present. The thanos-operator
// stamps these onto the component's ServiceAccount and pod template.
func (c *changeLog) setAnnotations(field string, dst *map[string]string, val map[string]string) {
	if len(val) == 0 {
		return
	}
	if *dst == nil {
		*dst = make(map[string]string, len(val))
	}
	for k, v := range val {
		old, ok := (*dst)[k]
		if !ok {
			(*dst)[k] = v
			c.set(field+"."+k, "<unset>", v)
			continue
		}
		if old != v {
			(*dst)[k] = v
			c.set(field+"."+k, old, v)
		}
	}
}

// setStrPtr assigns to optional *string CR fields that may be unset.
func (c *changeLog) setStrPtr(field string, dst **string, val string) {
	if val == "" {
		return
	}
	if *dst == nil {
		*dst = &val
		c.set(field, "<unset>", val)
		return
	}
	if old := **dst; old != val {
		*dst = &val
		c.set(field, old, val)
	}
}

// setDurationPtr assigns to optional *Duration CR fields that may be unset.
func (c *changeLog) setDurationPtr(field string, dst **thanosv1alpha1.Duration, val string) {
	if val == "" {
		return
	}
	if *dst == nil {
		d := thanosv1alpha1.Duration(val)
		*dst = &d
		c.set(field, "<unset>", val)
		return
	}
	if old := string(**dst); old != val {
		d := thanosv1alpha1.Duration(val)
		*dst = &d
		c.set(field, old, val)
	}
}

// setInt32Ptr assigns to optional *int32 CR fields that may be unset.
func (c *changeLog) setInt32Ptr(field string, dst **int32, val *int32) {
	if val == nil {
		return
	}
	if *dst == nil {
		*dst = val
		c.set(field, "<unset>", fmt.Sprintf("%d", *val))
		return
	}
	if old := **dst; old != *val {
		*dst = val
		c.set(field, fmt.Sprintf("%d", old), fmt.Sprintf("%d", *val))
	}
}

// setStorageSize validates the quantity syntax and assigns it to the CR's
// storage size (a resource.Quantity).
func (c *changeLog) setStorageSize(field string, dst *resource.Quantity, val string) {
	if val == "" {
		return
	}
	q, err := resource.ParseQuantity(val)
	if err != nil {
		c.note("%s: %q is not a valid quantity: %v", field, val, err)
		return
	}
	if dst.Cmp(q) != 0 {
		old := dst.String()
		*dst = q
		c.set(field, old, val)
	}
}

// setStorageClass only sets the class when the CR does not have one yet:
// storageClass is immutable on the Thanos CRs (XValidation self == oldSelf)
// and PVCs already carry the class they were created with.
func (c *changeLog) setStorageClass(field string, dst **string, val string) {
	if val == "" {
		return
	}
	if *dst != nil {
		if **dst == val {
			return
		}
		c.note("%s: %q not applied, CR already has immutable storageClass %q", field, val, **dst)
		return
	}
	*dst = &val
	c.set(field, "<default>", val)
	c.note("%s: storageClass only takes effect while PVCs do not exist yet; existing volumes keep their class", field)
}

func (c *changeLog) setContainers(field string, dst *[]corev1.Container, val []corev1.Container, include bool) {
	if len(val) == 0 || !include {
		return
	}
	if len(*dst) == 0 {
		*dst = val
		c.set(field, "<none>", fmt.Sprintf("%d container(s)", len(val)))
		return
	}
	c.note("%s: CR already has %d container(s); MCO sidecars not overwritten (merge manually if needed)",
		field, len(*dst))
}

func applyCompact(ctx context.Context, cl client.Client, key types.NamespacedName, cfg *MigrationConfig, includeSidecars bool) (*changeLog, error) {
	obj := &thanosv1alpha1.ThanosCompact{}
	if err := cl.Get(ctx, key, obj); err != nil {
		return nil, fmt.Errorf("failed to get CR (was the thanos-operator stack enabled and are the CRs created?): %w", err)
	}
	log := &changeLog{obj: obj}
	c := cfg.Compact

	log.setDuration("spec.retentionConfig.raw", &obj.Spec.RetentionConfig.Raw, c.RetentionRaw)
	log.setDuration("spec.retentionConfig.fiveMinutes", &obj.Spec.RetentionConfig.FiveMinutes, c.Retention5m)
	log.setDuration("spec.retentionConfig.oneHour", &obj.Spec.RetentionConfig.OneHour, c.Retention1h)
	log.setStrPtr("spec.logLevel", &obj.Spec.LogLevel, c.LogLevel)
	log.setAnnotations("spec.annotations", &obj.Spec.Annotations, c.Annotations)
	log.setStorageSize("spec.storage.size", &obj.Spec.StorageConfiguration.Size, c.StorageSize)
	log.setStorageClass("spec.storage.storageClass", &obj.Spec.StorageConfiguration.StorageClass, c.StorageClass)

	if obj.Spec.DownsamplingConfig == nil {
		obj.Spec.DownsamplingConfig = &thanosv1alpha1.DownsamplingConfig{}
	}
	log.setInt32Ptr("spec.downsamplingConfig.concurrency", &obj.Spec.DownsamplingConfig.Concurrency, c.DownsampleConcurrency)

	if c.BlockMetaFetchConcurrency != nil {
		if obj.Spec.BlockConfig == nil {
			obj.Spec.BlockConfig = &thanosv1alpha1.BlockConfig{}
		}
		log.setInt32Ptr("spec.blockConfig.blockMetaFetchConcurrency", &obj.Spec.BlockConfig.BlockMetaFetchConcurrency, c.BlockMetaFetchConcurrency)
	}

	if c.CleanupInterval != "" {
		if obj.Spec.CompactConfig == nil {
			obj.Spec.CompactConfig = &thanosv1alpha1.CompactConfig{}
		}
		log.setDurationPtr("spec.compactConfig.cleanupInterval", &obj.Spec.CompactConfig.CleanupInterval, c.CleanupInterval)
	}

	log.setContainers("spec.additionalContainers", &obj.Spec.Containers, c.Containers, includeSidecars)
	return log, nil
}

func applyReceive(ctx context.Context, cl client.Client, key types.NamespacedName, cfg *MigrationConfig, includeSidecars bool) (*changeLog, error) {
	obj := &thanosv1alpha1.ThanosReceive{}
	if err := cl.Get(ctx, key, obj); err != nil {
		return nil, fmt.Errorf("failed to get CR (was the thanos-operator stack enabled and are the CRs created?): %w", err)
	}
	log := &changeLog{obj: obj}
	c := cfg.Receive

	if len(obj.Spec.Ingester.Hashrings) == 0 {
		log.note("spec.ingester.hashrings: CR has no hashrings; receive settings not applied")
		return log, nil
	}
	hs := &obj.Spec.Ingester.Hashrings[0]

	// MCO ran a single receive component; MCOA runs a router and an ingester
	// per hashring, so the log level and annotations go to both.
	log.setStrPtr("spec.router.logLevel", &obj.Spec.Router.LogLevel, c.LogLevel)
	log.setStrPtr("spec.ingester.hashrings[0].logLevel", &hs.LogLevel, c.LogLevel)
	log.setAnnotations("spec.router.annotations", &obj.Spec.Router.Annotations, c.Annotations)
	log.setAnnotations("spec.ingester.hashrings[0].annotations", &hs.Annotations, c.Annotations)
	log.setInt32("spec.ingester.hashrings[0].replicas", &hs.Replicas, c.Replicas)
	log.setDuration("spec.ingester.hashrings[0].tsdbConfig.retention", &hs.TSDBConfig.Retention, c.Retention)
	log.setStorageSize("spec.ingester.hashrings[0].storage.size", &hs.StorageConfiguration.Size, c.StorageSize)
	log.setStorageClass("spec.ingester.hashrings[0].storage.storageClass", &hs.StorageConfiguration.StorageClass, c.StorageClass)
	log.setContainers("spec.ingester.additionalContainers", &obj.Spec.Ingester.Containers, c.Containers, includeSidecars)
	return log, nil
}

func applyRuler(ctx context.Context, cl client.Client, key types.NamespacedName, cfg *MigrationConfig, includeSidecars bool) (*changeLog, error) {
	obj := &thanosv1alpha1.ThanosRuler{}
	if err := cl.Get(ctx, key, obj); err != nil {
		return nil, fmt.Errorf("failed to get CR (was the thanos-operator stack enabled and are the CRs created?): %w", err)
	}
	log := &changeLog{obj: obj}
	c := cfg.Ruler

	log.setInt32("spec.replicas", &obj.Spec.Replicas, c.Replicas)
	log.setDuration("spec.evaluationInterval", &obj.Spec.EvaluationInterval, c.EvalInterval)
	log.setAnnotations("spec.annotations", &obj.Spec.Annotations, c.Annotations)
	// Retention lives under the stateful ruler mode in the current API.
	if obj.Spec.RulerMode.Stateful == nil {
		log.note("spec.rulerMode.stateful.retention: ruler is not in stateful mode; retention not applied")
	} else {
		log.setDuration("spec.rulerMode.stateful.retention", &obj.Spec.RulerMode.Stateful.Retention, c.Retention)
	}
	log.setStorageSize("spec.storage.size", &obj.Spec.StorageConfiguration.Size, c.StorageSize)
	log.setStorageClass("spec.storage.storageClass", &obj.Spec.StorageConfiguration.StorageClass, c.StorageClass)
	log.setContainers("spec.additionalContainers", &obj.Spec.Containers, c.Containers, includeSidecars)
	return log, nil
}

func applyQuery(ctx context.Context, cl client.Client, key types.NamespacedName, cfg *MigrationConfig, includeSidecars bool) (*changeLog, error) {
	obj := &thanosv1alpha1.ThanosQuery{}
	if err := cl.Get(ctx, key, obj); err != nil {
		return nil, fmt.Errorf("failed to get CR (was the thanos-operator stack enabled and are the CRs created?): %w", err)
	}
	log := &changeLog{obj: obj}
	c := cfg.Query

	log.setInt32("spec.replicas", &obj.Spec.Replicas, c.Replicas)
	log.setAnnotations("spec.annotations", &obj.Spec.Annotations, c.Annotations)
	if c.QueryFrontendReplicas != nil {
		if obj.Spec.QueryFrontend != nil {
			log.setInt32("spec.queryFrontend.replicas", &obj.Spec.QueryFrontend.Replicas, c.QueryFrontendReplicas)
		} else {
			log.note("spec.queryFrontend.replicas: CR has no query frontend section; not applied")
		}
	}
	if obj.Spec.QueryFrontend != nil {
		log.setAnnotations("spec.queryFrontend.annotations", &obj.Spec.QueryFrontend.Annotations, c.QueryFrontendAnnotations)
	}
	log.setContainers("spec.additionalContainers", &obj.Spec.Containers, c.Containers, includeSidecars)
	if obj.Spec.QueryFrontend != nil {
		log.setContainers("spec.queryFrontend.additionalContainers", &obj.Spec.QueryFrontend.Containers, c.QueryFrontendContainers, includeSidecars)
	}
	return log, nil
}

func applyStore(ctx context.Context, cl client.Client, key types.NamespacedName, cfg *MigrationConfig, includeSidecars bool) (*changeLog, error) {
	obj := &thanosv1alpha1.ThanosStore{}
	if err := cl.Get(ctx, key, obj); err != nil {
		return nil, fmt.Errorf("failed to get CR (was the thanos-operator stack enabled and are the CRs created?): %w", err)
	}
	log := &changeLog{obj: obj}
	c := cfg.Store

	log.setInt32("spec.replicas", &obj.Spec.Replicas, c.Replicas)
	log.setAnnotations("spec.annotations", &obj.Spec.Annotations, c.Annotations)
	log.setStorageSize("spec.storage.size", &obj.Spec.StorageConfiguration.Size, c.StorageSize)
	log.setStorageClass("spec.storage.storageClass", &obj.Spec.StorageConfiguration.StorageClass, c.StorageClass)
	log.setContainers("spec.additionalContainers", &obj.Spec.Containers, c.Containers, includeSidecars)
	return log, nil
}

func applyMemcachedStore(ctx context.Context, cl client.Client, key types.NamespacedName, cfg *MigrationConfig, _ bool) (*changeLog, error) {
	return applyMemcached(ctx, cl, key, cfg.StoreMemcached)
}

func applyMemcachedQueryFrontend(ctx context.Context, cl client.Client, key types.NamespacedName, cfg *MigrationConfig, _ bool) (*changeLog, error) {
	return applyMemcached(ctx, cl, key, cfg.QueryFrontendMemcached)
}

// applyMemcached patches a memcached Deployment rendered by MCOA: the replica
// count and the tuning flags of the "memcached" container (-m memory limit,
// -I max item size, -c connection limit). These edits persist only while MCOA
// manages the Deployments with SSA + OnSpokePresent on those paths.
func applyMemcached(ctx context.Context, cl client.Client, key types.NamespacedName, c MemcachedConfig) (*changeLog, error) {
	obj := &appsv1.Deployment{}
	if err := cl.Get(ctx, key, obj); err != nil {
		return nil, fmt.Errorf("failed to get memcached deployment (was the thanos-operator stack enabled?): %w", err)
	}
	log := &changeLog{obj: obj}

	log.setInt32("spec.replicas", obj.Spec.Replicas, c.Replicas)

	for i := range obj.Spec.Template.Spec.Containers {
		container := &obj.Spec.Template.Spec.Containers[i]
		if container.Name != "memcached" {
			continue
		}
		setArg(log, "args[-m]", &container.Args, "-m", c.MemoryLimitMB)
		setArgStr(log, "args[-I]", &container.Args, "-I", c.MaxItemSize)
		setArg(log, "args[-c]", &container.Args, "-c", c.ConnectionLimit)
		break
	}

	if len(log.lines) > 0 {
		log.note("memcached tuning persists only on MCOA versions managing these Deployments with SSA + OnSpokePresent")
	}
	return log, nil
}

// setArgStr is the string flavor of setArg for knobs kept as plain strings.
func setArgStr(log *changeLog, field string, args *[]string, flag string, val string) {
	if val == "" {
		return
	}
	v := val
	setArg(log, field, args, flag, &v)
}

// setArg replaces (or appends) a "<flag> <value>" style memcached argument.
// The value parameter is generic so int32 and string knobs share the logic.
func setArg[T int32 | string](log *changeLog, field string, args *[]string, flag string, val *T) {
	if val == nil {
		return
	}
	newArg := fmt.Sprintf("%s %v", flag, *val)
	for i, arg := range *args {
		if arg == flag || len(arg) > len(flag) && arg[:len(flag)] == flag && arg[len(flag)] == ' ' {
			if arg == newArg {
				return
			}
			log.set(field, arg, newArg)
			(*args)[i] = newArg
			return
		}
	}
	*args = append(*args, newArg)
	log.set(field, "<unset>", newArg)
}

// printPVCGuidance reminds the operator that raising storage sizes on the CRs
// does not resize existing PVCs: StatefulSet volume claim templates only apply
// to newly created volumes. The thanos-operator may or may not reconcile
// volumes, so this stays a manual verification step.
func printPVCGuidance(logs []*changeLog, namespace string) {
	changed := false
	for _, l := range logs {
		for _, line := range l.lines {
			if len(line) > 5 && line[:5] == "NOTE " {
				continue
			}
			// crude but sufficient: any non-NOTE line mentioning storage means sizes or classes were touched
			if containsStorage(line) {
				changed = true
			}
		}
	}
	if !changed {
		return
	}
	fmt.Printf(`
NOTE: storage size changes above do not resize existing PVCs. Verify and expand manually
(if the storage class allows expansion):
  kubectl get pvc -n %s
  kubectl patch pvc <name> -n %s -p '{"spec":{"resources":{"requests":{"storage":"<new-size>"}}}}'
`, namespace, namespace)
}

func containsStorage(s string) bool {
	return strings.Contains(s, "storage.size") || strings.Contains(s, "storageClass")
}
