package manifests

import (
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// LokiOperatorNamespace is the namespace Loki Operator is installed into via OLM. Loki
	// Operator is a global (cluster-scoped) operator, so it follows the same convention as other
	// Red Hat cluster-scoped operators and lives in openshift-operators-redhat.
	LokiOperatorNamespace = "openshift-operators-redhat"

	// LokiOperatorPackageName is the OLM package/Subscription name for Loki Operator.
	LokiOperatorPackageName = "loki-operator"

	// LokiOperatorChannel is the default subscription channel used when installing Loki
	// Operator.
	LokiOperatorChannel = "stable-6.3"

	// LokiOperatorCatalogSource and LokiOperatorCatalogSourceNamespace point at the default
	// Red Hat operator catalog.
	LokiOperatorCatalogSource          = "redhat-operators"
	LokiOperatorCatalogSourceNamespace = "openshift-marketplace"

	// LokiStackCRDName is the CustomResourceDefinition Loki Operator registers once installed.
	// MCOA waits for this CRD to be Established before creating any LokiStack custom resource,
	// regardless of whether it was MCOA or an admin that installed the operator.
	LokiStackCRDName = "lokistacks.loki.grafana.com"
)

// BuildLokiOperatorResources returns the Namespace, OperatorGroup and Subscription objects
// required to install Loki Operator via OLM.
//
// These objects are applied directly by the resourcecreator controller via Server-Side Apply
// against the hub's own API server, the same way ClusterLogForwarder/LokiStack/Certificate
// objects are, rather than being rendered through the addon Helm chart and shipped via
// ManifestWork. This keeps the operator's lifecycle independent of any ManagedClusterAddOn/Helm
// rendering path, since Loki Operator only ever needs to run on the hub.
func BuildLokiOperatorResources() []client.Object {
	ns := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: LokiOperatorNamespace,
			Labels: map[string]string{
				"openshift.io/cluster-monitoring": "true",
			},
		},
	}

	og := &operatorsv1.OperatorGroup{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OperatorGroup",
			APIVersion: operatorsv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      LokiOperatorPackageName,
			Namespace: LokiOperatorNamespace,
		},
		Spec: operatorsv1.OperatorGroupSpec{
			UpgradeStrategy: operatorsv1.UpgradeStrategyDefault,
		},
	}

	sub := &operatorsv1alpha1.Subscription{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Subscription",
			APIVersion: operatorsv1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      LokiOperatorPackageName,
			Namespace: LokiOperatorNamespace,
			Labels: map[string]string{
				"operators.coreos.com/" + LokiOperatorPackageName + "." + LokiOperatorNamespace: "",
			},
		},
		Spec: &operatorsv1alpha1.SubscriptionSpec{
			CatalogSource:          LokiOperatorCatalogSource,
			CatalogSourceNamespace: LokiOperatorCatalogSourceNamespace,
			Package:                LokiOperatorPackageName,
			Channel:                LokiOperatorChannel,
			InstallPlanApproval:    operatorsv1alpha1.ApprovalAutomatic,
		},
	}

	return []client.Object{ns, og, sub}
}
