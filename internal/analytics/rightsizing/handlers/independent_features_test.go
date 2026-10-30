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

func TestBuildWorkloadPodEvaluationIntervals(t *testing.T) {
	defaults := intervalPair{"5m", "15m"}
	cluster := &clusterv1.ManagedCluster{ObjectMeta: metav1.ObjectMeta{
		Name: "cluster-1", Labels: map[string]string{"vendor": "OpenShift"},
	}}
	opts := newPlatformOpts(false, false)
	opts.Platform.AnalyticsOptions.RightSizing.WorkloadEnabled = true
	opts.Platform.AnalyticsOptions.RightSizing.PodEnabled = true

	assertBuild := func(t *testing.T, builder *OptionsBuilder, workload, pod intervalPair) {
		t.Helper()
		got, err := builder.Build(t.Context(), cluster, opts)
		require.NoError(t, err)
		for _, feature := range []struct {
			name      string
			component ComponentOptions
			want      intervalPair
		}{
			{rightsizing.WorkloadConfigMapName, got.WorkloadRightSizing, workload},
			{rightsizing.PodConfigMapName, got.PodRightSizing, pod},
		} {
			require.True(t, feature.component.Enabled)
			require.Len(t, feature.component.PrometheusRules, 1)
			groups := feature.component.PrometheusRules[0].Spec.Groups
			require.Len(t, groups, 2)
			require.NotNil(t, groups[0].Interval)
			require.NotNil(t, groups[1].Interval)
			require.Len(t, groups[0].Rules, 7)
			rolling, daily := feature.want.rolling, feature.want.daily
			if rolling == "" {
				rolling = defaults.rolling
			}
			if daily == "" {
				daily = defaults.daily
			}
			require.Equal(t, rolling, string(*groups[0].Interval), feature.name)
			require.Equal(t, daily, string(*groups[1].Interval), feature.name)
			for _, rule := range groups[0].Rules[1:] {
				require.Contains(t, rule.Expr.String(), "["+rolling+":]", rule.Record)
			}
			var cm corev1.ConfigMap
			require.NoError(t, builder.Client.Get(t.Context(), types.NamespacedName{
				Name: feature.name, Namespace: addoncfg.InstallNamespace,
			}, &cm))
			stored, err := rightsizing.ParseConfigMapData(cm.Data)
			require.NoError(t, err)
			require.Equal(t, feature.want.rolling, stored.PrometheusRuleConfig.EvaluationInterval, feature.name)
			require.Equal(t, feature.want.daily, stored.PrometheusRuleConfig.EvaluationInterval1d, feature.name)
		}
	}

	t.Run("new ConfigMaps expose the shared defaults", func(t *testing.T) {
		resetIntervalState(t)
		assertBuild(t, newTestOptionsBuilder(t), defaults, defaults)
	})

	t.Run("existing ConfigMaps without keys are not rewritten", func(t *testing.T) {
		resetIntervalState(t)
		builder := newTestOptionsBuilder(t,
			newIntervalConfigMap(rightsizing.WorkloadConfigMapName, "", ""),
			newIntervalConfigMap(rightsizing.PodConfigMapName, "", ""))
		for _, name := range []string{rightsizing.WorkloadConfigMapName, rightsizing.PodConfigMapName} {
			var before, after corev1.ConfigMap
			key := types.NamespacedName{Name: name, Namespace: addoncfg.InstallNamespace}
			require.NoError(t, builder.Client.Get(t.Context(), key, &before))
			assertBuild(t, builder, intervalPair{}, intervalPair{})
			require.NoError(t, builder.Client.Get(t.Context(), key, &after))
			require.Equal(t, before, after)
		}
	})

	t.Run("edits and invalid values are handled independently", func(t *testing.T) {
		resetIntervalState(t)
		builder := newTestOptionsBuilder(t,
			newIntervalConfigMap(rightsizing.WorkloadConfigMapName, "", ""),
			newIntervalConfigMap(rightsizing.PodConfigMapName, "", ""))
		for _, step := range []struct {
			name                                 string
			workload, pod, wantWorkload, wantPod intervalPair
		}{
			{"custom intervals", intervalPair{"10m", "1h"}, intervalPair{"20m", "2h"}, intervalPair{"10m", "1h"}, intervalPair{"20m", "2h"}},
			{"reject malformed and excessive intervals", intervalPair{"bad", "7h"}, intervalPair{"2h", "0m"}, intervalPair{"10m", "1h"}, intervalPair{"20m", "2h"}},
			{"keep valid fields when the other is below its floor", intervalPair{"4m", "3h"}, intervalPair{"30m", "10m"}, intervalPair{"10m", "3h"}, intervalPair{"30m", "2h"}},
			{"remove keys", intervalPair{}, intervalPair{}, intervalPair{}, intervalPair{}},
			{"invalid edits after removal use defaults", intervalPair{"0m", "bad"}, intervalPair{"2h", "7h"}, defaults, defaults},
		} {
			t.Run(step.name, func(t *testing.T) {
				for name, pair := range map[string]intervalPair{
					rightsizing.WorkloadConfigMapName: step.workload,
					rightsizing.PodConfigMapName:      step.pod,
				} {
					var cm corev1.ConfigMap
					require.NoError(t, builder.Client.Get(t.Context(), types.NamespacedName{Name: name, Namespace: addoncfg.InstallNamespace}, &cm))
					cm.Data = newIntervalConfigMap(name, pair.rolling, pair.daily).Data
					require.NoError(t, builder.Client.Update(t.Context(), &cm))
				}
				assertBuild(t, builder, step.wantWorkload, step.wantPod)
			})
		}
	})
}
