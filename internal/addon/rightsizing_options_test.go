package addon

import (
	"testing"

	"github.com/stretchr/testify/require"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
)

func TestIndependentRightSizingOptions(t *testing.T) {
	for _, tt := range []struct {
		name          string
		variables     []addonv1beta1.CustomizedVariable
		workload, pod bool
	}{
		{"defaults", nil, false, false},
		{"workload enabled", []addonv1beta1.CustomizedVariable{{Name: KeyPlatformWorkloadRightSizing, Value: "enabled"}}, true, false},
		{"pod enabled", []addonv1beta1.CustomizedVariable{{Name: KeyPlatformPodRightSizing, Value: "enabled"}}, false, true},
		{"both disabled", []addonv1beta1.CustomizedVariable{{Name: KeyPlatformWorkloadRightSizing, Value: "disabled"}, {Name: KeyPlatformPodRightSizing, Value: "disabled"}}, false, false},
		{"both enabled", []addonv1beta1.CustomizedVariable{{Name: KeyPlatformWorkloadRightSizing, Value: "enabled"}, {Name: KeyPlatformPodRightSizing, Value: "enabled"}}, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := BuildOptions(&addonv1beta1.AddOnDeploymentConfig{Spec: addonv1beta1.AddOnDeploymentConfigSpec{CustomizedVariables: tt.variables}})
			require.NoError(t, err)
			require.Equal(t, tt.workload, opts.Platform.AnalyticsOptions.RightSizing.WorkloadEnabled)
			require.Equal(t, tt.pod, opts.Platform.AnalyticsOptions.RightSizing.PodEnabled)
		})
	}
}
