package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/controllers/watcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	workv1 "open-cluster-management.io/api/work/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

type watcherTrigger struct {
	ClusterName string
	AddonName   string
}

type watcherFakeAddonManager struct {
	addonmanager.AddonManager
	mu       sync.Mutex
	triggers []watcherTrigger
}

func (f *watcherFakeAddonManager) Trigger(clusterName, addonName string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.triggers = append(f.triggers, watcherTrigger{
		ClusterName: clusterName,
		AddonName:   addonName,
	})
}

func (f *watcherFakeAddonManager) getTriggers() []watcherTrigger {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]watcherTrigger, len(f.triggers))
	copy(out, f.triggers)
	return out
}

func (f *watcherFakeAddonManager) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.triggers = nil
}

// TestWatcherReconciler_SecretMutationTrigger serves as the representative end-to-end integration test
// for the Watcher controller.
//
// Architectural Scope:
// This test exercises the most complex asynchronous path in WatcherReconciler:
//  1. Live controller-runtime Manager bootstrap and Informer cache synchronization.
//  2. Dynamic cache ingestion (updateCache) extracting original-resource annotations from ManifestWorks.
//  3. HTTP/2 watch stream event dispatch from kube-apiserver upon discrete upstream Secret mutation.
//  4. ReferenceCache reverse-lookup resolving the mutated Secret to the downstream spoke cluster namespace.
//  5. Reconciliation queue dispatch invoking Reconcile() and addonManager.Trigger().
//
// Testing Pyramid Boundary:
// Once this test proves that the live Informer -> Cache -> Queue -> Reconciler pipeline functions correctly against
// real kube-apiserver and etcd binaries, other watch triggers (such as the global images ConfigMap, Hypershift
// ServiceMonitors, and MultiClusterHub network policies) do not need redundant envtest suites. Their custom behavior
// lies strictly in deterministic predicates and queue mappers (e.g., mchNetworkPoliciesPredicate, isHypershiftServiceMonitor,
// enqueueForAllManagedClusters), which are tested exhaustively in milliseconds in internal/controllers/watcher/controller_test.go.
func TestWatcherReconciler_SecretMutationTrigger(t *testing.T) {
	testEnv := SetupTestEnv(t)
	ctx := t.Context()

	addonMgr := &watcherFakeAddonManager{}

	disableNameValidation := true
	mgr, err := ctrl.NewManager(testEnv.Cfg, ctrl.Options{
		Scheme: testEnv.Scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		Controller: config.Controller{
			SkipNameValidation: &disableNameValidation,
		},
	})
	require.NoError(t, err)

	require.NoError(t, watcher.SetupWithManager(mgr, addonMgr, testr.New(t)))

	mgrCtx, cancelMgr := context.WithCancel(ctx)
	mgrErrCh := make(chan error, 1)
	go func() {
		mgrErrCh <- mgr.Start(mgrCtx)
	}()

	t.Cleanup(func() {
		cancelMgr()
		select {
		case mgrErr := <-mgrErrCh:
			assert.NoError(t, mgrErr, "controller manager exited with unexpected error")
		case <-time.After(5 * time.Second):
			t.Error("timed out waiting for controller manager to shutdown")
		}
	})

	require.True(t, mgr.GetCache().WaitForCacheSync(ctx), "failed waiting for manager caches to sync")

	spokeCluster := "spoke-cluster-alpha"
	hubConfigNs := "hub-config-ns"

	for _, nsName := range []string{spokeCluster, hubConfigNs} {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName}}
		require.NoError(t, testEnv.K8sClient.Create(ctx, ns))
	}

	// 1. Create upstream Secret on the hub
	upstreamSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "upstream-config-secret",
			Namespace: hubConfigNs,
		},
		StringData: map[string]string{
			"key": "initial-secret-data",
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, upstreamSecret))

	// 2. Wrap secret into ManifestWork
	embeddedSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spoke-secret",
			Namespace: spokeCluster,
			Annotations: map[string]string{
				addoncfg.AnnotationOriginalResource: fmt.Sprintf("%s/%s", hubConfigNs, upstreamSecret.Name),
			},
		},
		Data: map[string][]byte{
			"key": []byte("initial-secret-data"),
		},
	}
	rawSecret, err := json.Marshal(embeddedSecret)
	require.NoError(t, err)

	mw := &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mcoa-spoke-work",
			Namespace: spokeCluster,
			Labels: map[string]string{
				addoncfg.LabelOCMAddonName: addoncfg.Name,
			},
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: []workv1.Manifest{
					{RawExtension: runtime.RawExtension{Raw: rawSecret}},
				},
			},
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, mw))

	// 3. Wait for the controller manager to index the ManifestWork in its informer cache.
	require.Eventually(t, func() bool {
		var syncedMw workv1.ManifestWork
		return mgr.GetClient().Get(ctx, client.ObjectKeyFromObject(mw), &syncedMw) == nil
	}, 5*time.Second, 20*time.Millisecond, "expected ManifestWork to be cached by manager")

	addonMgr.reset()

	// 4. Mutate upstream secret ONCE
	var curSecret corev1.Secret
	require.NoError(t, testEnv.K8sClient.Get(ctx, client.ObjectKeyFromObject(upstreamSecret), &curSecret))
	curSecret.StringData = map[string]string{
		"updated-at": time.Now().String(),
	}
	require.NoError(t, testEnv.K8sClient.Update(ctx, &curSecret))

	// 5. Verify that Watcher controller detects the mutation and triggers addon reconciliation for spokeCluster.
	assert.Eventually(t, func() bool {
		for _, tr := range addonMgr.getTriggers() {
			if tr.ClusterName == spokeCluster && tr.AddonName == addoncfg.Name {
				return true
			}
		}
		return false
	}, 5*time.Second, 50*time.Millisecond, "expected secret mutation to trigger addon reconciliation for spoke cluster")
}
