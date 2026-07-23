package main

import (
	"context"
	"crypto/tls"
	"errors"
	goflag "flag"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"

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
	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	addonctrl "github.com/stolostron/multicluster-observability-addon/internal/controllers/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/controllers/resourcecreator"
	"github.com/stolostron/multicluster-observability-addon/internal/controllers/watcher"
	tlshelper "github.com/stolostron/multicluster-observability-addon/pkg/util"
	thanosv1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	crdClientSet "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	utilflag "k8s.io/component-base/cli/flag"
	logs "k8s.io/component-base/logs/api/v1"
	cmdfactory "open-cluster-management.io/addon-framework/pkg/cmd/factory"
	"open-cluster-management.io/addon-framework/pkg/version"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	workv1 "open-cluster-management.io/api/work/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
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

const ocpAPIServerCRDName = "apiservers.config.openshift.io"

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

	httpClient, err := rest.HTTPClientFor(kubeConfig)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client for kubeConfig: %w", err)
	}

	mapper, err := apiutil.NewDynamicRESTMapper(kubeConfig, httpClient)
	if err != nil {
		return fmt.Errorf("failed to create dynamic REST mapper: %w", err)
	}

	addonMgr, err := addonctrl.NewAddonManager(ctx, kubeConfig, scheme, logger, httpClient, mapper)
	if err != nil {
		return fmt.Errorf("failed to create addon manager: %w", err)
	}

	tlsOpts, err := tlshelper.GetOrCreateTLSConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get TLS config: %w", err)
	}

	// Create a single shared controller-runtime Manager for our custom controllers
	sharedMgr, err := ctrl.NewManager(kubeConfig, ctrl.Options{
		Scheme: scheme,
		Logger: logger.WithName("manager"),
		MapperProvider: func(c *rest.Config, hc *http.Client) (meta.RESTMapper, error) {
			return mapper, nil
		},
		Client: client.Options{
			HTTPClient: httpClient,
		},
		Metrics: server.Options{
			BindAddress:   ":8084",
			SecureServing: true,
			TLSOpts:       []func(*tls.Config){tlsOpts},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to start shared manager: %w", err)
	}
	if err = sharedMgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		return fmt.Errorf("failed to set up health check: %w", err)
	}
	if err = sharedMgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		return fmt.Errorf("failed to set up ready check: %w", err)
	}

	if err = setupSecurityProfileWatcher(ctx, kubeConfig, sharedMgr, logger); err != nil {
		logger.Error(err, "unable to set up TLS security profile watcher")
	}

	disableReconciliation := os.Getenv("DISABLE_WATCHER_CONTROLLER")
	if disableReconciliation == "" {
		if err = watcher.SetupWithManager(sharedMgr, addonMgr, logger); err != nil {
			return fmt.Errorf("unable to create watcher controller: %w", err)
		}
	}

	if err = resourcecreator.SetupWithManager(sharedMgr, logger); err != nil {
		return fmt.Errorf("unable to create resource creator controller: %w", err)
	}

	go func() {
		logger.Info("Starting shared controller-runtime manager")
		if startErr := sharedMgr.Start(ctx); startErr != nil {
			logger.Error(startErr, "shared manager exited with error")
		}
	}()

	err = addonMgr.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start addon manager: %w", err)
	}

	<-ctx.Done()

	return nil
}

func setupSecurityProfileWatcher(ctx context.Context, kubeConfig *rest.Config, mgr ctrl.Manager, logger logr.Logger) error {
	crdClient, err := crdClientSet.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("failed to create CRD client: %w", err)
	}

	_, err = crdClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, ocpAPIServerCRDName, metav1.GetOptions{})
	if err != nil {
		logger.Info("APIServer CRD not found, skipping TLS security profile watcher", "crd", ocpAPIServerCRDName)
		return nil
	}

	tlsProfileSpec, err := tlshelper.GetOrCreateTLSProfileSpec(ctx)
	if err != nil {
		return fmt.Errorf("failed to get TLS profile spec: %w", err)
	}

	watcher := &tlsutil.SecurityProfileWatcher{
		Client:                mgr.GetClient(),
		InitialTLSProfileSpec: *tlsProfileSpec,
		OnProfileChange: func(_ context.Context, oldProfile, newProfile configv1.TLSProfileSpec) {
			logger.Info("TLS profile changed, shutting down to reload",
				"oldProfile", oldProfile,
				"newProfile", newProfile,
			)
			os.Exit(0)
		},
	}

	return watcher.SetupWithManager(mgr)
}
