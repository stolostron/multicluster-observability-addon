package acm

import (
	"flag"

	dashboards "github.com/perses/community-mixins/pkg/dashboards"
	k8sEtcd "github.com/perses/community-mixins/pkg/dashboards/etcd"
	k8sApiServer "github.com/perses/community-mixins/pkg/dashboards/kubernetes/apiserver"
	k8sComputeResources "github.com/perses/community-mixins/pkg/dashboards/kubernetes/compute_resources"
	"github.com/perses/perses/go-sdk/dashboard"
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

// Upstream dashboards imported from the community-dashboards repository. https://github.com/perses/community-dashboards/tree/main/pkg/dashboards/kubernetes
func BuildK8sDashboards(project string, datasource string, clusterLabelName string) (obj []runtime.Object, err error) {
	dashboardWriter := dashboards.NewDashboardWriter()

	dashboardVars := []dashboard.Option{
		GetClusterVariable(datasource),
		GetNodeVariable(datasource),
	}
	dashboardWriter.Add(k8sComputeResources.BuildKubernetesNodeResourcesOverview(project, datasource, clusterLabelName, dashboardVars...))

	dashboardWriter.Add(k8sComputeResources.BuildKubernetesMultiClusterOverview(project, datasource, clusterLabelName))

	dashboardVars = []dashboard.Option{
		GetClusterVariable(datasource),
		GetNamespaceVariable(datasource),
	}
	dashboardWriter.Add(k8sComputeResources.BuildKubernetesNamespaceOverview(project, datasource, clusterLabelName, dashboardVars...))

	dashboardVars = []dashboard.Option{
		GetClusterVariable(datasource),
		GetNamespaceVariable(datasource),
		GetPodVariable(datasource),
	}
	dashboardWriter.Add(k8sComputeResources.BuildKubernetesPodOverview(project, datasource, clusterLabelName, dashboardVars...))

	dashboardVars = []dashboard.Option{
		GetClusterVariable(datasource),
		GetNamespaceVariable(datasource),
		GetWorkloadVariable(datasource),
		GetTypeVariable(datasource),
	}
	dashboardWriter.Add(k8sComputeResources.BuildKubernetesWorkloadOverview(project, datasource, clusterLabelName, dashboardVars...))
	dashboardWriter.Add(k8sComputeResources.BuildKubernetesWorkloadNamespaceOverview(project, datasource, clusterLabelName, dashboardVars...))

	dashboardVars = []dashboard.Option{
		GetClusterVariable(datasource),
		GetInstanceVariable(datasource),
	}
	dashboardWriter.Add(k8sApiServer.BuildAPIServerOverview(project, datasource, clusterLabelName, dashboardVars...))

	// TODO: Add kubernetes networking dashboards
	//dashboardVars = []dashboard.Option{
	//	GetClusterVariable(datasource),
	//	GetNamespaceVariable(datasource),
	//}
	//dashboardWriter.Add(k8sNetworking.BuildKubernetesClusterOverview(project, datasource, clusterLabelName, dashboardVars...))
	//dashboardWriter.Add(k8sNetworking.BuildKubernetesPodOverview(project, datasource, clusterLabelName, dashboardVars...))
	//dashboardWriter.Add(k8sNetworking.BuildKubernetesWorkloadOverview(project, datasource, clusterLabelName, dashboardVars...))
	//dashboardWriter.Add(k8sNetworking.BuildKubernetesNamespaceByPodOverview(project, datasource, clusterLabelName, dashboardVars...))
	//dashboardWriter.Add(k8sNetworking.BuildKubernetesNamespaceByWorkloadOverview(project, datasource, clusterLabelName, dashboardVars...))
	//
	objs := dashboardWriter.OperatorResources()
	return objs, nil
}

func BuildETCDDashboards(project string, datasource string, clusterLabelName string) (obj []runtime.Object, err error) {
	dashboardWriter := dashboards.NewDashboardWriter()
	dashboardWriter.Add(k8sEtcd.BuildETCDOverview(project, datasource, clusterLabelName))
	objs := dashboardWriter.OperatorResources()
	return objs, nil
}
