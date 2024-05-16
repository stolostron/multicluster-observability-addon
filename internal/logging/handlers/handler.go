package handlers

import (
	"context"

	"github.com/ViaQ/logerr/v2/kverrors"
	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/addon/authentication"
	"github.com/rhobs/multicluster-observability-addon/internal/logging/manifests"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	clusterLogForwarderResource = "clusterlogforwarders"
)

func BuildOptions(k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, adoc *addonapiv1alpha1.AddOnDeploymentConfig) (manifests.Options, error) {
	resources := manifests.Options{
		AddOnDeploymentConfig: adoc,
	}

	key := addon.GetObjectKey(mcAddon.Status.ConfigReferences, loggingv1.GroupVersion.Group, clusterLogForwarderResource)
	clf := &loggingv1.ClusterLogForwarder{}
	if err := k8s.Get(context.Background(), key, clf, &client.GetOptions{}); err != nil {
		return resources, err
	}
	resources.ClusterLogForwarder = clf

	klog.Info("looking for configmaps with key to clusterlogforwarder", "key", client.ObjectKey{Name: clf.Name, Namespace: clf.Namespace})
	configmapList := &corev1.ConfigMapList{}
	if err := k8s.List(context.Background(), configmapList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{
			manifests.LabelCLFNamespace: clf.Namespace,
			manifests.LabelCLFName:      clf.Name,
		}),
	}); err != nil {
		return resources, err
	}

	caCM := corev1.ConfigMap{}
	for _, cm := range configmapList.Items {
		// If a cm has the ca annotation then it's the configmap containing the ca
		if _, ok := cm.Annotations[authentication.AnnotationCAToInject]; ok {
			caCM = cm
			continue
		}

		// Discard cm's that belong to other clusters
		if mcAddon.Namespace != cm.Namespace {
			continue
		}

		resources.ConfigMaps = append(resources.ConfigMaps, cm)
	}

	ctx := context.Background()
	authConfig := manifests.AuthDefaultConfig
	authConfig.MTLSConfig.CommonName = mcAddon.Namespace
	if len(caCM.Data) > 0 {
		if ca, ok := caCM.Data["service-ca.crt"]; ok {
			authConfig.MTLSConfig.CAToInject = ca
		} else {
			return resources, kverrors.New("missing ca bundle in configmap", "key", "service-ca.crt")
		}
	}
	authConfig.OwnerLabels = map[string]string{
		manifests.LabelCLFNamespace: clf.Namespace,
		manifests.LabelCLFName:      clf.Name,
	}

	secretsProvider, err := authentication.NewSecretsProvider(k8s, mcAddon.Namespace, addon.Logging, authConfig)
	if err != nil {
		return resources, err
	}

	authMap, err := authentication.BuildAuthenticationFromAnnotations(clf.Annotations)
	if err != nil {
		return resources, err
	}

	targetsSecret, err := secretsProvider.GenerateSecrets(ctx, authMap)
	if err != nil {
		return resources, err
	}

	resources.Secrets, err = secretsProvider.FetchSecrets(ctx, targetsSecret, manifests.AnnotationTargetOutputName)
	if err != nil {
		return resources, err
	}

	return resources, nil
}
