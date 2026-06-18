package common

import (
	"context"
	"errors"
	"fmt"

	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	errMissingResource     = errors.New("resource referenced in ManagedClusterAddOn config not found")
	errMissingResourceRefs = errors.New("no references to the resource found in ManagedClusterAddOn config")
	errMissingOwnerRef     = errors.New("no resource owned by MCOA found in references")
)

func GetResourceWithOwnerRef[T client.Object](
	ctx context.Context,
	k8s client.Client,
	mcAddon *addonv1beta1.ManagedClusterAddOn,
	group, resource string,
	obj T,
) (T, error) {
	keys := GetObjectKeys(mcAddon.Status.ConfigReferences, group, resource)
	if len(keys) == 0 {
		return obj, fmt.Errorf("%w: %s/%s", errMissingResourceRefs, group, resource)
	}

	cmao := &addonv1beta1.ClusterManagementAddOn{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterManagementAddOn",
			APIVersion: addonv1beta1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: addoncfg.Name,
		},
	}

	for _, key := range keys {
		tempObj := obj.DeepCopyObject().(T)
		if err := k8s.Get(ctx, key, tempObj, &client.GetOptions{}); err != nil {
			if k8serrors.IsNotFound(err) {
				return obj, fmt.Errorf("%w: %s/%s %s/%s", errMissingResource, group, resource, key.Namespace, key.Name)
			}
			return obj, err
		}

		hasOwnerRef, err := controllerutil.HasOwnerReference(tempObj.GetOwnerReferences(), cmao, k8s.Scheme())
		if err != nil {
			continue
		}

		if hasOwnerRef {
			obj = tempObj
			break
		}
	}

	if obj.GetName() == "" {
		return obj, fmt.Errorf("%w: group=%s, resource=%s", errMissingOwnerRef, group, resource)
	}

	return obj, nil
}
