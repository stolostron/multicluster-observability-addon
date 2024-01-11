package logging

import (
	"context"
	"embed"
	"encoding/json"

	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/agent"
	"open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AnnotationTargetOutputName = "logging.openshift.io/target-output-name"
)

//go:embed manifests
//go:embed manifests/charts/logging
//go:embed manifests/charts/logging/templates/_helpers.tpl
var FS embed.FS

type helmChartValues struct {
	CLFSpec string `json:"clfSpec"`
}

func NewRegistrationOption(agentName string) *agent.RegistrationOption {
	return &agent.RegistrationOption{
		CSRConfigurations: agent.KubeClientSignerConfigurations(addon.Name, agentName),
		CSRApproveCheck:   utils.DefaultCSRApprover(agentName),
	}
}

func GetValuesFunc(k8s client.Client) addonfactory.GetValuesFunc {
	return func(
		cluster *clusterv1.ManagedCluster,
		addon *addonapiv1alpha1.ManagedClusterAddOn,
	) (addonfactory.Values, error) {
		clfSpec, err := getClusterLogForwarderSpec(k8s, cluster, addon)
		if err != nil {
			return nil, err
		}

		b, err := json.Marshal(clfSpec)
		if err != nil {
			return nil, err
		}

		userValues := helmChartValues{CLFSpec: string(b)}
		return addonfactory.JsonStructToValues(userValues)
	}
}

func getClusterLogForwarderSpec(k8s client.Client, cluster *clusterv1.ManagedCluster, addon *addonapiv1alpha1.ManagedClusterAddOn) (*loggingv1.ClusterLogForwarderSpec, error) {
	var key client.ObjectKey
	for _, config := range addon.Status.ConfigReferences {
		if config.ConfigGroupResource.Group != loggingv1.GroupVersion.Group {
			continue
		}
		if config.ConfigGroupResource.Resource != "clusterlogforwarders" {
			continue
		}

		key.Name = config.Name
		key.Namespace = config.Namespace
	}

	clf := &loggingv1.ClusterLogForwarder{}
	if err := k8s.Get(context.TODO(), key, clf, &client.GetOptions{}); err != nil {
		return nil, err
	}

	for _, config := range addon.Status.ConfigReferences {
		if config.ConfigGroupResource.Group != "" {
			continue
		}

		switch config.ConfigGroupResource.Resource {
		case "configmaps":
			if err := getManagedClusterConfigMaps(k8s, &clf.Spec, config, cluster.Namespace); err != nil {
				return nil, err
			}
		case "secrets":
			if err := getManagedClusterSecrets(k8s, &clf.Spec, config, cluster.Namespace); err != nil {
				return nil, err
			}
		}
	}

	return &clf.Spec, nil
}

func getManagedClusterConfigMaps(k8s client.Client, spec *loggingv1.ClusterLogForwarderSpec, config addonapiv1alpha1.ConfigReference, clusterNs string) error {
	key := client.ObjectKey{Name: config.Name, Namespace: clusterNs}
	cm := &corev1.ConfigMap{}
	if err := k8s.Get(context.TODO(), key, cm, &client.GetOptions{}); err != nil {
		if errors.IsNotFound(err); err != nil {
			return nil
		}
		return err
	}

	clfOutputName, ok := cm.Annotations[AnnotationTargetOutputName]
	if !ok {
		return nil
	}

	outputURL := cm.Data["url"]

	for k, output := range spec.Outputs {
		if output.Name == clfOutputName {
			spec.Outputs[k].URL = outputURL
		}
	}

	return nil
}

func getManagedClusterSecrets(k8s client.Client, spec *loggingv1.ClusterLogForwarderSpec, config addonapiv1alpha1.ConfigReference, clusterNs string) error {
	key := client.ObjectKey{Name: config.Name, Namespace: clusterNs}
	secret := &corev1.Secret{}
	if err := k8s.Get(context.TODO(), key, secret, &client.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	clfOutputName, ok := secret.Annotations[AnnotationTargetOutputName]
	if !ok {
		return nil
	}

	for k, output := range spec.Outputs {
		if output.Name == clfOutputName {
			output.Secret = &loggingv1.OutputSecretSpec{
				Name: secret.Name,
			}
			spec.Outputs[k] = output
		}
	}

	return nil
}
