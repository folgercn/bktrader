package http

import "net/http"

func registerOrderRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/orders", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, []map[string]any{
			{
				"id":          "sample-order-1",
				"accountId":   "paper-main",
				"symbol":      "BTCUSDT",
				"side":        "BUY",
				"type":        "MARKET",
				"status":      "FILLED",
				"quantity":    0.01,
				"price":       68000.0,
				"source":      "paper-trading",
				"entryReason": "SL-Reentry",
			},
		})
	})
}
