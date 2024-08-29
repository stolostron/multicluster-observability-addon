package handlers

import (
	"context"
	"errors"
	"fmt"

	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/logging/manifests"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	fieldAuthentication     = "authentication"
	fieldCloudwatch         = "cloudwatch"
	fieldGoogleCloudLogging = "googleCloudLogging"
	fieldAzureMonitor       = "azureMonitor"
	fieldElasticSearch      = "elasticSearch"
)

var (
	errMissingCLFRef         = errors.New("missing ClusterLogForwarder reference on addon installation")
	errMultipleCLFRef        = errors.New("multiple ClusterLogForwarder references on addon installation")
	errMissingImplementation = errors.New("missing secret implementation for output type")
	errMissingField          = errors.New("missing field needed by output type")
)

func BuildOptions(ctx context.Context, k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, platform, userWorkloads addon.LogsOptions) (manifests.Options, error) {
	opts := manifests.Options{
		Platform:      platform,
		UserWorkloads: userWorkloads,
	}

	if platform.SubscriptionChannel != "" {
		opts.SubscriptionChannel = platform.SubscriptionChannel
	} else {
		opts.SubscriptionChannel = userWorkloads.SubscriptionChannel
	}

	keys := addon.GetObjectKeys(mcAddon.Status.ConfigReferences, loggingv1.GroupVersion.Group, addon.ClusterLogForwardersResource)
	switch {
	case len(keys) == 0:
		return opts, errMissingCLFRef
	case len(keys) > 1:
		return opts, errMultipleCLFRef
	}
	clf := &loggingv1.ClusterLogForwarder{}
	if err := k8s.Get(ctx, keys[0], clf, &client.GetOptions{}); err != nil {
		return opts, err
	}
	opts.ClusterLogForwarder = clf

	secretNames := []string{}
	for _, output := range clf.Spec.Outputs {
		extractedSecretsNames, err := getOutputSecretNames(output)
		if err != nil {
			return opts, err
		}
		secretNames = append(secretNames, extractedSecretsNames...)
	}

	secrets, err := addon.GetSecrets(ctx, k8s, clf.Namespace, mcAddon.Namespace, secretNames)
	if err != nil {
		return opts, err
	}
	opts.Secrets = secrets

	return opts, nil
}

func getOutputSecretNames(output loggingv1.OutputSpec) ([]string, error) {
	getSecretsFromHTTPAuthentication := func(secretNames map[string]struct{}, auth *loggingv1.HTTPAuthentication) {
		if auth.Token != nil {
			secretNames[auth.Token.Secret.Name] = struct{}{}
		}
		if auth.Username != nil {
			secretNames[auth.Username.SecretName] = struct{}{}
		}
		if auth.Password != nil {
			secretNames[auth.Password.SecretName] = struct{}{}
		}
	}

	extractedSecretsNames := map[string]struct{}{}
	switch output.Type {
	case loggingv1.OutputTypeCloudwatch:
		if output.Cloudwatch == nil {
			return []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldCloudwatch, output.Name)
		}
		if output.Cloudwatch.Authentication == nil {
			return []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldAuthentication, output.Name)
		}
		switch output.Cloudwatch.Authentication.Type {
		case loggingv1.CloudwatchAuthTypeAccessKey:
			if output.Cloudwatch.Authentication.AWSAccessKey == nil {
				return []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, loggingv1.CloudwatchAuthTypeAccessKey, output.Name)
			}
			secretName := output.Cloudwatch.Authentication.AWSAccessKey.KeyId.SecretName
			extractedSecretsNames[secretName] = struct{}{}
			secretName = output.Cloudwatch.Authentication.AWSAccessKey.KeySecret.SecretName
			extractedSecretsNames[secretName] = struct{}{}
		case loggingv1.CloudwatchAuthTypeIAMRole:
			// TODO @JoaoBraveCoding: Implement IAM Role support
			return []string{}, fmt.Errorf("%w: secretType: %s, outputName: %s", errMissingImplementation, loggingv1.CloudwatchAuthTypeIAMRole, output.Name)
		}
	case loggingv1.OutputTypeGoogleCloudLogging:
		if output.GoogleCloudLogging == nil {
			return []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldGoogleCloudLogging, output.Name)
		}
		if output.GoogleCloudLogging.Authentication == nil {
			return []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldAuthentication, output.Name)
		}
		secretName := output.GoogleCloudLogging.Authentication.Credentials.SecretName
		extractedSecretsNames[secretName] = struct{}{}
	case loggingv1.OutputTypeAzureMonitor:
		if output.AzureMonitor == nil {
			return []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldAzureMonitor, output.Name)
		}
		if output.AzureMonitor.Authentication == nil {
			return []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldAuthentication, output.Name)
		}
		secretName := output.AzureMonitor.Authentication.SharedKey.SecretName
		extractedSecretsNames[secretName] = struct{}{}
	case loggingv1.OutputTypeLoki:
		if output.Loki == nil {
			return []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldElasticSearch, output.Name)
		}
		if output.Loki.Authentication == nil {
			return []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldAuthentication, output.Name)
		}
		getSecretsFromHTTPAuthentication(extractedSecretsNames, output.Loki.Authentication)

	case loggingv1.OutputTypeElasticsearch:
		if output.Elasticsearch == nil {
			return []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldElasticSearch, output.Name)
		}
		if output.Elasticsearch.Authentication == nil {
			return []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldAuthentication, output.Name)
		}
		getSecretsFromHTTPAuthentication(extractedSecretsNames, output.Elasticsearch.Authentication)
	default:
		return []string{}, fmt.Errorf("%w: secretType: %s, outputName: %s", errMissingImplementation, output.Type, output.Name)
	}

	secretNames := make([]string, 0, len(extractedSecretsNames))
	for secretName := range extractedSecretsNames {
		secretNames = append(secretNames, secretName)
	}
	return secretNames, nil
}
