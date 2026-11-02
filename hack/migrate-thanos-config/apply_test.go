// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License, Version 2.0

package main

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	thanosv1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := thanosv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return scheme
}

func loadTestMCO(t *testing.T) *unstructured.Unstructured {
	t.Helper()
	data, err := os.ReadFile("testdata/mco.yaml")
	if err != nil {
		t.Fatal(err)
	}
	mco := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(data, &mco.Object); err != nil {
		t.Fatal(err)
	}
	return mco
}

func TestExtract(t *testing.T) {
	cfg := extract(loadTestMCO(t))

	if cfg.Compact.RetentionRaw != "730d" || cfg.Compact.Retention5m != "730d" || cfg.Compact.Retention1h != "1825d" {
		t.Errorf("compact retention not captured: %+v", cfg.Compact)
	}
	if cfg.Receive.Retention != "48h" || cfg.Ruler.Retention != "48h" {
		t.Errorf("retentionInLocal not split to receive and ruler: %+v %+v", cfg.Receive, cfg.Ruler)
	}
	if cfg.Ruler.StorageSize != "5Gi" || cfg.Compact.StorageSize != "200Gi" ||
		cfg.Receive.StorageSize != "150Gi" || cfg.Store.StorageSize != "50Gi" {
		t.Errorf("storage sizes not captured")
	}
	if cfg.Ruler.StorageClass != "gp2-csi" || cfg.Store.StorageClass != "gp2-csi" {
		t.Errorf("storage class not captured")
	}
	if cfg.Query.Replicas == nil || *cfg.Query.Replicas != 3 {
		t.Errorf("query replicas not captured")
	}
	if cfg.Query.QueryFrontendReplicas == nil || *cfg.Query.QueryFrontendReplicas != 3 {
		t.Errorf("query frontend replicas not captured")
	}
	if cfg.Receive.Replicas == nil || *cfg.Receive.Replicas != 6 {
		t.Errorf("receive replicas not captured")
	}
	if cfg.Ruler.Replicas == nil || *cfg.Ruler.Replicas != 3 {
		t.Errorf("ruler replicas not captured")
	}
	if cfg.Store.Replicas == nil || *cfg.Store.Replicas != 3 {
		t.Errorf("store replicas not captured")
	}
	if cfg.Ruler.EvalInterval != "30s" {
		t.Errorf("eval interval not captured")
	}
	if cfg.Compact.DownsampleConcurrency == nil || *cfg.Compact.DownsampleConcurrency != 8 {
		t.Errorf("downsample concurrency not captured")
	}
	if cfg.Compact.BlockMetaFetchConcurrency == nil || *cfg.Compact.BlockMetaFetchConcurrency != 32 {
		t.Errorf("block meta fetch concurrency not captured")
	}
	if cfg.Compact.CleanupInterval != "5m" {
		t.Errorf("wait interval not mapped to cleanup interval")
	}
	if len(cfg.Query.Containers) != 1 || cfg.Query.Containers[0].Name != "oauth-proxy" {
		t.Errorf("query sidecar not captured: %+v", cfg.Query.Containers)
	}

	// ADC suggestion.
	sugg := cfg.SuggestedAddOnDeploymentConfig
	if sugg == nil {
		t.Fatal("expected ADC suggestion")
	}
	if sugg.NodeSelector["node-role.kubernetes.io/infra"] != "" {
		t.Errorf("node selector not captured: %+v", sugg.NodeSelector)
	}
	if len(sugg.Tolerations) != 1 {
		t.Errorf("tolerations not captured")
	}
	if len(sugg.ResourceRequirements) != 1 || sugg.ResourceRequirements[0].ContainerID != "deployments:mcoa:thanos-query" {
		t.Errorf("query resources not mapped to ADC container: %+v", sugg.ResourceRequirements)
	}

	// Memcached tuning must be captured (it maps to the Deployments).
	if cfg.StoreMemcached.Replicas == nil || *cfg.StoreMemcached.Replicas != 2 {
		t.Errorf("store memcached replicas not captured: %+v", cfg.StoreMemcached)
	}
	if cfg.StoreMemcached.MemoryLimitMB == nil || *cfg.StoreMemcached.MemoryLimitMB != 2048 {
		t.Errorf("store memcached memory limit not captured: %+v", cfg.StoreMemcached)
	}
	if cfg.StoreMemcached.MaxItemSize != "5m" {
		t.Errorf("store memcached max item size not captured: %+v", cfg.StoreMemcached)
	}
	if cfg.StoreMemcached.ConnectionLimit == nil || *cfg.StoreMemcached.ConnectionLimit != 2048 {
		t.Errorf("store memcached connection limit not captured: %+v", cfg.StoreMemcached)
	}
	if cfg.QueryFrontendMemcached.MemoryLimitMB == nil || *cfg.QueryFrontendMemcached.MemoryLimitMB != 1024 {
		t.Errorf("query frontend memcached memory limit not captured: %+v", cfg.QueryFrontendMemcached)
	}

	// Gaps must be warned about.
	joined := strings.Join(cfg.Warnings, "\n")
	for _, gap := range []string{"deleteDelay", "blockDuration", "logLevel", "usePrometheusEngine"} {
		if !strings.Contains(joined, gap) {
			t.Errorf("expected a warning mentioning %q, warnings: %v", gap, cfg.Warnings)
		}
	}
}

func TestApplyCompact(t *testing.T) {
	cl := clientfake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(newCompact()).Build()
	key := types.NamespacedName{Name: "mcoa", Namespace: "open-cluster-management-observability"}

	cfg := extract(loadTestMCO(t))
	log, err := applyCompact(context.Background(), cl, key, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := cl.Update(context.Background(), log.obj); err != nil {
		t.Fatal(err)
	}

	applied := &thanosv1alpha1.ThanosCompact{}
	if err := cl.Get(context.Background(), key, applied); err != nil {
		t.Fatal(err)
	}
	if applied.Spec.RetentionConfig.Raw != "730d" {
		t.Errorf("retention raw not applied: %s", applied.Spec.RetentionConfig.Raw)
	}
	if applied.Spec.StorageConfiguration.Size.Cmp(resource.MustParse("200Gi")) != 0 {
		t.Errorf("storage size not applied: %s", applied.Spec.StorageConfiguration.Size.String())
	}
	if applied.Spec.DownsamplingConfig == nil || applied.Spec.DownsamplingConfig.Concurrency == nil ||
		*applied.Spec.DownsamplingConfig.Concurrency != 8 {
		t.Errorf("downsample concurrency not applied")
	}
	if applied.Spec.BlockConfig == nil || applied.Spec.BlockConfig.BlockMetaFetchConcurrency == nil ||
		*applied.Spec.BlockConfig.BlockMetaFetchConcurrency != 32 {
		t.Errorf("block meta fetch concurrency not applied")
	}
	if applied.Spec.CompactConfig == nil || applied.Spec.CompactConfig.CleanupInterval == nil ||
		string(*applied.Spec.CompactConfig.CleanupInterval) != "5m" {
		t.Errorf("cleanup interval not applied")
	}
	// Sidecars must NOT be applied without the flag.
	if len(applied.Spec.Containers) != 0 {
		t.Errorf("sidecars applied without --include-sidecars")
	}
	if len(log.lines) == 0 {
		t.Errorf("expected change log entries")
	}
}

func TestApplyReceiveAndRuler(t *testing.T) {
	cl := clientfake.NewClientBuilder().WithScheme(testScheme(t)).
		WithObjects(newReceive(), newRuler()).Build()
	key := types.NamespacedName{Name: "mcoa", Namespace: "open-cluster-management-observability"}

	cfg := extract(loadTestMCO(t))
	recvLog, err := applyReceive(context.Background(), cl, key, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = cl.Update(context.Background(), recvLog.obj); err != nil {
		t.Fatal(err)
	}
	rulerLog, err := applyRuler(context.Background(), cl, key, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := cl.Update(context.Background(), rulerLog.obj); err != nil {
		t.Fatal(err)
	}

	receive := &thanosv1alpha1.ThanosReceive{}
	if err := cl.Get(context.Background(), key, receive); err != nil {
		t.Fatal(err)
	}
	if len(receive.Spec.Ingester.Hashrings) != 1 {
		t.Fatalf("expected 1 hashring")
	}
	hs := receive.Spec.Ingester.Hashrings[0]
	if hs.Replicas != 6 {
		t.Errorf("hashring replicas not applied: %d", hs.Replicas)
	}
	if string(hs.TSDBConfig.Retention) != "48h" {
		t.Errorf("hashring retention not applied: %s", hs.TSDBConfig.Retention)
	}
	if hs.StorageConfiguration.Size.Cmp(resource.MustParse("150Gi")) != 0 {
		t.Errorf("hashring storage size not applied: %s", hs.StorageConfiguration.Size.String())
	}

	ruler := &thanosv1alpha1.ThanosRuler{}
	if err := cl.Get(context.Background(), key, ruler); err != nil {
		t.Fatal(err)
	}
	if ruler.Spec.Replicas != 3 {
		t.Errorf("ruler replicas not applied: %d", ruler.Spec.Replicas)
	}
	if string(ruler.Spec.EvaluationInterval) != "30s" {
		t.Errorf("eval interval not applied: %s", ruler.Spec.EvaluationInterval)
	}
	if ruler.Spec.RulerMode.Stateful == nil || string(ruler.Spec.RulerMode.Stateful.Retention) != "48h" {
		t.Errorf("ruler retention not applied: %+v", ruler.Spec.RulerMode.Stateful)
	}
}

func TestApplyStorageClassImmutable(t *testing.T) {
	compact := newCompact()
	existing := "fast-rwo"
	compact.Spec.StorageConfiguration.StorageClass = &existing
	cl := clientfake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(compact).Build()
	key := types.NamespacedName{Name: "mcoa", Namespace: "open-cluster-management-observability"}

	cfg := extract(loadTestMCO(t))
	log, err := applyCompact(context.Background(), cl, key, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := cl.Update(context.Background(), log.obj); err != nil {
		t.Fatal(err)
	}

	applied := &thanosv1alpha1.ThanosCompact{}
	if err := cl.Get(context.Background(), key, applied); err != nil {
		t.Fatal(err)
	}
	if applied.Spec.StorageConfiguration.StorageClass == nil || *applied.Spec.StorageConfiguration.StorageClass != "fast-rwo" {
		t.Errorf("immutable storage class was overwritten: %v", applied.Spec.StorageConfiguration.StorageClass)
	}
	found := false
	for _, line := range log.lines {
		if strings.Contains(line, "immutable") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an immutability note in the change log, got: %v", log.lines)
	}
}

func TestApplyQuerySidecars(t *testing.T) {
	cl := clientfake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(newQuery()).Build()
	key := types.NamespacedName{Name: "mcoa", Namespace: "open-cluster-management-observability"}

	cfg := extract(loadTestMCO(t))
	queryLog, err := applyQuery(context.Background(), cl, key, cfg, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := cl.Update(context.Background(), queryLog.obj); err != nil {
		t.Fatal(err)
	}

	query := &thanosv1alpha1.ThanosQuery{}
	if err := cl.Get(context.Background(), key, query); err != nil {
		t.Fatal(err)
	}
	if len(query.Spec.Containers) != 1 || query.Spec.Containers[0].Name != "oauth-proxy" {
		t.Errorf("sidecar not applied with --include-sidecars: %+v", query.Spec.Containers)
	}
}

func TestApplyMemcachedStore(t *testing.T) {
	cl := clientfake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(newMemcachedStore()).Build()
	key := types.NamespacedName{Name: "memcached-store", Namespace: "open-cluster-management-observability"}

	cfg := extract(loadTestMCO(t))
	log, err := applyMemcachedStore(context.Background(), cl, key, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := cl.Update(context.Background(), log.obj); err != nil {
		t.Fatal(err)
	}

	applied := &appsv1.Deployment{}
	if err := cl.Get(context.Background(), key, applied); err != nil {
		t.Fatal(err)
	}
	if applied.Spec.Replicas == nil || *applied.Spec.Replicas != 2 {
		t.Errorf("memcached replicas not applied: %v", applied.Spec.Replicas)
	}
	args := memcachedArgs(applied)
	if !containsArg(args, "-m 2048") {
		t.Errorf("memory limit arg not applied: %v", args)
	}
	if !containsArg(args, "-I 5m") {
		t.Errorf("max item size arg not applied: %v", args)
	}
	if !containsArg(args, "-c 2048") {
		t.Errorf("connection limit arg not applied: %v", args)
	}
	if !containsArg(args, "-v") {
		t.Errorf("-v arg must be preserved: %v", args)
	}
	if len(log.lines) == 0 {
		t.Errorf("expected change log entries")
	}
}

func memcachedArgs(d *appsv1.Deployment) []string {
	for _, c := range d.Spec.Template.Spec.Containers {
		if c.Name == "memcached" {
			return c.Args
		}
	}
	return nil
}

func containsArg(args []string, want string) bool {
	return slices.Contains(args, want)
}

// --- fixtures with the same defaults MCOA renders ---------------------------

func newCompact() *thanosv1alpha1.ThanosCompact {
	return &thanosv1alpha1.ThanosCompact{
		ObjectMeta: metav1.ObjectMeta{Name: "mcoa", Namespace: "open-cluster-management-observability"},
		Spec: thanosv1alpha1.ThanosCompactSpec{
			StorageConfiguration: thanosv1alpha1.StorageConfiguration{
				Size: resource.MustParse("10Gi"),
			},
			RetentionConfig: thanosv1alpha1.RetentionResolutionConfig{
				Raw:         "365d",
				FiveMinutes: "365d",
				OneHour:     "365d",
			},
		},
	}
}

func newReceive() *thanosv1alpha1.ThanosReceive {
	return &thanosv1alpha1.ThanosReceive{
		ObjectMeta: metav1.ObjectMeta{Name: "mcoa", Namespace: "open-cluster-management-observability"},
		Spec: thanosv1alpha1.ThanosReceiveSpec{
			Ingester: thanosv1alpha1.IngesterSpec{
				Hashrings: []thanosv1alpha1.IngesterHashringSpec{
					{
						Name:                 "default",
						Replicas:             3,
						TSDBConfig:           thanosv1alpha1.TSDBConfig{Retention: "24h"},
						StorageConfiguration: thanosv1alpha1.StorageConfiguration{Size: resource.MustParse("10Gi")},
					},
				},
			},
		},
	}
}

func newRuler() *thanosv1alpha1.ThanosRuler {
	return &thanosv1alpha1.ThanosRuler{
		ObjectMeta: metav1.ObjectMeta{Name: "mcoa", Namespace: "open-cluster-management-observability"},
		Spec: thanosv1alpha1.ThanosRulerSpec{
			Replicas:           3,
			EvaluationInterval: "1m",
			RulerMode: thanosv1alpha1.RulerMode{
				Type:     "Stateful",
				Stateful: &thanosv1alpha1.StatefulSpec{Retention: "2h"},
			},
			StorageConfiguration: thanosv1alpha1.StorageConfiguration{Size: resource.MustParse("1Gi")},
		},
	}
}

func newQuery() *thanosv1alpha1.ThanosQuery {
	return &thanosv1alpha1.ThanosQuery{
		ObjectMeta: metav1.ObjectMeta{Name: "mcoa", Namespace: "open-cluster-management-observability"},
		Spec: thanosv1alpha1.ThanosQuerySpec{
			Replicas:      2,
			QueryFrontend: &thanosv1alpha1.QueryFrontendSpec{Replicas: 2},
		},
	}
}

func newMemcachedStore() *appsv1.Deployment {
	replicas := int32(3)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "memcached-store", Namespace: "open-cluster-management-observability"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "memcached",
							Args: []string{"-m 1024", "-I 1m", "-c 1024", "-v"},
						},
					},
				},
			},
		},
	}
}

var _ = resource.Quantity{} // keep resource import if fixtures evolve
