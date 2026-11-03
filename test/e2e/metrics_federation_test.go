//go:build e2e

package e2e

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var errMetricsHubHostnameNotFound = errors.New("metricsHubHostname variable not found in AddOnDeploymentConfig")

func TestMetricsFederation(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, clusterv1.Install(scheme))
	require.NoError(t, addonv1beta1.Install(scheme))
	require.NoError(t, workv1.Install(scheme))
	require.NoError(t, cooprometheusv1alpha1.AddToScheme(scheme))
	require.NoError(t, cooprometheusv1.AddToScheme(scheme))

	kubeconfigPath := os.Getenv("E2E_KUBECONFIG")
	if kubeconfigPath == "" {
		kubeconfigPath = os.Getenv("KUBECONFIG")
	}
	if kubeconfigPath == "" {
		kubeconfigPath = "/tmp/mcoa-e2e.kubeconfig"
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	require.NoError(t, err, "Failed to build config from kubeconfig %s", kubeconfigPath)

	k8sClient, err := client.New(config, client.Options{Scheme: scheme})
	require.NoError(t, err, "Failed to create k8s client")

	ctx, cancel := context.WithTimeout(t.Context(), 12*time.Minute)
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
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		require.NoError(t, err, "Failed to create ManagedClusterAddOn")
	}

	t.Log("=== Step 3: Asserting ManifestWork is created and applied by klusterlet ===")
	require.EventuallyWithT(t, func(c *assert.CollectT) {
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

	t.Log("=== Step 4: Asserting PrometheusAgent platform-metrics-collector is created with mTLS ===")
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		agent := &cooprometheusv1alpha1.PrometheusAgent{}
		agentErr := k8sClient.Get(ctx, client.ObjectKey{
			Namespace: "open-cluster-management-addon-observability",
			Name:      "platform-metrics-collector",
		}, agent)
		if !assert.NoError(c, agentErr, "PrometheusAgent 'platform-metrics-collector' not found in spoke namespace") {
			return
		}
		if !assert.NotEmpty(c, agent.Spec.RemoteWrite, "PrometheusAgent remoteWrite is empty") {
			return
		}
		rw := agent.Spec.RemoteWrite[0]
		assert.Equal(c, "https://hub-prometheus.open-cluster-management-observability.svc:8443/api/metrics/v1/default/api/v1/receive", string(rw.URL))
		assert.NotNil(c, rw.TLSConfig, "TLSConfig should be configured for mTLS")
	}, 2*time.Minute, 3*time.Second, "PrometheusAgent was not created in spoke namespace within timeout")

	t.Log("=== Step 4a: Asserting hub-mtls-ca secret contains Hub CA ===")
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		secretList := &corev1.SecretList{}
		if !assert.NoError(c, k8sClient.List(ctx, secretList, client.InNamespace("open-cluster-management-addon-observability"))) {
			return
		}
		var caSecret *corev1.Secret
		for i := range secretList.Items {
			if strings.HasPrefix(secretList.Items[i].Name, "hub-mtls-ca-") {
				caSecret = &secretList.Items[i]
				break
			}
		}
		if !assert.NotNil(c, caSecret, "Secret hub-mtls-ca-* not found in spoke namespace") {
			return
		}

		caPEM := caSecret.Data["ca.crt"]
		if !assert.NotEmpty(c, caPEM, "ca.crt is empty in hub-mtls-ca secret") {
			return
		}

		block, _ := pem.Decode(caPEM)
		if !assert.NotNil(c, block, "Failed to decode PEM block from ca.crt") {
			return
		}
		cert, parseErr := x509.ParseCertificate(block.Bytes)
		if !assert.NoError(c, parseErr, "Failed to parse certificate from ca.crt") {
			return
		}
		assert.Equal(c, "observability-ca", cert.Subject.CommonName)
	}, 2*time.Minute, 3*time.Second, "hub-mtls-ca secret with Hub CA was not found within timeout")

	t.Log("=== Step 4b: Asserting prom-agent-platform-metrics-collector pod is running and ready ===")
	require.EventuallyWithT(t, func(c *assert.CollectT) {
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

	t.Log("=== Step 5: Asserting Hub Prometheus receives federated metrics via direct mTLS from local-cluster ===")
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

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		queryURL := fmt.Sprintf("%s/api/v1/query?query=%s", hubURL, url.QueryEscape(`up{cluster="local-cluster",job="prometheus"} == 1`))
		resp, reqErr := httpClient.Get(queryURL)
		if !assert.NoError(c, reqErr, "HTTP GET to Hub Prometheus failed") {
			return
		}
		defer func() { _ = resp.Body.Close() }()

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

		if !assert.NotEmpty(c, pResp.Data.Result, "Prometheus has not received any healthy samples for up{cluster=\"local-cluster\"} == 1 yet") {
			return
		}

		t.Logf("Successfully queried Hub Prometheus! Sample: %+v", pResp.Data.Result[0])
	}, 3*time.Minute, 5*time.Second, "Hub Prometheus did not receive healthy federated metrics (up == 1) from local-cluster within timeout")

	t.Run("HTTPSProxyWithMTLS", func(t *testing.T) {
		t.Log("=== Step 6: Validating HTTPS Proxy with mTLS ===")

		// 1. Read Proxy CA from e2e-proxy-ca ConfigMap
		proxyCAConfigMap := &corev1.ConfigMap{}
		err := k8sClient.Get(ctx, client.ObjectKey{
			Namespace: "open-cluster-management-observability",
			Name:      "e2e-proxy-ca",
		}, proxyCAConfigMap)
		require.NoError(t, err, "Failed to get e2e-proxy-ca ConfigMap")
		proxyCABundle := []byte(proxyCAConfigMap.Data["ca.crt"])
		require.NotEmpty(t, proxyCABundle, "Proxy CA bundle must not be empty")

		// 2. Update AddOnDeploymentConfig to point to mTLS port (:8443) via HTTPS forward proxy
		const expectedTargetHost = "hub-prometheus.open-cluster-management-observability.svc:8443"
		const expectedProxyURL = "https://https-proxy.open-cluster-management-observability.svc:8443"
		const expectedNoProxy = "localhost,127.0.0.1"
		const expectedTargetURL = "https://" + expectedTargetHost + "/api/metrics/v1/default/api/v1/receive"

		err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			aodc := &addonv1beta1.AddOnDeploymentConfig{}
			if getErr := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: "open-cluster-management-observability",
				Name:      "multicluster-observability-addon",
			}, aodc); getErr != nil {
				return getErr
			}
			foundMetricsHubHostname := false
			for i := range aodc.Spec.CustomizedVariables {
				if aodc.Spec.CustomizedVariables[i].Name == "metricsHubHostname" {
					aodc.Spec.CustomizedVariables[i].Value = "https://" + expectedTargetHost
					foundMetricsHubHostname = true
				}
			}
			if !foundMetricsHubHostname {
				return errMetricsHubHostnameNotFound
			}
			aodc.Spec.ProxyConfig = addonv1beta1.ProxyConfig{
				HTTPSProxy: expectedProxyURL,
				NoProxy:    expectedNoProxy,
				CABundle:   proxyCABundle,
			}
			return k8sClient.Update(ctx, aodc)
		})
		require.NoError(t, err, "Failed to update AddOnDeploymentConfig with proxyConfig")

		// 3. Assert Control-Plane: Secret hub-mtls-ca-<trimmedID> contains merged Hub CA + Proxy CA
		t.Log("=== Step 6a: Asserting hub-mtls-ca secret contains merged CAs ===")
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			secretList := &corev1.SecretList{}
			if !assert.NoError(c, k8sClient.List(ctx, secretList, client.InNamespace("open-cluster-management-addon-observability"))) {
				return
			}
			var caSecret *corev1.Secret
			for i := range secretList.Items {
				if strings.HasPrefix(secretList.Items[i].Name, "hub-mtls-ca-") {
					caSecret = &secretList.Items[i]
					break
				}
			}
			if !assert.NotNil(c, caSecret, "Secret hub-mtls-ca-* not found in spoke namespace") {
				return
			}

			caPEM := caSecret.Data["ca.crt"]
			if !assert.NotEmpty(c, caPEM, "ca.crt is empty in hub-mtls-ca secret") {
				return
			}

			var commonNames []string
			rest := caPEM
			for {
				var block *pem.Block
				block, rest = pem.Decode(rest)
				if block == nil {
					break
				}
				if block.Type == "CERTIFICATE" {
					cert, parseErr := x509.ParseCertificate(block.Bytes)
					if parseErr == nil {
						commonNames = append(commonNames, cert.Subject.CommonName)
					}
				}
			}
			assert.Contains(c, commonNames, "observability-ca", "Merged secret must contain Hub CA")
			assert.Contains(c, commonNames, "e2e-proxy-ca", "Merged secret must contain Proxy CA")
		}, 2*time.Minute, 3*time.Second, "hub-mtls-ca secret was not updated with merged CAs")

		// 4. Assert Control-Plane: PrometheusAgent platform-metrics-collector spec is updated
		t.Log("=== Step 6b: Asserting PrometheusAgent spec is updated with httpsProxy and https URL ===")
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			agent := &cooprometheusv1alpha1.PrometheusAgent{}
			if !assert.NoError(c, k8sClient.Get(ctx, client.ObjectKey{
				Namespace: "open-cluster-management-addon-observability",
				Name:      "platform-metrics-collector",
			}, agent)) {
				return
			}
			if !assert.NotEmpty(c, agent.Spec.RemoteWrite, "remoteWrite spec is empty") {
				return
			}
			rw := agent.Spec.RemoteWrite[0]
			assert.Equal(c, expectedTargetURL, string(rw.URL))
			if assert.NotNil(c, rw.ProxyURL, "proxyURL is nil") {
				assert.Equal(c, expectedProxyURL, *rw.ProxyURL)
			}
			if assert.NotNil(c, rw.NoProxy, "noProxy is nil") {
				assert.Equal(c, expectedNoProxy, string(*rw.NoProxy))
			}
		}, 2*time.Minute, 3*time.Second, "PrometheusAgent spec was not updated within timeout")

		// 5. Assert Data-Plane: HTTPS forward proxy receives CONNECT tunnel requests and transfers bytes
		t.Log("=== Step 6c: Asserting HTTPS forward proxy receives CONNECT tunnel requests ===")
		proxyStatsURL := os.Getenv("PROXY_STATS_URL")
		if proxyStatsURL == "" {
			proxyStatsURL = "http://127.0.0.1:18080/stats"
		}
		var proxyActiveTime int64
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			resp, reqErr := httpClient.Get(proxyStatsURL)
			if !assert.NoError(c, reqErr, "HTTP GET to proxy /stats failed") {
				return
			}
			defer func() { _ = resp.Body.Close() }()

			var stats struct {
				ConnectCount map[string]uint64 `json:"connectCount"`
				BytesCopied  uint64            `json:"bytesCopied"`
			}
			if !assert.NoError(c, json.NewDecoder(resp.Body).Decode(&stats)) {
				return
			}
			assert.GreaterOrEqual(c, stats.ConnectCount[expectedTargetHost], uint64(1),
				"Expected HTTPS proxy to record CONNECT tunnel to %s", expectedTargetHost)
			assert.Positive(c, stats.BytesCopied,
				"Expected HTTPS proxy to transfer mTLS remote-write bytes")
			proxyActiveTime = time.Now().Unix()
		}, 2*time.Minute, 3*time.Second, "HTTPS proxy did not record CONNECT tunnel or bytes transferred")

		// 6. Assert Data-Plane: Hub Prometheus receives fresh federated metrics via mTLS over tunnel
		t.Log("=== Step 6d: Asserting Hub Prometheus receives fresh federated metrics via mTLS ===")
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			queryURL := fmt.Sprintf("%s/api/v1/query?query=%s", hubURL, url.QueryEscape(`timestamp(up{cluster="local-cluster",job="prometheus"} == 1)`))
			resp, reqErr := httpClient.Get(queryURL)
			if !assert.NoError(c, reqErr, "HTTP GET to Hub Prometheus failed") {
				return
			}
			defer func() { _ = resp.Body.Close() }()

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

			if !assert.NotEmpty(c, pResp.Data.Result, "Prometheus has no result for query") {
				return
			}

			valSlice := pResp.Data.Result[0].Value
			if !assert.Len(c, valSlice, 2, "Expected Value slice to have 2 elements [evalTime, sampleTimeStr]") {
				return
			}
			sampleTimeStr, ok := valSlice[1].(string)
			if !assert.True(c, ok, "Expected timestamp value to be string, got %T", valSlice[1]) {
				return
			}
			sampleTime, parseErr := strconv.ParseFloat(sampleTimeStr, 64)
			if !assert.NoError(c, parseErr, "Failed to parse sample timestamp") {
				return
			}
			// Require the sample timestamp to be at or after proxyActiveTime, ensuring that
			// the sample was ingested after the proxy tunnel was active and PrometheusAgent had reloaded.
			assert.GreaterOrEqual(c, int64(sampleTime), proxyActiveTime,
				"Expected fresh metric sample delivered after proxy tunnel was established (sample: %v, proxyActive: %v)", int64(sampleTime), proxyActiveTime)
		}, 3*time.Minute, 5*time.Second, "Hub Prometheus did not receive fresh federated metrics over mTLS tunnel within timeout")
	})
}
