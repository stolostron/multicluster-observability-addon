package main

import (
	"github.com/perses/community-dashboards/pkg/dashboards"
)

func main() {
	//clusterLabelName := ""
	//project := flag.String("project", "open-cluster-management-observability", "project name")
	//datasource := flag.String("datasource", "thanos-query-frontend", "datasource name")
	//flag.Parse()

	dashboardWriter := dashboards.NewDashboardWriter()
	// dashboardWriter.Add(acm.BuildACMClustersOverview(*project, *datasource, clusterLabelName))
	// dashboardWriter.Add(acm.BuildACMIncidentsOverview(*project, *datasource, clusterLabelName))
	// dashboardWriter.Add(acm.BuildACMOptimizationOverview(*project, *datasource, clusterLabelName))
	// dashboardWriter.Add(acm.BuildClusterResourceUse(*project, *datasource, clusterLabelName))
	// dashboardWriter.Add(acm.BuildNodeResourceUse(*project, *datasource, clusterLabelName))
	dashboardWriter.Write()
}
