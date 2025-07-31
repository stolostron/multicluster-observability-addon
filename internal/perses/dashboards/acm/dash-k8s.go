package acm

import (
	dashboards "github.com/perses/community-dashboards/pkg/dashboards"
	k8sComputeResources "github.com/perses/community-dashboards/pkg/dashboards/kubernetes/compute_resources"
	"k8s.io/apimachinery/pkg/runtime"
)

// Upstream dashboards imported from the community-dashboards repository. https://github.com/perses/community-dashboards/tree/main/pkg/dashboards/kubernetes
func BuildK8sDashboards(project string, datasource string, clusterLabelName string) (obj []runtime.Object, err error) {
	dashboardWriter := dashboards.NewDashboardWriter()
	dashboardWriter.Add(k8sComputeResources.BuildKubernetesNodeResourcesOverview(project, datasource, clusterLabelName))
	dashboardWriter.Add(k8sComputeResources.BuildKubernetesClusterOverview(project, datasource, clusterLabelName))
	dashboardWriter.Add(k8sComputeResources.BuildKubernetesNamespaceOverview(project, datasource, clusterLabelName))
	dashboardWriter.Add(k8sComputeResources.BuildKubernetesPodOverview(project, datasource, clusterLabelName))
	dashboardWriter.Add(k8sComputeResources.BuildKubernetesWorkloadOverview(project, datasource, clusterLabelName))
	dashboardWriter.Add(k8sComputeResources.BuildKubernetesWorkloadNamespaceOverview(project, datasource, clusterLabelName))
	dashboardWriter.Add(k8sComputeResources.BuildKubernetesMultiClusterOverview(project, datasource, clusterLabelName))

	objs := dashboardWriter.OperatorResources()
	return objs, nil
}
