package handlers

import (
	"context"
	"errors"
	"fmt"
	"slices"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/logging/manifests"
	addonmanifests "github.com/rhobs/multicluster-observability-addon/internal/manifests"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	fieldAuthentication = "authentication"
	fieldSASL           = "sasl"

	managedCollectionCertCommonName = "mcoa-logging-collection-storage"
	managedCollectionSecretName     = "mcoa-logging-managed-collection-tls"

	managedStorageCertCommonName = "mcoa-logging-managed-storage"
	managedStorageSecretName     = "mcoa-logging-managed-storage-tls"

	openshiftLoggingNamespace = "openshift-logging"
)

var (
	errMissingCLFRef         = errors.New("missing ClusterLogForwarder reference on addon installation")
	errMultipleCLFRef        = errors.New("multiple ClusterLogForwarder references on addon installation")
	errMissingImplementation = errors.New("missing secret implementation for output type")
	errMissingField          = errors.New("missing field needed by output type")
)

func BuildOptions(ctx context.Context, k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, platform, userWorkloads addon.LogsOptions, isHubCluster bool) (manifests.Options, error) {
	opts := manifests.Options{
		Platform:      platform,
		UserWorkloads: userWorkloads,
		IsHubCluster:  isHubCluster,
	}

	if platform.SubscriptionChannel != "" {
		opts.SubscriptionChannel = platform.SubscriptionChannel
	} else {
		opts.SubscriptionChannel = userWorkloads.SubscriptionChannel
	}

	if err := createResourcesManaged(ctx, k8s, mcAddon, opts); err != nil {
		return opts, err
	}

	if err := unmagedBuildOptions(ctx, k8s, mcAddon, &opts); err != nil {
		return opts, err
	}

	if err := managedBuildOptions(ctx, k8s, mcAddon, &opts); err != nil {
		return opts, err
	}

	return opts, nil
}

func createResourcesManaged(ctx context.Context, k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, opts manifests.Options) error {
	if !opts.ManagedStackEnabled() {
		return nil
	}

	objects := []client.Object{}
	if !opts.IsHubCluster {
		certConfig := addonmanifests.CertificateConfig{
			CommonName: managedCollectionCertCommonName,
			Subject: &certmanagerv1.X509Subject{
				// Gateway uses Organizational unit to identify the tenant
				Organizations: []string{mcAddon.Namespace},
			},
			DNSNames: []string{managedCollectionCertCommonName},
		}
		key := client.ObjectKey{Name: managedCollectionSecretName, Namespace: mcAddon.Namespace}
		cert, err := addonmanifests.BuildClientCertificate(key, certConfig)
		if err != nil {
			return err
		}
		objects = append(objects, cert)
	}

	if opts.IsHubCluster {
		certConfig := addonmanifests.CertificateConfig{
			CommonName: managedStorageCertCommonName,
			Subject:    &certmanagerv1.X509Subject{},
			DNSNames:   []string{managedStorageCertCommonName},
		}
		key := client.ObjectKey{Name: managedStorageSecretName, Namespace: mcAddon.Namespace}
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

func managedBuildOptions(ctx context.Context, k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, opts *manifests.Options) error {
	if !opts.ManagedStackEnabled() {
		return nil
	}

	if !opts.IsHubCluster {
		secret := &corev1.Secret{}
		key := client.ObjectKey{Name: managedCollectionSecretName, Namespace: mcAddon.Namespace}
		err := k8s.Get(ctx, key, secret, &client.GetOptions{})
		if err != nil {
			// Even for not found we probably just want to return and wait for the next
			// reconciliation loop to try again.
			return err
		}
		opts.Managed.Collection.Secrets = []corev1.Secret{*secret}

		// Get the cluster hostname
		config, err := rest.InClusterConfig()
		if err != nil {
			return err
		}
		opts.Managed.LokiURL = fmt.Sprintf("https://lokistack-hub-openshift-logging.%s/api/logs/v1/%s", config.Host, mcAddon.Namespace)

		return nil
	}

	if opts.IsHubCluster {
		// Get objstorage secret

		// Get mTLS secret
		secret := &corev1.Secret{}
		key := client.ObjectKey{Name: managedStorageSecretName, Namespace: openshiftLoggingNamespace}
		err := k8s.Get(ctx, key, secret, &client.GetOptions{})
		if err != nil {
			// Even for not found we probably just want to return and wait for the next
			// reconciliation loop to try again.
			return err
		}
		opts.Managed.Storage.MTLSSecret = *secret

		// List all the tenants
		mcaoList := &addonapiv1alpha1.ManagedClusterAddOnList{}
		labelSelector := labels.SelectorFromSet(labels.Set{
			addon.LabelOCMAddonName: addon.Name,
		})
		if err := k8s.List(ctx, mcaoList, &client.ListOptions{LabelSelector: labelSelector}); err != nil {
			return err
		}

		tenants := make([]string, 0, len(mcaoList.Items)-1)
		for _, tenant := range mcaoList.Items {
			if tenant.Name != mcAddon.Name {
				tenants = append(tenants, tenant.Name)
			}
		}
		opts.Managed.Storage.Tenants = tenants

		return nil
	}

	return nil
}

func unmagedBuildOptions(ctx context.Context, k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, opts *manifests.Options) error {
	if !opts.UnmanagedCollectionEnabled() {
		return nil
	}

	keys := addon.GetObjectKeys(mcAddon.Status.ConfigReferences, loggingv1.GroupVersion.Group, addon.ClusterLogForwardersResource)
	switch {
	case len(keys) == 0:
		return errMissingCLFRef
	case len(keys) > 1:
		return errMultipleCLFRef
	}
	clf := &loggingv1.ClusterLogForwarder{}
	if err := k8s.Get(ctx, keys[0], clf, &client.GetOptions{}); err != nil {
		return err
	}
	opts.Unmanaged.Collection.ClusterLogForwarder = clf

	secretNames := []string{}
	configmapNames := []string{}
	for _, output := range clf.Spec.Outputs {
		extractedSecretsNames, extracedConfigmapNames, err := getOutputResourcesNames(output)
		if err != nil {
			return err
		}
		secretNames = append(secretNames, extractedSecretsNames...)
		configmapNames = append(configmapNames, extracedConfigmapNames...)
	}

	secrets, err := addon.GetSecrets(ctx, k8s, clf.Namespace, mcAddon.Namespace, secretNames)
	if err != nil {
		return err
	}
	opts.Unmanaged.Collection.Secrets = secrets

	configMaps, err := addon.GetConfigMaps(ctx, k8s, clf.Namespace, mcAddon.Namespace, configmapNames)
	if err != nil {
		return err
	}
	opts.Unmanaged.Collection.ConfigMaps = configMaps

	return nil
}

func getOutputResourcesNames(output loggingv1.OutputSpec) ([]string, []string, error) {
	extractedSecretsNames := map[string]struct{}{}
	extractedConfigMapNames := map[string]struct{}{}

	getSecretsFromTokenAuthentication := func(secretNames map[string]struct{}, token *loggingv1.BearerToken) {
		switch token.From {
		case loggingv1.BearerTokenFromSecret:
			secretNames[token.Secret.Name] = struct{}{}
		case loggingv1.BearerTokenFromServiceAccount:
			// In this authentication method MCOA should't do anything since
			// the token is associated with the SA.
			// Maybe we should instead create the necessary RBAC for the SA?
		}
	}

	getSecretsFromHTTPAuthentication := func(secretNames map[string]struct{}, auth *loggingv1.HTTPAuthentication) {
		if auth.Token != nil {
			getSecretsFromTokenAuthentication(secretNames, auth.Token)
		}
		if auth.Username != nil {
			secretNames[auth.Username.SecretName] = struct{}{}
		}
		if auth.Password != nil {
			secretNames[auth.Password.SecretName] = struct{}{}
		}
	}

	if output.TLS != nil {
		if output.TLS.Certificate != nil {
			if output.TLS.Certificate.SecretName != "" {
				secretName := output.TLS.Certificate.SecretName
				extractedSecretsNames[secretName] = struct{}{}
			}
			if output.TLS.Certificate.ConfigMapName != "" {
				configMapName := output.TLS.Certificate.ConfigMapName
				extractedConfigMapNames[configMapName] = struct{}{}
			}
		}
		if output.TLS.Key != nil {
			secretName := output.TLS.Key.SecretName
			extractedSecretsNames[secretName] = struct{}{}
		}
		if output.TLS.CA != nil {
			if output.TLS.CA.SecretName != "" {
				secretName := output.TLS.CA.SecretName
				extractedSecretsNames[secretName] = struct{}{}
			}
			if output.TLS.CA.ConfigMapName != "" {
				configMapName := output.TLS.CA.ConfigMapName
				extractedConfigMapNames[configMapName] = struct{}{}
			}
		}
		if output.TLS.KeyPassphrase != nil {
			secretName := output.TLS.KeyPassphrase.SecretName
			extractedSecretsNames[secretName] = struct{}{}
		}
	}
	switch output.Type {
	case loggingv1.OutputTypeCloudwatch:
		if output.Cloudwatch == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, loggingv1.OutputTypeCloudwatch, output.Name)
		}
		if output.Cloudwatch.Authentication == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldAuthentication, output.Name)
		}
		switch output.Cloudwatch.Authentication.Type {
		case loggingv1.CloudwatchAuthTypeAccessKey:
			if output.Cloudwatch.Authentication.AWSAccessKey == nil {
				return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, loggingv1.CloudwatchAuthTypeAccessKey, output.Name)
			}
			secretName := output.Cloudwatch.Authentication.AWSAccessKey.KeyId.SecretName
			extractedSecretsNames[secretName] = struct{}{}
			secretName = output.Cloudwatch.Authentication.AWSAccessKey.KeySecret.SecretName
			extractedSecretsNames[secretName] = struct{}{}
		case loggingv1.CloudwatchAuthTypeIAMRole:
			secretName := output.Cloudwatch.Authentication.IAMRole.RoleARN.SecretName
			extractedSecretsNames[secretName] = struct{}{}
			getSecretsFromTokenAuthentication(extractedSecretsNames, &output.Cloudwatch.Authentication.IAMRole.Token)
		}

	case loggingv1.OutputTypeGoogleCloudLogging:
		if output.GoogleCloudLogging == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, loggingv1.OutputTypeGoogleCloudLogging, output.Name)
		}
		if output.GoogleCloudLogging.Authentication == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldAuthentication, output.Name)
		}
		secretName := output.GoogleCloudLogging.Authentication.Credentials.SecretName
		extractedSecretsNames[secretName] = struct{}{}

	case loggingv1.OutputTypeAzureMonitor:
		if output.AzureMonitor == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, loggingv1.OutputTypeAzureMonitor, output.Name)
		}
		if output.AzureMonitor.Authentication == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldAuthentication, output.Name)
		}
		secretName := output.AzureMonitor.Authentication.SharedKey.SecretName
		extractedSecretsNames[secretName] = struct{}{}

	case loggingv1.OutputTypeLoki:
		if output.Loki == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, loggingv1.OutputTypeLoki, output.Name)
		}
		if output.Loki.Authentication == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldAuthentication, output.Name)
		}
		getSecretsFromHTTPAuthentication(extractedSecretsNames, output.Loki.Authentication)

	case loggingv1.OutputTypeLokiStack:
		if output.LokiStack == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, loggingv1.OutputTypeLokiStack, output.Name)
		}
		if output.LokiStack.Authentication == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldAuthentication, output.Name)
		}
		getSecretsFromTokenAuthentication(extractedSecretsNames, output.LokiStack.Authentication.Token)

	case loggingv1.OutputTypeElasticsearch:
		if output.Elasticsearch == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, loggingv1.OutputTypeElasticsearch, output.Name)
		}
		if output.Elasticsearch.Authentication == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldAuthentication, output.Name)
		}
		getSecretsFromHTTPAuthentication(extractedSecretsNames, output.Elasticsearch.Authentication)

	case loggingv1.OutputTypeHTTP:
		if output.HTTP == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, loggingv1.OutputTypeHTTP, output.Name)
		}
		if output.HTTP.Authentication == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldAuthentication, output.Name)
		}
		getSecretsFromHTTPAuthentication(extractedSecretsNames, output.HTTP.Authentication)

	case loggingv1.OutputTypeKafka:
		if output.Kafka == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, loggingv1.OutputTypeKafka, output.Name)
		}
		if output.Kafka.Authentication == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldAuthentication, output.Name)
		}
		if output.Kafka.Authentication.SASL == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldSASL, output.Name)
		}
		if output.Kafka.Authentication.SASL.Username != nil {
			secretName := output.Kafka.Authentication.SASL.Username.SecretName
			extractedSecretsNames[secretName] = struct{}{}
		}
		if output.Kafka.Authentication.SASL.Password != nil {
			secretName := output.Kafka.Authentication.SASL.Password.SecretName
			extractedSecretsNames[secretName] = struct{}{}
		}

	case loggingv1.OutputTypeSplunk:
		if output.Splunk == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, loggingv1.OutputTypeSplunk, output.Name)
		}
		if output.Splunk.Authentication == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldAuthentication, output.Name)
		}
		if output.Splunk.Authentication.Token != nil {
			secretName := output.Splunk.Authentication.Token.SecretName
			extractedSecretsNames[secretName] = struct{}{}
		}

	case loggingv1.OutputTypeOTLP:
		if output.OTLP == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, loggingv1.OutputTypeOTLP, output.Name)
		}
		if output.OTLP.Authentication == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldAuthentication, output.Name)
		}
		getSecretsFromHTTPAuthentication(extractedSecretsNames, output.OTLP.Authentication)

	default:
		return []string{}, []string{}, fmt.Errorf("%w: secretType: %s, outputName: %s", errMissingImplementation, output.Type, output.Name)
	}

	secretNames := make([]string, 0, len(extractedSecretsNames))
	for secretName := range extractedSecretsNames {
		secretNames = append(secretNames, secretName)
	}
	configMapNames := make([]string, 0, len(extractedConfigMapNames))
	for configMapName := range extractedConfigMapNames {
		configMapNames = append(configMapNames, configMapName)
	}
	// Since we use a map we have to sort the slice to make the manifest generation
	// deterministic.
	slices.Sort(configMapNames)
	return secretNames, configMapNames, nil
}
