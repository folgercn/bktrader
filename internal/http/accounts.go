package http

import "net/http"

func registerAccountRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/accounts", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, []map[string]any{
			{"id": "paper-main", "name": "Paper Main", "mode": "PAPER", "exchange": "binance-futures", "status": "READY"},
			{"id": "live-main", "name": "Live Main", "mode": "LIVE", "exchange": "binance-futures", "status": "PENDING_SETUP"},
		})
	})
}
