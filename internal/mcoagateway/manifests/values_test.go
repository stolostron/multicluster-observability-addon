package manifests

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildValues(t *testing.T) {
	t.Run("spoke is skipped", func(t *testing.T) {
		assert.Nil(t, BuildValues(false, true, false, true))
	})

	t.Run("hub without annotation or logs is skipped", func(t *testing.T) {
		assert.Nil(t, BuildValues(true, false, false, false))
	})

	t.Run("default logging enables obs-api on the hub", func(t *testing.T) {
		got := BuildValues(true, false, false, true)
		if assert.NotNil(t, got) {
			assert.True(t, got.Enabled)
			assert.True(t, got.LogsEnabled)
			assert.Equal(t, legacyReceiveEndpoint, got.MetricsWriteEndpoint)
			assert.Equal(t, legacyReadEndpoint, got.MetricsReadEndpoint)
		}
	})

	t.Run("annotation enables metrics endpoints without logs", func(t *testing.T) {
		got := BuildValues(true, true, true, false)
		if assert.NotNil(t, got) {
			assert.True(t, got.Enabled)
			assert.False(t, got.LogsEnabled)
			assert.Equal(t, mcoaReceiveEndpoint, got.MetricsWriteEndpoint)
			assert.Equal(t, mcoaReadEndpoint, got.MetricsReadEndpoint)
		}
	})
}
