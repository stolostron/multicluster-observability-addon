package privacy

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceAccountName is the dedicated SA for the prediction engine.
func ServiceAccountName() string {
	return "rs-prediction-sa"
}

// ClusterRoleName is the cluster role bound to the prediction service account.
func ClusterRoleName() string {
	return "rs-prediction-role"
}

// GenerateServiceAccount returns a ServiceAccount for the prediction workload.
func GenerateServiceAccount(namespace string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServiceAccountName(),
			Namespace: namespace,
		},
	}
}

// GenerateClusterRole returns rules for reading config data and PrometheusRules.
// Secret access should be bound only in the prediction namespace via a RoleBinding.
func GenerateClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: ClusterRoleName(),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"monitoring.coreos.com"},
				Resources: []string{"prometheusrules"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
}
