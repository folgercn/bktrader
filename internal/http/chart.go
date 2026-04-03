package http

import (
	"net/http"
	"strconv"

	"github.com/wuyaocheng/bktrader/internal/service"
)

func registerChartRoutes(mux *http.ServeMux, platform *service.Platform) {
	mux.HandleFunc("/api/v1/chart/annotations", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		symbol := service.NormalizeSymbol(r.URL.Query().Get("symbol"))
		writeJSON(w, http.StatusOK, platform.ListAnnotations(symbol))
	})

	mux.HandleFunc("/api/v1/chart/candles", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		query := r.URL.Query()
		symbol := service.NormalizeSymbol(query.Get("symbol"))
		resolution := query.Get("resolution")
		from, _ := strconv.ParseInt(query.Get("from"), 10, 64)
		to, _ := strconv.ParseInt(query.Get("to"), 10, 64)
		limit, _ := strconv.Atoi(query.Get("limit"))

		writeJSON(w, http.StatusOK, map[string]any{
			"symbol":     symbol,
			"resolution": resolution,
			"candles":    platform.CandleSeries(symbol, resolution, from, to, limit),
		})
	})
}
