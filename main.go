package main

import (
	"context"
	goflag "flag"
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
	persesv1 "github.com/perses/perses-operator/api/v1alpha1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	addonctrl "github.com/stolostron/multicluster-observability-addon/internal/controllers/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/controllers/resourcecreator"
	"github.com/stolostron/multicluster-observability-addon/internal/controllers/watcher"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	utilflag "k8s.io/component-base/cli/flag"
	logs "k8s.io/component-base/logs/api/v1"
	cmdfactory "open-cluster-management.io/addon-framework/pkg/cmd/factory"
	"open-cluster-management.io/addon-framework/pkg/version"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	ctrl "sigs.k8s.io/controller-runtime"
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
	utilruntime.Must(persesv1.AddToScheme(scheme))

	// +kubebuilder:scaffold:scheme
}

var logVerbosity int

func main() {
	rand.Seed(time.Now().UTC().UnixNano()) // nolint:staticcheck

	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	logs.AddFlags(logs.NewLoggingConfiguration(), pflag.CommandLine)

	command := newCommand()
	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
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

	return cmd
}

func newControllerCommand() *cobra.Command {
	cmd := cmdfactory.
		NewControllerCommandConfig("multicluster-observability-addon-controller", version.Get(), runControllers).
		NewCommand()
	cmd.Use = "controller"
	cmd.Short = "Start the addon controller"
	cmd.Flags().IntVar(&logVerbosity, "log-verbosity", 0, "Log verbosity level. The higher the level, the noisier the logs.")

	return cmd
}

func runControllers(ctx context.Context, kubeConfig *rest.Config) error {
	logger := log.NewLogger("mcoa", log.WithVerbosity(logVerbosity))
	ctrl.SetLogger(logger)

	mgr, err := addonctrl.NewAddonManager(ctx, kubeConfig, scheme, logger)
	if err != nil {
		return fmt.Errorf("failed to create addon manager: %w", err)
	}

	disableReconciliation := os.Getenv("DISABLE_WATCHER_CONTROLLER")
	if disableReconciliation == "" {
		var wm *watcher.WatcherManager
		wm, err = watcher.NewWatcherManager(&mgr, scheme, logger)
		if err != nil {
			return fmt.Errorf("unable to create watcher manager: %w", err)
		}

		wm.Start(ctx)
	}

	var rcm *resourcecreator.ResourceCreatorManager
	rcm, err = resourcecreator.NewResourceCreatorManager(logger, scheme)
	if err != nil {
		return fmt.Errorf("unable to create resource creator manager: %w", err)
	}
	rcm.Start(ctx)

	err = mgr.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start addon manager: %w", err)
	}

	<-ctx.Done()

	return nil
}
