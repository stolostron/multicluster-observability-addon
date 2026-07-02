package virtualization

import (
	"github.com/perses/perses/go-sdk/dashboard"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	promqlVar "github.com/perses/plugins/prometheus/sdk/go/variable/promql"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/virtualization"
)

// topConsumersPairedGrid places a table (left) next to its time-series (right).
var topConsumersPairedGrid = []GridItem{
	{X: 0, Y: 0, W: 12, H: 9},
	{X: 12, Y: 0, W: 12, H: 9},
}

// topNHiddenVariable creates a hidden list variable backed by a
// PrometheusPromQL query. It evaluates expr (typically topk($topn, ...)),
// extracts the "name" label from the results, selects all values, and
// joins them into a pipe-separated regex for use in name=~"$varName".
func topNHiddenVariable(varName, expr, datasource string) dashboard.Option {
	return dashboard.AddVariable(varName,
		listVar.List(
			promqlVar.PrometheusPromQL(expr,
				promqlVar.LabelName("name"),
				promqlVar.Datasource(datasource),
			),
			listVar.Hidden(true),
			listVar.AllowAllValue(true),
			listVar.AllowMultiple(true),
			listVar.DefaultValues("$__all"),
		),
	)
}

func withTopConsumersMemoryGroup(datasource, project string) dashboard.Option {
	return AddCustomPanelGroup("Memory",
		topConsumersPairedGrid,
		panels.TopConsumersMemoryTable(datasource, project),
		panels.TopConsumersMemoryTimeSeries(datasource),
	)
}

func withTopConsumersCPUGroup(datasource, project string) dashboard.Option {
	return AddCustomPanelGroup("CPU",
		topConsumersPairedGrid,
		panels.TopConsumersCPUTable(datasource, project),
		panels.TopConsumersCPUTimeSeries(datasource),
	)
}

func withTopConsumersStorageTrafficGroup(datasource, project string) dashboard.Option {
	return AddCustomPanelGroup("Storage Traffic",
		topConsumersPairedGrid,
		panels.TopConsumersStorageTrafficTable(datasource, project),
		panels.TopConsumersStorageTrafficTimeSeries(datasource),
	)
}

func withTopConsumersStorageIOPSGroup(datasource, project string) dashboard.Option {
	return AddCustomPanelGroup("Storage IOPS",
		topConsumersPairedGrid,
		panels.TopConsumersStorageIOPSTable(datasource, project),
		panels.TopConsumersStorageIOPSTimeSeries(datasource),
	)
}

func withTopConsumersNetworkTrafficGroup(datasource, project string) dashboard.Option {
	return AddCustomPanelGroup("Network Traffic",
		topConsumersPairedGrid,
		panels.TopConsumersNetworkTrafficTable(datasource, project),
		panels.TopConsumersNetworkTrafficTimeSeries(datasource),
	)
}

func withTopConsumersVCPUWaitGroup(datasource, project string) dashboard.Option {
	return AddCustomPanelGroup("vCPU Wait",
		topConsumersPairedGrid,
		panels.TopConsumersVCPUWaitTable(datasource, project),
		panels.TopConsumersVCPUWaitTimeSeries(datasource),
	)
}

func withTopConsumersMemorySwapGroup(datasource, project string) dashboard.Option {
	return AddCustomPanelGroup("Memory Swap Traffic",
		topConsumersPairedGrid,
		panels.TopConsumersMemorySwapTable(datasource, project),
		panels.TopConsumersMemorySwapTimeSeries(datasource),
	)
}

// BuildTopConsumers creates the Top Consumers dashboard showing the highest
// resource consumers across clusters, adapted from the Grafana
// "KubeVirt / Infrastructure Resources / Top Consumers" dashboard.
// Each resource type is a collapsible group with a table and time-series pair.
func BuildTopConsumers(project string, datasource string) (dashboard.Builder, error) {
	return dashboard.New("acm-virtual-machines-top-consumers",
		dashboard.ProjectName(project),
		dashboard.Name("Virtualization / Top Consumers"),

		VMClusterVariable(datasource),
		VMNamespaceVariable(datasource),
		AddStaticListVariable("topn", "Top N", "Number of top consumers to display",
			[]StaticListValue{
				{Label: "5", Value: "5"},
				{Label: "10", Value: "10"},
				{Label: "20", Value: "20"},
				{Label: "50", Value: "50"},
			},
			"5", false, false, "",
		),

		topNHiddenVariable(panels.TopNMemoryVarName, panels.TopNMemoryVarExpr, datasource),
		topNHiddenVariable(panels.TopNCPUVarName, panels.TopNCPUVarExpr, datasource),
		topNHiddenVariable(panels.TopNStorageTrafficVarName, panels.TopNStorageTrafficVarExpr, datasource),
		topNHiddenVariable(panels.TopNStorageIOPSVarName, panels.TopNStorageIOPSVarExpr, datasource),
		topNHiddenVariable(panels.TopNNetworkTrafficVarName, panels.TopNNetworkTrafficVarExpr, datasource),
		topNHiddenVariable(panels.TopNVCPUWaitVarName, panels.TopNVCPUWaitVarExpr, datasource),
		topNHiddenVariable(panels.TopNMemorySwapVarName, panels.TopNMemorySwapVarExpr, datasource),

		withTopConsumersMemoryGroup(datasource, project),
		withTopConsumersCPUGroup(datasource, project),
		withTopConsumersStorageTrafficGroup(datasource, project),
		withTopConsumersStorageIOPSGroup(datasource, project),
		withTopConsumersNetworkTrafficGroup(datasource, project),
		withTopConsumersVCPUWaitGroup(datasource, project),
		withTopConsumersMemorySwapGroup(datasource, project),
	)
}
