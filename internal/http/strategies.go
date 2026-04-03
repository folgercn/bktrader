package http

import "net/http"

func registerStrategyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/strategies", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, []map[string]any{
			{
				"id":     "strategy-bk-1d",
				"name":   "BK 1D Zero Initial",
				"status": "ACTIVE",
				"currentVersion": map[string]any{
					"version":            "v0.1.0",
					"signalTimeframe":    "1D",
					"executionTimeframe": "1m",
					"maxTradesPerBar":    3,
					"reentrySizes":       []float64{0.10, 0.20},
					"stopMode":           "atr",
					"profitProtectATR":   1.0,
				},
			},
		})
	})
}
