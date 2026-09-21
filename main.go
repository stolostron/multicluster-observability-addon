package main

import (
	"context"
	"crypto/tls"
	goflag "flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/ViaQ/logerr/v2/log"
	"github.com/go-logr/logr"
	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	tlsutil "github.com/openshift/controller-runtime-common/pkg/tls"
	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	persesv1 "github.com/perses/perses-operator/api/v1alpha1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	monitoringv1alpha1 "github.com/rhobs/observability-operator/pkg/apis/monitoring/v1alpha1"
	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	addonctrl "github.com/stolostron/multicluster-observability-addon/internal/controllers/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/controllers/resourcecreator"
	"github.com/stolostron/multicluster-observability-addon/internal/controllers/tlsprofile"
	"github.com/stolostron/multicluster-observability-addon/internal/controllers/watcher"
	tlshelper "github.com/stolostron/multicluster-observability-addon/pkg/util"
	thanosv1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	utilflag "k8s.io/component-base/cli/flag"
	kubelogs "k8s.io/component-base/logs"
	logs "k8s.io/component-base/logs/api/v1"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	"open-cluster-management.io/addon-framework/pkg/version"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	workv1 "open-cluster-management.io/api/work/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(workv1.Install(scheme))
	utilruntime.Must(loggingv1.AddToScheme(scheme))
	utilruntime.Must(otelv1beta1.AddToScheme(scheme))
	utilruntime.Must(otelv1alpha1.AddToScheme(scheme))
	utilruntime.Must(operatorsv1.AddToScheme(scheme))
	utilruntime.Must(operatorsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(clusterv1.Install(scheme))
	utilruntime.Must(clusterv1beta1.Install(scheme))
	utilruntime.Must(operatorv1.AddToScheme(scheme))
	utilruntime.Must(cooprometheusv1.AddToScheme(scheme))
	utilruntime.Must(cooprometheusv1alpha1.AddToScheme(scheme)) // Adds prometheusAgent and scrapeConfig
	utilruntime.Must(monitoringv1alpha1.AddToScheme(scheme))    // Adds monitoringstack
	utilruntime.Must(prometheusv1.AddToScheme(scheme))          // Adds prometheusRule
	utilruntime.Must(uiplugin.AddToScheme(scheme))
	utilruntime.Must(hyperv1.AddToScheme(scheme))
	utilruntime.Must(persesv1.AddToScheme(scheme))
	utilruntime.Must(routev1.AddToScheme(scheme))
	utilruntime.Must(addonv1beta1.Install(scheme))
	utilruntime.Must(thanosv1alpha1.AddToScheme(scheme))
	utilruntime.Must(configv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

var logVerbosity int

func main() {
	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	logs.AddFlags(logs.NewLoggingConfiguration(), pflag.CommandLine)

	command := newCommand()
	if err := command.ExecuteContext(ctrl.SetupSignalHandler()); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

const (
	controllerName        = "multicluster-observability-addon-controller"
	defaultMetricsAddress = ":8084"
	defaultHealthAddress  = ":8081"
	// defaultPprofAddress is the loopback address the pprof server binds to when
	// profiling is enabled. It is unauthenticated plain HTTP, so it must never
	// be exposed beyond localhost; reach it via `kubectl port-forward`.
	defaultPprofAddress = "127.0.0.1:6060"
)

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

type controllerOptions struct {
	kubeConfigFile string
	namespace      string
	leaderElection bool
	metricsAddress string
	healthAddress  string
	enablePprof    bool
	pprofAddress   string
}

func newControllerCommand() *cobra.Command {
	options := &controllerOptions{}
	cmd := &cobra.Command{
		Use:   "controller",
		Short: "Start the addon controller",
		RunE: func(cmd *cobra.Command, _ []string) error {
			kubelogs.InitLogs()
			defer kubelogs.FlushLogs()
			ctrl.SetLogger(log.NewLogger("mcoa", log.WithVerbosity(logVerbosity)))
			return options.run(cmd.Context())
		},
	}
	cmd.Flags().StringVar(&options.kubeConfigFile, "kubeconfig", "", "Location of the kubeconfig file. Defaults to in-cluster configuration.")
	cmd.Flags().StringVar(&options.namespace, "component-namespace", "", "Namespace where the leader election lease is created. Defaults to the service account namespace.")
	cmd.Flags().BoolVar(&options.leaderElection, "enable-leader-election", false, "Enable leader election for the controller manager.")
	cmd.Flags().StringVar(&options.metricsAddress, "metrics-bind-address", defaultMetricsAddress, "The address the metrics endpoint binds to. Use 0 to disable.")
	cmd.Flags().StringVar(&options.healthAddress, "health-probe-bind-address", defaultHealthAddress, "The address the health/readiness probe endpoint binds to.")
	cmd.Flags().BoolVar(&options.enablePprof, "enable-pprof", false, "Enable pprof profiling on --pprof-addr. The endpoint is unauthenticated, so keep it bound to localhost and reach it via port-forward.")
	cmd.Flags().StringVar(&options.pprofAddress, "pprof-addr", defaultPprofAddress, "The address the pprof server binds to when --enable-pprof is set.")
	cmd.Flags().IntVar(&logVerbosity, "log-verbosity", 0, "Log verbosity level. The higher the level, the noisier the logs.")
	return cmd
}

func (o *controllerOptions) run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	logger := log.NewLogger("mcoa", log.WithVerbosity(logVerbosity))

	kubeConfig := ctrl.GetConfigOrDie()

	// Increase client-side throttling limits to support large number of managed clusters
	kubeConfig.QPS = 50.0
	kubeConfig.Burst = 100

	profile, err := tlshelper.GetOrCreateTLSProfileSpec(ctx, kubeConfig)
	if err != nil {
		return fmt.Errorf("failed to get TLS profile: %w", err)
	}

	options := newManagerOptions(*profile)
	options.Logger = logger.WithName("manager")
	options.LeaderElection = o.leaderElection
	options.LeaderElectionNamespace = o.namespace
	options.Metrics.BindAddress = o.metricsAddress
	options.HealthProbeBindAddress = o.healthAddress

	// pprof is opt-in and unauthenticated; keep it on localhost only.
	if o.enablePprof {
		options.PprofBindAddress = o.pprofAddress
		logger.Info("pprof profiling enabled", "address", o.pprofAddress)
	}

	httpClient, err := rest.HTTPClientFor(kubeConfig)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client for kubeConfig: %w", err)
	}
	mapper, err := apiutil.NewDynamicRESTMapper(kubeConfig, httpClient)
	if err != nil {
		return fmt.Errorf("failed to create dynamic REST mapper: %w", err)
	}
	// Share a single HTTP client and REST mapper between the manager and the
	// addon framework so we do not open redundant connections to the hub.
	options.MapperProvider = func(*rest.Config, *http.Client) (meta.RESTMapper, error) {
		return mapper, nil
	}
	options.Client.HTTPClient = httpClient

	mgr, err := ctrl.NewManager(kubeConfig, options)
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	if err = mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		return fmt.Errorf("failed to set up health check: %w", err)
	}
	if err = mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		return fmt.Errorf("failed to set up ready check: %w", err)
	}

	// Restart the process when the cluster TLS profile changes so the manager
	// rebuilds its serving TLS configuration from the new profile.
	if err = tlsprofile.SetupWithManager(ctx, mgr, kubeConfig, *profile, cancel); err != nil {
		return fmt.Errorf("failed to set up TLS profile watcher: %w", err)
	}

	// The addon framework is not a controller-runtime controller: it owns its
	// own informers and control loops. Wrap it as a Runnable so the manager is
	// the single owner of the process lifecycle and leader election gates it.
	addonMgr, err := addonctrl.NewAddonManager(ctx, kubeConfig, scheme, logger, httpClient, mapper)
	if err != nil {
		return fmt.Errorf("failed to create addon manager: %w", err)
	}
	if err = mgr.Add(&addonManagerRunnable{
		start: addonMgr.Start,
		setupControllers: func() error {
			return setupAddonControllers(mgr, addonMgr, logger)
		},
	}); err != nil {
		return fmt.Errorf("failed to register addon manager: %w", err)
	}

	logger.Info("Starting manager")
	if err = mgr.Start(ctx); err != nil {
		return fmt.Errorf("manager exited with error: %w", err)
	}
	return nil
}

// setupAddonControllers registers the controllers that need to run alongside the
// addon
func setupAddonControllers(mgr ctrl.Manager, addonMgr addonmanager.AddonManager, logger logr.Logger) error {
	if os.Getenv("DISABLE_WATCHER_CONTROLLER") == "" {
		if err := watcher.SetupWithManager(mgr, addonMgr, logger); err != nil {
			return fmt.Errorf("unable to create watcher controller: %w", err)
		}
	}
	if err := resourcecreator.SetupWithManager(mgr, logger); err != nil {
		return fmt.Errorf("unable to create resource creator controller: %w", err)
	}
	return nil
}

// newManagerOptions builds controller-runtime manager options, serving metrics
// over TLS derived from the cluster TLS profile. The caller overrides the
// runtime-specific fields (logger, addresses, leader election, mapper).
func newManagerOptions(profile configv1.TLSProfileSpec) ctrl.Options {
	configureTLS, unsupportedCiphers := tlsutil.NewTLSConfigFromProfile(profile)
	if len(unsupportedCiphers) > 0 {
		ctrl.Log.Info("TLS profile contains unsupported ciphers that will be ignored", "ciphers", unsupportedCiphers)
	}

	leaseDuration := 137 * time.Second
	renewDeadline := 107 * time.Second
	retryPeriod := 26 * time.Second
	shutdownTimeout := 10 * time.Second

	return ctrl.Options{
		Scheme:                        scheme,
		LeaderElectionID:              controllerName,
		LeaderElectionResourceLock:    "leases",
		LeaderElectionReleaseOnCancel: true,
		LeaseDuration:                 &leaseDuration,
		RenewDeadline:                 &renewDeadline,
		RetryPeriod:                   &retryPeriod,
		GracefulShutdownTimeout:       &shutdownTimeout,
		HealthProbeBindAddress:        defaultHealthAddress,
		// Disabled by default; run enables it on a loopback address when
		// --enable-pprof is set.
		PprofBindAddress: "0",
		Metrics: metricsserver.Options{
			BindAddress:   defaultMetricsAddress,
			SecureServing: true,
			TLSOpts:       []func(*tls.Config){configureTLS},
		},
	}
}

// addonManagerRunnable adapts the addon framework to controller-runtime's
// Runnable interface. Start blocks until the context is canceled so the
// manager treats it like any other long-running controller.
type addonManagerRunnable struct {
	// start starts the addon framework's own control loops. It returns once the
	// framework has started (it does not block).
	start func(ctx context.Context) error
	// setupControllers registers the controllers that depend on the started
	// framework (e.g. the watcher that triggers addon reconciles).
	setupControllers func() error
}

// NeedLeaderElection ensures the addon framework only runs on the elected
// leader, matching every other controller in the manager.
func (a *addonManagerRunnable) NeedLeaderElection() bool { return true }

func (a *addonManagerRunnable) Start(ctx context.Context) error {
	// The framework runs on its own child context so canceling it does not
	// tear down the whole manager, and it must be started before its dependent
	// controllers are registered.
	frameworkCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := a.start(frameworkCtx); err != nil {
		return fmt.Errorf("failed to start addon framework: %w", err)
	}
	if err := a.setupControllers(); err != nil {
		return fmt.Errorf("failed to set up addon controllers: %w", err)
	}
	<-ctx.Done()
	return nil
}
