package hosted_control_plane

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	markdownPanel "github.com/perses/plugins/markdown/sdk/go"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	statPanel "github.com/perses/plugins/statchart/sdk/go"
	tablePanel "github.com/perses/plugins/table/sdk/go"
)

func RequestBasedLimitEstimation() panelgroup.Option {
	return panelgroup.AddPanel(" Resource Request-base Limit Estimation",
		markdownPanel.Markdown(
			"## Request-based resource limit\n\n"+
				"To understand the request-based resource limit, consider the total request value of a hosted control plane. "+
				"To calculate that value, add the request values of all highly available hosted control plane pods across the namespace. "+
				"The estimates are calculated based on the following resource request samples:\n\n"+
				"* 78 pods\n"+
				"* Five vCPU requests for each highly available hosted control plane\n"+
				"* 18 GiB memory requests for each highly available hosted control plane",
		),
	)
}

func WorkerNodeCapacities(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Worker Nodes Capacities",
		panel.Description("These are the worker nodes that can run hosted control planes."),
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name: "timestamp",
					Hide: true,
				},
				{
					Name:   "node",
					Header: "Worker Node",
					Align:  tablePanel.LeftAlign,
				},
				{
					Name:   "cpu",
					Header: "CPU",
					Align:  tablePanel.LeftAlign,
				},
				{
					Name:   "memory",
					Header: "Memory (GiB)",
					Align:  tablePanel.LeftAlign,
				},
				{
					Name:   "maxPods",
					Header: "Pod Limit",
					Align:  tablePanel.LeftAlign,
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["WorkerNodeCapacities"].Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func NumberOfHCPsRequestBased(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Number of HCPs",
		panel.Description("This panel displays the current number of unavailable/failing and available hosted control planes. Based on the hosted control plane resource requirements, it also displays the estimated maximum number of hosted control planes that can be hosted in this cluster."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["HCPStatusUnavailable"].Pretty(0)+" or vector(0)",
				query.SeriesNameFormat("Currently Unavailable"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["HCPStatusAvailable"].Pretty(0)+" or vector(0)",
				query.SeriesNameFormat("Currently Available"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["RequestBasedHCPCapacity"].Pretty(0)+" or vector(0)",
				query.SeriesNameFormat("est. Max"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func LoadBasedLimitEstimation() panelgroup.Option {
	return panelgroup.AddPanel("Load-based Limit Estimation",
		markdownPanel.Markdown(
			"## Load-based limit\n\n"+
				"Request-based sizing provides a maximum number of hosted control planes that can run based on the minimum request totals "+
				"for the `Burstable` class, which meet the average resource usage. For sizing guidance that is tuned to higher levels of "+
				"hosted cluster load, the load-based approach demonstrates resource usage at increasing API rates. The load-based approach "+
				"builds in resource capacity for each hosted control plane to handle higher API load points.\n\n"+
				"Resource utilization is measured as the workload increased to the total namespace count. This data provides an estimation "+
				"factor to increase the compute resource capacity based on the expected API load. Exact utilization rates can vary based on "+
				"the type and pace of the cluster workload. \n\n"+
				"| **Hosted control plane resource utilization scaling** | **vCPUs** | **Memory (GiB)** |\n"+
				"| --- | --- | --- |\n"+
				"| Default requests | 5 | 18 |\n"+
				"| Usage when idle | 2.9 | 11.1 |\n"+
				"| Incremental usage per 1000 increase in API rate | 9.0 | 2.5 |\n\n"+
				"By using these examples, you can factor in a load-based limit that is based on the expected rate of stress on the API, "+
				"which is measured as the aggregated QPS across the 3 hosted API servers. For general sizing purposes, consider a 1000 QPS "+
				"API rate to be a medium hosted cluster load and a 2000 QPS API to be a heavy hosted cluster load.",
		),
	)
}

func QPSSettings(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("QPS Settings",
		panel.Description("These API server loads are used for estimating the maximum number of hosted control planes that can be hosted. For example, the est. Max. (low QPS) in the panel below is the estimate maximum number of hosted control planes that can be hosted when all hosted control planes put low load on the API server."),
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name: "timestamp",
					Hide: true,
				},
				{
					Name:   "Value",
					Header: "Query Rate (QPS)",
					Align:  tablePanel.CenterAlign,
				},
				{
					Name:   "rate",
					Header: "Load on API Server",
					Align:  tablePanel.CenterAlign,
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["QPSSettings"].Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func NumberOfHCPsQPSBased(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Number of HCPs",
		panel.Description("This panel displays the current number of unavailable/failing and available hosted control planes. Based on various loads, it also displays the estimated maximum number of hosted control planes that can be hosted in this cluster."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["HCPStatusUnavailable"].Pretty(0)+" or vector(0)",
				query.SeriesNameFormat("Currently Unavailable"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["HCPStatusAvailable"].Pretty(0)+" or vector(0)",
				query.SeriesNameFormat("Currently Available"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["QPSBasedCapacityLow"].Pretty(0)+" or vector(0)",
				query.SeriesNameFormat("est. Max. (low QPS)"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["QPSBasedCapacityMedium"].Pretty(0)+" or vector(0)",
				query.SeriesNameFormat("est. Max. (medium QPS)"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["QPSBasedCapacityHigh"].Pretty(0)+" or vector(0)",
				query.SeriesNameFormat("est. Max. (high QPS)"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["QPSBasedCapacityAverage"].Pretty(0)+" or vector(0)",
				query.SeriesNameFormat("est. Max. (avg QPS)"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func HCPList(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Hosted Control Plane List",
		panel.Description("This is the list of all hosted control planes in this cluster. Click on the hosted control plane name to see its resource utilization."),
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name: "timestamp",
					Hide: true,
				},
				{
					Name:   "hcp_name",
					Header: "HCP name",
					Align:  tablePanel.LeftAlign,
				},
				{
					Name:   "hcp_namespace",
					Header: "HCP namespace",
					Align:  tablePanel.LeftAlign,
				},
				{
					Name:   "ready",
					Header: "Status",
					Align:  tablePanel.LeftAlign,
				},
				{
					Name:   "version",
					Header: "Version",
					Align:  tablePanel.LeftAlign,
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["HCPList"].Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}