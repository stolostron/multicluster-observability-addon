package helm

import (
	"context"
	"strconv"

	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/addon/authentication"
	lhandlers "github.com/rhobs/multicluster-observability-addon/internal/logging/handlers"
	lmanifests "github.com/rhobs/multicluster-observability-addon/internal/logging/manifests"
	"github.com/rhobs/multicluster-observability-addon/internal/metrics"
	nfhandlers "github.com/rhobs/multicluster-observability-addon/internal/netflows/handlers"
	nfmanifests "github.com/rhobs/multicluster-observability-addon/internal/netflows/manifests"
	thandlers "github.com/rhobs/multicluster-observability-addon/internal/tracing/handlers"
	tmanifests "github.com/rhobs/multicluster-observability-addon/internal/tracing/manifests"
	"k8s.io/klog/v2"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	addonutils "open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type HelmChartValues struct {
	Metrics metrics.MetricsValues     `json:"metrics"`
	Logging lmanifests.LoggingValues  `json:"logging"`
	Tracing tmanifests.TracingValues  `json:"tracing"`
	Netflow nfmanifests.NetflowValues `json:"netflow"`
}

type Options struct {
	MetricsDisabled bool
	LoggingDisabled bool
	TracingDisabled bool
	NetflowDisabled bool
}

func GetValuesFunc(k8s client.Client) addonfactory.GetValuesFunc {
	return func(
		cluster *clusterv1.ManagedCluster,
		addon *addonapiv1alpha1.ManagedClusterAddOn,
	) (addonfactory.Values, error) {
		err := authentication.CreateOrUpdateRootCertificate(k8s)
		if err != nil {
			return nil, err
		}

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
			metrics, err := metrics.GetValuesFunc(k8s, cluster, addon, aodc)
			if err != nil {
				return nil, err
			}
			userValues.Metrics = metrics
		}

		if !opts.LoggingDisabled {
			loggingOpts, err := lhandlers.BuildOptions(k8s, addon, aodc)
			if err != nil {
				return nil, err
			}

			logging, err := lmanifests.BuildValues(loggingOpts)
			if err != nil {
				return nil, err
			}
			userValues.Logging = *logging
		}

		if !opts.TracingDisabled {
			klog.Info("Tracing enabled")
			tracingOpts, err := thandlers.BuildOptions(k8s, addon, aodc)
			if err != nil {
				return nil, err
			}

			tracing, err := tmanifests.BuildValues(tracingOpts)
			if err != nil {
				return nil, err
			}
			userValues.Tracing = tracing
		}

		if !opts.NetflowDisabled {
			klog.Info("Netflow enabled")
			nfOpts, err := nfhandlers.BuildOptions(k8s, addon, aodc)
			if err != nil {
				return nil, err
			}

			nf, err := nfmanifests.BuildValues(nfOpts)
			if err != nil {
				return nil, err
			}
			userValues.Netflow = *nf
		}

		return addonfactory.JsonStructToValues(userValues)
	}
}

func getAddOnDeploymentConfig(k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn) (*addonapiv1alpha1.AddOnDeploymentConfig, error) {
	key := addon.GetObjectKey(mcAddon.Status.ConfigReferences, addonutils.AddOnDeploymentConfigGVR.Group, addon.AddonDeploymentConfigResource)
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
		if keyvalue.Name == addon.AdcMetricsDisabledKey {
			value, err := strconv.ParseBool(keyvalue.Value)
			if err != nil {
				return opts, err
			}
			opts.MetricsDisabled = value
		}
		if keyvalue.Name == addon.AdcLoggingDisabledKey {
			value, err := strconv.ParseBool(keyvalue.Value)
			if err != nil {
				return opts, err
			}
			opts.LoggingDisabled = value
		}
		if keyvalue.Name == addon.AdcTracingisabledKey {
			value, err := strconv.ParseBool(keyvalue.Value)
			if err != nil {
				return opts, err
			}
			opts.TracingDisabled = value
		}
		if keyvalue.Name == addon.AdcNetflowDisabledKey {
			value, err := strconv.ParseBool(keyvalue.Value)
			if err != nil {
				return opts, err
			}
			opts.NetflowDisabled = value
		}
	}
	return opts, nil
}
