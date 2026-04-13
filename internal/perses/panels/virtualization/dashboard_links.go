package virtualization

import "fmt"

const dashboardLinkBasePath = "/monitoring/v2/dashboards/view"

func dashboardLinkURL(dashboard, project string, vars ...string) string {
	url := fmt.Sprintf("%s?dashboard=%s&project=%s", dashboardLinkBasePath, dashboard, project)
	for i := 0; i+1 < len(vars); i += 2 {
		url += fmt.Sprintf("&var-%s=%s", vars[i], vars[i+1])
	}
	return url
}

func clusterDetailsDashboardLink(project string) map[string]any {
	return map[string]any{
		"openNewTab": true,
		"title":      "Cluster Details",
		"url": dashboardLinkURL(
			"acm-openshift-virtualization-single-cluster-view", project,
			"cluster", `${__data.fields["cluster"]}`,
		),
	}
}

func vmDetailsDashboardLinkByValue(project string) map[string]any {
	return map[string]any{
		"openNewTab": true,
		"title":      "Virtual Machine Details",
		"url": dashboardLinkURL(
			"acm-openshift-virtualization-single-vm-view", project,
			"cluster", `${__data.fields["cluster"]}`,
			"namespace", `${__data.fields["namespace"]}`,
			"name", `${__data.fields["name"]}`,
		),
	}
}

func vmDetailsDashboardLinkByField(project string) map[string]any {
	return map[string]any{
		"openNewTab": true,
		"title":      "Virtual Machine Details",
		"url": dashboardLinkURL(
			"acm-openshift-virtualization-single-vm-view", project,
			"cluster", "$cluster",
			"namespace", `${__data.fields["namespace"]}`,
			"name", `${__data.fields["name"]}`,
		),
	}
}
