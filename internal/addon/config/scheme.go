package config

import (
	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	persesv1 "github.com/perses/perses-operator/api/v1alpha1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	monitoringv1alpha1 "github.com/rhobs/observability-operator/pkg/apis/monitoring/v1alpha1"
	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	thanosv1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	workv1 "open-cluster-management.io/api/work/v1"
)

// AddToScheme adds all types needed by MCOA to the provided scheme.
func AddToScheme(s *runtime.Scheme) error {
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(workv1.Install(s))
	utilruntime.Must(loggingv1.AddToScheme(s))
	utilruntime.Must(otelv1beta1.AddToScheme(s))
	utilruntime.Must(otelv1alpha1.AddToScheme(s))
	utilruntime.Must(operatorsv1.AddToScheme(s))
	utilruntime.Must(operatorsv1alpha1.AddToScheme(s))
	utilruntime.Must(clusterv1.Install(s))
	utilruntime.Must(clusterv1beta1.Install(s))
	utilruntime.Must(operatorv1.AddToScheme(s))
	utilruntime.Must(cooprometheusv1.AddToScheme(s))
	utilruntime.Must(cooprometheusv1alpha1.AddToScheme(s)) // Adds prometheusAgent and scrapeConfig
	utilruntime.Must(monitoringv1alpha1.AddToScheme(s))    // Adds monitoringstack
	utilruntime.Must(prometheusv1.AddToScheme(s))          // Adds prometheusRule
	utilruntime.Must(uiplugin.AddToScheme(s))
	utilruntime.Must(hyperv1.AddToScheme(s))
	utilruntime.Must(persesv1.AddToScheme(s))
	utilruntime.Must(routev1.AddToScheme(s))
	utilruntime.Must(addonv1alpha1.Install(s))
	utilruntime.Must(addonv1beta1.Install(s))
	utilruntime.Must(thanosv1alpha1.AddToScheme(s))
	utilruntime.Must(configv1.AddToScheme(s))
	// +kubebuilder:scaffold:scheme
	return nil
}

// NewScheme returns a complete runtime.Scheme populated with all types needed by MCOA.
func NewScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = AddToScheme(s)
	return s
}
