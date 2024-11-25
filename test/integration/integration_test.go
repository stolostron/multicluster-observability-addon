package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	testEnvHub *envtest.Environment
	restCfgHub *rest.Config
)

func TestMain(m *testing.M) {
	opts := zap.Options{
		Development: true,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	var err error

	testEnvHub = &envtest.Environment{
		CRDDirectoryPaths:       []string{filepath.Join("..", "..", "tmp", "crds"), filepath.Join("..", "..", "deploy", "crds")},
		ControlPlaneStopTimeout: 5 * time.Minute,
	}

	restCfgHub, err = testEnvHub.Start()
	if err != nil {
		panic(fmt.Sprintf("Failed to start hub test environment: %v", err))
	}

	code := m.Run()

	err = testEnvHub.Stop()
	if err != nil {
		panic(fmt.Sprintf("Failed to stop hub test environment: %v", err))
	}

	os.Exit(code)
}
