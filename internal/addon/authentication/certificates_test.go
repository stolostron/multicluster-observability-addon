package authentication

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCertificates_CheckCertManagerCRDs(t *testing.T) {
	fakeKubeClient := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		Build()

	err := checkCertManagerCRDs(context.TODO(), fakeKubeClient)
	require.Error(t, err)
}
