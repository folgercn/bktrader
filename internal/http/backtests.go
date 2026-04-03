package http

import "net/http"

func registerBacktestRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/backtests", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, []map[string]any{
			{
				"id":         "backtest-20260403-001",
				"strategyId": "strategy-bk-1d",
				"status":     "COMPLETED",
				"summary": map[string]any{
					"return":       1.51,
					"maxDrawdown":  -0.0055,
					"tradePairs":   1098,
					"sampleWindow": "2020-01-01 ~ 2026-02-28",
				},
			},
		})
	})
}
