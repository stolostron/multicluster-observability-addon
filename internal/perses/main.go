package main

import (
	"flag"

	"github.com/perses/community-dashboards/pkg/dashboards"
	"github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
)

func main() {
	clusterLabelName := ""
	project := flag.String("project", "open-cluster-management-observability", "project name")
	datasource := flag.String("datasource", "thanos-query-frontend", "datasource name")
	flag.Parse()

	dashboardWriter := dashboards.NewDashboardWriter()
	// dashboardWriter.Add(acm.BuildACMClustersOverview(*project, *datasource, clusterLabelName))
	// dashboardWriter.Add(acm.BuildACMIncidentsOverview(*project, *datasource, clusterLabelName))
	// dashboardWriter.Add(acm.BuildACMOptimizationOverview(*project, *datasource, clusterLabelName))
	dashboardWriter.Add(acm.BuildClusterResourceUse(*project, *datasource, clusterLabelName))
	dashboardWriter.Add(acm.BuildNodeResourceUse(*project, *datasource, clusterLabelName))
	dashboardWriter.Write()
}
