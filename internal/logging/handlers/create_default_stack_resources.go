package handlers

import (
	"context"
	"fmt"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	lmanifests "github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	addonmanifests "github.com/stolostron/multicluster-observability-addon/internal/manifests"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BuildDefaultStackResources creates resources for the default logging stack
// based on the provided options and placement information.
func BuildDefaultStackResources(
	ctx context.Context,
	k8s client.Client,
	cmao *addonv1alpha1.ClusterManagementAddOn,
	platform, userWorkloads addon.LogsOptions,
	hubHostname string,
) ([]client.Object, error) {
	objects := []client.Object{}

	if !platform.DefaultStack {
		return objects, nil
	}

	// Build ClusterLogForwarder for each placement
	for _, placement := range cmao.Spec.InstallStrategy.Placements {
		resourceName := fmt.Sprintf("%s-%s", addon.DefaultStackPrefix, placement.Name)

		lOptsCollection, err := defaultStackOptionsCollection(ctx, k8s, platform, userWorkloads, hubHostname, resourceName)
		if err != nil {
			return nil, err
		}

		clf, err := lmanifests.BuildSSAClusterLogForwarder(lOptsCollection, resourceName)
		if err != nil {
			return nil, err
		}

		objects = append(objects, clf)
	}

	// Build tenants for LokiStack
	// TODO(JoaoBraveCoding): In the future we might want to do this based on
	// placements and have seperate LokiStacks for each placement
	// since this will require the hub to reconcile multiple LokiStacks we will
	// first focus on a single one
	managedClusters := &clusterv1.ManagedClusterList{}
	if err := k8s.List(ctx, managedClusters, &client.ListOptions{}); err != nil {
		return nil, err
	}

	tenants := make([]string, 0, len(managedClusters.Items))
	for _, cluster := range managedClusters.Items {
		tenants = append(tenants, cluster.Name)
	}

	resourceName := fmt.Sprintf("%s-%s", addon.DefaultStackPrefix, "global")
	lOptsStorage, err := defaultStackOptionsStorage(ctx, k8s, platform, userWorkloads, hubHostname, resourceName, tenants)
	if err != nil {
		return nil, err
	}

	ls, err := lmanifests.BuildSSALokiStack(lOptsStorage, resourceName)
	if err != nil {
		return nil, err
	}
	objects = append(objects, ls)

	// Build certiticate objects for each tenant + hub
	for _, tenant := range tenants {
		certObjs, err := buildCertificateObjects(tenant)
		if err != nil {
			return nil, err
		}
		objects = append(objects, certObjs...)
	}

	return objects, nil
}

func defaultStackOptionsCollection(ctx context.Context, k8s client.Client, platform, userWorkloads addon.LogsOptions, hubHostname, resourceName string) (lmanifests.Options, error) {
	opts := lmanifests.Options{
		Platform:      platform,
		UserWorkloads: userWorkloads,
		IsHub:         true,
		HubHostname:   hubHostname,
		DefaultStack: lmanifests.DefaultStack{
			LokiURL: fmt.Sprintf("https://mcoa-managed-instance-openshift-logging.apps.%s/api/logs/v1/%s/otlp/v1/logs", hubHostname, "tenant"),
			Collection: lmanifests.Collection{
				Secrets: []corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      lmanifests.DefaultCollectionMTLSSecretName,
							Namespace: addon.InstallNamespace,
						},
					},
				},
			},
		},
	}

	key := client.ObjectKey{Namespace: addon.InstallNamespace, Name: resourceName}
	existingCLF := &loggingv1.ClusterLogForwarder{}
	err := k8s.Get(ctx, key, existingCLF)
	if err != nil && !apierrors.IsNotFound(err) {
		return opts, err
	}
	opts.DefaultStack.Collection.ClusterLogForwarder = existingCLF

	return opts, nil
}

func defaultStackOptionsStorage(ctx context.Context, k8s client.Client, platform, userWorkloads addon.LogsOptions, hubHostname, resourceName string, tenants []string) (lmanifests.Options, error) {
	opts := lmanifests.Options{
		Platform:      platform,
		UserWorkloads: userWorkloads,
		IsHub:         true,
		HubHostname:   hubHostname,
		DefaultStack: lmanifests.DefaultStack{
			LokiURL: fmt.Sprintf("https://mcoa-managed-instance-openshift-logging.apps.%s/api/logs/v1/%s/otlp/v1/logs", hubHostname, "tenant"),
			Storage: lmanifests.Storage{
				ObjStorageSecret: corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      lmanifests.DefaultStorageMTLSSecretName,
						Namespace: addon.InstallNamespace,
					},
				},
				MTLSSecret: corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      lmanifests.DefaultStorageMTLSSecretName,
						Namespace: addon.HubNamespace,
					},
				},
			},
		},
	}

	key := client.ObjectKey{Namespace: addon.InstallNamespace, Name: resourceName}
	existingLS := &lokiv1.LokiStack{}
	err := k8s.Get(ctx, key, existingLS)
	if err != nil && !apierrors.IsNotFound(err) {
		return opts, err
	}
	opts.DefaultStack.Storage.LokiStack = existingLS
	opts.DefaultStack.Storage.Tenants = tenants

	return opts, nil
}

func buildCertificateObjects(cluster string) ([]client.Object, error) {
	objects := []client.Object{}
	certConfig := addonmanifests.CertificateConfig{
		CommonName: lmanifests.DefaultCollectionCertCommonName,
		Subject: &certmanagerv1.X509Subject{
			// Observatorium API uses OrganizationalUnits to authorize access to
			// the tenant
			OrganizationalUnits: []string{cluster},
		},
		DNSNames: []string{lmanifests.DefaultCollectionCertCommonName},
	}
	key := client.ObjectKey{Name: lmanifests.DefaultCollectionMTLSSecretName, Namespace: cluster}
	cert, err := addonmanifests.BuildClientCertificate(key, certConfig)
	if err != nil {
		return nil, err
	}
	objects = append(objects, cert)

	if cluster == "local-cluster" {
		certConfig := addonmanifests.CertificateConfig{
			CommonName: lmanifests.DefaultStorageCertCommonName,
			Subject:    &certmanagerv1.X509Subject{},
			DNSNames:   []string{lmanifests.DefaultStorageCertCommonName},
		}
		key := client.ObjectKey{Name: lmanifests.DefaultStorageMTLSSecretName, Namespace: cluster}
		cert, err := addonmanifests.BuildServerCertificate(key, certConfig)
		if err != nil {
			return nil, err
		}
		objects = append(objects, cert)
	}

	return objects, nil
}
