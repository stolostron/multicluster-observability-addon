//nolint:gci,gofumpt,goimports
package handlers

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	intstr "k8s.io/apimachinery/pkg/util/intstr"
	kubescheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestHypershift_Nominal(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, kubescheme.AddToScheme(scheme))
	require.NoError(t, hyperv1.AddToScheme(scheme))
	require.NoError(t, prometheusv1.AddToScheme(scheme))
	require.NoError(t, prometheusalpha1.AddToScheme(scheme))
	require.NoError(t, clusterv1.AddToScheme(scheme))

	mc := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"feature.open-cluster-management.io/addon-hypershift-addon": "available",
			},
		},
		Spec: clusterv1.ManagedClusterSpec{},
	}

	etcdScrapeConfig := &prometheusalpha1.ScrapeConfig{
		Spec: prometheusalpha1.ScrapeConfigSpec{
			Params: map[string][]string{
				"match[]": {
					`{__name__=":node_memory_MemAvailable_bytes:sum"}`, // ignore rules
					`{__name__=~"acm_"}`,                               // ignore regex
					`{__name__="acm_managed_cluster_labels"}`,
				},
			},
		},
	}
	etcdRule := &prometheusv1.PrometheusRule{
		Spec: prometheusv1.PrometheusRuleSpec{
			Groups: []prometheusv1.RuleGroup{
				{
					Rules: []prometheusv1.Rule{
						{
							Expr: intstr.IntOrString{
								StrVal: `(histogram_quantile(0.99,sum(rate(apiserver_request_duration_seconds_bucket{job="apiserver",
								verb!="WATCH",clusterID!=""}[5m])) by (le, verb, instance, cluster, clusterID, managementcluster, managementclusterID)))`,
							},
						},
					},
				},
			},
		},
	}
	apiserverScrapeConfig := &prometheusalpha1.ScrapeConfig{
		Spec: prometheusalpha1.ScrapeConfigSpec{
			Params: map[string][]string{
				"match[]": {
					`{__name__=":node_memory_MemAvailable_bytes:sum"}`, // ignore rules
					`{__name__=~"acm_"}`,                               // ignore regex
					`{__name__="grpc_server_handled_total"}`,
				},
			},
		},
	}
	apiserverRule := &prometheusv1.PrometheusRule{
		Spec: prometheusv1.PrometheusRuleSpec{
			Groups: []prometheusv1.RuleGroup{
				{
					Rules: []prometheusv1.Rule{
						{
							Expr: intstr.IntOrString{
								StrVal: `sum(grpc_server_started_total{job="etcd",grpc_service="etcdserverpb.Watch",grpc_type="bidi_stream",clusterID!=""}) by (cluster, clusterID, managementcluster, managementclusterID)
- sum(grpc_server_handled_total{job="etcd",grpc_service="etcdserverpb.Watch",grpc_type="bidi_stream",clusterID!=""})  by (cluster, clusterID, managementcluster, managementclusterID)`,
							},
						},
					},
				},
			},
		},
	}

	hostedCluster := &hyperv1.HostedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: hyperv1.HostedClusterSpec{
			ClusterID: "cluster-id",
		},
	}

	targetPort := intstr.FromString("target")
	hyperEtcdSM := &prometheusv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.HypershiftEtcdServiceMonitorName,
			Namespace: "namespace-name",
		},
		Spec: prometheusv1.ServiceMonitorSpec{
			Endpoints: []prometheusv1.Endpoint{
				{
					Port:       "metrics",
					TargetPort: &targetPort,
					TLSConfig: &prometheusv1.TLSConfig{
						CAFile: "cafile",
					},
				},
			},
			Selector: *metav1.SetAsLabelSelector(map[string]string{
				"test": "test",
			}),
			NamespaceSelector: prometheusv1.NamespaceSelector{
				Any: true,
			},
		},
	}

	hyperApiserverSM := &prometheusv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.HypershiftApiServerServiceMonitorName,
			Namespace: "namespace-name",
		},
		Spec: prometheusv1.ServiceMonitorSpec{
			Endpoints: []prometheusv1.Endpoint{
				{
					TargetPort: &targetPort,
					Port:       "client",
					TLSConfig: &prometheusv1.TLSConfig{
						CAFile: "cafile",
					},
				},
			},
			Selector: *metav1.SetAsLabelSelector(map[string]string{
				"test": "test",
			}),
			NamespaceSelector: prometheusv1.NamespaceSelector{
				Any: true,
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(hostedCluster, hyperEtcdSM, hyperApiserverSM).Build()

	hype := Hypershift{
		Client:         fakeClient,
		ManagedCluster: mc,
		Logger:         logr.Discard(),
	}

	res, err := hype.GenerateResources(context.Background(),
		CollectionConfig{ScrapeConfigs: []*prometheusalpha1.ScrapeConfig{etcdScrapeConfig}, Rules: []*prometheusv1.PrometheusRule{etcdRule}},
		CollectionConfig{ScrapeConfigs: []*prometheusalpha1.ScrapeConfig{apiserverScrapeConfig}, Rules: []*prometheusv1.PrometheusRule{apiserverRule}},
	)
	assert.NoError(t, err)
	assert.Len(t, res.ScrapeConfigs, 2)
	assert.Len(t, res.Rules, 2)
	assert.Len(t, res.ServiceMonitors, 2)

	// Ensure that serviceMonitors have correct relabellings, namespaces and connection configuration
	for _, sm := range res.ServiceMonitors {
		assert.Equal(t, "namespace-name", sm.Namespace)
		assert.Len(t, sm.Spec.Endpoints, 1)
		assert.Len(t, sm.Spec.Endpoints[0].MetricRelabelConfigs, 5) // metrics filter and 4 cluster and managedCluster
		assert.Len(t, sm.Spec.Endpoints[0].RelabelConfigs, 1)
		assert.NotEmpty(t, sm.Spec.Endpoints[0].Port)
		assert.NotEmpty(t, sm.Spec.Endpoints[0].TargetPort.StrVal)
		assert.NotEmpty(t, sm.Spec.Endpoints[0].TLSConfig)
		assert.NotEmpty(t, sm.Spec.Selector)
		assert.NotEmpty(t, sm.Spec.NamespaceSelector)
	}
}

func TestHypershift_NoHCP(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, kubescheme.AddToScheme(scheme))
	require.NoError(t, hyperv1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	mc := &clusterv1.ManagedCluster{}
	hype := Hypershift{
		Client:         fakeClient,
		ManagedCluster: mc,
		Logger:         logr.Discard(),
	}

	res, err := hype.GenerateResources(context.Background(), CollectionConfig{}, CollectionConfig{})
	assert.NoError(t, err)
	assert.Len(t, res.Rules, 0)
	assert.Len(t, res.ScrapeConfigs, 0)
	assert.Len(t, res.ServiceMonitors, 0)
}

func TestHypershift_NoScrapeConfigsAndRules(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, kubescheme.AddToScheme(scheme))
	require.NoError(t, hyperv1.AddToScheme(scheme))
	require.NoError(t, prometheusv1.AddToScheme(scheme))
	require.NoError(t, prometheusalpha1.AddToScheme(scheme))
	require.NoError(t, clusterv1.AddToScheme(scheme))

	hostedCluster := &hyperv1.HostedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: hyperv1.HostedClusterSpec{
			ClusterID: "cluster-id",
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(hostedCluster).Build()

	mc := &clusterv1.ManagedCluster{}
	hype := Hypershift{
		Client:         fakeClient,
		ManagedCluster: mc,
		Logger:         logr.Discard(),
	}

	res, err := hype.GenerateResources(context.Background(), CollectionConfig{}, CollectionConfig{})
	assert.NoError(t, err)
	assert.Len(t, res.Rules, 0)
	assert.Len(t, res.ScrapeConfigs, 0)
	assert.Len(t, res.ServiceMonitors, 0)
}

func TestHypershift_NoHypershiftServiceMonitors(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, kubescheme.AddToScheme(scheme))
	require.NoError(t, hyperv1.AddToScheme(scheme))
	require.NoError(t, prometheusv1.AddToScheme(scheme))
	require.NoError(t, prometheusalpha1.AddToScheme(scheme))
	require.NoError(t, clusterv1.AddToScheme(scheme))

	hostedCluster := &hyperv1.HostedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: hyperv1.HostedClusterSpec{
			ClusterID: "cluster-id",
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(hostedCluster).Build()

	apiserverScrapeConfig := &prometheusalpha1.ScrapeConfig{
		Spec: prometheusalpha1.ScrapeConfigSpec{
			Params: map[string][]string{
				"match[]": {
					`{__name__=":node_memory_MemAvailable_bytes:sum"}`, // ignore rules
					`{__name__=~"acm_"}`,                               // ignore regex
					`{__name__="grpc_server_handled_total"}`,
				},
			},
		},
	}

	etcdScrapeConfig := &prometheusalpha1.ScrapeConfig{
		Spec: prometheusalpha1.ScrapeConfigSpec{
			Params: map[string][]string{
				"match[]": {
					`{__name__=":node_memory_MemAvailable_bytes:sum"}`, // ignore rules
					`{__name__=~"acm_"}`,                               // ignore regex
					`{__name__="acm_managed_cluster_labels"}`,
				},
			},
		},
	}

	mc := &clusterv1.ManagedCluster{}
	hype := Hypershift{
		Client:         fakeClient,
		ManagedCluster: mc,
		Logger:         logr.Discard(),
	}

	res, err := hype.GenerateResources(context.Background(),
		CollectionConfig{ScrapeConfigs: []*prometheusalpha1.ScrapeConfig{etcdScrapeConfig}},
		CollectionConfig{ScrapeConfigs: []*prometheusalpha1.ScrapeConfig{apiserverScrapeConfig}},
	)
	assert.NoError(t, err)
	assert.Len(t, res.Rules, 0)
	assert.Len(t, res.ScrapeConfigs, 2)
	assert.Len(t, res.ServiceMonitors, 0)
}

func TestHypershift_ExtractDependentMetrics(t *testing.T) {
	testCases := map[string]struct {
		scrapeConfig *prometheusalpha1.ScrapeConfig
		rule         *prometheusv1.PrometheusRule
		expectResult []string
		expectError  bool
	}{
		"none": {},
		"invalid scrape config": {
			scrapeConfig: &prometheusalpha1.ScrapeConfig{
				Spec: prometheusalpha1.ScrapeConfigSpec{
					Params: map[string][]string{
						"match[]": {
							`{__name__"acm_label_names"}`,
						},
					},
				},
			},
			expectError: true,
		},
		"scrape config": {
			scrapeConfig: &prometheusalpha1.ScrapeConfig{
				Spec: prometheusalpha1.ScrapeConfigSpec{
					Params: map[string][]string{
						"match[]": {
							`{__name__=":node_memory_MemAvailable_bytes:sum"}`, // ignore rules
							`{__name__=~"acm_"}`,                               // ignore regex
							`{__name__="acm_managed_cluster_labels"}`,
							`{__name__="active_streams_lease:grpc_server_handled_total:sum"}`,
						},
					},
				},
			},
			expectResult: []string{
				"acm_managed_cluster_labels",
			},
		},
		"invalid rule": {
			rule: &prometheusv1.PrometheusRule{
				Spec: prometheusv1.PrometheusRuleSpec{
					Groups: []prometheusv1.RuleGroup{
						{
							Rules: []prometheusv1.Rule{
								{
									Expr: intstr.IntOrString{
										StrVal: `(histogram_quantile0.99,sum(rate(apiserver_request_duration_seconds_bucket{job="apiserver",
										verb!="WATCH",clusterID!=""}[5m])) by (le, verb, instance, cluster, clusterID, managementcluster, managementclusterID)))`,
									},
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		"rule": {
			rule: &prometheusv1.PrometheusRule{
				Spec: prometheusv1.PrometheusRuleSpec{
					Groups: []prometheusv1.RuleGroup{
						{
							Rules: []prometheusv1.Rule{
								{
									Expr: intstr.IntOrString{
										StrVal: `(histogram_quantile(0.99,sum(rate(apiserver_request_duration_seconds_bucket{job="apiserver",
										verb!="WATCH",clusterID!=""}[5m])) by (le, verb, instance, cluster, clusterID, managementcluster, managementclusterID)))`,
									},
								},
								{
									Expr: intstr.IntOrString{
										StrVal: `sum(grpc_server_started_total{job="etcd",grpc_service="etcdserverpb.Watch",grpc_type="bidi_stream",clusterID!=""}) by (cluster, clusterID, managementcluster, managementclusterID)
		- sum(grpc_server_handled_total{job="etcd",grpc_service="etcdserverpb.Watch",grpc_type="bidi_stream",clusterID!=""})  by (cluster, clusterID, managementcluster, managementclusterID)`,
									},
								},
							},
						},
					},
				},
			},
			expectResult: []string{
				"apiserver_request_duration_seconds_bucket",
				"grpc_server_handled_total",
				"grpc_server_started_total",
			},
		},
		"merged scrape config and rule": { // is deduplicated and sorted alphabetically
			scrapeConfig: &prometheusalpha1.ScrapeConfig{
				Spec: prometheusalpha1.ScrapeConfigSpec{
					Params: map[string][]string{
						"match[]": {
							`{__name__="grpc_server_started_total"}`,
							`{__name__="apiserver_request_duration_seconds_bucket"}`,
						},
					},
				},
			},
			rule: &prometheusv1.PrometheusRule{
				Spec: prometheusv1.PrometheusRuleSpec{
					Groups: []prometheusv1.RuleGroup{
						{
							Rules: []prometheusv1.Rule{
								{
									Expr: intstr.IntOrString{
										StrVal: `(histogram_quantile(0.99,sum(rate(apiserver_request_duration_seconds_bucket{job="apiserver",
										verb!="WATCH",clusterID!=""}[5m])) by (le, verb, instance, cluster, clusterID, managementcluster, managementclusterID)))`,
									},
								},
								{
									Expr: intstr.IntOrString{
										StrVal: `sum(grpc_server_started_total{job="etcd",grpc_service="etcdserverpb.Watch",grpc_type="bidi_stream",clusterID!=""}) by (cluster, clusterID, managementcluster, managementclusterID)
		- sum(grpc_server_handled_total{job="etcd",grpc_service="etcdserverpb.Watch",grpc_type="bidi_stream",clusterID!=""})  by (cluster, clusterID, managementcluster, managementclusterID)`,
									},
								},
							},
						},
					},
				},
			},
			expectResult: []string{
				"apiserver_request_duration_seconds_bucket",
				"grpc_server_handled_total",
				"grpc_server_started_total",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			hype := Hypershift{
				Logger: logr.Discard(),
			}
			scs := []*prometheusalpha1.ScrapeConfig{tc.scrapeConfig}
			rules := []*prometheusv1.PrometheusRule{tc.rule}
			res, err := hype.extractDependentMetrics(scs, rules)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expectResult, res)
		})
	}
}
