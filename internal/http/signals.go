package http

import "net/http"

func registerSignalRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/signal-sources", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, []map[string]any{
			{
				"id":          "signal-source-bk-1d",
				"name":        "BK 1D ATR Reentry",
				"type":        "internal-strategy",
				"status":      "ACTIVE",
				"dedupeKey":   "symbol+strategyVersion+reason+bar",
				"description": "1D signal / 1m execution strategy feed.",
			},
		})
	})
}
