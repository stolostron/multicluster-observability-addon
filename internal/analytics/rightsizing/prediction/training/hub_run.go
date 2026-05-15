package training

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// DefaultThanosQueryURL is the in-cluster Thanos query frontend when observability runs in the standard namespace.
const DefaultThanosQueryURL = "http://observability-thanos-query-frontend.open-cluster-management-observability.svc:9090"

// EnvThanosQueryURL overrides the Thanos base URL (scheme + host + port, no path). Set for non-standard installs.
const EnvThanosQueryURL = "MCOA_THANOS_QUERY_URL"

type hubPredictionFile struct {
	Provider              string  `json:"provider"`
	TrainingIntervalHours int     `json:"trainingIntervalHours"`
	HistoryDays           int     `json:"historyDays"`
	SafetyMarginPercent   float64 `json:"safetyMarginPercent"`
}

// StartHubControllerIfEnabled loads the hub AddOnDeploymentConfig. When workload prediction is enabled, it
// starts the training loop against Thanos and persists model state ConfigMaps in the observability namespace.
func StartHubControllerIfEnabled(ctx context.Context, restCfg *rest.Config, scheme *runtime.Scheme, logger logr.Logger) {
	l := logger.WithName("prediction-training")

	httpClient, err := rest.HTTPClientFor(restCfg)
	if err != nil {
		l.Error(err, "failed to create HTTP client for prediction training; skipping")
		return
	}
	mapper, err := apiutil.NewDynamicRESTMapper(restCfg, httpClient)
	if err != nil {
		l.Error(err, "failed to create REST mapper for prediction training; skipping")
		return
	}
	k8s, err := client.New(restCfg, client.Options{
		Scheme:     scheme,
		Mapper:     mapper,
		HTTPClient: httpClient,
	})
	if err != nil {
		l.Error(err, "failed to create Kubernetes client for prediction training; skipping")
		return
	}

	aodcKey := types.NamespacedName{Namespace: addoncfg.InstallNamespace, Name: addoncfg.Name}
	aodc := &addonapiv1alpha1.AddOnDeploymentConfig{}
	if err := k8s.Get(ctx, aodcKey, aodc); err != nil {
		if apierrors.IsNotFound(err) {
			l.V(1).Info("AddOnDeploymentConfig not found; skipping prediction training controller",
				"namespace", aodcKey.Namespace, "name", aodcKey.Name)
			return
		}
		l.Error(err, "failed to get AddOnDeploymentConfig; skipping prediction training controller")
		return
	}

	opts, err := addon.BuildOptions(aodc)
	if err != nil {
		l.Error(err, "failed to parse AddOnDeploymentConfig; skipping prediction training controller")
		return
	}
	if !opts.Platform.AnalyticsOptions.RightSizing.PredictionEnabled {
		l.V(1).Info("prediction disabled in AddOnDeploymentConfig; skipping prediction training controller")
		return
	}

	tc, err := buildTrainingConfigForHub(ctx, k8s, opts)
	if err != nil {
		l.Error(err, "failed to build training config; skipping prediction training controller")
		return
	}

	thanosBase := strings.TrimSpace(os.Getenv(EnvThanosQueryURL))
	if thanosBase == "" {
		thanosBase = DefaultThanosQueryURL
	}

	querier := NewThanosQuerier(thanosBase)
	ctrl := NewController(tc, querier, k8s, addoncfg.InstallNamespace)

	l.Info("starting prediction training controller",
		"namespace", addoncfg.InstallNamespace,
		"thanosURL", thanosBase,
		"intervalHours", tc.IntervalHours,
		"historyDays", tc.HistoryDays,
		"provider", tc.ProviderType,
	)

	go func() {
		err := ctrl.Start(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			l.Error(err, "prediction training controller exited")
		}
	}()
}

func buildTrainingConfigForHub(ctx context.Context, k8s client.Client, opts addon.Options) (TrainingConfig, error) {
	tc := DefaultTrainingConfig()
	rs := opts.Platform.AnalyticsOptions.RightSizing

	tc.NamespaceEnabled = rs.NamespaceEnabled
	tc.WorkloadEnabled = rs.WorkloadPodEnabled
	tc.GPUEnabled = rs.GPUEnabled
	tc.VMEnabled = rs.VirtualizationEnabled
	tc.ProviderType = rs.PredictionProvider

	var err error
	tc, err = mergePredictionConfigJSON(tc, rs.PredictionConfig)
	if err != nil {
		return tc, fmt.Errorf("merge ADC prediction config: %w", err)
	}

	cm := &corev1.ConfigMap{}
	cmKey := types.NamespacedName{
		Namespace: addoncfg.InstallNamespace,
		Name:      rightsizing.RSPredictionConfigMapName,
	}
	if err := k8s.Get(ctx, cmKey, cm); err == nil {
		if raw := strings.TrimSpace(cm.Data["config.json"]); raw != "" {
			tc, err = mergeHubPredictionFile(tc, raw)
			if err != nil {
				return tc, fmt.Errorf("merge hub prediction ConfigMap: %w", err)
			}
		}
	} else if !apierrors.IsNotFound(err) {
		return tc, fmt.Errorf("get prediction config ConfigMap: %w", err)
	}

	if tc.IntervalHours <= 0 {
		tc.IntervalHours = 6
	}
	if tc.HistoryDays <= 0 {
		tc.HistoryDays = 30
	}
	return tc, nil
}

func mergePredictionConfigJSON(tc TrainingConfig, raw string) (TrainingConfig, error) {
	if strings.TrimSpace(raw) == "" || raw == "null" {
		return tc, nil
	}
	var overlay struct {
		Provider              string `json:"provider"`
		TrainingIntervalHours int    `json:"trainingIntervalHours"`
		HistoryDays           int    `json:"historyDays"`
	}
	if err := json.Unmarshal([]byte(raw), &overlay); err != nil {
		return tc, err
	}
	if overlay.Provider != "" {
		tc.ProviderType = overlay.Provider
	}
	if overlay.TrainingIntervalHours != 0 {
		tc.IntervalHours = overlay.TrainingIntervalHours
	}
	if overlay.HistoryDays != 0 {
		tc.HistoryDays = overlay.HistoryDays
	}
	return tc, nil
}

func mergeHubPredictionFile(tc TrainingConfig, raw string) (TrainingConfig, error) {
	var f hubPredictionFile
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		return tc, err
	}
	if f.Provider != "" {
		tc.ProviderType = f.Provider
	}
	if f.TrainingIntervalHours != 0 {
		tc.IntervalHours = f.TrainingIntervalHours
	}
	if f.HistoryDays != 0 {
		tc.HistoryDays = f.HistoryDays
	}
	return tc, nil
}
