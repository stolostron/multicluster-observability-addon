package helm

import (
	"context"
	"strconv"

	"github.com/stolostron/multicluster-observability-addon/internal/logging"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics"
	"github.com/stolostron/multicluster-observability-addon/internal/tracing"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type HelmChartValues struct {
	Metrics metrics.MetricsValues `json:"metrics"`
	Logging logging.LoggingValues `json:"logging"`
	Tracing tracing.TracingValues `json:"tracing"`
}

type Options struct {
	MetricsDisabled bool
	LoggingDisabled bool
	TracingDisabled bool
}

func GetValuesFunc(k8s client.Client) addonfactory.GetValuesFunc {
	return func(
		cluster *clusterv1.ManagedCluster,
		addon *addonapiv1alpha1.ManagedClusterAddOn,
	) (addonfactory.Values, error) {
		aodc, err := getAddOnDeploymentConfig(k8s, addon)
		if err != nil {
			return nil, err
		}
		opts, err := buildOptions(aodc)
		if err != nil {
			return nil, err
		}

		var userValues HelmChartValues

		if !opts.MetricsDisabled {
			metrics, err := metrics.GetValuesFunc(k8s, cluster, addon)
			if err != nil {
				return nil, err
			}
			userValues.Metrics = metrics
		}

		if !opts.LoggingDisabled {
			logging, err := logging.GetValuesFunc(k8s, cluster, addon, aodc)
			if err != nil {
				return nil, err
			}
			userValues.Logging = logging
		}

		if !opts.TracingDisabled {
			tracing, err := tracing.GetValuesFunc(k8s, cluster, addon)
			if err != nil {
				return nil, err
			}
			userValues.Tracing = tracing
		}

		return addonfactory.JsonStructToValues(userValues)
	}
}

func getAddOnDeploymentConfig(k8s client.Client, addon *addonapiv1alpha1.ManagedClusterAddOn) (*addonapiv1alpha1.AddOnDeploymentConfig, error) {
	var key client.ObjectKey
	for _, config := range addon.Status.ConfigReferences {
		if config.ConfigGroupResource.Group != utils.AddOnDeploymentConfigGVR.Group {
			continue
		}
		if config.ConfigGroupResource.Resource != "addondeploymentconfigs" {
			continue
		}

		key.Name = "multicluster-observability-addon"
		key.Namespace = "open-cluster-management"
	}

	addOnDeployment := &addonapiv1alpha1.AddOnDeploymentConfig{}
	if err := k8s.Get(context.TODO(), key, addOnDeployment, &client.GetOptions{}); err != nil {
		// TODO(JoaoBraveCoding) Add proper error handling
		return addOnDeployment, err
	}
	return addOnDeployment, nil
}

func buildOptions(addOnDeployment *addonapiv1alpha1.AddOnDeploymentConfig) (Options, error) {
	var opts Options
	if addOnDeployment == nil {
		return opts, nil
	}

	if addOnDeployment.Spec.CustomizedVariables == nil {
		return opts, nil
	}

	for _, keyvalue := range addOnDeployment.Spec.CustomizedVariables {
		if keyvalue.Name == "metricsDisabled" {
			value, err := strconv.ParseBool(keyvalue.Value)
			if err != nil {
				return opts, err
			}
			opts.MetricsDisabled = value
		}
		if keyvalue.Name == "loggingDisabled" {
			value, err := strconv.ParseBool(keyvalue.Value)
			if err != nil {
				return opts, err
			}
			opts.LoggingDisabled = value
		}
		if keyvalue.Name == "tracingDisabled" {
			value, err := strconv.ParseBool(keyvalue.Value)
			if err != nil {
				return opts, err
			}
			opts.TracingDisabled = value
		}

	}
	return opts, nil
}
