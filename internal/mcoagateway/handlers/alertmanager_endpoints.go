package handlers

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	alertmanagerOperatedService = "alertmanager-operated"
	alertmanagerEndpointPort    = 9095
)

var errMultipleAlertmanagerStatefulSets = errors.New("multiple StatefulSets use Alertmanager Service in namespace")

// BuildAlertmanagerEndpoints derives one stable pod URL per desired Alertmanager
// replica from the StatefulSet that uses the alertmanager-operated Service.
func BuildAlertmanagerEndpoints(ctx context.Context, k8s client.Client) ([]string, error) {
	statefulSetList := &appsv1.StatefulSetList{}
	if err := k8s.List(ctx, statefulSetList,
		client.InNamespace(mconfig.HubInstallNamespace),
	); err != nil {
		return nil, fmt.Errorf("listing StatefulSets: %w", err)
	}

	var alertmanager *appsv1.StatefulSet
	for i := range statefulSetList.Items {
		statefulSet := &statefulSetList.Items[i]
		if statefulSet.Spec.ServiceName != alertmanagerOperatedService {
			continue
		}
		if alertmanager != nil {
			return nil, fmt.Errorf("%w %q: %q", errMultipleAlertmanagerStatefulSets, alertmanagerOperatedService, mconfig.HubInstallNamespace)
		}
		alertmanager = statefulSet
	}

	if alertmanager == nil {
		return []string{}, nil
	}

	replicas := int32(1)
	if alertmanager.Spec.Replicas != nil {
		replicas = *alertmanager.Spec.Replicas
	}

	urls := make([]string, 0, replicas)
	for ordinal := range replicas {
		urls = append(urls, "https://"+alertmanager.Name+"-"+strconv.Itoa(int(ordinal))+"."+
			alertmanagerOperatedService+"."+alertmanager.Namespace+".svc:"+
			strconv.Itoa(alertmanagerEndpointPort))
	}

	return urls, nil
}
