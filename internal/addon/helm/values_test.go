package helm

import (
	"testing"

	"github.com/go-logr/logr"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager/addontesting"
	"open-cluster-management.io/addon-framework/pkg/agent"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	_ = loggingv1.AddToScheme(scheme.Scheme)
	_ = operatorsv1.AddToScheme(scheme.Scheme)
	_ = operatorsv1alpha1.AddToScheme(scheme.Scheme)
	_ = addonapiv1alpha1.AddToScheme(scheme.Scheme)
	_ = apiextensionsv1.AddToScheme(scheme.Scheme)
	_ = uiplugin.AddToScheme(scheme.Scheme)
)

func Test_Supported_Vendors(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		managedClusterLabels  map[string]string
		addonDeploymentConfig []addonapiv1alpha1.CustomizedVariable
		expectedObjects       bool
	}{
		{
			name: "ManagedCluster with correct labels but no configuration",
			managedClusterLabels: map[string]string{
				"vendor": "OpenShift",
			},
			addonDeploymentConfig: []addonapiv1alpha1.CustomizedVariable{},
			expectedObjects:       false,
		},
		{
			name: "ManagedCluster with correct labels and platform log configuration",
			managedClusterLabels: map[string]string{
				"vendor": "OpenShift",
			},
			addonDeploymentConfig: []addonapiv1alpha1.CustomizedVariable{
				{
					Name:  addon.KeyPlatformLogsCollection,
					Value: string(addon.ClusterLogForwarderV1),
				},
			},
			expectedObjects: true,
		},
		{
			name: "ManagedCluster with unsupported kube vendor",
			managedClusterLabels: map[string]string{
				"vendor": "foo",
			},
			addonDeploymentConfig: []addonapiv1alpha1.CustomizedVariable{
				{
					Name:  addon.KeyPlatformLogsCollection,
					Value: string(addon.ClusterLogForwarderV1),
				},
			},
			expectedObjects: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var (
				managedCluster        *clusterv1.ManagedCluster
				managedClusterAddOn   *addonapiv1alpha1.ManagedClusterAddOn
				addOnDeploymentConfig *addonapiv1alpha1.AddOnDeploymentConfig
			)

			managedCluster = addontesting.NewManagedCluster("cluster-1")
			managedCluster.Labels = tc.managedClusterLabels
			managedClusterAddOn = addontesting.NewAddon("test", "cluster-1")

			managedClusterAddOn.Status.ConfigReferences = []addonapiv1alpha1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
						Group:    "addon.open-cluster-management.io",
						Resource: "addondeploymentconfigs",
					},
					ConfigReferent: addonapiv1alpha1.ConfigReferent{
						Name:      "multicluster-observability-addon",
						Namespace: "open-cluster-management-observability",
					},
				},
				{
					ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
						Group:    "observability.openshift.io",
						Resource: "clusterlogforwarders",
					},
					ConfigReferent: addonapiv1alpha1.ConfigReferent{
						Namespace: "open-cluster-management-observability",
						Name:      "mcoa-instance",
					},
				},
			}

			addOnDeploymentConfig = &addonapiv1alpha1.AddOnDeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multicluster-observability-addon",
					Namespace: "open-cluster-management-observability",
				},
				Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
					CustomizedVariables: tc.addonDeploymentConfig,
				},
			}

			clf := &loggingv1.ClusterLogForwarder{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mcoa-instance",
					Namespace: "open-cluster-management-observability",
				},
				Spec: loggingv1.ClusterLogForwarderSpec{
					Inputs: []loggingv1.InputSpec{
						{
							Name:           "infra-logs",
							Infrastructure: &loggingv1.Infrastructure{},
						},
					},
					Outputs: []loggingv1.OutputSpec{
						{
							Name: "cluster-logs",
							Type: loggingv1.OutputTypeCloudwatch,
							Cloudwatch: &loggingv1.Cloudwatch{
								Authentication: &loggingv1.CloudwatchAuthentication{
									Type: loggingv1.CloudwatchAuthTypeAccessKey,
									AWSAccessKey: &loggingv1.CloudwatchAWSAccessKey{
										KeyId: loggingv1.SecretReference{
											SecretName: "static-authentication",
											Key:        "key",
										},
										KeySecret: loggingv1.SecretReference{
											SecretName: "static-authentication",
											Key:        "pass",
										},
									},
								},
							},
						},
					},
					Pipelines: []loggingv1.PipelineSpec{
						{
							Name:       "cluster-logs",
							InputRefs:  []string{"infra-logs", string(loggingv1.InputTypeAudit)},
							OutputRefs: []string{"cluster-logs"},
						},
					},
				},
			}

			staticCred := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "static-authentication",
					Namespace: "open-cluster-management-observability",
				},
				Data: map[string][]byte{
					"key":  []byte("data"),
					"pass": []byte("data"),
				},
			}

			fakeKubeClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(addOnDeploymentConfig, clf, staticCred).
				Build()

			loggingAgentAddon, err := addonfactory.NewAgentAddonFactory(addoncfg.Name, addon.FS, addoncfg.McoaChartDir).
				WithGetValuesFuncs(GetValuesFunc(t.Context(), fakeKubeClient, logr.Discard())).
				WithAgentRegistrationOption(&agent.RegistrationOption{}).
				WithScheme(scheme.Scheme).
				BuildHelmAgentAddon()
			if err != nil {
				klog.Fatalf("failed to build agent %v", err)
			}

			objects, err := loggingAgentAddon.Manifests(managedCluster, managedClusterAddOn)
			require.NoError(t, err)
			if tc.expectedObjects {
				require.NotEmpty(t, objects)
			} else {
				require.Empty(t, objects)
			}
		})
	}
}
