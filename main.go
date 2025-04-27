package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/ViaQ/logerr/v2/log"
	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	dashboards "github.com/perses/community-dashboards/pkg/dashboards"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	"github.com/spf13/cobra"
	addonctrl "github.com/stolostron/multicluster-observability-addon/internal/controllers/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/controllers/watcher"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	cmdfactory "open-cluster-management.io/addon-framework/pkg/cmd/factory"
	"open-cluster-management.io/addon-framework/pkg/version"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"

	acm "github.com/stolostron/multicluster-observability-addon/internal/metrics/perses/dashboards/acm"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(addonapiv1alpha1.AddToScheme(scheme))
	utilruntime.Must(workv1.AddToScheme(scheme))
	utilruntime.Must(loggingv1.AddToScheme(scheme))
	utilruntime.Must(otelv1beta1.AddToScheme(scheme))
	utilruntime.Must(otelv1alpha1.AddToScheme(scheme))
	utilruntime.Must(operatorsv1.AddToScheme(scheme))
	utilruntime.Must(operatorsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(prometheusv1alpha1.AddToScheme(scheme))
	utilruntime.Must(prometheusv1.AddToScheme(scheme))
	utilruntime.Must(uiplugin.AddToScheme(scheme))
	utilruntime.Must(hyperv1.AddToScheme(scheme))

	// +kubebuilder:scaffold:scheme
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano()) // nolint:staticcheck

	var (
		projectName    string
		datasourceName string
	)

	rootCmd := &cobra.Command{
		Use:   "dashboard-generator",
		Short: "Short dashboard generating command",
		Run: func(cmd *cobra.Command, _ []string) {
			dashboardWriter := dashboards.NewDashboardWriter()

			dashboardWriter.Add(acm.BuildACMClustersOverview(projectName, datasourceName, ""))

			dashboardWriter.Write()
		},
	}

	rootCmd.Flags().StringVar(&projectName, "project", "perses-dev", "Project name for the dashboard")
	rootCmd.Flags().StringVar(&datasourceName, "datasource", "thanos-query-frontend", "Datasource name for the dashboard")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func newCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "multicluster-observability-addon",
		Short: "multicluster-observability-addon",
		Run: func(cmd *cobra.Command, _ []string) {
			if err := cmd.Help(); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
			}
			os.Exit(1)
		},
	}

	if v := version.Get().String(); len(v) == 0 {
		cmd.Version = "<unknown>"
	} else {
		cmd.Version = v
	}

	cmd.AddCommand(newControllerCommand())
	cmd.AddCommand(newPersesGenCommand())

	return cmd
}

func newPersesGenCommand() *cobra.Command {
	var (
		projectName    string
		datasourceName string
		outputFormat   string
		outputDir      string
	)

	cmd := &cobra.Command{
		Use:   "perses-gen",
		Short: "Short dashboard generating command",
		Run: func(cmd *cobra.Command, _ []string) {
			flag.Set("output", outputFormat)
			flag.Set("output-dir", outputDir)

			dashboardWriter := dashboards.NewDashboardWriter()
			dashboardWriter.Add(acm.BuildACMClustersOverview(projectName, datasourceName, ""))
			dashboardWriter.Write()
		},
	}

	cmd.Flags().StringVar(&projectName, "project", "perses-dev", "Project name for the dashboard")
	cmd.Flags().StringVar(&datasourceName, "datasource", "thanos-query-frontend", "Datasource name for the dashboard")
	cmd.Flags().StringVar(&outputFormat, "output", "operator", "Output format (json, yaml, or operator)")
	cmd.Flags().StringVar(&outputDir, "output-dir", "./dist", "Output directory for the generated dashboards")

	return cmd
}

func newControllerCommand() *cobra.Command {
	cmd := cmdfactory.
		NewControllerCommandConfig("multicluster-observability-addon-controller", version.Get(), runControllers).
		NewCommand()
	cmd.Use = "controller"
	cmd.Short = "Start the addon controller"

	return cmd
}

func runControllers(ctx context.Context, kubeConfig *rest.Config) error {
	logger := log.NewLogger("mcoa")
	mgr, err := addonctrl.NewAddonManager(ctx, kubeConfig, scheme)
	if err != nil {
		return err
	}

	disableReconciliation := os.Getenv("DISABLE_WATCHER_CONTROLLER")
	if disableReconciliation == "" {
		var wm *watcher.WatcherManager
		wm, err = watcher.NewWatcherManager(&mgr, scheme)
		if err != nil {
			logger.Error(err, "unable to create watcher manager")
			return err
		}

		wm.Start(ctx)
	}

	err = mgr.Start(ctx)
	if err != nil {
		return err
	}

	<-ctx.Done()

	return nil
}
