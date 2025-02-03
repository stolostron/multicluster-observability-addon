package addon

import (
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetResourcesByLabelSelector returns the list of resources of type T matching the label selector.
// It works generically for any Kubernetes resource that implements client.Object.
func GetResourcesByLabelSelector[T client.Object](resources []client.Object, selector map[string]string) []T {
	labelSelector := labels.SelectorFromSet(selector)
	ret := []T{}

	for _, obj := range resources {
		if resource, ok := obj.(T); ok {
			if labelSelector.Matches(labels.Set(resource.GetLabels())) {
				ret = append(ret, resource)
			}
		}
	}

	return ret
}
