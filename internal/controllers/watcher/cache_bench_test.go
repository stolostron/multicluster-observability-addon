package watcher

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
)

func BenchmarkReferenceCache_Add(b *testing.B) {
	fleetSizes := []int{100, 1000, 2500}

	for _, n := range fleetSizes {
		b.Run(fmt.Sprintf("Clusters=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				cache := NewReferenceCache()
				for j := range n {
					clusterName := "cluster-" + strconv.Itoa(j)
					mwName := "mcoa-mw-" + strconv.Itoa(j)
					configs := map[string]struct{}{
						"v1/Secret/open-cluster-management-observability/thanos-object-storage": {},
						"v1/ConfigMap/" + clusterName + "/custom-metrics":                       {},
					}
					cache.Add(clusterName, mwName, configs)
				}
			}
		})
	}
}

func BenchmarkReferenceCache_GetNamespaces(b *testing.B) {
	fleetSizes := []int{100, 1000, 2500}
	const sharedKey = "v1/Secret/open-cluster-management-observability/thanos-object-storage"

	for _, n := range fleetSizes {
		cache := NewReferenceCache()
		for j := range n {
			clusterName := "cluster-" + strconv.Itoa(j)
			mwName := "mcoa-mw-" + strconv.Itoa(j)
			cache.Add(clusterName, mwName, map[string]struct{}{
				sharedKey: {},
			})
		}

		b.Run(fmt.Sprintf("Clusters=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				nss := cache.GetNamespaces(sharedKey)
				if len(nss) != n {
					b.Fatalf("expected %d namespaces, got %d", n, len(nss))
				}
			}
		})
	}
}

func BenchmarkReferenceCache_Concurrent(b *testing.B) {
	const initialClusters = 1000
	const sharedKey = "v1/Secret/open-cluster-management-observability/thanos-object-storage"

	cache := NewReferenceCache()
	for j := range initialClusters {
		clusterName := "cluster-" + strconv.Itoa(j)
		mwName := "mcoa-mw-" + strconv.Itoa(j)
		cache.Add(clusterName, mwName, map[string]struct{}{
			sharedKey: {},
		})
	}

	var mu sync.Mutex
	counter := initialClusters

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		op := 0
		for pb.Next() {
			op++
			if op%10 == 0 {
				// 10% Write operations (Add/Remove)
				mu.Lock()
				counter++
				idx := counter
				mu.Unlock()
				cName := "cluster-" + strconv.Itoa(idx)
				mwName := "mcoa-mw-" + strconv.Itoa(idx)
				cache.Add(cName, mwName, map[string]struct{}{sharedKey: {}})
				cache.Remove(cName, mwName)
			} else {
				// 90% Read operations
				_ = cache.GetNamespaces(sharedKey)
			}
		}
	})
}
