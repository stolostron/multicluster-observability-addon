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

func clusterDetailsDashboardLinkURL(project string) string {
	return dashboardLinkURL(
		"acm-openshift-virtualization-single-cluster-view", project,
		"cluster", `${__data.fields["cluster"]}`,
	)
}

func vmDetailsDashboardLinkByValueURL(project string) string {
	return dashboardLinkURL(
		"acm-openshift-virtualization-single-vm-view", project,
		"cluster", `${__data.fields["cluster"]}`,
		"namespace", `${__data.fields["namespace"]}`,
		"name", `${__data.fields["name"]}`,
	)
}

func vmDetailsDashboardLinkByFieldURL(project string) string {
	return dashboardLinkURL(
		"acm-openshift-virtualization-single-vm-view", project,
		"cluster", "$cluster",
		"namespace", `${__data.fields["namespace"]}`,
		"name", `${__data.fields["name"]}`,
	)
}

// tableDataLink builds the dataLink map used in table column settings.
func tableDataLink(title, url string) map[string]any {
	return map[string]any{
		"openNewTab": true,
		"title":      title,
		"url":        url,
	}
}

// vmsByTimeInStatusLinkURL returns the URL for the "VMs by Time in Status"
// dashboard, pre-filtered to the given status value and the current cluster.
// Pass an empty status to link without a status filter.
func vmsByTimeInStatusLinkURL(project, status string) string {
	vars := []string{"cluster", "$cluster"}
	if status != "" {
		vars = append(vars, "status", status)
	}
	return dashboardLinkURL("acm-virtual-machines-by-time-in-status", project, vars...)
}

// vmInventoryLinkURL returns the URL for the VM Inventory dashboard
// pre-scoped to the current cluster.
func vmInventoryLinkURL(project string) string {
	return dashboardLinkURL(
		"acm-virtual-machines-inventory", project,
		"cluster", "$cluster",
	)
}

// vmServiceLevelLinkURL returns the URL for the "VM Service Level" dashboard
// pre-scoped to the current cluster.
func vmServiceLevelLinkURL(project string) string {
	return dashboardLinkURL(
		"acm-virtual-machines-service-level", project,
		"cluster", "$cluster",
	)
}
