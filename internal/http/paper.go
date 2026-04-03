package http

import "net/http"

func registerPaperRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/paper/sessions", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, []map[string]any{
			{
				"id":          "paper-session-main",
				"accountId":   "paper-main",
				"strategyId":  "strategy-bk-1d",
				"status":      "RUNNING",
				"startEquity": 100000.0,
			},
		})
	})
}
