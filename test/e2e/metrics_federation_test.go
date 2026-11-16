//go:build e2e

package e2e

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	hubNamespace   = "open-cluster-management-observability"
	spokeNamespace = "open-cluster-management-addon-observability"
	uwmNamespace   = "openshift-user-workload-monitoring"
	addonName      = "multicluster-observability-addon"
	clusterName    = "local-cluster"
)

var errMetricsHubHostnameNotFound = errors.New("metricsHubHostname variable not found in AddOnDeploymentConfig")

type promSample struct {
	Metric map[string]string `json:"metric"`
	Value  []any             `json:"value"`
}

type promResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string       `json:"resultType"`
		Result     []promSample `json:"result"`
	} `json:"data"`
}

//nolint:testifylint // c is an *assert.CollectT passed from require.EventuallyWithT callbacks
func queryPrometheus(c *assert.CollectT, httpClient *http.Client, hubURL, promQL string) ([]promSample, bool) {
	queryURL := fmt.Sprintf("%s/api/v1/query?query=%s", hubURL, url.QueryEscape(promQL))
	resp, reqErr := httpClient.Get(queryURL)
	if !assert.NoError(c, reqErr, "HTTP GET to Hub Prometheus failed for query %s", promQL) {
		return nil, false
	}
	defer func() { _ = resp.Body.Close() }()

	body, readErr := io.ReadAll(resp.Body)
	if !assert.NoError(c, readErr, "Failed to read response body") {
		return nil, false
	}

	if !assert.Equal(c, http.StatusOK, resp.StatusCode, "Hub Prometheus returned status %d: %s", resp.StatusCode, string(body)) {
		return nil, false
	}

	var pResp promResponse
	if !assert.NoError(c, json.Unmarshal(body, &pResp), "Failed to unmarshal Prometheus response JSON") {
		return nil, false
	}

	if !assert.Equal(c, "success", pResp.Status, "Prometheus response status is not success") {
		return nil, false
	}

	if !assert.NotEmpty(c, pResp.Data.Result, "Prometheus returned no samples for query: %s", promQL) {
		return nil, false
	}

	return pResp.Data.Result, true
}

func isPodReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func parseCommonNamesFromPEM(pemBytes []byte) []string {
	var commonNames []string
	rest := pemBytes
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
	return commonNames
}

type rotatedCerts struct {
	caCertPEM     []byte
	serverCertPEM []byte
	serverKeyPEM  []byte
	clientCertPEM []byte
	clientKeyPEM  []byte
}

func generateRotatedMTLSCerts(caCommonName, clientCommonName string) (*rotatedCerts, error) {
	now := time.Now()

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate CA key: %w", err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(100),
		Subject: pkix.Name{
			CommonName: caCommonName,
		},
		NotBefore:             now.Add(-1 * time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create CA certificate: %w", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}
	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})

	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate server key: %w", err)
	}
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(101),
		Subject: pkix.Name{
			CommonName: "hub-prometheus.open-cluster-management-observability.svc",
		},
		DNSNames: []string{
			"hub-prometheus.open-cluster-management-observability.svc",
			"hub-prometheus.open-cluster-management-observability.svc.cluster.local",
		},
		NotBefore:   now.Add(-1 * time.Hour),
		NotAfter:    now.Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create server certificate: %w", err)
	}
	serverCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverDER})
	serverKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKey)})

	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate client key: %w", err)
	}
	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(102),
		Subject: pkix.Name{
			CommonName: clientCommonName,
		},
		NotBefore:   now.Add(-1 * time.Hour),
		NotAfter:    now.Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create client certificate: %w", err)
	}
	clientCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER})
	clientKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)})

	return &rotatedCerts{
		caCertPEM:     caCertPEM,
		serverCertPEM: serverCertPEM,
		serverKeyPEM:  serverKeyPEM,
		clientCertPEM: clientCertPEM,
		clientKeyPEM:  clientKeyPEM,
	}, nil
}

func TestMetricsFederation(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, clusterv1.Install(scheme))
	require.NoError(t, clusterv1beta1.Install(scheme))
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

	ctx, cancel := context.WithTimeout(t.Context(), 18*time.Minute)
	defer cancel()

	t.Log("=== Step 1: Validating prerequisites on Hub ===")
	managedCluster := &clusterv1.ManagedCluster{}
	err = k8sClient.Get(ctx, client.ObjectKey{Name: clusterName}, managedCluster)
	require.NoError(t, err, "ManagedCluster 'local-cluster' must exist")
	require.Equal(t, "OpenShift", managedCluster.Labels["vendor"], "ManagedCluster vendor must be OpenShift")

	cmao := &addonv1beta1.ClusterManagementAddOn{}
	err = k8sClient.Get(ctx, client.ObjectKey{Name: addonName}, cmao)
	require.NoError(t, err, "ClusterManagementAddOn 'multicluster-observability-addon' must exist")

	aodc := &addonv1beta1.AddOnDeploymentConfig{}
	err = k8sClient.Get(ctx, client.ObjectKey{
		Namespace: hubNamespace,
		Name:      addonName,
	}, aodc)
	require.NoError(t, err, "AddOnDeploymentConfig must exist in open-cluster-management-observability")

	t.Log("=== Step 2: Creating ManagedClusterAddOn for local-cluster ===")
	mca := &addonv1beta1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addonName,
			Namespace: clusterName,
		},
	}
	err = k8sClient.Create(ctx, mca)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		require.NoError(t, err, "Failed to create ManagedClusterAddOn")
	}

	t.Log("=== Step 3: Asserting ManifestWork is created and applied by klusterlet ===")
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		mwList := &workv1.ManifestWorkList{}
		listErr := k8sClient.List(ctx, mwList, client.InNamespace(clusterName), client.MatchingLabels{
			addonv1beta1.AddonLabelKey: addonName,
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
			Namespace: spokeNamespace,
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
		if !assert.NoError(c, k8sClient.List(ctx, secretList, client.InNamespace(spokeNamespace))) {
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

		commonNames := parseCommonNamesFromPEM(caPEM)
		assert.Contains(c, commonNames, "observability-ca")
	}, 2*time.Minute, 3*time.Second, "hub-mtls-ca secret with Hub CA was not found within timeout")

	t.Log("=== Step 4b: Asserting prom-agent-platform-metrics-collector pod is running and ready ===")
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		pod := &corev1.Pod{}
		podErr := k8sClient.Get(ctx, client.ObjectKey{
			Namespace: spokeNamespace,
			Name:      "prom-agent-platform-metrics-collector-0",
		}, pod)
		if !assert.NoError(c, podErr, "Pod 'prom-agent-platform-metrics-collector-0' not found yet") {
			return
		}
		assert.True(c, isPodReady(pod), "Pod 'prom-agent-platform-metrics-collector-0' is not ready yet (phase: %s)", pod.Status.Phase)
	}, 3*time.Minute, 3*time.Second, "prom-agent-platform-metrics-collector-0 did not reach Ready state within timeout")

	t.Log("=== Step 5: Asserting Hub Prometheus receives federated metrics via direct mTLS from local-cluster ===")
	hubURL := os.Getenv("HUB_PROMETHEUS_URL")
	if hubURL == "" {
		hubURL = "http://127.0.0.1:9090"
	}
	httpClient := &http.Client{Timeout: 5 * time.Second}

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		samples, ok := queryPrometheus(c, httpClient, hubURL, `up{cluster="local-cluster",job="prometheus"} == 1`)
		if !ok {
			return
		}
		t.Logf("Successfully queried Hub Prometheus! Sample: %+v", samples[0])
	}, 3*time.Minute, 5*time.Second, "Hub Prometheus did not receive healthy federated metrics (up == 1) from local-cluster within timeout")

	t.Run("DynamicScrapeConfigHotReload", func(t *testing.T) {
		t.Log("=== Validating Dynamic ScrapeConfig Hot-Reloading (ACM-47976) ===")

		// Capture baseline Pod UID and verify zero container restarts before mutating ScrapeConfigs
		initialPod := &corev1.Pod{}
		require.NoError(t, k8sClient.Get(ctx, client.ObjectKey{
			Namespace: spokeNamespace,
			Name:      "prom-agent-platform-metrics-collector-0",
		}, initialPod))
		require.NotEmpty(t, initialPod.UID)
		for _, cs := range initialPod.Status.ContainerStatuses {
			require.Equal(t, int32(0), cs.RestartCount, "Expected 0 initial restarts for container %s", cs.Name)
		}

		// 1. Mutate existing ScrapeConfig 'platform-metrics' on the Hub to federate additional metrics
		// (including node_cpu_seconds_total and kube_pod_status_phase for the subsequent Non-OCP test)
		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			sc := &cooprometheusv1alpha1.ScrapeConfig{}
			if getErr := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: hubNamespace,
				Name:      "platform-metrics",
			}, sc); getErr != nil {
				return getErr
			}
			sc.Spec.Params = map[string][]string{
				"match[]": {
					`{__name__="up"}`,
					`{__name__="prometheus_build_info"}`,
					`{__name__="node_cpu_seconds_total"}`,
					`{__name__="kube_pod_status_phase"}`,
				},
			}
			return k8sClient.Update(ctx, sc)
		})
		require.NoError(t, err, "Failed to update platform-metrics ScrapeConfig on Hub")

		// 2. Create a new ScrapeConfig 'platform-metrics-extra' on the Hub at runtime
		extraSC := &cooprometheusv1alpha1.ScrapeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "platform-metrics-extra",
				Namespace: hubNamespace,
				Labels: map[string]string{
					"app.kubernetes.io/component":  "platform-metrics-collector",
					"app.kubernetes.io/managed-by": "multicluster-observability-operator",
					"app.kubernetes.io/part-of":    addonName,
				},
				Annotations: map[string]string{
					"observability.open-cluster-management.io/placements": "open-cluster-management-global-set/global",
				},
			},
			Spec: cooprometheusv1alpha1.ScrapeConfigSpec{
				ScrapeInterval: ptr.To(cooprometheusv1.Duration("5s")),
				JobName:        ptr.To("platform-extra"),
				MetricsPath:    ptr.To("/federate"),
				Params: map[string][]string{
					"match[]": {
						`{__name__="go_goroutines"}`,
					},
				},
			},
		}
		err = k8sClient.Create(ctx, extraSC)
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			require.NoError(t, err, "Failed to create platform-metrics-extra ScrapeConfig on Hub")
		}

		// 3. Assert both ScrapeConfigs are propagated to the spoke namespace
		t.Log("=== Asserting mutated and newly added ScrapeConfigs are propagated to spoke ===")
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			spokeSC := &cooprometheusv1alpha1.ScrapeConfig{}
			if !assert.NoError(c, k8sClient.Get(ctx, client.ObjectKey{
				Namespace: spokeNamespace,
				Name:      "platform-metrics",
			}, spokeSC)) {
				return
			}
			assert.Contains(c, spokeSC.Spec.Params["match[]"], `{__name__="prometheus_build_info"}`)

			spokeExtraSC := &cooprometheusv1alpha1.ScrapeConfig{}
			if !assert.NoError(c, k8sClient.Get(ctx, client.ObjectKey{
				Namespace: spokeNamespace,
				Name:      "platform-metrics-extra",
			}, spokeExtraSC)) {
				return
			}
			assert.Contains(c, spokeExtraSC.Spec.Params["match[]"], `{__name__="go_goroutines"}`)
		}, 2*time.Minute, 3*time.Second, "Updated and added ScrapeConfigs were not propagated to spoke within timeout")

		// 4. Assert newly configured metrics from both mutated and added ScrapeConfigs appear in Hub Prometheus
		t.Log("=== Asserting newly federated metrics arrive in Hub Prometheus ===")
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			if _, ok := queryPrometheus(c, httpClient, hubURL, `prometheus_build_info{cluster="local-cluster"} == 1`); !ok {
				return
			}
			_, ok := queryPrometheus(c, httpClient, hubURL, `go_goroutines{cluster="local-cluster"} > 0`)
			assert.True(c, ok, "Expected go_goroutines metric from platform-metrics-extra ScrapeConfig")
		}, 3*time.Minute, 5*time.Second, "Hub Prometheus did not receive metrics from dynamically reloaded ScrapeConfigs")

		// 5. Assert prometheus-config-reloader reloaded Prometheus in-place with zero container restarts
		currentPod := &corev1.Pod{}
		require.NoError(t, k8sClient.Get(ctx, client.ObjectKey{
			Namespace: spokeNamespace,
			Name:      "prom-agent-platform-metrics-collector-0",
		}, currentPod))
		assert.Equal(t, initialPod.UID, currentPod.UID, "PrometheusAgent pod must not be recreated during ScrapeConfig hot-reload")
		for _, cs := range currentPod.Status.ContainerStatuses {
			assert.Equal(t, int32(0), cs.RestartCount, "Container %s must have 0 restarts after ScrapeConfig hot-reload", cs.Name)
		}
	})

	t.Run("HTTPSProxyWithMTLS", func(t *testing.T) {
		t.Log("=== Step 6: Validating HTTPS Proxy with mTLS ===")

		// 1. Read Proxy CA from e2e-proxy-ca ConfigMap
		proxyCAConfigMap := &corev1.ConfigMap{}
		err := k8sClient.Get(ctx, client.ObjectKey{
			Namespace: hubNamespace,
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
				Namespace: hubNamespace,
				Name:      addonName,
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
			if !assert.NoError(c, k8sClient.List(ctx, secretList, client.InNamespace(spokeNamespace))) {
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

			commonNames := parseCommonNamesFromPEM(caPEM)
			assert.Contains(c, commonNames, "observability-ca", "Merged secret must contain Hub CA")
			assert.Contains(c, commonNames, "e2e-proxy-ca", "Merged secret must contain Proxy CA")
		}, 2*time.Minute, 3*time.Second, "hub-mtls-ca secret was not updated with merged CAs")

		// 4. Assert Control-Plane: PrometheusAgent platform-metrics-collector spec is updated
		t.Log("=== Step 6b: Asserting PrometheusAgent spec is updated with httpsProxy and https URL ===")
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			agent := &cooprometheusv1alpha1.PrometheusAgent{}
			if !assert.NoError(c, k8sClient.Get(ctx, client.ObjectKey{
				Namespace: spokeNamespace,
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
		var proxyActiveTime float64
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
			proxyActiveTime = float64(time.Now().UnixNano()) / 1e9
		}, 3*time.Minute, 3*time.Second, "HTTPS proxy did not record CONNECT tunnel or bytes transferred")

		// 6. Assert Data-Plane: Hub Prometheus receives fresh federated metrics via mTLS over tunnel
		t.Log("=== Step 6d: Asserting Hub Prometheus receives fresh federated metrics via mTLS ===")
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			samples, ok := queryPrometheus(c, httpClient, hubURL, `timestamp(up{cluster="local-cluster",job="prometheus"} == 1)`)
			if !ok {
				return
			}
			valSlice := samples[0].Value
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
			assert.GreaterOrEqual(c, sampleTime, proxyActiveTime,
				"Expected fresh metric sample delivered after proxy tunnel was established (sample: %v, proxyActive: %v)", sampleTime, proxyActiveTime)
		}, 3*time.Minute, 5*time.Second, "Hub Prometheus did not receive fresh federated metrics over mTLS tunnel within timeout")
	})

	t.Run("SecretAndMTLSRotationRecovery", func(t *testing.T) {
		t.Log("=== Validating Secret & mTLS Rotation Recovery (ACM-47976) ===")

		const rotatedCACommonName = "observability-ca-rotated"
		const rotatedClientCommonName = "observability-client-rotated"

		certs, err := generateRotatedMTLSCerts(rotatedCACommonName, rotatedClientCommonName)
		require.NoError(t, err, "Failed to generate rotated mTLS certificates")

		// 1. Update Hub CA secret (observability-managed-cluster-certs)
		err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			caSecret := &corev1.Secret{}
			if getErr := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: hubNamespace,
				Name:      "observability-managed-cluster-certs",
			}, caSecret); getErr != nil {
				return getErr
			}
			caSecret.Data["ca.crt"] = certs.caCertPEM
			return k8sClient.Update(ctx, caSecret)
		})
		require.NoError(t, err, "Failed to update observability-managed-cluster-certs on Hub")

		// 2. Update Hub Prometheus server TLS secret (hub-prometheus-tls)
		err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			serverSecret := &corev1.Secret{}
			if getErr := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: hubNamespace,
				Name:      "hub-prometheus-tls",
			}, serverSecret); getErr != nil {
				return getErr
			}
			serverSecret.Data["tls.crt"] = certs.serverCertPEM
			serverSecret.Data["tls.key"] = certs.serverKeyPEM
			return k8sClient.Update(ctx, serverSecret)
		})
		require.NoError(t, err, "Failed to update hub-prometheus-tls on Hub")

		// 3. Update Hub mTLS client certificate secret
		err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			clientSecret := &corev1.Secret{}
			if getErr := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: hubNamespace,
				Name:      "observability-controller-open-cluster-management.io-observability-signer-client-cert",
			}, clientSecret); getErr != nil {
				return getErr
			}
			clientSecret.Data["tls.crt"] = certs.clientCertPEM
			clientSecret.Data["tls.key"] = certs.clientKeyPEM
			return k8sClient.Update(ctx, clientSecret)
		})
		require.NoError(t, err, "Failed to update client cert secret on Hub")

		// 4. Restart hub-prometheus Deployment so Nginx immediately enforces the rotated CA and server cert
		err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			deploy := &appsv1.Deployment{}
			if getErr := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: hubNamespace,
				Name:      "hub-prometheus",
			}, deploy); getErr != nil {
				return getErr
			}
			if deploy.Spec.Template.Annotations == nil {
				deploy.Spec.Template.Annotations = map[string]string{}
			}
			deploy.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339Nano)
			return k8sClient.Update(ctx, deploy)
		})
		require.NoError(t, err, "Failed to trigger rollout of hub-prometheus")

		// 5. Assert WatcherReconciler propagates rotated CA and client cert secrets to spoke namespace
		t.Log("=== Asserting rotated CA and client cert secrets are propagated to spoke ===")
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			secretList := &corev1.SecretList{}
			if !assert.NoError(c, k8sClient.List(ctx, secretList, client.InNamespace(spokeNamespace))) {
				return
			}
			var caSecret, certSecret *corev1.Secret
			for i := range secretList.Items {
				if strings.HasPrefix(secretList.Items[i].Name, "hub-mtls-ca-") {
					caSecret = &secretList.Items[i]
				}
				if strings.HasPrefix(secretList.Items[i].Name, "hub-mtls-cert-") {
					certSecret = &secretList.Items[i]
				}
			}
			if !assert.NotNil(c, caSecret, "Secret hub-mtls-ca-* not found in spoke namespace") {
				return
			}
			if !assert.NotNil(c, certSecret, "Secret hub-mtls-cert-* not found in spoke namespace") {
				return
			}

			caNames := parseCommonNamesFromPEM(caSecret.Data["ca.crt"])
			assert.Contains(c, caNames, rotatedCACommonName, "Spoke CA secret must contain rotated Hub CA")
			assert.Contains(c, caNames, "e2e-proxy-ca", "Spoke CA secret must retain Proxy CA")

			certNames := parseCommonNamesFromPEM(certSecret.Data["tls.crt"])
			assert.Contains(c, certNames, rotatedClientCommonName, "Spoke client cert secret must contain rotated client cert")
		}, 2*time.Minute, 3*time.Second, "Rotated mTLS secrets were not propagated to spoke within timeout")

		// 6. Assert PrometheusAgent recovers connection to Hub Prometheus receiver with rotated certs
		t.Log("=== Asserting PrometheusAgent recovers mTLS connection and delivers fresh metrics ===")
		rotationTime := float64(time.Now().UnixNano()) / 1e9
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			samples, ok := queryPrometheus(c, httpClient, hubURL, `timestamp(up{cluster="local-cluster",job="prometheus"} == 1)`)
			if !ok {
				return
			}
			valSlice := samples[0].Value
			if !assert.Len(c, valSlice, 2) {
				return
			}
			sampleTimeStr, ok := valSlice[1].(string)
			if !assert.True(c, ok) {
				return
			}
			sampleTime, parseErr := strconv.ParseFloat(sampleTimeStr, 64)
			if !assert.NoError(c, parseErr) {
				return
			}
			assert.GreaterOrEqual(c, sampleTime, rotationTime,
				"Expected fresh metric sample after mTLS rotation (sample: %v, rotation: %v)", sampleTime, rotationTime)
		}, 3*time.Minute, 5*time.Second, "PrometheusAgent did not recover mTLS remote-write after certificate rotation")

		// Verify PrometheusAgent pod is healthy and not crashlooping
		agentPod := &corev1.Pod{}
		require.NoError(t, k8sClient.Get(ctx, client.ObjectKey{
			Namespace: spokeNamespace,
			Name:      "prom-agent-platform-metrics-collector-0",
		}, agentPod))
		assert.True(t, isPodReady(agentPod), "prom-agent-platform-metrics-collector-0 should be Ready after cert rotation")
		for _, cs := range agentPod.Status.ContainerStatuses {
			assert.Equal(t, int32(0), cs.RestartCount, "Container %s should not have restarted during mTLS rotation", cs.Name)
		}
	})

	t.Run("UserWorkloadMonitoring", func(t *testing.T) {
		t.Log("=== Validating User Workload Monitoring (ACM-47975) ===")

		// Ensure openshift-user-workload-monitoring namespace exists on spoke
		uwmNS := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: uwmNamespace,
				Labels: map[string]string{
					"openshift.io/user-monitoring": "true",
				},
			},
		}
		err := k8sClient.Create(ctx, uwmNS)
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			require.NoError(t, err, "Failed to create openshift-user-workload-monitoring namespace")
		}

		// 1. Deploy mock application workload Service with service-ca serving certificate annotation
		appLabels := map[string]string{"app": "sample-uwm-app"}
		appService := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sample-uwm-app",
				Namespace: uwmNamespace,
				Annotations: map[string]string{
					"service.beta.openshift.io/serving-cert-secret-name": "sample-uwm-app-tls",
				},
				Labels: appLabels,
			},
			Spec: corev1.ServiceSpec{
				Selector: appLabels,
				Ports: []corev1.ServicePort{
					{
						Name:       "https-metrics",
						Port:       8443,
						TargetPort: intstr.FromInt32(8443),
					},
				},
			},
		}
		err = k8sClient.Create(ctx, appService)
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			require.NoError(t, err, "Failed to create sample-uwm-app Service")
		}

		t.Log("=== Waiting for service-ca-operator to provision sample-uwm-app-tls Secret ===")
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			tlsSecret := &corev1.Secret{}
			if !assert.NoError(c, k8sClient.Get(ctx, client.ObjectKey{
				Namespace: uwmNamespace,
				Name:      "sample-uwm-app-tls",
			}, tlsSecret)) {
				return
			}
			assert.NotEmpty(c, tlsSecret.Data["tls.crt"])
			assert.NotEmpty(c, tlsSecret.Data["tls.key"])
		}, 2*time.Minute, 2*time.Second, "service-ca-operator did not provision sample-uwm-app-tls within timeout")

		// 2. Deploy ConfigMap and Deployment for sample-uwm-app exposing custom metrics over HTTPS
		appConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sample-uwm-app-config",
				Namespace: uwmNamespace,
			},
			Data: map[string]string{
				"nginx.conf": `events {
    worker_connections 1024;
}
http {
    server {
        listen 8443 ssl;
        ssl_certificate /etc/tls/private/tls.crt;
        ssl_certificate_key /etc/tls/private/tls.key;
        location / {
            default_type text/plain;
            return 200 "# HELP uwm_custom_app_requests_total Total custom requests for UWM e2e test.\n# TYPE uwm_custom_app_requests_total counter\nuwm_custom_app_requests_total{app=\"sample-uwm-app\",env=\"e2e\"} 42\n";
        }
    }
}
`,
			},
		}
		err = k8sClient.Create(ctx, appConfigMap)
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			require.NoError(t, err, "Failed to create sample-uwm-app-config ConfigMap")
		}

		appDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sample-uwm-app",
				Namespace: uwmNamespace,
				Labels:    appLabels,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(1)),
				Selector: &metav1.LabelSelector{MatchLabels: appLabels},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: appLabels},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:            "nginx",
								Image:           "docker.io/library/nginx:alpine",
								ImagePullPolicy: corev1.PullIfNotPresent,
								Ports: []corev1.ContainerPort{
									{ContainerPort: 8443, Name: "https-metrics"},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "nginx-conf",
										MountPath: "/etc/nginx/nginx.conf",
										SubPath:   "nginx.conf",
										ReadOnly:  true,
									},
									{
										Name:      "tls",
										MountPath: "/etc/tls/private",
										ReadOnly:  true,
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "nginx-conf",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{Name: "sample-uwm-app-config"},
									},
								},
							},
							{
								Name: "tls",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: "sample-uwm-app-tls",
									},
								},
							},
						},
					},
				},
			},
		}
		err = k8sClient.Create(ctx, appDeployment)
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			require.NoError(t, err, "Failed to create sample-uwm-app Deployment")
		}

		// 3. Create UWM ScrapeConfig on Hub targeting sample-uwm-app
		uwmSC := &cooprometheusv1alpha1.ScrapeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "uwm-custom-metrics",
				Namespace: hubNamespace,
				Labels: map[string]string{
					"app.kubernetes.io/component":  "user-workload-metrics-collector",
					"app.kubernetes.io/managed-by": "multicluster-observability-operator",
					"app.kubernetes.io/part-of":    addonName,
				},
				Annotations: map[string]string{
					"observability.open-cluster-management.io/placements": "open-cluster-management-global-set/global",
				},
			},
			Spec: cooprometheusv1alpha1.ScrapeConfigSpec{
				ScrapeInterval: ptr.To(cooprometheusv1.Duration("5s")),
				JobName:        ptr.To("sample-uwm-app"),
				MetricsPath:    ptr.To("/metrics"),
				Scheme:         ptr.To(cooprometheusv1.Scheme("HTTPS")),
				StaticConfigs: []cooprometheusv1alpha1.StaticConfig{
					{
						Targets: []cooprometheusv1alpha1.Target{
							"sample-uwm-app.openshift-user-workload-monitoring.svc:8443",
						},
					},
				},
			},
		}
		err = k8sClient.Create(ctx, uwmSC)
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			require.NoError(t, err, "Failed to create uwm-custom-metrics ScrapeConfig on Hub")
		}

		// 4. Enable userWorkloadMetricsCollection in AddOnDeploymentConfig
		err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			aodc := &addonv1beta1.AddOnDeploymentConfig{}
			if getErr := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: hubNamespace,
				Name:      addonName,
			}, aodc); getErr != nil {
				return getErr
			}
			found := false
			for i := range aodc.Spec.CustomizedVariables {
				if aodc.Spec.CustomizedVariables[i].Name == "userWorkloadMetricsCollection" {
					aodc.Spec.CustomizedVariables[i].Value = "prometheusagents.v1alpha1.monitoring.rhobs"
					found = true
				}
			}
			if !found {
				aodc.Spec.CustomizedVariables = append(aodc.Spec.CustomizedVariables, addonv1beta1.CustomizedVariable{
					Name:  "userWorkloadMetricsCollection",
					Value: "prometheusagents.v1alpha1.monitoring.rhobs",
				})
			}
			return k8sClient.Update(ctx, aodc)
		})
		require.NoError(t, err, "Failed to enable userWorkloadMetricsCollection in AddOnDeploymentConfig")

		// 5. Assert user-workload-metrics-collector PrometheusAgent and Pod are deployed and Ready
		t.Log("=== Asserting user-workload-metrics-collector PrometheusAgent and Pod reach Ready state ===")
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			uwmAgent := &cooprometheusv1alpha1.PrometheusAgent{}
			if !assert.NoError(c, k8sClient.Get(ctx, client.ObjectKey{
				Namespace: spokeNamespace,
				Name:      "user-workload-metrics-collector",
			}, uwmAgent)) {
				return
			}
			assert.NotEmpty(c, uwmAgent.Spec.RemoteWrite)

			uwmPod := &corev1.Pod{}
			if !assert.NoError(c, k8sClient.Get(ctx, client.ObjectKey{
				Namespace: spokeNamespace,
				Name:      "prom-agent-user-workload-metrics-collector-0",
			}, uwmPod)) {
				return
			}
			assert.True(c, isPodReady(uwmPod), "Pod prom-agent-user-workload-metrics-collector-0 is not ready yet")
		}, 3*time.Minute, 3*time.Second, "user-workload-metrics-collector did not reach Ready state within timeout")

		// 6. Assert custom application metrics arrive at Hub Prometheus
		t.Log("=== Asserting custom user workload metrics arrive at Hub Prometheus ===")
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			samples, ok := queryPrometheus(c, httpClient, hubURL, `uwm_custom_app_requests_total{cluster="local-cluster",app="sample-uwm-app",env="e2e"} == 42`)
			if !ok {
				return
			}
			t.Logf("Successfully received UWM metric sample on Hub Prometheus: %+v", samples[0])
		}, 3*time.Minute, 5*time.Second, "Hub Prometheus did not receive custom UWM metrics within timeout")

		// Clean up UWM collector and mock app after validation to conserve cluster memory for subsequent tests
		_ = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			aodc := &addonv1beta1.AddOnDeploymentConfig{}
			if getErr := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: hubNamespace,
				Name:      addonName,
			}, aodc); getErr != nil {
				return getErr
			}
			for i := range aodc.Spec.CustomizedVariables {
				if aodc.Spec.CustomizedVariables[i].Name == "userWorkloadMetricsCollection" {
					aodc.Spec.CustomizedVariables[i].Value = "disabled"
				}
			}
			return k8sClient.Update(ctx, aodc)
		})
		_ = k8sClient.Delete(ctx, appDeployment)
	})

	t.Run("NonOCPManagedClusterStack", func(t *testing.T) {
		t.Log("=== Validating Non-OCP Managed Cluster Stack via mcoa-override-vendor (ACM-47975) ===")

		// 1. Annotate ManagedCluster 'local-cluster' with mcoa-override-vendor: "Other"
		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			mc := &clusterv1.ManagedCluster{}
			if getErr := k8sClient.Get(ctx, client.ObjectKey{Name: clusterName}, mc); getErr != nil {
				return getErr
			}
			if mc.Annotations == nil {
				mc.Annotations = map[string]string{}
			}
			mc.Annotations["mcoa-override-vendor"] = "Other"
			return k8sClient.Update(ctx, mc)
		})
		require.NoError(t, err, "Failed to set mcoa-override-vendor annotation on ManagedCluster")

		// 2. Assert deployment and readiness of node-exporter DaemonSet, kube-state-metrics Deployment, and acm-prometheus-k8s Prometheus server
		t.Log("=== Asserting node-exporter, kube-state-metrics, and acm-prometheus-k8s reach Ready state ===")
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			nodeExporterDS := &appsv1.DaemonSet{}
			if !assert.NoError(c, k8sClient.Get(ctx, client.ObjectKey{
				Namespace: spokeNamespace,
				Name:      "node-exporter",
			}, nodeExporterDS), "node-exporter DaemonSet not found yet") {
				return
			}
			assert.GreaterOrEqual(c, nodeExporterDS.Status.NumberReady, int32(1), "Expected at least 1 ready node-exporter pod")

			ksmDeploy := &appsv1.Deployment{}
			if !assert.NoError(c, k8sClient.Get(ctx, client.ObjectKey{
				Namespace: spokeNamespace,
				Name:      "kube-state-metrics",
			}, ksmDeploy), "kube-state-metrics Deployment not found yet") {
				return
			}
			assert.GreaterOrEqual(c, ksmDeploy.Status.AvailableReplicas, int32(1), "Expected at least 1 available kube-state-metrics replica")

			promCR := &cooprometheusv1.Prometheus{}
			if !assert.NoError(c, k8sClient.Get(ctx, client.ObjectKey{
				Namespace: spokeNamespace,
				Name:      "acm-prometheus-k8s",
			}, promCR), "acm-prometheus-k8s Prometheus CR not found yet") {
				return
			}

			promServerPod := &corev1.Pod{}
			if !assert.NoError(c, k8sClient.Get(ctx, client.ObjectKey{
				Namespace: spokeNamespace,
				Name:      "prometheus-acm-prometheus-k8s-0",
			}, promServerPod), "prometheus-acm-prometheus-k8s-0 Pod not found yet") {
				return
			}
			assert.True(c, isPodReady(promServerPod), "prometheus-acm-prometheus-k8s-0 is not Ready yet")

			spokeSC := &cooprometheusv1alpha1.ScrapeConfig{}
			if !assert.NoError(c, k8sClient.Get(ctx, client.ObjectKey{
				Namespace: spokeNamespace,
				Name:      "platform-metrics",
			}, spokeSC)) {
				return
			}
			if assert.NotNil(c, spokeSC.Spec.ScrapeClassName) {
				assert.Equal(c, "non-ocp-monitoring", *spokeSC.Spec.ScrapeClassName)
			}
			if assert.NotEmpty(c, spokeSC.Spec.StaticConfigs) && assert.NotEmpty(c, spokeSC.Spec.StaticConfigs[0].Targets) {
				assert.Equal(c, "acm-prometheus-k8s.open-cluster-management-addon-observability.svc:9091", string(spokeSC.Spec.StaticConfigs[0].Targets[0]))
			}
		}, 3*time.Minute, 3*time.Second, "Non-OCP monitoring stack did not become Ready within timeout")

		// 3. Assert host (node-exporter) and cluster (kube-state-metrics) metrics federation back to Hub Prometheus
		t.Log("=== Asserting host and cluster metrics from Non-OCP stack arrive at Hub Prometheus ===")
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			if _, ok := queryPrometheus(c, httpClient, hubURL, `node_cpu_seconds_total{cluster="local-cluster"}`); !ok {
				return
			}
			_, ok := queryPrometheus(c, httpClient, hubURL, `kube_pod_status_phase{cluster="local-cluster"}`)
			assert.True(c, ok, "Expected kube_pod_status_phase metric from kube-state-metrics")
		}, 3*time.Minute, 5*time.Second, "Hub Prometheus did not receive host and cluster metrics from Non-OCP stack within timeout")

		// 4. Restore OpenShift vendor on local-cluster and wait for Non-OCP Prometheus server to be removed
		err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			mc := &clusterv1.ManagedCluster{}
			if getErr := k8sClient.Get(ctx, client.ObjectKey{Name: clusterName}, mc); getErr != nil {
				return getErr
			}
			delete(mc.Annotations, "mcoa-override-vendor")
			return k8sClient.Update(ctx, mc)
		})
		require.NoError(t, err, "Failed to remove mcoa-override-vendor annotation on ManagedCluster")

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			promPod := &corev1.Pod{}
			podErr := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: spokeNamespace,
				Name:      "prometheus-acm-prometheus-k8s-0",
			}, promPod)
			assert.True(c, k8serrors.IsNotFound(podErr), "Expected prometheus-acm-prometheus-k8s-0 pod to be removed after reverting vendor")

			agentPod := &corev1.Pod{}
			if !assert.NoError(c, k8sClient.Get(ctx, client.ObjectKey{
				Namespace: spokeNamespace,
				Name:      "prom-agent-platform-metrics-collector-0",
			}, agentPod), "Expected prom-agent-platform-metrics-collector-0 pod to exist") {
				return
			}
			assert.True(c, isPodReady(agentPod), "Expected prom-agent-platform-metrics-collector-0 to be Ready after reverting vendor")

			emoDeploy := &appsv1.Deployment{}
			if !assert.NoError(c, k8sClient.Get(ctx, client.ObjectKey{
				Namespace: spokeNamespace,
				Name:      "endpoint-monitoring-operator",
			}, emoDeploy)) {
				return
			}
			assert.Equal(c, int32(1), emoDeploy.Status.UpdatedReplicas)
			assert.Equal(c, int32(1), emoDeploy.Status.ReadyReplicas)
			assert.Equal(c, int32(1), emoDeploy.Status.Replicas, "Expected old endpoint-monitoring-operator replica to finish terminating")
		}, 2*time.Minute, 3*time.Second, "Non-OCP Prometheus pod was not removed or OCP stack did not stabilize after reverting vendor override")
	})

	t.Run("AddonTeardownAndReinstall", func(t *testing.T) {
		t.Log("=== Validating Addon Teardown, Garbage Collection & Idempotent Re-installation (ACM-47976) ===")

		t.Cleanup(func() {
			_ = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				placement := &clusterv1beta1.Placement{}
				if getErr := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: "open-cluster-management-global-set",
					Name:      "global",
				}, placement); getErr != nil {
					return getErr
				}
				if len(placement.Spec.Predicates) == 0 {
					return nil
				}
				placement.Spec.Predicates = nil
				return k8sClient.Update(ctx, placement)
			})
		})

		// 1. Exclude local-cluster from the global Placement so OCM addon-manager triggers
		// ManagedClusterAddOn uninstallation and does not immediately recreate it while we assert GC.
		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			placement := &clusterv1beta1.Placement{}
			if getErr := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: "open-cluster-management-global-set",
				Name:      "global",
			}, placement); getErr != nil {
				return getErr
			}
			placement.Spec.Predicates = []clusterv1beta1.ClusterPredicate{
				{
					RequiredClusterSelector: clusterv1beta1.ClusterSelector{
						LabelSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"mcoa-e2e-teardown-hold": "true",
							},
						},
					},
				},
			}
			return k8sClient.Update(ctx, placement)
		})
		require.NoError(t, err, "Failed to update global Placement predicates for teardown")

		// 2. Assert ManagedClusterAddOn and ManifestWorks are cleanly deleted
		t.Log("=== Asserting ManagedClusterAddOn and ManifestWorks are cleanly deleted ===")
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			mcaCheck := &addonv1beta1.ManagedClusterAddOn{}
			getErr := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: clusterName,
				Name:      addonName,
			}, mcaCheck)
			assert.True(c, k8serrors.IsNotFound(getErr), "ManagedClusterAddOn should be deleted, got err: %v", getErr)

			mwList := &workv1.ManifestWorkList{}
			if !assert.NoError(c, k8sClient.List(ctx, mwList, client.InNamespace(clusterName), client.MatchingLabels{
				addonv1beta1.AddonLabelKey: addonName,
			})) {
				return
			}
			assert.Empty(c, mwList.Items, "All addon ManifestWorks in local-cluster should be deleted")
		}, 3*time.Minute, 3*time.Second, "ManagedClusterAddOn or ManifestWorks were not deleted within timeout")

		// 3. Assert spoke workload pods terminate cleanly and no orphaned cluster-scoped RBAC remains
		t.Log("=== Asserting spoke pods terminate cleanly and no orphaned cluster-scoped RBAC remains ===")
		expectedDeletedClusterRoles := []string{
			"open-cluster-management:multicluster-observability-addon:klusterlet-work:agent",
			"open-cluster-management:multicluster-observability-addon:platform-metrics-collector-federate",
			"open-cluster-management:multicluster-observability-addon:platform-metrics-collector-metrics",
			"open-cluster-management:multicluster-observability-addon:prometheus-operator-metrics",
			"open-cluster-management:multicluster-observability-addon:endpoint-monitoring-operator",
			"open-cluster-management:multicluster-observability-addon:endpoint-monitoring-operator-metrics",
			"open-cluster-management:multicluster-observability-addon:kube-state-metrics",
			"open-cluster-management:multicluster-observability-addon:node-exporter",
			"open-cluster-management:multicluster-observability-addon:acm-prometheus-k8s",
		}

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			podList := &corev1.PodList{}
			if !assert.NoError(c, k8sClient.List(ctx, podList, client.InNamespace(spokeNamespace))) {
				return
			}
			var activePods []string
			for _, pod := range podList.Items {
				// Ignore completed pre-delete cleanup job pods waiting for TTL controller GC
				if pod.Labels["app.kubernetes.io/component"] == "endpoint-monitoring-operator-cleanup" && pod.Status.Phase == corev1.PodSucceeded {
					continue
				}
				activePods = append(activePods, pod.Name)
			}
			assert.Empty(c, activePods, "Expected all spoke workload pods to terminate, still active: %v", activePods)

			for _, crName := range expectedDeletedClusterRoles {
				cr := &rbacv1.ClusterRole{}
				crErr := k8sClient.Get(ctx, client.ObjectKey{Name: crName}, cr)
				assert.True(c, k8serrors.IsNotFound(crErr), "ClusterRole %s should be garbage collected", crName)
			}

			crbList := &rbacv1.ClusterRoleBindingList{}
			if !assert.NoError(c, k8sClient.List(ctx, crbList)) {
				return
			}
			for _, crb := range crbList.Items {
				if slices.Contains(expectedDeletedClusterRoles, crb.Name) {
					assert.Failf(c, "Orphaned ClusterRoleBinding found", "ClusterRoleBinding %s should be garbage collected", crb.Name)
				}
			}
		}, 3*time.Minute, 3*time.Second, "Spoke pods or cluster-scoped RBAC were not cleaned up within timeout")

		// 4. Restore global Placement predicates and re-install ManagedClusterAddOn
		t.Log("=== Re-installing ManagedClusterAddOn and asserting convergence back to Available ===")
		err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			placement := &clusterv1beta1.Placement{}
			if getErr := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: "open-cluster-management-global-set",
				Name:      "global",
			}, placement); getErr != nil {
				return getErr
			}
			placement.Spec.Predicates = nil
			return k8sClient.Update(ctx, placement)
		})
		require.NoError(t, err, "Failed to restore global Placement predicates")

		reinstallTime := float64(time.Now().UnixNano()) / 1e9

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			mcaCheck := &addonv1beta1.ManagedClusterAddOn{}
			if !assert.NoError(c, k8sClient.Get(ctx, client.ObjectKey{
				Namespace: clusterName,
				Name:      addonName,
			}, mcaCheck)) {
				return
			}
			available := false
			for _, cond := range mcaCheck.Status.Conditions {
				if cond.Type == addonv1beta1.ManagedClusterAddOnConditionAvailable && cond.Status == metav1.ConditionTrue {
					available = true
					break
				}
			}
			assert.True(c, available, "Expected re-installed ManagedClusterAddOn to reach Available=True, conditions: %+v", mcaCheck.Status.Conditions)

			samples, ok := queryPrometheus(c, httpClient, hubURL, `timestamp(up{cluster="local-cluster",job="prometheus"} == 1)`)
			if !ok {
				return
			}
			valSlice := samples[0].Value
			if !assert.Len(c, valSlice, 2) {
				return
			}
			sampleTimeStr, ok := valSlice[1].(string)
			if !assert.True(c, ok) {
				return
			}
			sampleTime, parseErr := strconv.ParseFloat(sampleTimeStr, 64)
			if !assert.NoError(c, parseErr) {
				return
			}
			assert.GreaterOrEqual(c, sampleTime, reinstallTime,
				"Expected fresh federated metric sample after addon re-installation (sample: %v, reinstall: %v)", sampleTime, reinstallTime)
		}, 3*time.Minute, 5*time.Second, "Re-installed ManagedClusterAddOn did not converge back to Available with active metrics federation")
	})
}
