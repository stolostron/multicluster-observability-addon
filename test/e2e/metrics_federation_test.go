//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestMetricsFederation(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, clusterv1.Install(scheme))
	require.NoError(t, addonv1beta1.Install(scheme))
	require.NoError(t, workv1.Install(scheme))
	require.NoError(t, cooprometheusv1alpha1.AddToScheme(scheme))
	require.NoError(t, cooprometheusv1.AddToScheme(scheme))

	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		kubeconfigPath = "/tmp/mcoa-e2e.kubeconfig"
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	require.NoError(t, err, "Failed to build config from kubeconfig %s", kubeconfigPath)

	k8sClient, err := client.New(config, client.Options{Scheme: scheme})
	require.NoError(t, err, "Failed to create k8s client")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	t.Log("=== Step 1: Validating prerequisites on Hub ===")
	managedCluster := &clusterv1.ManagedCluster{}
	err = k8sClient.Get(ctx, client.ObjectKey{Name: "local-cluster"}, managedCluster)
	require.NoError(t, err, "ManagedCluster 'local-cluster' must exist")
	require.Equal(t, "OpenShift", managedCluster.Labels["vendor"], "ManagedCluster vendor must be OpenShift")

	cmao := &addonv1beta1.ClusterManagementAddOn{}
	err = k8sClient.Get(ctx, client.ObjectKey{Name: "multicluster-observability-addon"}, cmao)
	require.NoError(t, err, "ClusterManagementAddOn 'multicluster-observability-addon' must exist")

	aodc := &addonv1beta1.AddOnDeploymentConfig{}
	err = k8sClient.Get(ctx, client.ObjectKey{
		Namespace: "open-cluster-management-observability",
		Name:      "multicluster-observability-addon",
	}, aodc)
	require.NoError(t, err, "AddOnDeploymentConfig must exist in open-cluster-management-observability")

	t.Log("=== Step 2: Creating ManagedClusterAddOn for local-cluster ===")
	mca := &addonv1beta1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multicluster-observability-addon",
			Namespace: "local-cluster",
		},
	}
	err = k8sClient.Create(ctx, mca)
	if err != nil && !errors.IsAlreadyExists(err) {
		require.NoError(t, err, "Failed to create ManagedClusterAddOn")
	}

	t.Log("=== Step 3: Asserting ManifestWork is created and applied by klusterlet ===")
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		mwList := &workv1.ManifestWorkList{}
		listErr := k8sClient.List(ctx, mwList, client.InNamespace("local-cluster"), client.MatchingLabels{
			addonv1beta1.AddonLabelKey: "multicluster-observability-addon",
		})
		if !assert.NoError(c, listErr, "Failed to list ManifestWorks in local-cluster") {
			return
		}
		if !assert.NotEmpty(c, mwList.Items, "No ManifestWork found yet for addon in local-cluster") {
			return
		}

		applied := false
		for _, mw := range mwList.Items {
			for _, cond := range mw.Status.Conditions {
				if cond.Type == workv1.WorkApplied && cond.Status == metav1.ConditionTrue {
					applied = true
					break
				}
			}
		}
		assert.True(c, applied, "Expected at least one ManifestWork to have Condition Applied=True")
	}, 2*time.Minute, 3*time.Second, "ManifestWork was not applied in local-cluster within timeout")

	t.Log("=== Step 4: Asserting PrometheusAgent platform-metrics-collector is created ===")
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		agent := &cooprometheusv1alpha1.PrometheusAgent{}
		agentErr := k8sClient.Get(ctx, client.ObjectKey{
			Namespace: "open-cluster-management-addon-observability",
			Name:      "platform-metrics-collector",
		}, agent)
		assert.NoError(c, agentErr, "PrometheusAgent 'platform-metrics-collector' not found in spoke namespace")
	}, 2*time.Minute, 3*time.Second, "PrometheusAgent was not created in spoke namespace within timeout")

	t.Log("=== Step 4b: Asserting prom-agent-platform-metrics-collector pod is running and ready ===")
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		pod := &corev1.Pod{}
		podErr := k8sClient.Get(ctx, client.ObjectKey{
			Namespace: "open-cluster-management-addon-observability",
			Name:      "prom-agent-platform-metrics-collector-0",
		}, pod)
		if !assert.NoError(c, podErr, "Pod 'prom-agent-platform-metrics-collector-0' not found yet") {
			return
		}
		ready := false
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				ready = true
				break
			}
		}
		assert.True(c, ready, "Pod 'prom-agent-platform-metrics-collector-0' is not ready yet (phase: %s)", pod.Status.Phase)
	}, 3*time.Minute, 3*time.Second, "prom-agent-platform-metrics-collector-0 did not reach Ready state within timeout")

	t.Log("=== Step 5: Asserting Hub Prometheus receives federated metrics from local-cluster ===")
	hubURL := os.Getenv("HUB_PROMETHEUS_URL")
	if hubURL == "" {
		hubURL = "http://127.0.0.1:9090"
	}
	httpClient := &http.Client{Timeout: 5 * time.Second}

	type PromResponse struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Metric map[string]string `json:"metric"`
				Value  []any             `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		queryURL := fmt.Sprintf("%s/api/v1/query?query=%s", hubURL, url.QueryEscape(`up{cluster="local-cluster"}`))
		resp, reqErr := httpClient.Get(queryURL)
		if !assert.NoError(c, reqErr, "HTTP GET to Hub Prometheus failed") {
			return
		}
		defer resp.Body.Close()

		body, readErr := io.ReadAll(resp.Body)
		if !assert.NoError(c, readErr, "Failed to read response body") {
			return
		}

		if !assert.Equal(c, http.StatusOK, resp.StatusCode, "Hub Prometheus returned status %d: %s", resp.StatusCode, string(body)) {
			return
		}

		var pResp PromResponse
		if !assert.NoError(c, json.Unmarshal(body, &pResp), "Failed to unmarshal Prometheus response JSON") {
			return
		}

		if !assert.Equal(c, "success", pResp.Status, "Prometheus response status is not success") {
			return
		}

		if !assert.NotEmpty(c, pResp.Data.Result, "Prometheus has not received any samples for up{cluster=\"local-cluster\"} yet") {
			return
		}

		t.Logf("Successfully queried Hub Prometheus! Sample: %+v", pResp.Data.Result[0])
	}, 3*time.Minute, 5*time.Second, "Hub Prometheus did not receive federated metrics from local-cluster within timeout")
}
