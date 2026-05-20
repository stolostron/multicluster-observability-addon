package main

import (
	"context"
	"errors"
	goflag "flag"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"

	"github.com/ViaQ/logerr/v2/log"
	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	persesv1 "github.com/perses/perses-operator/api/v1alpha1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	addonctrl "github.com/stolostron/multicluster-observability-addon/internal/controllers/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/controllers/resourcecreator"
	"github.com/stolostron/multicluster-observability-addon/internal/controllers/watcher"
	corev1 "k8s.io/api/core/v1"
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
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	workv1 "open-cluster-management.io/api/work/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
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
	utilruntime.Must(clusterv1beta1.AddToScheme(scheme))
	utilruntime.Must(operatorv1.AddToScheme(scheme))
	utilruntime.Must(cooprometheusv1.AddToScheme(scheme))
	utilruntime.Must(cooprometheusv1alpha1.AddToScheme(scheme)) // Adds prometheusAgent and scrapeConfig
	utilruntime.Must(prometheusv1.AddToScheme(scheme))          // Adds prometheusRule
	utilruntime.Must(uiplugin.AddToScheme(scheme))
	utilruntime.Must(hyperv1.AddToScheme(scheme))
	utilruntime.Must(persesv1.AddToScheme(scheme))
	utilruntime.Must(routev1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

var (
	logVerbosity int
	enablePprof  bool
	pprofAddr    string
)

func main() {
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
	cmd.Flags().BoolVar(&enablePprof, "enable-pprof", false, "Enable pprof profiling.")
	cmd.Flags().StringVar(&pprofAddr, "pprof-addr", "127.0.0.1:6060", "The address the pprof server will bind to.")

	return cmd
}

func runControllers(ctx context.Context, kubeConfig *rest.Config) error {
	logger := log.NewLogger("mcoa", log.WithVerbosity(logVerbosity))
	ctrl.SetLogger(logger)

	if enablePprof {
		mux := http.NewServeMux()
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

		srv := &http.Server{
			Addr:    pprofAddr,
			Handler: mux,
		}

		go func() {
			logger.Info("starting pprof server", "addr", pprofAddr)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Error(err, "pprof server failed")
			}
		}()

		go func() {
			<-ctx.Done()
			logger.Info("shutting down pprof server")
			if err := srv.Shutdown(context.Background()); err != nil {
				logger.Error(err, "failed to shutdown pprof server")
			}
		}()
	} else {
		logger.V(1).Info("pprof server disabled")
	}

	// Increase client-side throttling limits to support large number of managed clusters
	kubeConfig.QPS = 50.0
	kubeConfig.Burst = 100

	mgr, err := ctrl.NewManager(kubeConfig, ctrl.Options{
		Scheme: scheme,
		Logger: logger.WithName("manager"),
		Cache: cache.Options{
			// We restrict ConfigMap and Secret caching to specific namespaces (e.g., the addon's
			// installation namespace and openshift-ingress for router certs). Reads for these types
			// in other namespaces (like the managed cluster namespaces) will intentionally bypass
			// the cache and hit the API server directly to prevent memory bloat.
			ByObject: map[client.Object]cache.ByObject{
				&corev1.ConfigMap{}: {
					Namespaces: map[string]cache.Config{
						addoncfg.InstallNamespace: {},
					},
				},
				&corev1.Secret{}: {
					Namespaces: map[string]cache.Config{
						addoncfg.InstallNamespace: {},
						"openshift-ingress":       {},
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to start manager: %w", err)
	}

	if err = mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		return fmt.Errorf("failed to set up health check: %w", err)
	}
	if err = mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		return fmt.Errorf("failed to set up ready check: %w", err)
	}

	addonManager, err := addonctrl.NewAddonManager(ctx, kubeConfig, mgr, logger)
	if err != nil {
		return fmt.Errorf("failed to create addon manager: %w", err)
	}

	if err = mgr.Add(addonManager); err != nil {
		return fmt.Errorf("failed to add addon manager to controller-runtime manager: %w", err)
	}

	disableReconciliation := os.Getenv("DISABLE_WATCHER_CONTROLLER")
	if disableReconciliation == "" {
		if err = watcher.SetupWithManager(mgr, addonManager, logger); err != nil {
			return fmt.Errorf("failed to setup watcher manager: %w", err)
		}
	}

	if err = resourcecreator.SetupWithManager(mgr, logger); err != nil {
		return fmt.Errorf("failed to setup resource creator manager: %w", err)
	}

	err = mgr.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start controller-runtime manager: %w", err)
	}

	return nil
}
