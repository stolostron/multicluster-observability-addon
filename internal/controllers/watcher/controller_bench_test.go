package watcher

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	workv1 "open-cluster-management.io/api/work/v1"
)

func BenchmarkUpdateCache(b *testing.B) {
	// Create realistic manifests: 2 tracked configs (Secret & ConfigMap with original-resource annotation)
	// and 13 untracked manifests (typical of MCOA ManifestWork).
	manifests := make([]workv1.Manifest, 0, 15)

	// 1. Tracked Secret
	trackedSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "thanos-object-storage",
			Namespace: "open-cluster-management-addon-observability",
			Annotations: map[string]string{
				addoncfg.AnnotationOriginalResource: "open-cluster-management-observability/thanos-object-storage",
			},
		},
	}
	secRaw, _ := json.Marshal(trackedSecret)
	manifests = append(manifests, workv1.Manifest{RawExtension: runtime.RawExtension{Raw: secRaw}})

	// 2. Tracked ConfigMap
	trackedCM := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom-metrics-rules",
			Namespace: "open-cluster-management-addon-observability",
			Annotations: map[string]string{
				addoncfg.AnnotationOriginalResource: "open-cluster-management-observability/custom-metrics",
			},
		},
	}
	cmRaw, _ := json.Marshal(trackedCM)
	manifests = append(manifests, workv1.Manifest{RawExtension: runtime.RawExtension{Raw: cmRaw}})

	// 3-15. 13 Untracked manifests (ServiceAccount, RBAC, Services, ScrapeConfigs, untracked CMs/Secrets)
	for i := range 13 {
		var raw []byte
		switch i % 5 {
		case 0:
			raw = fmt.Appendf(nil, `{"apiVersion":"v1","kind":"ServiceAccount","metadata":{"name":"sa-%d","namespace":"open-cluster-management-addon-observability"}}`, i)
		case 1:
			raw = fmt.Appendf(nil, `{"apiVersion":"rbac.authorization.k8s.io/v1","kind":"ClusterRoleBinding","metadata":{"name":"crb-%d"}}`, i)
		case 2:
			raw = fmt.Appendf(nil, `{"apiVersion":"v1","kind":"Service","metadata":{"name":"svc-%d","namespace":"open-cluster-management-addon-observability"}}`, i)
		case 3:
			raw = fmt.Appendf(nil, `{"apiVersion":"monitoring.rhobs/v1alpha1","kind":"ScrapeConfig","metadata":{"name":"scrape-%d","namespace":"open-cluster-management-addon-observability"}}`, i)
		case 4:
			raw = fmt.Appendf(nil, `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"untracked-cm-%d","namespace":"open-cluster-management-addon-observability"}}`, i)
		}
		manifests = append(manifests, workv1.Manifest{RawExtension: runtime.RawExtension{Raw: raw}})
	}

	mw := &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "addon-mw",
			Namespace: "cluster-1",
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: manifests,
			},
		},
	}

	r := &WatcherReconciler{
		Cache: NewReferenceCache(),
		Log:   logr.Discard(),
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r.updateCache(mw)
	}
}
