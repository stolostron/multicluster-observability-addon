package features

import (
	"math"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

// ExtractCorrelation returns the Pearson correlation between two aligned series.
func ExtractCorrelation(cpuPoints, memPoints []prediction.DataPoint) float64 {
	n := len(cpuPoints)
	if len(memPoints) < n {
		n = len(memPoints)
	}
	if n < 2 {
		return 0
	}
	var sumX, sumY float64
	for i := 0; i < n; i++ {
		sumX += cpuPoints[i].Value
		sumY += memPoints[i].Value
	}
	meanX := sumX / float64(n)
	meanY := sumY / float64(n)
	var num, denX, denY float64
	for i := 0; i < n; i++ {
		dx := cpuPoints[i].Value - meanX
		dy := memPoints[i].Value - meanY
		num += dx * dy
		denX += dx * dx
		denY += dy * dy
	}
	if denX <= 0 || denY <= 0 {
		return 0
	}
	return num / math.Sqrt(denX*denY)
}
