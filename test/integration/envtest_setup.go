package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var setLoggerOnce sync.Once

// NewTestScheme returns a complete runtime.Scheme populated with all types needed for MCOA integration testing.
func NewTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(workv1.Install(s))
	utilruntime.Must(clusterv1.Install(s))
	utilruntime.Must(addonv1alpha1.Install(s))
	utilruntime.Must(addonv1beta1.Install(s))
	utilruntime.Must(cooprometheusv1.AddToScheme(s))
	utilruntime.Must(cooprometheusv1alpha1.AddToScheme(s))
	utilruntime.Must(prometheusv1.AddToScheme(s))
	utilruntime.Must(hyperv1.AddToScheme(s))
	return s
}

// FindRepoRoot traverses directory parents to locate the repository root containing go.mod.
func FindRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

const defaultEnvtestK8sVersion = "1.35.0"

// ResolveEnvtestAssetsDir returns the KubeBuilder assets path.
// Precedence order:
//  1. KUBEBUILDER_ASSETS environment variable (standard controller-runtime / Makefile).
//  2. Local repo ./bin/k8s/ directory containing pre-downloaded binaries.
//  3. Dynamic resolution via setup-envtest CLI using ENVTEST_K8S_VERSION (or defaultEnvtestK8sVersion).
func ResolveEnvtestAssetsDir() string {
	if assets := os.Getenv("KUBEBUILDER_ASSETS"); assets != "" {
		return assets
	}

	version := os.Getenv("ENVTEST_K8S_VERSION")
	repoRoot := FindRepoRoot()

	if version != "" {
		matches, _ := filepath.Glob(filepath.Join(repoRoot, "bin", "k8s", version+"-*"))
		if len(matches) > 0 {
			return matches[0]
		}
	} else {
		matches, _ := filepath.Glob(filepath.Join(repoRoot, "bin", "k8s", "*"))
		if len(matches) > 0 {
			return matches[0]
		}
		version = defaultEnvtestK8sVersion
	}

	out, err := exec.Command("setup-envtest", "use", "-p", "path", version).Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

// TestEnv bundles the envtest environment, rest config, and k8s client for integration tests.
type TestEnv struct {
	Cfg       *rest.Config
	K8sClient client.Client
	Scheme    *runtime.Scheme
	Env       *envtest.Environment
}

// SetupTestEnv bootstraps a real kube-apiserver and etcd via envtest.
// Registers automatic cleanup via t.Cleanup.
func SetupTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	setLoggerOnce.Do(func() {
		log.SetLogger(logr.Discard())
	})

	assetsDir := ResolveEnvtestAssetsDir()
	crdDir := filepath.Join(FindRepoRoot(), "test", "integration", "crds")

	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{crdDir},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: assetsDir,
	}

	cfg, err := testEnv.Start()
	require.NoError(t, err, "failed to start envtest environment")

	s := NewTestScheme()
	k8sClient, err := client.New(cfg, client.Options{Scheme: s})
	require.NoError(t, err, "failed to create k8s client for envtest")

	t.Cleanup(func() {
		assert.NoError(t, testEnv.Stop(), "failed to stop envtest environment")
	})

	return &TestEnv{
		Cfg:       cfg,
		K8sClient: k8sClient,
		Scheme:    s,
		Env:       testEnv,
	}
}
