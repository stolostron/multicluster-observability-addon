package handlers

import (
	"fmt"
	"testing"

	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/stretchr/testify/assert"
)

func TestGetOutputSecretNames(t *testing.T) {
	tests := []struct {
		name                    string
		output                  loggingv1.OutputSpec
		extractedSecretNames    []string
		extractedConfigMapNames []string
		wantErr                 error
	}{
		{
			name: "TLS secrets",
			output: loggingv1.OutputSpec{
				Type: loggingv1.OutputTypeLokiStack,
				LokiStack: &loggingv1.LokiStack{
					Authentication: &loggingv1.LokiStackAuthentication{
						Token: &loggingv1.BearerToken{
							From:   loggingv1.BearerTokenFromSecret,
							Secret: &loggingv1.BearerTokenSecretKey{Name: "token-secret"},
						},
					},
				},
				TLS: &loggingv1.OutputTLSSpec{
					TLSSpec: loggingv1.TLSSpec{
						Certificate:   &loggingv1.ValueReference{SecretName: "cert-secret"},
						Key:           &loggingv1.SecretReference{SecretName: "key-secret"},
						CA:            &loggingv1.ValueReference{ConfigMapName: "ca-secret"},
						KeyPassphrase: &loggingv1.SecretReference{SecretName: "passphrase-secret"},
					},
				},
			},
			extractedSecretNames:    []string{"cert-secret", "key-secret", "passphrase-secret", "token-secret"},
			extractedConfigMapNames: []string{"ca-secret"},
		},
		{
			name: "Cloudwatch authentication secrets",
			output: loggingv1.OutputSpec{
				Type: loggingv1.OutputTypeCloudwatch,
				Cloudwatch: &loggingv1.Cloudwatch{
					Authentication: &loggingv1.CloudwatchAuthentication{
						Type: loggingv1.CloudwatchAuthTypeAccessKey,
						AWSAccessKey: &loggingv1.CloudwatchAWSAccessKey{
							KeyId:     loggingv1.SecretReference{SecretName: "key-id-secret"},
							KeySecret: loggingv1.SecretReference{SecretName: "key-secret"},
						},
					},
				},
			},
			extractedSecretNames: []string{"key-id-secret", "key-secret"},
		},
		{
			name: "Google Cloud Logging authentication secrets",
			output: loggingv1.OutputSpec{
				Type: loggingv1.OutputTypeGoogleCloudLogging,
				GoogleCloudLogging: &loggingv1.GoogleCloudLogging{
					Authentication: &loggingv1.GoogleCloudLoggingAuthentication{
						Credentials: &loggingv1.SecretReference{SecretName: "gcp-credentials-secret"},
					},
				},
			},
			extractedSecretNames: []string{"gcp-credentials-secret"},
		},
		{
			name: "Azure Monitor authentication secrets",
			output: loggingv1.OutputSpec{
				Type: loggingv1.OutputTypeAzureMonitor,
				AzureMonitor: &loggingv1.AzureMonitor{
					Authentication: &loggingv1.AzureMonitorAuthentication{
						SharedKey: &loggingv1.SecretReference{SecretName: "azure-shared-key-secret"},
					},
				},
			},
			extractedSecretNames: []string{"azure-shared-key-secret"},
		},
		{
			name: "Loki authentication secrets",
			output: loggingv1.OutputSpec{
				Type: loggingv1.OutputTypeLoki,
				Loki: &loggingv1.Loki{
					Authentication: &loggingv1.HTTPAuthentication{
						Username: &loggingv1.SecretReference{SecretName: "loki-username-secret"},
						Password: &loggingv1.SecretReference{SecretName: "loki-password-secret"},
					},
				},
			},
			extractedSecretNames: []string{"loki-username-secret", "loki-password-secret"},
		},
		{
			name: "Elasticsearch authentication secrets",
			output: loggingv1.OutputSpec{
				Type: loggingv1.OutputTypeElasticsearch,
				Elasticsearch: &loggingv1.Elasticsearch{
					Authentication: &loggingv1.HTTPAuthentication{
						Username: &loggingv1.SecretReference{SecretName: "es-username-secret"},
						Password: &loggingv1.SecretReference{SecretName: "es-password-secret"},
					},
				},
			},
			extractedSecretNames: []string{"es-username-secret", "es-password-secret"},
		},
		{
			name: "HTTP authentication secrets",
			output: loggingv1.OutputSpec{
				Type: loggingv1.OutputTypeHTTP,
				HTTP: &loggingv1.HTTP{
					Authentication: &loggingv1.HTTPAuthentication{
						Username: &loggingv1.SecretReference{SecretName: "http-username-secret"},
						Password: &loggingv1.SecretReference{SecretName: "http-password-secret"},
					},
				},
			},
			extractedSecretNames: []string{"http-username-secret", "http-password-secret"},
		},
		{
			name: "Kafka authentication secrets",
			output: loggingv1.OutputSpec{
				Type: loggingv1.OutputTypeKafka,
				Kafka: &loggingv1.Kafka{
					Authentication: &loggingv1.KafkaAuthentication{
						SASL: &loggingv1.SASLAuthentication{
							Username: &loggingv1.SecretReference{SecretName: "kafka-username-secret"},
							Password: &loggingv1.SecretReference{SecretName: "kafka-password-secret"},
						},
					},
				},
			},
			extractedSecretNames: []string{"kafka-username-secret", "kafka-password-secret"},
		},
		{
			name: "Splunk authentication secrets",
			output: loggingv1.OutputSpec{
				Type: loggingv1.OutputTypeSplunk,
				Splunk: &loggingv1.Splunk{
					Authentication: &loggingv1.SplunkAuthentication{
						Token: &loggingv1.SecretReference{SecretName: "splunk-token-secret"},
					},
				},
			},
			extractedSecretNames: []string{"splunk-token-secret"},
		},
		{
			name: "OTLP authentication secrets",
			output: loggingv1.OutputSpec{
				Type: loggingv1.OutputTypeOTLP,
				OTLP: &loggingv1.OTLP{
					Authentication: &loggingv1.HTTPAuthentication{
						Username: &loggingv1.SecretReference{SecretName: "otlp-username-secret"},
						Password: &loggingv1.SecretReference{SecretName: "otlp-password-secret"},
					},
				},
			},
			extractedSecretNames: []string{"otlp-username-secret", "otlp-password-secret"},
		},
		{
			name: "Unsupported output type",
			output: loggingv1.OutputSpec{
				Type: "unsupported",
			},
			wantErr: fmt.Errorf("%w: secretType: %s, outputName: %s", errMissingImplementation, "unsupported", ""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractedSecretsNames, extractedConfigMapNames, err := getOutputResourcesNames(tt.output)
			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.wantErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.ElementsMatch(t, tt.extractedSecretNames, extractedSecretsNames)
				assert.ElementsMatch(t, tt.extractedConfigMapNames, extractedConfigMapNames)
			}
		})
	}
}
