package handlers

import (
	"fmt"
	"strings"
	"testing"

	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

func TestIndependentWorkloadPodLifecycle(t *testing.T) {
	for _, workload := range []bool{false, true} {
		for _, pod := range []bool{false, true} {
			t.Run(fmt.Sprintf("workload=%t/pod=%t", workload, pod), func(t *testing.T) {
				builder := newTestOptionsBuilder(t)
				opts := newPlatformOpts(false, false)
				opts.Platform.Metrics.CollectionEnabled = true
				opts.Platform.AnalyticsOptions.RightSizing.WorkloadEnabled = workload
				opts.Platform.AnalyticsOptions.RightSizing.PodEnabled = pod
				cluster := &clusterv1.ManagedCluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster-1", Labels: map[string]string{"vendor": "OpenShift"}}}
				got, err := builder.Build(t.Context(), cluster, opts)
				require.NoError(t, err)
				require.Equal(t, workload, got.WorkloadRightSizing.Enabled)
				require.Equal(t, pod, got.PodRightSizing.Enabled)
				values, err := BuildValues(got)
				require.NoError(t, err)
				if workload || pod {
					require.NotNil(t, values)
					require.Equal(t, workload, values.WorkloadRightSizing != nil)
					require.Equal(t, pod, values.PodRightSizing != nil)
					require.NotNil(t, got.ScrapeConfig)
					matches := strings.Join(got.ScrapeConfig.Spec.Params["match[]"], " ")
					require.Equal(t, workload, strings.Contains(matches, "acm_rs:workload:"))
					require.Equal(t, pod, strings.Contains(matches, "acm_rs:pod:"))
				} else {
					require.Nil(t, values)
					require.Nil(t, got.ScrapeConfig)
				}
				for _, component := range []struct {
					enabled          bool
					cm, rule, prefix string
					options          ComponentOptions
				}{
					{workload, rightsizing.WorkloadConfigMapName, rightsizing.WorkloadPrometheusRuleName, "acm_rs:workload:", got.WorkloadRightSizing},
					{pod, rightsizing.PodConfigMapName, rightsizing.PodPrometheusRuleName, "acm_rs:pod", got.PodRightSizing},
				} {
					err := builder.Client.Get(t.Context(), types.NamespacedName{Name: component.cm, Namespace: addoncfg.InstallNamespace}, &corev1.ConfigMap{})
					if component.enabled {
						require.NoError(t, err)
						require.Len(t, component.options.PrometheusRules, 1)
						rule := component.options.PrometheusRules[0]
						require.Equal(t, component.rule, rule.Name)
						for _, group := range rule.Spec.Groups {
							for _, record := range group.Rules {
								require.True(t, strings.HasPrefix(record.Record, component.prefix), record.Record)
							}
						}
					} else {
						require.True(t, apierrors.IsNotFound(err))
						// Seed a stale ConfigMap to verify disable cleanup.
						require.NoError(t, builder.Client.Create(t.Context(), createTestConfigMap(component.cm)))
					}
				}
				require.NoError(t, builder.ReconcileRSResources(t.Context(), opts))
				for name, enabled := range map[string]bool{rightsizing.WorkloadConfigMapName: workload, rightsizing.PodConfigMapName: pod} {
					err := builder.Client.Get(t.Context(), types.NamespacedName{Name: name, Namespace: addoncfg.InstallNamespace}, &corev1.ConfigMap{})
					if enabled {
						require.NoError(t, err)
					} else {
						require.True(t, apierrors.IsNotFound(err))
					}
				}
			})
		}
	}
}
