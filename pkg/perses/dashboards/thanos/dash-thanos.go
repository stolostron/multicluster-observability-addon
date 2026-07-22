package thanos

import (
	"flag"

	"github.com/perses/community-mixins/pkg/dashboards"
	thanosDashboards "github.com/perses/community-mixins/pkg/dashboards/thanos"
	"github.com/perses/community-mixins/pkg/panels/gostats"
	thanosPanels "github.com/perses/community-mixins/pkg/panels/thanos"
	promqlbuilder "github.com/perses/promql-builder"
	"github.com/prometheus/prometheus/promql/parser"
	"k8s.io/apimachinery/pkg/runtime"
)

func init() {
	// These flags are required by the github.com/perses/community-mixins/pkg/dashboards library.
	// They are looked up in NewExec() which is called by NewDashboardWriter().
	if flag.Lookup("output") == nil {
		flag.String("output", "", "output format of the dashboard exec")
	}
	if flag.Lookup("output-dir") == nil {
		flag.String("output-dir", "", "output directory of the dashboard exec")
	}
}

// init rewrites every upstream Thanos and shared "gostats" (the Resources
// panel group: memory/goroutines/GC/CPU) panel query to reference RHACM's
// actual metric names: platform metrics are relabeled with an "acm_"
// prefix (e.g. thanos_compact_halted -> acm_thanos_compact_halted, or
// go_goroutines -> acm_go_goroutines) before they land in the observability
// backend, so the community-mixins queries (which target the raw upstream
// names) need this override to return data.
func init() {
	thanosPanels.OverrideThanosPanelQueries(rewriteQueries(thanosPanels.ThanosCommonPanelQueries))
	gostats.OverrideGoPanelQueries(rewriteQueries(gostats.GoCommonPanelQueries))
}

func rewriteQueries(queries map[string]parser.Expr) map[string]parser.Expr {
	rewritten := make(map[string]parser.Expr, len(queries))
	for key, expr := range queries {
		e := promqlbuilder.DeepCopyExpr(expr)
		promqlbuilder.Inspect(e, func(node parser.Node, _ []parser.Node) error {
			vs, ok := node.(*parser.VectorSelector)
			if !ok {
				return nil
			}
			if vs.Name != "" {
				vs.Name = "acm_" + vs.Name
			}
			for _, m := range vs.LabelMatchers {
				if m.Name == "__name__" && m.Value != "" {
					m.Value = "acm_" + m.Value
				}
			}
			return nil
		})
		rewritten[key] = e
	}
	return rewritten
}

// BuildThanosDashboards renders the upstream community-mixins Thanos
// dashboards (Query, Query Frontend, Store, Compact, Receive, Rule), with
// panel queries pointed at RHACM's own metric names via the init() override
// above.
func BuildThanosDashboards(project string, datasource string, clusterLabelName string) ([]runtime.Object, error) {
	dashboardWriter := dashboards.NewDashboardWriter()
	dashboardWriter.Add(thanosDashboards.BuildThanosQueryOverview(project, datasource, clusterLabelName))
	dashboardWriter.Add(thanosDashboards.BuildThanosQueryFrontendOverview(project, datasource, clusterLabelName))
	dashboardWriter.Add(thanosDashboards.BuildThanosStoreOverview(project, datasource, clusterLabelName))
	dashboardWriter.Add(thanosDashboards.BuildThanosCompactOverview(project, datasource, clusterLabelName))
	dashboardWriter.Add(thanosDashboards.BuildThanosReceiveOverview(project, datasource, clusterLabelName))
	dashboardWriter.Add(thanosDashboards.BuildThanosRulerOverview(project, datasource, clusterLabelName))

	return dashboardWriter.OperatorResources(), nil
}
