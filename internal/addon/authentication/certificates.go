package authentication

import (
	"context"

	"github.com/ViaQ/logerr/v2/kverrors"
	"github.com/rhobs/multicluster-observability-addon/internal/manifests"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateOrUpdateRootCertificate(k8s client.Client) error {
	ctx := context.Background()

	err := checkCertManagerCRDs(ctx, k8s)
	if err != nil {
		return err
	}

	objects := manifests.BuildAllRootCertificate()

	for _, obj := range objects {
		desired := obj.DeepCopyObject().(client.Object)
		mutateFn := manifests.MutateFuncFor(obj, desired, nil)

		klog.Infof("Trying to create/update the %s resource", obj.GetName())

		op, err := ctrl.CreateOrUpdate(ctx, k8s, obj, mutateFn)
		if err != nil {
			klog.Error(err, "failed to configure resource")
			continue
		}

		klog.Infof("Resource %s has been %s",obj.GetName(), op)
	}

	return nil
}

func checkCertManagerCRDs(ctx context.Context, k8s client.Client) error {
	for _, crdName := range certManagerCRDs {
		key := client.ObjectKey{Name: crdName}
		crd := &apiextensionsv1.CustomResourceDefinition{}
		err := k8s.Get(ctx, key, crd, &client.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return kverrors.New("cert-manager CRD is missing", "name", crdName)
			}
			return err
		}
	}

	return nil
}
