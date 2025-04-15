package handlers

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type clusterIdentity struct {
	ID   string
	Name string
}

type HypershiftResources struct {
	ServiceMonitors []*prometheusv1.ServiceMonitor
	ScrapeConfigs   []*prometheusalpha1.ScrapeConfig
	Rules           []*prometheusv1.PrometheusRule
}

type CollectionConfig struct {
	ScrapeConfigs []*prometheusalpha1.ScrapeConfig
	Rules         []*prometheusv1.PrometheusRule
}

type Hypershift struct {
	Client         client.Client
	ManagedCluster *clusterv1.ManagedCluster
	Logger         logr.Logger
}

func (h *Hypershift) GenerateResources(ctx context.Context, etcdConfig, apiServerConfig CollectionConfig) (*HypershiftResources, error) {
	ret := &HypershiftResources{}
	hostedClusters := &hyperv1.HostedClusterList{}
	if err := h.Client.List(ctx, hostedClusters, &client.ListOptions{}); err != nil {
		return ret, fmt.Errorf("failed to list HostedClusterList: %w", err)
	}

	if len(hostedClusters.Items) == 0 {
		h.Logger.V(1).Info("no hostedCluster found, skipping hypershift resources creation")
		return ret, nil
	}

	// Keep only metrics from our own serviceMonitors to avoid collecting metrics from original serviceMonitor that would end up being incorrectly labeled.
	scrapeConfigsMetricsFilter := []prometheusv1.RelabelConfig{
		{
			SourceLabels: []prometheusv1.LabelName{config.ClusterIDMetricLabel}, // ClusterID is not empty (the hosted cluster one)
			Regex:        ".+",
			Action:       "keep",
		},
		{
			SourceLabels: []prometheusv1.LabelName{config.ManagementClusterIDMetricLabel}, // Management cluster is the current ManagedCluster
			Regex:        h.ManagedCluster.Labels[config.ManagedClusterLabelClusterID],
			Action:       "keep",
		},
	}

	for _, cfg := range etcdConfig.ScrapeConfigs {
		cfg.Spec.RelabelConfigs = append(cfg.Spec.RelabelConfigs, scrapeConfigsMetricsFilter...)
	}

	ret.ScrapeConfigs = append(ret.ScrapeConfigs, etcdConfig.ScrapeConfigs...)
	ret.Rules = append(ret.Rules, etcdConfig.Rules...)

	etcdMetrics, err := h.extractDependentMetrics(etcdConfig.ScrapeConfigs, etcdConfig.Rules)
	if err != nil {
		return ret, fmt.Errorf("failed to extract etcd dependent metrics: %w", err)
	}

	for _, cfg := range apiServerConfig.ScrapeConfigs {
		cfg.Spec.RelabelConfigs = append(cfg.Spec.RelabelConfigs, scrapeConfigsMetricsFilter...)
	}

	ret.ScrapeConfigs = append(ret.ScrapeConfigs, apiServerConfig.ScrapeConfigs...)
	ret.Rules = append(ret.Rules, apiServerConfig.Rules...)

	apiserverMetrics, err := h.extractDependentMetrics(apiServerConfig.ScrapeConfigs, apiServerConfig.Rules)
	if err != nil {
		return ret, fmt.Errorf("failed to extract etcd dependent metrics: %w", err)
	}

	ret.ServiceMonitors = make([]*prometheusv1.ServiceMonitor, 0, len(hostedClusters.Items))
	for _, hostedCluster := range hostedClusters.Items {
		namespace := fmt.Sprintf("%s-%s", hostedCluster.Namespace, hostedCluster.Name)
		hostedClusterIdentity := clusterIdentity{
			ID:   hostedCluster.Spec.ClusterID,
			Name: hostedCluster.Name,
		}

		if len(hostedClusterIdentity.ID) == 0 {
			h.Logger.Info("hoster cluster is missing clusterID, skipping resources creation", "name", hostedClusterIdentity.Name)
			continue
		}

		acmEtcdSm, err := h.generateEtcdServiceMonitor(ctx, namespace, hostedClusterIdentity, etcdMetrics)
		if err != nil {
			return ret, fmt.Errorf("failed to generate etcd ServiceMonitor for namespace %s: %w", namespace, err)
		}
		if acmEtcdSm != nil {
			ret.ServiceMonitors = append(ret.ServiceMonitors, acmEtcdSm)
		}

		acmApiserverSm, err := h.generateApiServerServiceMonitor(ctx, namespace, hostedClusterIdentity, apiserverMetrics)
		if err != nil {
			return ret, fmt.Errorf("failed to generate etcd ServiceMonitor for namespace %s: %w", namespace, err)
		}
		if acmApiserverSm != nil {
			ret.ServiceMonitors = append(ret.ServiceMonitors, acmApiserverSm)
		}
	}

	return ret, nil
}

func (h *Hypershift) generateEtcdServiceMonitor(ctx context.Context, namespace string, hostedCluster clusterIdentity, metrics []string) (*prometheusv1.ServiceMonitor, error) {
	if len(metrics) == 0 {
		h.Logger.V(1).Info("no metrics to collect for etcd, skipping serviceMonitor creation", "hostedClusterName", hostedCluster.Name)
		return nil, nil
	}

	// Get the hypershift's etcd service monitor to replicate some of its settings
	hypershiftEtcdSM := &prometheusv1.ServiceMonitor{}
	if err := h.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: config.HypershiftEtcdServiceMonitorName}, hypershiftEtcdSM); err != nil {
		if apierrors.IsNotFound(err) {
			// Permanent error, no need to retry, just log the error
			h.Logger.Error(err, fmt.Sprintf("the etcd serviceMonitor %s/%s deployed by hypershift is not found, cannot set observability for etcd", namespace, config.HypershiftEtcdServiceMonitorName), "hostedClusterName", hostedCluster.Name)
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get hypershift's etcd ServiceMonitor: %w", err)
	}

	ret := &prometheusv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.AcmEtcdServiceMonitorName,
			Namespace: namespace,
		},
		Spec: prometheusv1.ServiceMonitorSpec{
			Selector:          hypershiftEtcdSM.Spec.Selector,
			NamespaceSelector: hypershiftEtcdSM.Spec.NamespaceSelector,
		},
	}

	for _, endpoint := range hypershiftEtcdSM.Spec.Endpoints {
		ret.Spec.Endpoints = append(ret.Spec.Endpoints, prometheusv1.Endpoint{
			Interval:             "30s",
			Scheme:               endpoint.Scheme,
			Port:                 endpoint.Port,
			TargetPort:           endpoint.TargetPort,
			BearerTokenSecret:    &corev1.SecretKeySelector{},
			TLSConfig:            endpoint.TLSConfig,
			MetricRelabelConfigs: h.generateMetricsRelabelConfigs(hostedCluster, metrics),
			RelabelConfigs: []prometheusv1.RelabelConfig{
				{
					TargetLabel: "job",
					Action:      "replace",
					Replacement: ptr.To("etcd"),
				},
			},
		})
	}

	return ret, nil
}

func (h *Hypershift) generateApiServerServiceMonitor(ctx context.Context, namespace string, hostedCluster clusterIdentity, metrics []string) (*prometheusv1.ServiceMonitor, error) {
	if len(metrics) == 0 {
		h.Logger.V(1).Info("no metrics to collect for apiserver, skipping serviceMonitor creation", "hostedClusterName", hostedCluster.Name)
		return nil, nil
	}

	// Get the hypershift's api-server service monitor and replicate some of its settings
	hypershiftApiServerSM := &prometheusv1.ServiceMonitor{}
	if err := h.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: config.HypershiftApiServerServiceMonitorName}, hypershiftApiServerSM); err != nil {
		if apierrors.IsNotFound(err) {
			// Permanent error, no need to retry, just log the error
			h.Logger.Error(err, fmt.Sprintf("the apiserver serviceMonitor %s/%s deployed by hypershift is not found, cannot set observability for etcd", namespace, config.HypershiftApiServerServiceMonitorName), "hostedClusterName", hostedCluster.Name)
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get hypershift's kube-apiserver ServiceMonitor: %w", err)
	}

	ret := &prometheusv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.AcmApiServerServiceMonitorName,
			Namespace: namespace,
		},
		Spec: prometheusv1.ServiceMonitorSpec{
			Selector:          hypershiftApiServerSM.Spec.Selector,
			NamespaceSelector: hypershiftApiServerSM.Spec.NamespaceSelector,
		},
	}

	for _, endpoint := range hypershiftApiServerSM.Spec.Endpoints {
		ret.Spec.Endpoints = append(ret.Spec.Endpoints, prometheusv1.Endpoint{
			Interval:             "30s",
			Scheme:               endpoint.Scheme,
			Port:                 endpoint.Port,
			TargetPort:           endpoint.TargetPort,
			BearerTokenSecret:    &corev1.SecretKeySelector{},
			TLSConfig:            endpoint.TLSConfig,
			MetricRelabelConfigs: h.generateMetricsRelabelConfigs(hostedCluster, metrics),
			RelabelConfigs: []prometheusv1.RelabelConfig{
				{
					TargetLabel: "job",
					Action:      "replace",
					Replacement: ptr.To("apiserver"),
				},
			},
		})
	}

	return ret, nil
}

// extractDependentMetrics extracts the list of metrics that the input scrapeConfig and rule are dependent on.
// It ignores metrics reulting from rules.
// Result is alphabetically sorted.
// This function is used to extract the list of metrics that must be collected by the in-cluster prometheus.
func (h *Hypershift) extractDependentMetrics(sc []*prometheusalpha1.ScrapeConfig, rule []*prometheusv1.PrometheusRule) ([]string, error) {
	scMetrics, err := h.federatedMetrics(sc)
	if err != nil {
		return nil, fmt.Errorf("failed to extract federated metrics from ScrapeConfig: %w", err)
	}

	dedup := make(map[string]struct{}, len(scMetrics))
	for _, metricsName := range scMetrics {
		dedup[metricsName] = struct{}{}
	}

	ruleMetrics, err := h.rulesDependentMetrics(rule)
	if err != nil {
		return nil, fmt.Errorf("failed to extract dependent metrics from PrometheusRule: %w", err)
	}

	for _, metricsName := range ruleMetrics {
		dedup[metricsName] = struct{}{}
	}

	return slices.Sorted(maps.Keys(dedup)), nil
}

// federatedMetrics returns the list of collected metrics from a scrapeConfig, parsing the
// federated metrics list. It ignores metrics resulting from rules evaluation.
func (h *Hypershift) federatedMetrics(scrapeConfigs []*prometheusalpha1.ScrapeConfig) ([]string, error) {
	ret := []string{}

	for _, scrapeConfig := range scrapeConfigs {
		if scrapeConfig == nil {
			continue
		}

		for _, query := range scrapeConfig.Spec.Params["match[]"] {
			expr, err := parser.ParseExpr(query)
			if err != nil {
				return nil, fmt.Errorf("failed to parse query %s: %w", query, err)
			}

			selectors := parser.ExtractSelectors(expr)
			for _, node := range selectors {
				for _, matcher := range node {
					if matcher.Name != "__name__" || isRuleMetricName(matcher.Value) {
						continue
					}

					if matcher.Type != labels.MatchEqual {
						h.Logger.V(1).Info(fmt.Sprintf("ignoring non equal type labels matcher in %q scrapeConfig, not supported: %s", scrapeConfig.Name, matcher.String()))
						continue
					}

					ret = append(ret, matcher.Value)
				}
			}
		}
	}

	return ret, nil
}

func (h *Hypershift) rulesDependentMetrics(promRules []*prometheusv1.PrometheusRule) ([]string, error) {
	ret := []string{}

	for _, promRule := range promRules {
		if promRule == nil {
			continue
		}

		for _, group := range promRule.Spec.Groups {
			for _, rule := range group.Rules {
				expr, err := parser.ParseExpr(rule.Expr.StrVal)
				if err != nil {
					return nil, fmt.Errorf("failed to parse query for rule named %q: %w", rule.Record, err)
				}

				selectors := parser.ExtractSelectors(expr)
				for _, node := range selectors {
					for _, matcher := range node {
						if matcher.Name != "__name__" || isRuleMetricName(matcher.Value) {
							continue
						}

						if matcher.Type != labels.MatchEqual {
							h.Logger.V(1).Info(fmt.Sprintf("ignoring non equal type labels matcher in rule %q, not supported: %s", promRule.Name, matcher.String()))
							continue
						}

						ret = append(ret, matcher.Value)
					}
				}
			}
		}
	}

	return ret, nil
}

func (h *Hypershift) generateMetricsRelabelConfigs(hostedCluster clusterIdentity, metrics []string) []prometheusv1.RelabelConfig {
	return []prometheusv1.RelabelConfig{
		{
			SourceLabels: []prometheusv1.LabelName{"__name__"},
			Action:       "keep",
			Regex:        fmt.Sprintf("(%s)", strings.Join(metrics, "|")),
		},
		{
			TargetLabel: config.ClusterIDMetricLabel,
			Action:      "replace",
			Replacement: &hostedCluster.ID,
		},
		{
			TargetLabel: config.ClusterNameMetricLabel,
			Action:      "replace",
			Replacement: &hostedCluster.Name,
		},
		{
			TargetLabel: config.ManagementClusterIDMetricLabel,
			Action:      "replace",
			Replacement: ptr.To(h.ManagedCluster.Labels[config.ManagedClusterLabelClusterID]),
		},
		{
			TargetLabel: config.ManagementClusterNameMetricLabel,
			Action:      "replace",
			Replacement: &h.ManagedCluster.Name,
		},
	}
}

func IsHypershiftEnabled(managedCluster *clusterv1.ManagedCluster) bool {
	// Check if is hub
	isLocalCluster, ok := managedCluster.Labels[config.LocalManagedClusterLabel]
	if !ok || isLocalCluster != "true" {
		return false
	}

	// Check if hypershift addon is active
	hypershiftAddonStatus, ok := managedCluster.Labels[config.HypershiftAddonStateLabel]
	if !ok {
		return false
	}

	if hypershiftAddonStatus == "disabled" {
		return false
	}

	return true
}

func isRuleMetricName(name string) bool {
	return strings.Contains(name, ":")
}

func stringPtr(s string) *string {
	return &s
}
