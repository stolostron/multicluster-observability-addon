// Package tlsprofile provides a controller that watches the cluster's
// APIServer TLS security profile and restarts the process when it changes so
// the manager can rebuild its serving TLS configuration from the new profile.
package tlsprofile

import (
	"context"
	"fmt"
	"reflect"

	configv1 "github.com/openshift/api/config/v1"
	tlsutil "github.com/openshift/controller-runtime-common/pkg/tls"
	libgocrypto "github.com/openshift/library-go/pkg/crypto"
	crdClientSet "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	// apiServerName is the name of the singleton cluster APIServer resource.
	apiServerName = "cluster"
	// ocpAPIServerCRDName is the CRD backing the APIServer resource. On
	// non-OpenShift clusters it is absent, so the watcher is skipped.
	ocpAPIServerCRDName = "apiservers.config.openshift.io"
)

// SetupWithManager registers the TLS profile watcher with the manager. It is a
// no-op on clusters without the OpenShift APIServer CRD.
func SetupWithManager(ctx context.Context, mgr ctrl.Manager, kubeConfig *rest.Config, initial configv1.TLSProfileSpec, restart context.CancelFunc) error {
	if !apiServerCRDExists(ctx, kubeConfig) {
		ctrl.Log.Info("APIServer CRD not found, skipping TLS security profile watcher", "crd", ocpAPIServerCRDName)
		return nil
	}
	r := NewReconciler(mgr.GetClient(), initial, restart)
	return ctrl.NewControllerManagedBy(mgr).
		Named("tls-profile-watcher").
		For(&configv1.APIServer{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			return obj.GetName() == apiServerName
		}))).
		Complete(r)
}

func apiServerCRDExists(ctx context.Context, kubeConfig *rest.Config) bool {
	crdClient, err := crdClientSet.NewForConfig(kubeConfig)
	if err != nil {
		ctrl.Log.Error(err, "failed to create CRD client")
		return false
	}
	_, err = crdClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, ocpAPIServerCRDName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false
	}
	if err != nil {
		ctrl.Log.Error(err, "failed to check APIServer CRD")
		return false
	}
	return true
}

// Reconciler restarts the process when the cluster TLS profile changes so the
// manager can rebuild its serving TLS configuration from the new profile.
type Reconciler struct {
	client  client.Client
	initial configv1.TLSProfileSpec
	restart context.CancelFunc
}

// NewReconciler builds a Reconciler that compares observed profiles against
// initial and calls restart when they diverge.
func NewReconciler(c client.Client, initial configv1.TLSProfileSpec, restart context.CancelFunc) *Reconciler {
	return &Reconciler{client: c, initial: initial, restart: restart}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.Name != apiServerName {
		return ctrl.Result{}, nil
	}
	apiServer := &configv1.APIServer{}
	if err := r.client.Get(ctx, req.NamespacedName, apiServer); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	profile := configv1.TLSProfiles[libgocrypto.DefaultTLSProfileType]
	if libgocrypto.ShouldHonorClusterTLSProfile(apiServer.Spec.TLSAdherence) {
		configured, err := tlsutil.GetTLSProfileSpec(apiServer.Spec.TLSSecurityProfile)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to read changed TLS profile: %w", err)
		}
		profile = &configured
	}

	if !reflect.DeepEqual(r.initial, *profile) {
		ctrl.Log.Info("TLS profile changed, shutting down to reload", "oldProfile", r.initial, "newProfile", *profile)
		r.restart()
	}
	return ctrl.Result{}, nil
}
