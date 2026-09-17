package main

import (
	"context"
	"crypto/tls"
	"errors"
	goflag "flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/ViaQ/logerr/v2/log"
	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	configinformers "github.com/openshift/client-go/config/informers/externalversions"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	tlsutil "github.com/openshift/controller-runtime-common/pkg/tls"
	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	libgoclient "github.com/openshift/library-go/pkg/config/client"
	"github.com/openshift/library-go/pkg/config/serving"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	libgocrypto "github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/serviceability"
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
	"github.com/stolostron/multicluster-observability-addon/internal/controllers/watcher"
	tlshelper "github.com/stolostron/multicluster-observability-addon/pkg/util"
	thanosv1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"golang.org/x/sync/errgroup"
	crdClientSet "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	utilflag "k8s.io/component-base/cli/flag"
	kubelogs "k8s.io/component-base/logs"
	logs "k8s.io/component-base/logs/api/v1"
	"k8s.io/utils/clock"
	"open-cluster-management.io/addon-framework/pkg/version"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	workv1 "open-cluster-management.io/api/work/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
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

var errNoSupportedTLSCiphers = errors.New("TLS profile has no supported cipher suites")

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
	ocpAPIServerCRDName   = "apiservers.config.openshift.io"
	controllerName        = "multicluster-observability-addon-controller"
	defaultListenAddress  = "0.0.0.0:8443"
	defaultMetricsAddress = ":8084"
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
	kubeConfigFile      string
	namespace           string
	leaderElection      bool
	listenAddress       string
	addonMetricsAddress string
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
	cmd.Flags().StringVar(&options.namespace, "component-namespace", "", "Namespace of the component. Defaults to the service account namespace.")
	cmd.Flags().BoolVar(&options.leaderElection, "enable-leader-election", false, "Enable leader election for both controller managers.")
	cmd.Flags().StringVar(&options.listenAddress, "listen", defaultListenAddress, "The IP:port for authenticated metrics, health checks and diagnostics.")
	cmd.Flags().StringVar(&options.addonMetricsAddress, "metrics-bind-address", defaultMetricsAddress, "The address for authenticated controller-runtime metrics. Use 0 to disable.")
	cmd.Flags().IntVar(&logVerbosity, "log-verbosity", 0, "Log verbosity level. The higher the level, the noisier the logs.")
	return cmd
}

func (o *controllerOptions) run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	servingOptions, err := serving.ToServingOptions(configv1.HTTPServingInfo{
		ServingInfo: configv1.ServingInfo{BindAddress: o.listenAddress},
	})
	if err != nil {
		return fmt.Errorf("invalid serving address: %w", err)
	}
	servingOptions.Required = true
	if err = errors.Join(servingOptions.Validate()...); err != nil {
		return fmt.Errorf("invalid serving options: %w", err)
	}
	kubeConfigFile := o.kubeConfigFile
	if kubeConfigFile != "" {
		// library-go loads kubeconfig bytes without resolving file-relative paths.
		// Keep the original file untouched and pass a private, resolved copy.
		dir, tempErr := os.MkdirTemp("", "mcoa-kubeconfig-")
		if tempErr != nil {
			return fmt.Errorf("failed to create kubeconfig directory: %w", tempErr)
		}
		defer func() {
			if removeErr := os.RemoveAll(dir); removeErr != nil {
				ctrl.Log.Error(removeErr, "failed to remove temporary kubeconfig")
			}
		}()
		kubeConfigFile = filepath.Join(dir, "kubeconfig")
		if err = writeResolvedKubeConfig(o.kubeConfigFile, kubeConfigFile); err != nil {
			return fmt.Errorf("failed to resolve kubeconfig: %w", err)
		}
	}
	kubeConfig, err := libgoclient.GetKubeConfigOrInClusterConfig(kubeConfigFile, nil)
	if err != nil {
		return fmt.Errorf("failed to load Kubernetes config: %w", err)
	}
	profile, err := tlshelper.GetOrCreateTLSProfileSpec(ctx, kubeConfig)
	if err != nil {
		return fmt.Errorf("failed to get TLS profile: %w", err)
	}
	config, metricsOptions, err := newServerOptions(*profile)
	if err != nil {
		return fmt.Errorf("failed to configure servers: %w", err)
	}
	config.ServingInfo.BindAddress = o.listenAddress
	config.LeaderElection.Disable = !o.leaderElection
	metricsOptions.BindAddress = o.addonMetricsAddress
	if err = startSecurityProfileWatcher(ctx, kubeConfig, *profile, cancel); err != nil {
		return fmt.Errorf("failed to start TLS profile watcher: %w", err)
	}

	start := func(ctx context.Context, controllerContext *controllercmd.ControllerContext) error {
		return runControllers(ctx, controllerContext.KubeConfig, metricsOptions)
	}
	commandConfig := controllercmd.NewControllerCommandConfig(controllerName, version.Get(), start, clock.RealClock{})
	startingFiles, observedFiles, err := commandConfig.AddDefaultRotationToConfig(&config, nil)
	if err != nil {
		return fmt.Errorf("failed to configure serving certificates: %w", err)
	}
	// Both listeners use the same certificate and restart when it rotates.
	metricsOptions.CertDir = filepath.Dir(config.ServingInfo.CertFile)
	metricsOptions.CertName = filepath.Base(config.ServingInfo.CertFile)
	metricsOptions.KeyName = filepath.Base(config.ServingInfo.KeyFile)
	certChanged := make(chan struct{})
	go func() {
		select {
		case <-certChanged:
			cancel()
		case <-ctx.Done():
		}
	}()

	// Unlike the command wrapper, the builder lets us apply the cluster TLS
	// profile before serving starts. Profiling remains library-go's opt-in facility.
	serviceability.StartProfiler()
	return controllercmd.NewController(controllerName, start, clock.RealClock{}).
		WithKubeConfigFile(kubeConfigFile, nil).
		WithComponentNamespace(o.namespace).
		WithLeaderElection(config.LeaderElection, o.namespace, controllerName).
		WithVersion(version.Get()).
		WithServer(config.ServingInfo, config.Authentication, config.Authorization).
		WithRestartOnChange(certChanged, startingFiles, observedFiles...).
		Run(ctx, nil)
}

func writeResolvedKubeConfig(source, destination string) error {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = source
	config, err := rules.Load()
	if err != nil {
		return err
	}
	return clientcmd.WriteToFile(*config, destination)
}

func newServerOptions(profile configv1.TLSProfileSpec) (operatorv1alpha1.GenericOperatorConfig, server.Options, error) {
	configureTLS, unsupportedCiphers := tlsutil.NewTLSConfigFromProfile(profile)
	if len(unsupportedCiphers) > 0 {
		ctrl.Log.Info("TLS profile contains unsupported ciphers", "ciphers", unsupportedCiphers)
	}
	tlsConfig := &tls.Config{}
	configureTLS(tlsConfig)
	if tlsConfig.MinVersion < tls.VersionTLS13 && len(tlsConfig.CipherSuites) == 0 {
		return operatorv1alpha1.GenericOperatorConfig{}, server.Options{}, errNoSupportedTLSCiphers
	}
	cipherSuites := make([]string, 0, len(tlsConfig.CipherSuites))
	for _, cipher := range tlsConfig.CipherSuites {
		cipherSuites = append(cipherSuites, tls.CipherSuiteName(cipher))
	}
	config := operatorv1alpha1.GenericOperatorConfig{
		ServingInfo: configv1.HTTPServingInfo{
			ServingInfo: configv1.ServingInfo{
				BindAddress:   defaultListenAddress,
				MinTLSVersion: string(profile.MinTLSVersion),
				CipherSuites:  cipherSuites,
			},
		},
		// Preserve the existing lease timings, including on single-node clusters.
		LeaderElection: configv1.LeaderElection{
			Disable:       true,
			LeaseDuration: metav1.Duration{Duration: 137 * time.Second},
			RenewDeadline: metav1.Duration{Duration: 107 * time.Second},
			RetryPeriod:   metav1.Duration{Duration: 26 * time.Second},
		},
	}
	// Delegated authentication and authorization are enabled by default in
	// library-go. Apply the same policy to the separate metrics registry.
	return config, server.Options{
		BindAddress:   defaultMetricsAddress,
		SecureServing: true,
		TLSOpts: []func(*tls.Config){func(config *tls.Config) {
			configureTLS(config)
			// Match library-go's default of disabling HTTP/2.
			config.NextProtos = []string{"http/1.1"}
		}},
		FilterProvider: filters.WithAuthenticationAndAuthorization,
	}, nil
}

func runControllers(ctx context.Context, kubeConfig *rest.Config, metricsOptions server.Options) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	group, ctx := errgroup.WithContext(ctx)
	logger := log.NewLogger("mcoa", log.WithVerbosity(logVerbosity))

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
		Metrics: metricsOptions,
	})
	if err != nil {
		return fmt.Errorf("failed to start shared manager: %w", err)
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

	group.Go(func() error {
		logger.Info("Starting shared controller-runtime manager")
		if startErr := sharedMgr.Start(ctx); startErr != nil {
			return fmt.Errorf("shared manager exited with error: %w", startErr)
		}
		return nil
	})
	group.Go(func() error {
		if startErr := addonMgr.Start(ctx); startErr != nil {
			return fmt.Errorf("failed to start addon manager: %w", startErr)
		}
		<-ctx.Done()
		return nil
	})

	return group.Wait()
}

func startSecurityProfileWatcher(ctx context.Context, kubeConfig *rest.Config, profile configv1.TLSProfileSpec, restart context.CancelFunc) error {
	crdClient, err := crdClientSet.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("failed to create CRD client: %w", err)
	}

	_, err = crdClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, ocpAPIServerCRDName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		ctrl.Log.Info("APIServer CRD not found, skipping TLS security profile watcher", "crd", ocpAPIServerCRDName)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check APIServer CRD: %w", err)
	}
	configClient, err := configclient.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("failed to create config client: %w", err)
	}
	factory := configinformers.NewSharedInformerFactoryWithOptions(configClient, 0,
		configinformers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = "metadata.name=cluster"
		}))
	informer := factory.Config().V1().APIServers().Informer()
	if _, err = informer.AddEventHandler(tlsProfileEventHandler(profile, restart)); err != nil {
		return err
	}
	// Serving starts on followers too, so profile observation must not wait for leadership.
	go informer.Run(ctx.Done())
	return nil
}

func tlsProfileEventHandler(initial configv1.TLSProfileSpec, restart context.CancelFunc) cache.ResourceEventHandler {
	check := func(obj any) {
		apiServer, ok := obj.(*configv1.APIServer)
		if !ok || apiServer.Name != "cluster" {
			return
		}
		profile := configv1.TLSProfiles[libgocrypto.DefaultTLSProfileType]
		if libgocrypto.ShouldHonorClusterTLSProfile(apiServer.Spec.TLSAdherence) {
			configured, err := tlsutil.GetTLSProfileSpec(apiServer.Spec.TLSSecurityProfile)
			if err != nil {
				ctrl.Log.Error(err, "failed to read changed TLS profile")
				return
			}
			profile = &configured
		}
		if !reflect.DeepEqual(initial, *profile) {
			ctrl.Log.Info("TLS profile changed, shutting down to reload", "oldProfile", initial, "newProfile", *profile)
			restart()
		}
	}
	return cache.ResourceEventHandlerFuncs{
		AddFunc: check,
		UpdateFunc: func(_, current any) {
			check(current)
		},
	}
}
