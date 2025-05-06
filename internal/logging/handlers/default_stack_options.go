package handlers

import (
	"context"
	"fmt"
	"strings"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	"github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	addonmanifests "github.com/stolostron/multicluster-observability-addon/internal/manifests"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createDefaultStackCertificates(ctx context.Context, k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, opts manifests.Options) error {
	if !opts.DefaultStackEnabled() {
		return nil
	}

	objects := []client.Object{}
	certConfig := addonmanifests.CertificateConfig{
		CommonName: manifests.DefaultCollectionCertCommonName,
		Subject: &certmanagerv1.X509Subject{
			// Observatorium API uses OrganizationalUnits to authorize access to
			// the tenant
			OrganizationalUnits: []string{mcAddon.Namespace},
		},
		DNSNames: []string{manifests.DefaultCollectionCertCommonName},
	}
	key := client.ObjectKey{Name: manifests.DefaultCollectionMTLSSecretName, Namespace: mcAddon.Namespace}
	cert, err := addonmanifests.BuildClientCertificate(key, certConfig)
	if err != nil {
		return err
	}
	objects = append(objects, cert)

	if opts.IsHub {
		certConfig := addonmanifests.CertificateConfig{
			CommonName: manifests.DefaultStorageCertCommonName,
			Subject:    &certmanagerv1.X509Subject{},
			DNSNames:   []string{manifests.DefaultStorageCertCommonName},
		}
		key := client.ObjectKey{Name: manifests.DefaultStorageMTLSSecretName, Namespace: mcAddon.Namespace}
		cert, err := addonmanifests.BuildServerCertificate(key, certConfig)
		if err != nil {
			return err
		}
		objects = append(objects, cert)
	}

	for _, obj := range objects {
		desired := obj.DeepCopyObject().(client.Object)
		mutateFn := addonmanifests.MutateFuncFor(obj, desired, nil)

		op, err := ctrl.CreateOrUpdate(ctx, k8s, obj, mutateFn)
		if err != nil {
			klog.Error(err, "failed to configure resource")
			continue
		}

		msg := fmt.Sprintf("Resource has been %s", op)
		switch op {
		default:
			klog.Info(msg)
		}
	}

	return nil
}

func buildDefaultStackOptions(ctx context.Context, k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, opts *manifests.Options) error {
	if !opts.DefaultStackEnabled() {
		return nil
	}

	// Get CLF from ManagedClusterAddOn
	keys := common.GetObjectKeys(mcAddon.Status.ConfigReferences, loggingv1.GroupVersion.Group, addon.ClusterLogForwardersResource)
	if len(keys) == 0 {
		return errMissingDefaultCLFRef
	}
	clfKey := client.ObjectKey{}
	// TODO(JoaoBraveCoding): This needs to be changed to use ownerReferences
	for _, key := range keys {
		if strings.HasPrefix(key.Name, addon.DefaultStackPrefix) {
			clfKey = key
			break
		}
	}
	if clfKey.Name == "" {
		return errMissingDefaultCLFRef
	}

	clf := &loggingv1.ClusterLogForwarder{}
	if err := k8s.Get(ctx, clfKey, clf, &client.GetOptions{}); err != nil {
		return err
	}
	opts.DefaultStack.Collection.ClusterLogForwarder = clf

	mTLSSecret, err := common.GetSecret(ctx, k8s, clf.Namespace, mcAddon.Namespace, manifests.DefaultCollectionMTLSSecretName)
	if err != nil {
		// Even for not found we probably just want to return and wait for the next
		// reconciliation loop to try again.
		return err
	}
	opts.DefaultStack.Collection.Secrets = []corev1.Secret{*mTLSSecret}

	// Get the cluster hostname
	opts.DefaultStack.LokiURL = fmt.Sprintf("https://mcoa-managed-instance-openshift-logging.apps.%s/api/logs/v1/%s/otlp/v1/logs", opts.HubHostname, mcAddon.Namespace)

	if opts.IsHub {
		// Get LS from ManagedClusterAddOn
		keys := common.GetObjectKeys(mcAddon.Status.ConfigReferences, lokiv1.GroupVersion.Group, addon.LokiStacksResource)
		if len(keys) == 0 {
			return errMissingDefaultLSRef
		}
		lsKey := client.ObjectKey{}
		// TODO(JoaoBraveCoding): This needs to be changed to use ownerReferences
		for _, key := range keys {
			if strings.HasPrefix(key.Name, addon.DefaultStackPrefix) {
				lsKey = key
				break
			}
		}
		if lsKey.Name == "" {
			return errMissingDefaultLSRef
		}

		ls := &lokiv1.LokiStack{}
		if err := k8s.Get(ctx, lsKey, ls, &client.GetOptions{}); err != nil {
			return err
		}
		opts.DefaultStack.Storage.LokiStack = ls

		// Get objstorage secret
		objStorageSecret, err := common.GetSecret(ctx, k8s, ls.Namespace, mcAddon.Namespace, ls.Spec.Storage.Secret.Name)
		if err != nil {
			// Even for not found we probably just want to return and wait for the next
			// reconciliation loop to try again.
			return err
		}
		opts.DefaultStack.Storage.ObjStorageSecret = *objStorageSecret

		// Get mTLS secret
		mTLSSecret, err := common.GetSecret(ctx, k8s, ls.Namespace, mcAddon.Namespace, manifests.DefaultStorageMTLSSecretName)
		if err != nil {
			// Even for not found we probably just want to return and wait for the next
			// reconciliation loop to try again.
			return err
		}
		opts.DefaultStack.Storage.MTLSSecret = *mTLSSecret

		// TODO(JoaoBraveCoding): This might be rather heavy in big clusters,
		// this might be a good place to lower memory consumption.
		mcaoList := &addonapiv1alpha1.ManagedClusterAddOnList{}
		if err := k8s.List(ctx, mcaoList, &client.ListOptions{}); err != nil {
			return err
		}

		tenants := make([]string, 0, len(mcaoList.Items))
		for _, tenant := range mcaoList.Items {
			// TODO(JoaoBraveCoding): This is not the best way to match tenants due
			// to the addon-framework supporting tenantcy, but it will do for now
			if tenant.Name == addon.Name && tenant.Namespace != mcAddon.Namespace {
				tenants = append(tenants, tenant.Namespace)
			}
		}
		opts.DefaultStack.Storage.Tenants = tenants

		return nil
	}

	return nil
}

func DefaultStackOptions(ctx context.Context, k8s client.Client, platform, userWorkloads addon.LogsOptions, hubHostname, resourceName string) (manifests.Options, error) {
	opts := manifests.Options{
		Platform:      platform,
		UserWorkloads: userWorkloads,
		IsHub:         true,
		HubHostname:   hubHostname,
		DefaultStack: manifests.DefaultStack{
			LokiURL: fmt.Sprintf("https://mcoa-managed-instance-openshift-logging.apps.%s/api/logs/v1/%s/otlp/v1/logs", hubHostname, "tenant"),
			Collection: manifests.Collection{
				Secrets: []corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      manifests.DefaultCollectionMTLSSecretName,
							Namespace: addon.InstallNamespace,
						},
					},
				},
			},
			Storage: manifests.Storage{
				ObjStorageSecret: corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      manifests.DefaultStorageMTLSSecretName,
						Namespace: addon.InstallNamespace,
					},
				},
				MTLSSecret: corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      manifests.DefaultStorageMTLSSecretName,
						Namespace: addon.HubNamespace,
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

	key = client.ObjectKey{Namespace: addon.InstallNamespace, Name: resourceName}
	existingLS := &lokiv1.LokiStack{}
	err = k8s.Get(ctx, key, existingLS)
	if err != nil && !apierrors.IsNotFound(err) {
		return opts, err
	}
	opts.DefaultStack.Storage.LokiStack = existingLS

	// TODO(JoaoBraveCoding): This might be rather heavy in big clusters,
	// good place to lower memory consumption.
	mcaoList := &addonapiv1alpha1.ManagedClusterAddOnList{}
	if err := k8s.List(ctx, mcaoList, &client.ListOptions{}); err != nil {
		return opts, err
	}

	tenants := make([]string, 0, len(mcaoList.Items))
	for _, tenant := range mcaoList.Items {
		if tenant.Name == addon.Name {
			tenants = append(tenants, tenant.Namespace)
		}
	}
	opts.DefaultStack.Storage.Tenants = tenants

	return opts, nil
}
