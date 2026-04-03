package http

import "net/http"

func registerChartRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/chart/annotations", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, []map[string]any{
			{
				"id":     "anno-1",
				"source": "backtest",
				"type":   "entry_long",
				"symbol": "BTCUSDT",
				"time":   "2024-02-05T14:21:00Z",
				"price":  43125.0,
				"label":  "SL-Reentry",
			},
			{
				"id":     "anno-2",
				"source": "backtest",
				"type":   "exit_tp",
				"symbol": "BTCUSDT",
				"time":   "2024-02-17T10:12:00Z",
				"price":  52520.0,
				"label":  "PT",
			},
		})
	})
}
