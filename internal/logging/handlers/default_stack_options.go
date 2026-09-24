package handlers

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"slices"

	routev1 "github.com/openshift/api/route/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const obsAPIRouteName = "mcoa-observatorium-api"

var (
	errObsAPIRouteHostMissing     = errors.New("obs-api route has no host yet")
	errObsAPIServerCertIncomplete = errors.New("obs-api server certificate secret has no tls.crt yet")
	errObsAPIServerCertHost       = errors.New("obs-api server certificate does not include route host yet")
)

func buildDefaultStackCollectionOptions(ctx context.Context, k8s client.Client, mcAddon *addonapiv1beta1.ManagedClusterAddOn, opts *manifests.Options) error {
	if !opts.DefaultStackEnabled() {
		return nil
	}

	// The hub withholds the CLF placement config until LokiStack is requested.
	// Until that reference shows up, skip collection so the rest of the addon
	// (metrics included) can still render.
	if len(common.GetObjectKeys(mcAddon.Status.ConfigReferences, loggingv1.GroupVersion.Group, addoncfg.ClusterLogForwardersResource)) == 0 {
		return nil
	}

	clf, err := common.GetResourceWithOwnerRef(ctx, k8s, mcAddon, loggingv1.GroupVersion.Group, addoncfg.ClusterLogForwardersResource, &loggingv1.ClusterLogForwarder{})
	if err != nil {
		return err
	}
	opts.DefaultStack.Collection.ClusterLogForwarder = clf

	mTLSSecret, err := common.GetSecret(ctx, k8s, clf.Namespace, mcAddon.Namespace, manifests.DefaultCollectionMTLSSecretName)
	if err != nil {
		return err
	}
	opts.DefaultStack.Collection.Secrets = []corev1.Secret{*mTLSSecret}

	lokiURL, err := obsAPILogsURL(ctx, k8s)
	if err != nil {
		return err
	}
	opts.DefaultStack.LokiURL = lokiURL

	return nil
}

// BuildObsAPIServerCertificate returns the hub obs-api server certificate.
// ready is false until the route has a host to put on the certificate, because
// collectors verify that host against this cert.
func BuildObsAPIServerCertificate(ctx context.Context, k8s client.Client) (client.Object, bool, error) {
	host, err := obsAPIRouteHost(ctx, k8s)
	if err != nil {
		return nil, false, err
	}
	cert, err := manifests.BuildSSAObsAPIServerCertificate(host)
	if err != nil {
		return nil, false, err
	}
	return cert, host != "", nil
}

// obsAPILogsURL is the spoke-reachable OTLP endpoint. Cluster-local Service DNS
// only resolves on the hub, so collectors use the obs-api route host.
func obsAPILogsURL(ctx context.Context, k8s client.Client) (string, error) {
	host, err := obsAPIRouteHost(ctx, k8s)
	if err != nil {
		return "", err
	}
	if host == "" {
		return "", fmt.Errorf("%w: %s/%s", errObsAPIRouteHostMissing, addoncfg.InstallNamespace, obsAPIRouteName)
	}
	if err := obsAPIServerCertIncludesHost(ctx, k8s, host); err != nil {
		return "", err
	}
	return fmt.Sprintf("https://%s/api/logs/v1/otlp/v1/logs", host), nil
}

func obsAPIRouteHost(ctx context.Context, k8s client.Client) (string, error) {
	route := &routev1.Route{}
	key := types.NamespacedName{Name: obsAPIRouteName, Namespace: addoncfg.InstallNamespace}
	if err := k8s.Get(ctx, key, route); err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to get obs-api route %s/%s: %w", key.Namespace, key.Name, err)
	}
	return route.Spec.Host, nil
}

func obsAPIServerCertIncludesHost(ctx context.Context, k8s client.Client, host string) error {
	secret := &corev1.Secret{}
	key := types.NamespacedName{Name: manifests.ObsAPIServerMTLSSecretName, Namespace: addoncfg.InstallNamespace}
	if err := k8s.Get(ctx, key, secret); err != nil {
		return fmt.Errorf("failed to get obs-api server certificate secret %s/%s: %w", key.Namespace, key.Name, err)
	}
	block, _ := pem.Decode(secret.Data[corev1.TLSCertKey])
	if block == nil {
		return fmt.Errorf("%w: %s/%s", errObsAPIServerCertIncomplete, key.Namespace, key.Name)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse obs-api server certificate %s/%s: %w", key.Namespace, key.Name, err)
	}
	if slices.Contains(cert.DNSNames, host) {
		return nil
	}
	return fmt.Errorf("%w: %s/%s host %s", errObsAPIServerCertHost, key.Namespace, key.Name, host)
}
