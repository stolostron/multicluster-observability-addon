package acm

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	commonSdk "github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	markdownPanel "github.com/perses/plugins/markdown/sdk/go"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	statPanel "github.com/perses/plugins/statchart/sdk/go"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	timeSeriesPanel "github.com/perses/plugins/timeserieschart/sdk/go"
	dl "github.com/stolostron/multicluster-observability-addon/pkg/perses/panels/datalinks"
)

func addCardinalityQuery(datasourceName string, expr string, opts ...query.Option) panel.Option {
	allOpts := append([]query.Option{dashboards.AddQueryDataSource(datasourceName)}, opts...)
	return panel.AddQuery(query.PromQL(expr, allOpts...))
}

func cardinalityStatOptions(threshold float64) panel.Option {
	return statPanel.Chart(
		statPanel.Calculation("last-number"),
		statPanel.Thresholds(commonSdk.Thresholds{
			DefaultColor: "green",
			Mode:         commonSdk.AbsoluteMode,
			Steps: []commonSdk.StepOption{
				{Value: threshold, Color: "red"},
			},
		}),
	)
}

func cardinalityTimeSeriesOptions() panel.Option {
	return timeSeriesPanel.Chart(
		timeSeriesPanel.WithLegend(timeSeriesPanel.Legend{
			Mode:     "list",
			Position: "bottom",
		}),
		timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
			Display:   "line",
			Palette:   &timeSeriesPanel.Palette{Mode: timeSeriesPanel.AutoMode},
			LineWidth: 1,
		}),
	)
}

func cardinalityTableOptions() panel.Option {
	return tablePanel.Table(tablePanel.WithDensity("compact"))
}

// Overview - Dashboard Guide

const cardinalityGuideText = `### Managing Monitoring Cardinality

This dashboard helps you visualize the metric cardinality ingested from all managed clusters. Understanding and controlling cardinality is crucial, as the number of unique metrics directly correlates with the CPU, memory, and storage resources required by the Thanos monitoring platform.

#### Important: Dashboard Prerequisites

For this dashboard to display any data, a specific set of Prometheus recording rules must be configured on the hub. These rules perform the heavy cardinality calculations in the background and save the results as new, efficient metrics, such as ` + "`cluster_name:cardinality`" + `.

These rules run at a **30-minute interval** to protect system performance. Therefore, the data on this dashboard will update every half hour.

If the panels are empty, the rules are likely not configured. Refer to the [ACM Dev Preview Documentation](https://github.com/stolostron/stolostron/blob/main/dev-preview/README.md) for complete setup instructions.

#### What is Cardinality?

**Cardinality** refers to the total number of unique **time series** in our monitoring system. A time series is a unique combination of a **metric name** and its descriptive **labels**.

Adding a label with thousands of unique values (like a request_id) causes a "cardinality explosion," which severely degrades system performance.

#### Why It Matters

High cardinality is the primary cause of performance issues in the Thanos-based monitoring system. It directly leads to:

* **High CPU and Memory Usage:** The system struggles to ingest, index, and query the massive number of series.
* **Slow Dashboards and Alerts:** Queries take longer to complete, making dashboards slow and alerts unreliable.
* **Increased Storage Costs:** More unique series consume more disk space.

#### How to Use These Dashboards

This overview uses the pre-calculated metrics to help you investigate issues without overloading the system:

1. **Cardinality Outliers:** The top row identifies clusters and metrics with disproportionately high cardinality. It flags any item with a value greater than three times the standard deviation from the average.
2. **Cardinality by Cluster Name:** Shows the total number of series sent by each managed cluster. Click on a cluster to drill down into namespace and pod breakdowns.
3. **Cardinality by Metric Name:** Shows the highest-cardinality metrics across all clusters. Click on a metric to drill down by cluster, namespace, and pod.

#### Best Practices for Reducing Cardinality

* **Federate Aggregated Metrics via Recording Rules:** Push Prometheus recording rules to managed clusters to pre-aggregate data. Configure federation to scrape only the aggregated results.
* **Be Selective About What You Federate:** Only federate metrics that are truly essential for global dashboards and alerting.
* **Drop Unneeded Labels:** Discard volatile labels like pod, instance, or pod_uuid when writing recording rules.`

func CardinalityDashboardGuide() panelgroup.Option {
	return panelgroup.AddPanel("Dashboard Guide",
		markdownPanel.Markdown(cardinalityGuideText),
	)
}

// Overview - Excluded Clusters

const excludedClustersNote = `#### Note on Data Coverage & Sharding

The recording rules that power these dashboards use **sharding** based on the clusterID label. OpenShift cluster IDs (UUIDs) work well for balanced sharding. Other cluster types using the cluster name as clusterID may cause:

1. **Exclusion:** Cluster names with characters outside the hexadecimal set may be ignored by the sharding rules.
2. **Skew:** Many cluster names starting with the same character can cause unbalanced sharding.

If any managed clusters appear in the excluded list, refer to the ACM Documentation for configuration instructions.`

func ExcludedClustersCount(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Excluded Clusters",
		cardinalityStatOptions(1),
		addCardinalityQuery(datasourceName, CardinalityQueries["ExcludedClustersCount"]),
	)
}

func ExcludedClustersTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cluster List",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{Name: "value", Hide: true},
				{Name: "cluster", Header: "Cluster"},
			}),
			tablePanel.WithDensity("compact"),
		),
		addCardinalityQuery(datasourceName, CardinalityQueries["ExcludedClustersList"]),
	)
}

func ExcludedClustersNote() panelgroup.Option {
	return panelgroup.AddPanel("Note on Data Coverage",
		markdownPanel.Markdown(excludedClustersNote),
	)
}

// Overview - Outlier stat panels

func ClusterOutliersCount(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cluster Outliers",
		cardinalityStatOptions(1),
		addCardinalityQuery(datasourceName, CardinalityQueries["ClusterOutliersCount"]),
	)
}

func MetricOutliersCount(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Metric Outliers",
		cardinalityStatOptions(1),
		addCardinalityQuery(datasourceName, CardinalityQueries["MetricOutliersCount"]),
	)
}

// Overview - Outlier table panels

func ClusterOutliersTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Top Clusters",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{
					Name:     "cluster",
					Header:   "Cluster",
					DataLink: dl.NewTableLinkNewTab("acm-metrics-cardinality-cluster", "cluster", "Drill down"),
				},
				{Name: "value", Header: "Cardinality"},
			}),
			tablePanel.WithDensity("compact"),
		),
		addCardinalityQuery(datasourceName, CardinalityQueries["ClusterOutliersTable"]),
	)
}

func MetricOutliersTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Top Metrics",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{
					Name:     "metric_name",
					Header:   "Metric Name",
					DataLink: dl.NewTableLinkNewTab("acm-metrics-cardinality-name", "metric_name", "Drill down"),
				},
				{Name: "value", Header: "Cardinality"},
			}),
			tablePanel.WithDensity("compact"),
		),
		addCardinalityQuery(datasourceName, CardinalityQueries["MetricOutliersTable"]),
	)
}

// Overview - Cluster Cardinality section

func ClusterCardinalityOverTime(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cardinality Over Time",
		cardinalityTimeSeriesOptions(),
		addCardinalityQuery(datasourceName, CardinalityQueries["ClusterCardinalityOverTime"],
			query.SeriesNameFormat("{{cluster}}"),
		),
	)
}

func ClusterCardinalityTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Current Cardinality",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{
					Name:     "cluster",
					Header:   "Cluster",
					DataLink: dl.NewTableLinkNewTab("acm-metrics-cardinality-cluster", "cluster", "Drill down"),
				},
				{Name: "value", Header: "Cardinality"},
			}),
			tablePanel.WithDensity("compact"),
		),
		addCardinalityQuery(datasourceName, CardinalityQueries["ClusterCardinalityNow"]),
	)
}

// Overview - Metric Cardinality section

func MetricCardinalityOverTime(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cardinality Over Time",
		cardinalityTimeSeriesOptions(),
		addCardinalityQuery(datasourceName, CardinalityQueries["MetricCardinalityOverTime"],
			query.SeriesNameFormat("{{metric_name}}"),
		),
	)
}

func MetricCardinalityTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Current Cardinality",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{
					Name:     "metric_name",
					Header:   "Metric Name",
					DataLink: dl.NewTableLinkNewTab("acm-metrics-cardinality-name", "metric_name", "Drill down"),
				},
				{Name: "value", Header: "Cardinality"},
			}),
			tablePanel.WithDensity("compact"),
		),
		addCardinalityQuery(datasourceName, CardinalityQueries["MetricCardinalityNow"]),
	)
}

// Overview - Global Recording Rules section

func GlobalRecordingRulesOverTime(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cardinality Over Time",
		cardinalityTimeSeriesOptions(),
		addCardinalityQuery(datasourceName, CardinalityQueries["GlobalRulesOverTime"],
			query.SeriesNameFormat("{{metric_name}}"),
		),
	)
}

func GlobalRecordingRulesTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Current Cardinality",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{
					Name:     "metric_name",
					Header:   "Metric Name",
					DataLink: dl.NewTableLinkNewTab("acm-metrics-cardinality-name", "metric_name", "Drill down"),
				},
				{Name: "value", Header: "Cardinality"},
			}),
			tablePanel.WithDensity("compact"),
		),
		addCardinalityQuery(datasourceName, CardinalityQueries["GlobalRulesNow"]),
	)
}

// Overview - Total Cardinality section

func TotalCardinalityOverTime(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cardinality Over Time",
		cardinalityTimeSeriesOptions(),
		addCardinalityQuery(datasourceName, CardinalityQueries["TotalCardinalityOverTime"],
			query.SeriesNameFormat("Total Cardinality"),
		),
	)
}
