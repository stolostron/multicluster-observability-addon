package handlers

import (
	"context"
	"fmt"
	"slices"

	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func buildUnmanagedOptions(ctx context.Context, k8s client.Client, mcAddon *addonapiv1beta1.ManagedClusterAddOn, opts *manifests.Options) error {
	if !opts.UnmanagedCollectionEnabled() {
		return nil
	}

	keys := common.GetObjectKeys(mcAddon.Status.ConfigReferences, loggingv1.GroupVersion.Group, addoncfg.ClusterLogForwardersResource)
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

	secrets, err := common.GetSecrets(ctx, k8s, clf.Namespace, mcAddon.Namespace, secretNames)
	if err != nil {
		return err
	}
	opts.Unmanaged.Collection.Secrets = secrets

	configMaps, err := common.GetConfigMaps(ctx, k8s, clf.Namespace, mcAddon.Namespace, configmapNames)
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
				extractedSecretsNames[output.TLS.Certificate.SecretName] = struct{}{}
			}
			if output.TLS.Certificate.ConfigMapName != "" {
				extractedConfigMapNames[output.TLS.Certificate.ConfigMapName] = struct{}{}
			}
		}
		if output.TLS.Key != nil {
			extractedSecretsNames[output.TLS.Key.SecretName] = struct{}{}
		}
		if output.TLS.CA != nil {
			if output.TLS.CA.SecretName != "" {
				extractedSecretsNames[output.TLS.CA.SecretName] = struct{}{}
			}
			if output.TLS.CA.ConfigMapName != "" {
				extractedConfigMapNames[output.TLS.CA.ConfigMapName] = struct{}{}
			}
		}
		if output.TLS.KeyPassphrase != nil {
			extractedSecretsNames[output.TLS.KeyPassphrase.SecretName] = struct{}{}
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
			extractedSecretsNames[output.Cloudwatch.Authentication.AWSAccessKey.KeyId.SecretName] = struct{}{}
			extractedSecretsNames[output.Cloudwatch.Authentication.AWSAccessKey.KeySecret.SecretName] = struct{}{}
		case loggingv1.CloudwatchAuthTypeIAMRole:
			extractedSecretsNames[output.Cloudwatch.Authentication.IAMRole.RoleARN.SecretName] = struct{}{}
			getSecretsFromTokenAuthentication(extractedSecretsNames, &output.Cloudwatch.Authentication.IAMRole.Token)
		}

	case loggingv1.OutputTypeGoogleCloudLogging:
		if output.GoogleCloudLogging == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, loggingv1.OutputTypeGoogleCloudLogging, output.Name)
		}
		if output.GoogleCloudLogging.Authentication == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldAuthentication, output.Name)
		}
		extractedSecretsNames[output.GoogleCloudLogging.Authentication.Credentials.SecretName] = struct{}{}

	case loggingv1.OutputTypeAzureMonitor:
		if output.AzureMonitor == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, loggingv1.OutputTypeAzureMonitor, output.Name)
		}
		if output.AzureMonitor.Authentication == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldAuthentication, output.Name)
		}
		extractedSecretsNames[output.AzureMonitor.Authentication.SharedKey.SecretName] = struct{}{}

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
			extractedSecretsNames[output.Kafka.Authentication.SASL.Username.SecretName] = struct{}{}
		}
		if output.Kafka.Authentication.SASL.Password != nil {
			extractedSecretsNames[output.Kafka.Authentication.SASL.Password.SecretName] = struct{}{}
		}

	case loggingv1.OutputTypeSplunk:
		if output.Splunk == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, loggingv1.OutputTypeSplunk, output.Name)
		}
		if output.Splunk.Authentication == nil {
			return []string{}, []string{}, fmt.Errorf("%w: field: %s, outputName: %s", errMissingField, fieldAuthentication, output.Name)
		}
		if output.Splunk.Authentication.Token != nil {
			extractedSecretsNames[output.Splunk.Authentication.Token.SecretName] = struct{}{}
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
	slices.Sort(configMapNames)
	return secretNames, configMapNames, nil
}
