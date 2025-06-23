package handlers

import (
	"context"
	"errors"
	"fmt"

	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	fieldAuthentication = "authentication"
	fieldSASL           = "sasl"
)

var (
	errMissingCLFRef         = errors.New("missing ClusterLogForwarder reference on addon installation")
	errMultipleCLFRef        = errors.New("multiple ClusterLogForwarder references on addon installation")
	errMissingImplementation = errors.New("missing secret implementation for output type")
	errMissingField          = errors.New("missing field needed by output type")
)

func BuildOptions(ctx context.Context, k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, platform, userWorkloads addon.LogsOptions, isHub bool, hubHostname string) (manifests.Options, error) {
	opts := manifests.Options{
		Platform:      platform,
		UserWorkloads: userWorkloads,
		IsHub:         isHub,
		HubHostname:   hubHostname,
	}

	if platform.SubscriptionChannel != "" {
		opts.SubscriptionChannel = platform.SubscriptionChannel
	} else {
		opts.SubscriptionChannel = userWorkloads.SubscriptionChannel
	}

	if err := buildUnmagedOptions(ctx, k8s, mcAddon, &opts); err != nil {
		return opts, err
	}

	if err := buildDefaultStackOptions(ctx, k8s, mcAddon, &opts); err != nil {
		return opts, err
	}

	// Currently we are only able to access the cluster-logging subscription in the hub
	// since we don't have k8s clients for the spokes
	if isHub {
		subscription := &operatorv1alpha1.Subscription{}
		key := client.ObjectKey{Name: manifests.CloSubscriptionInstallName, Namespace: manifests.LoggingNamespace}
		if err := k8s.Get(ctx, key, subscription, &client.GetOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			return opts, fmt.Errorf("failed to get cluster-logging subscription: %w", err)
		}
		opts.ClusterLoggingSubscription = subscription
	}

	return opts, nil
}
