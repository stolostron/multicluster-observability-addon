package resource

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	persesv1 "github.com/perses/perses-operator/api/v1alpha1"
	"github.com/perses/perses/go-sdk/dashboard"
	persesmodelv1 "github.com/perses/perses/pkg/model/api/v1"
	persescommon "github.com/perses/perses/pkg/model/api/v1/common"
	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	imanifests "github.com/stolostron/multicluster-observability-addon/internal/analytics/incident-detection/manifests"
	cmanifests "github.com/stolostron/multicluster-observability-addon/internal/coo/manifests"
	rsperses "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/rightsizing"
	"github.com/stolostron/multicluster-observability-addon/pkg/perses/dashboards/acm"
	hcp "github.com/stolostron/multicluster-observability-addon/pkg/perses/dashboards/acm/hosted-control-plane"
	compute "github.com/stolostron/multicluster-observability-addon/pkg/perses/dashboards/acm/k8s/compute"
	networking "github.com/stolostron/multicluster-observability-addon/pkg/perses/dashboards/acm/k8s/networking"
	slo "github.com/stolostron/multicluster-observability-addon/pkg/perses/dashboards/acm/k8s/slo"
	incident_management "github.com/stolostron/multicluster-observability-addon/pkg/perses/dashboards/incident-management"
	"github.com/stolostron/multicluster-observability-addon/pkg/perses/dashboards/thanos"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	dsThanos             = "rbac-query-proxy-datasource"
	dsPlatformPrometheus = "platform-prometheus-datasource"
	clusterLabelName     = ""

	ManagedByLabelValue = "mcoa-hub-resource-reconciler"

	uiPluginName = "monitoring"
)

var errUnexpectedObjectType = stderrors.New("unexpected object type, expected *PersesDashboard")

var managedResourceLabels = map[string]string{
	addoncfg.ManagedByK8sLabelKey: ManagedByLabelValue,
}

var thanosVariableMetricRenames = strings.NewReplacer(
	"thanos_build_info", "acm_thanos_build_info",
	"thanos_status", "acm_thanos_status",
	"prometheus_tsdb_head_max_time", "acm_prometheus_tsdb_head_max_time",
)

// HubResourceReconciler reconciles hub-only COO resources (PersesDashboards,
// PersesDatasources, UIPlugin, analytics namespace) directly on the hub,
// independent of the ManifestWork lifecycle.
type HubResourceReconciler struct {
	Client client.Client
	CMAO   *addonv1beta1.ClusterManagementAddOn
	Logger logr.Logger
	Opts   addon.Options
}

func (r *HubResourceReconciler) Reconcile(ctx context.Context, hasCardinalityRules bool) error {
	metricsUI := cmanifests.EnableUI(r.Opts.Platform.Metrics, true)
	hasDashboards := metricsUI != nil && metricsUI.Enabled

	incidentDetection := imanifests.EnableUI(r.Opts.Platform.AnalyticsOptions.IncidentDetection)
	incidentDetectionEnabled := incidentDetection != nil && incidentDetection.Enabled

	hasAnalyticsDashboards := incidentDetectionEnabled ||
		(r.Opts.Platform.AnalyticsOptions.RightSizing.Delegated &&
			(r.Opts.Platform.AnalyticsOptions.RightSizing.NamespaceEnabled ||
				r.Opts.Platform.AnalyticsOptions.RightSizing.VirtualizationEnabled))

	persesEnabled := hasDashboards || hasAnalyticsDashboards

	if hasAnalyticsDashboards {
		if err := r.ensureAnalyticsNamespace(ctx); err != nil {
			return fmt.Errorf("failed to ensure analytics namespace: %w", err)
		}
	}

	if err := r.reconcileDashboards(ctx, hasCardinalityRules, incidentDetectionEnabled); err != nil {
		return fmt.Errorf("failed to reconcile dashboards: %w", err)
	}

	if err := r.reconcileDatasources(ctx, hasDashboards, hasAnalyticsDashboards); err != nil {
		return fmt.Errorf("failed to reconcile datasources: %w", err)
	}

	if err := r.reconcileUIPlugin(ctx, persesEnabled, incidentDetectionEnabled); err != nil {
		return fmt.Errorf("failed to reconcile UIPlugin: %w", err)
	}

	return nil
}

// --- Analytics namespace ---

func (r *HubResourceReconciler) ensureAnalyticsNamespace(ctx context.Context) error {
	ns := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
		ObjectMeta: metav1.ObjectMeta{
			Name:   addoncfg.AnalyticsNamespace,
			Labels: managedResourceLabels,
		},
	}
	existing := &corev1.Namespace{}
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(ns), existing); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get namespace %s: %w", ns.Name, err)
		}
		if err := r.Client.Create(ctx, ns); err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create namespace %s: %w", ns.Name, err)
		}
		r.Logger.Info("created namespace", "name", ns.Name)
	}
	return nil
}

// --- UIPlugin ---

func (r *HubResourceReconciler) reconcileUIPlugin(ctx context.Context, persesEnabled, incidentDetectionEnabled bool) error {
	monitoringUINeeded := persesEnabled || incidentDetectionEnabled
	metricsUI := cmanifests.EnableUI(r.Opts.Platform.Metrics, true)
	metricsEnabled := metricsUI != nil && metricsUI.Enabled

	if !monitoringUINeeded {
		return r.deleteIfManaged(ctx, &uiplugin.UIPlugin{
			ObjectMeta: metav1.ObjectMeta{Name: uiPluginName},
		})
	}

	desired := &uiplugin.UIPlugin{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "observability.openshift.io/v1alpha1",
			Kind:       "UIPlugin",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   uiPluginName,
			Labels: managedResourceLabels,
		},
		Spec: uiplugin.UIPluginSpec{
			Type: uiplugin.TypeMonitoring,
			Monitoring: &uiplugin.MonitoringConfig{
				Perses: &uiplugin.PersesReference{Enabled: persesEnabled},
			},
		},
	}

	if metricsEnabled {
		desired.Spec.Monitoring.ACM = &uiplugin.AdvancedClusterManagementReference{
			Enabled: true,
			Alertmanager: uiplugin.AlertmanagerReference{
				Url: "https://alertmanager.open-cluster-management-observability.svc:9095",
			},
			ThanosQuerier: uiplugin.ThanosQuerierReference{
				Url: "https://rbac-query-proxy.open-cluster-management-observability.svc:8443",
			},
		}
	}

	if incidentDetectionEnabled {
		desired.Spec.Monitoring.Incidents = &uiplugin.IncidentsReference{Enabled: true}
	}

	return r.applyResource(ctx, desired)
}

// --- Datasources ---

func (r *HubResourceReconciler) reconcileDatasources(ctx context.Context, hasDashboards, hasAnalyticsDashboards bool) error {
	if hasDashboards {
		if err := r.applyResource(ctx, buildRBACQueryProxyDatasource(addoncfg.InstallNamespace)); err != nil {
			return fmt.Errorf("failed to apply rbac-query-proxy datasource: %w", err)
		}
		if err := r.applyResource(ctx, buildPlatformPrometheusDatasource(addoncfg.InstallNamespace)); err != nil {
			return fmt.Errorf("failed to apply platform-prometheus datasource: %w", err)
		}
	} else {
		for _, name := range []string{dsThanos, dsPlatformPrometheus} {
			if err := r.deleteIfManaged(ctx, &persesv1.PersesDatasource{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: addoncfg.InstallNamespace},
			}); err != nil {
				return err
			}
		}
	}

	if hasAnalyticsDashboards {
		if err := r.applyResource(ctx, buildRBACQueryProxyDatasource(addoncfg.AnalyticsNamespace)); err != nil {
			return fmt.Errorf("failed to apply analytics datasource: %w", err)
		}
	} else {
		if err := r.deleteIfManaged(ctx, &persesv1.PersesDatasource{
			ObjectMeta: metav1.ObjectMeta{Name: dsThanos, Namespace: addoncfg.AnalyticsNamespace},
		}); err != nil {
			return err
		}
	}

	return nil
}

func buildRBACQueryProxyDatasource(namespace string) *persesv1.PersesDatasource {
	return &persesv1.PersesDatasource{
		TypeMeta: metav1.TypeMeta{APIVersion: "perses.dev/v1alpha1", Kind: "PersesDatasource"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dsThanos,
			Namespace: namespace,
			Labels:    managedResourceLabels,
		},
		Spec: persesv1.DatasourceSpec{
			Client: &persesv1.Client{
				TLS: &persesv1.TLS{
					Enable: true,
					CaCert: &persesv1.Certificate{
						SecretSource: persesv1.SecretSource{Type: persesv1.SecretSourceTypeFile},
						CertPath:     "/ca/service-ca.crt",
					},
				},
			},
			Config: persesv1.Datasource{
				DatasourceSpec: persesmodelv1.DatasourceSpec{
					Default: false,
					Plugin: persescommon.Plugin{
						Kind: "PrometheusDatasource",
						Spec: map[string]any{
							"scrapeInterval": "5m",
							"proxy": map[string]any{
								"kind": "HTTPProxy",
								"spec": map[string]any{
									"url": "http://rbac-query-proxy.open-cluster-management-observability.svc.cluster.local:8080",
								},
							},
						},
					},
				},
			},
		},
	}
}

func buildPlatformPrometheusDatasource(namespace string) *persesv1.PersesDatasource {
	return &persesv1.PersesDatasource{
		TypeMeta: metav1.TypeMeta{APIVersion: "perses.dev/v1alpha1", Kind: "PersesDatasource"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dsPlatformPrometheus,
			Namespace: namespace,
			Labels:    managedResourceLabels,
		},
		Spec: persesv1.DatasourceSpec{
			Client: &persesv1.Client{
				TLS: &persesv1.TLS{
					Enable: true,
					CaCert: &persesv1.Certificate{
						SecretSource: persesv1.SecretSource{Type: persesv1.SecretSourceTypeFile},
						CertPath:     "/ca/service-ca.crt",
					},
				},
			},
			Config: persesv1.Datasource{
				DatasourceSpec: persesmodelv1.DatasourceSpec{
					Default: false,
					Plugin: persescommon.Plugin{
						Kind: "PrometheusDatasource",
						Spec: map[string]any{
							"proxy": map[string]any{
								"kind": "HTTPProxy",
								"spec": map[string]any{
									"url":    "https://thanos-querier.openshift-monitoring.svc.cluster.local:9091",
									"secret": "platform-prometheus-datasource-secret",
								},
							},
							"scrapeInterval": "5m",
						},
					},
				},
			},
		},
	}
}

// --- Common helpers ---

func (r *HubResourceReconciler) applyResource(ctx context.Context, obj client.Object) error {
	if err := common.ServerSideApply(ctx, r.Client, obj, nil); err != nil {
		return fmt.Errorf("failed to apply %s %s/%s: %w",
			obj.GetObjectKind().GroupVersionKind().Kind,
			obj.GetNamespace(), obj.GetName(), err)
	}
	return nil
}

func (r *HubResourceReconciler) deleteIfManaged(ctx context.Context, obj client.Object) error {
	existing := obj.DeepCopyObject().(client.Object)
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(obj), existing); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get %s %s/%s: %w",
			obj.GetObjectKind().GroupVersionKind().Kind,
			obj.GetNamespace(), obj.GetName(), err)
	}
	if existing.GetLabels()[addoncfg.ManagedByK8sLabelKey] != ManagedByLabelValue {
		return nil
	}
	if err := r.Client.Delete(ctx, existing); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete %s %s/%s: %w",
			obj.GetObjectKind().GroupVersionKind().Kind,
			obj.GetNamespace(), obj.GetName(), err)
	}
	r.Logger.Info("deleted managed resource",
		"kind", obj.GetObjectKind().GroupVersionKind().Kind,
		"namespace", obj.GetNamespace(), "name", obj.GetName())
	return nil
}

// --- Dashboards ---

func (r *HubResourceReconciler) reconcileDashboards(ctx context.Context, hasCardinalityRules, incidentDetectionEnabled bool) error {
	desired := r.buildDesiredDashboards(hasCardinalityRules, incidentDetectionEnabled)

	desiredNames := map[string]struct{}{}
	for i := range desired {
		db := &desired[i]
		desiredNames[db.Namespace+"/"+db.Name] = struct{}{}

		if err := r.applyResource(ctx, db); err != nil {
			return fmt.Errorf("failed to apply dashboard %s/%s: %w", db.Namespace, db.Name, err)
		}
	}

	return r.cleanupOrphanDashboards(ctx, desiredNames)
}

func (r *HubResourceReconciler) cleanupOrphanDashboards(ctx context.Context, desiredNames map[string]struct{}) error {
	// Match both new (managed-by label) and legacy Helm-managed dashboards
	// (app.kubernetes.io/name + part-of labels from the old Helm templates).
	selectors := []labels.Selector{
		labels.SelectorFromSet(managedResourceLabels),
		labels.SelectorFromSet(map[string]string{
			"app.kubernetes.io/name":    "perses-dashboard",
			"app.kubernetes.io/part-of": "perses-operator",
		}),
	}

	for _, ns := range []string{addoncfg.InstallNamespace, addoncfg.AnalyticsNamespace} {
		for _, selector := range selectors {
			existingList := &persesv1.PersesDashboardList{}
			if err := r.Client.List(ctx, existingList, client.InNamespace(ns), client.MatchingLabelsSelector{Selector: selector}); err != nil {
				if errors.IsNotFound(err) {
					continue
				}
				return fmt.Errorf("failed to list dashboards in %s: %w", ns, err)
			}

			for i := range existingList.Items {
				db := &existingList.Items[i]
				key := db.Namespace + "/" + db.Name
				if _, ok := desiredNames[key]; !ok {
					if err := r.Client.Delete(ctx, db); err != nil && !errors.IsNotFound(err) {
						return fmt.Errorf("failed to delete orphan dashboard %s: %w", key, err)
					}
					r.Logger.Info("deleted orphan PersesDashboard", "namespace", db.Namespace, "name", db.Name)
				}
			}
		}
	}

	return nil
}

func (r *HubResourceReconciler) buildDesiredDashboards(hasCardinalityRules, incidentDetectionEnabled bool) []persesv1.PersesDashboard {
	var dashboards []persesv1.PersesDashboard

	metricsUI := cmanifests.EnableUI(r.Opts.Platform.Metrics, true)
	if metricsUI != nil && metricsUI.Enabled {
		dashboards = r.appendDashboards(dashboards, "ACM", buildACMPersesDashboards)
		dashboards = r.appendDashboards(dashboards, "K8s", buildK8sPersesDashboards)
		dashboards = r.appendDashboards(dashboards, "Thanos", buildThanosPersesDashboards)
		if hasCardinalityRules {
			dashboards = r.appendDashboards(dashboards, "cardinality", buildCardinalityPersesDashboards)
		}
	}

	if incidentDetectionEnabled {
		dashboards = r.appendDashboards(dashboards, "incident-detection", buildIncidentDetectionPersesDashboards)
	}

	if r.Opts.Platform.AnalyticsOptions.RightSizing.Delegated {
		if r.Opts.Platform.AnalyticsOptions.RightSizing.NamespaceEnabled {
			dashboards = r.appendDashboards(dashboards, "namespace-rightsizing", buildNamespaceRSPersesDashboards)
		}
		if r.Opts.Platform.AnalyticsOptions.RightSizing.VirtualizationEnabled {
			dashboards = r.appendDashboards(dashboards, "vm-rightsizing", buildVMRSPersesDashboards)
		}
	}

	return dashboards
}

func (r *HubResourceReconciler) appendDashboards(dashboards []persesv1.PersesDashboard, group string, buildFn func() ([]persesv1.PersesDashboard, error)) []persesv1.PersesDashboard {
	dbs, err := buildFn()
	if err != nil {
		r.Logger.Error(err, "failed to build dashboards, skipping group", "group", group)
		return dashboards
	}
	return append(dashboards, dbs...)
}

// --- Dashboard builders ---

type dashboardBuilderFunc func(project string, datasource string, clusterLabelName string) (dashboard.Builder, error)

func buildFromBuilders(builders []dashboardBuilderFunc, datasource, namespace string) ([]persesv1.PersesDashboard, error) {
	var result []persesv1.PersesDashboard
	for _, fn := range builders {
		db, err := fn(namespace, datasource, clusterLabelName)
		if err != nil {
			return nil, fmt.Errorf("failed to build dashboard: %w", err)
		}
		result = append(result, toDashboardCR(db, namespace))
	}
	return result, nil
}

func toDashboardCR(builder dashboard.Builder, namespace string) persesv1.PersesDashboard {
	lbls := map[string]string{
		"app.kubernetes.io/name":      "perses-dashboard",
		"app.kubernetes.io/instance":  builder.Dashboard.Metadata.Name,
		"app.kubernetes.io/part-of":   "perses-operator",
		"app.kubernetes.io/component": "dashboard",
		addoncfg.ManagedByK8sLabelKey: ManagedByLabelValue,
	}

	return persesv1.PersesDashboard{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "perses.dev/v1alpha1",
			Kind:       "PersesDashboard",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Dashboard.Metadata.Name,
			Namespace: namespace,
			Labels:    lbls,
		},
		Spec: persesv1.Dashboard{
			DashboardSpec: builder.Dashboard.Spec,
		},
	}
}

func runtimeObjectsToDashboardCRs(objs []runtime.Object, namespace string) ([]persesv1.PersesDashboard, error) {
	var result []persesv1.PersesDashboard
	for _, obj := range objs {
		db, ok := obj.(*persesv1.PersesDashboard)
		if !ok {
			return nil, fmt.Errorf("%w: got %T", errUnexpectedObjectType, obj)
		}
		if db.Labels == nil {
			db.Labels = map[string]string{}
		}
		db.Labels[addoncfg.ManagedByK8sLabelKey] = ManagedByLabelValue
		db.Namespace = namespace
		result = append(result, *db)
	}
	return result, nil
}

func buildACMPersesDashboards() ([]persesv1.PersesDashboard, error) {
	return buildFromBuilders([]dashboardBuilderFunc{
		acm.BuildClusterResourceUse,
		acm.BuildNodeResourceUse,
		acm.BuildACMOptimizationOverview,
		acm.BuildACMClustersOverview,
		acm.BuildACMAlertAnalysis,
		acm.BuildACMAlertsByCluster,
		acm.BuildACMClustersByAlert,
		hcp.BuildACMHCPOverview,
		hcp.BuildACMHCPResources,
		slo.BuildSLOAPIServer,
		slo.BuildSLOAPIServerCluster,
		networking.BuildNetworkingCluster,
		networking.BuildNetworkingNamespacePods,
		networking.BuildNetworkingNode,
		networking.BuildNetworkingPod,
		compute.BuildComputeCluster,
		compute.BuildComputeNamespacePods,
		compute.BuildComputeNamespaceWorkloads,
		compute.BuildComputeNodePods,
		compute.BuildComputePod,
		compute.BuildComputeWorkload,
	}, dsThanos, addoncfg.InstallNamespace)
}

func buildThanosPersesDashboards() ([]persesv1.PersesDashboard, error) {
	objs, err := thanos.BuildThanosDashboards(addoncfg.InstallNamespace, dsPlatformPrometheus, clusterLabelName)
	if err != nil {
		return nil, fmt.Errorf("failed to build Thanos dashboards: %w", err)
	}

	dbs, err := runtimeObjectsToDashboardCRs(objs, addoncfg.InstallNamespace)
	if err != nil {
		return nil, err
	}
	for i := range dbs {
		specBytes, err := json.Marshal(dbs[i].Spec)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal Thanos dashboard %s spec: %w", dbs[i].Name, err)
		}
		replaced := thanosVariableMetricRenames.Replace(string(specBytes))
		var spec persesv1.Dashboard
		if err := json.Unmarshal([]byte(replaced), &spec); err != nil {
			return nil, fmt.Errorf("failed to unmarshal Thanos dashboard %s spec: %w", dbs[i].Name, err)
		}
		dbs[i].Spec = spec
	}
	return dbs, nil
}

func buildK8sPersesDashboards() ([]persesv1.PersesDashboard, error) {
	type multiBuilder struct {
		fn   func(string, string, string) ([]runtime.Object, error)
		name string
	}
	builders := []multiBuilder{
		{acm.BuildK8sDashboards, "Kubernetes"},
		{acm.BuildETCDDashboards, "ETCD"},
	}

	var result []persesv1.PersesDashboard
	for _, b := range builders {
		objs, err := b.fn(addoncfg.InstallNamespace, dsThanos, clusterLabelName)
		if err != nil {
			return nil, fmt.Errorf("failed to build %s dashboards: %w", b.name, err)
		}
		dbs, err := runtimeObjectsToDashboardCRs(objs, addoncfg.InstallNamespace)
		if err != nil {
			return nil, err
		}
		result = append(result, dbs...)
	}
	return result, nil
}

func buildCardinalityPersesDashboards() ([]persesv1.PersesDashboard, error) {
	return buildFromBuilders([]dashboardBuilderFunc{
		acm.BuildACMMetricsCardinalityOverview,
		acm.BuildACMMetricsCardinalityCluster,
		acm.BuildACMMetricsCardinalityName,
	}, dsThanos, addoncfg.InstallNamespace)
}

func buildIncidentDetectionPersesDashboards() ([]persesv1.PersesDashboard, error) {
	return buildFromBuilders([]dashboardBuilderFunc{
		incident_management.BuildACMIncidentsOverview,
	}, dsThanos, addoncfg.AnalyticsNamespace)
}

func buildNamespaceRSPersesDashboards() ([]persesv1.PersesDashboard, error) {
	return buildFromBuilders([]dashboardBuilderFunc{
		rsperses.BuildNamespaceRightSizing,
	}, dsThanos, addoncfg.AnalyticsNamespace)
}

func buildVMRSPersesDashboards() ([]persesv1.PersesDashboard, error) {
	return buildFromBuilders([]dashboardBuilderFunc{
		rsperses.BuildVMOverview,
		rsperses.BuildVMOverestimation,
		rsperses.BuildVMUnderestimation,
	}, dsThanos, addoncfg.AnalyticsNamespace)
}
