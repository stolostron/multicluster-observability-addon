package features

import (
	"time"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/prediction"
)

func extractTemporalFeatures(points []prediction.DataPoint, cfg FeatureConfig, out *FeatureVector) {
	if len(points) == 0 {
		return
	}
	ts := points[len(points)-1].Timestamp.In(time.UTC)
	out.HourOfDay = float64(ts.Hour()) + float64(ts.Minute())/60.0 + float64(ts.Second())/3600.0
	out.DayOfWeek = float64(ts.Weekday())

	h := ts.Hour()
	if cfg.BusinessHoursStart <= cfg.BusinessHoursEnd {
		if h >= cfg.BusinessHoursStart && h < cfg.BusinessHoursEnd {
			out.IsBusinessHours = 1.0
		} else {
			out.IsBusinessHours = 0.0
		}
	} else {
		// overnight range, e.g. 22–6
		if h >= cfg.BusinessHoursStart || h < cfg.BusinessHoursEnd {
			out.IsBusinessHours = 1.0
		} else {
			out.IsBusinessHours = 0.0
		}
	}

	wd := ts.Weekday()
	if wd == time.Saturday || wd == time.Sunday {
		out.IsWeekend = 1.0
	} else {
		out.IsWeekend = 0.0
	}

	// Week of month: ceil(day/7), 1-based
	day := ts.Day()
	out.WeekOfMonth = float64((day-1)/7 + 1)
}
