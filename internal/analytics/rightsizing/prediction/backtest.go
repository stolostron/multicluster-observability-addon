package prediction

import (
	"math"
)

// BacktestResult reports validation scores for one model on a held-out slice.
type BacktestResult struct {
	ModelName string
	MAPE      float64
	RMSE      float64
}

// Backtest trains on the first 80% of points and scores one-step-ahead forecasts over the last 20%.
func Backtest(points []DataPoint, cfg ModelConfig) []BacktestResult {
	n := len(points)
	if n < 5 {
		return nil
	}
	split := (n * 8) / 10
	if split < 2 || split >= n {
		return nil
	}
	train := points[:split]
	test := points[split:]
	h := len(test)
	if h == 0 {
		return nil
	}

	hw := NewHoltWintersModel(cfg)
	if err := hw.Fit(train); err != nil {
		return nil
	}
	hPred := hw.Forecast(h)
	stl := NewSTLModel(cfg)
	ar := NewARModel(cfg)

	sPred := stl.Forecast(train, h)
	_ = ar.Fit(train)
	aPred := ar.Forecast(h)

	hm, hr := mapeRMSE(test, hPred)
	sm, sr := mapeRMSE(test, sPred)
	am, ars := mapeRMSE(test, aPred)

	return []BacktestResult{
		{ModelName: "holt_winters", MAPE: hm, RMSE: hr},
		{ModelName: "stl", MAPE: sm, RMSE: sr},
		{ModelName: "ar", MAPE: am, RMSE: ars},
	}
}

func mapeRMSE(test []DataPoint, pred []ForecastResult) (mape, rmse float64) {
	if len(test) == 0 || len(pred) == 0 {
		return 0, 0
	}
	m := min(len(pred), len(test))
	eps := 1e-9
	var sumAPE, sumSE float64
	var count int
	for i := range m {
		a := test[i].Value
		p := pred[i].PredictedValue
		den := math.Abs(a)
		if den < eps {
			den = eps
		}
		sumAPE += math.Abs(a-p) / den
		sumSE += (a - p) * (a - p)
		count++
	}
	if count == 0 {
		return 0, 0
	}
	return sumAPE / float64(count), math.Sqrt(sumSE / float64(count))
}
