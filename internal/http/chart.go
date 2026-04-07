package http

import (
	"net/http"
	"strconv"

	"github.com/wuyaocheng/bktrader/internal/service"
)

// registerChartRoutes 注册 TradingView 图表相关路由（标注数据和 K 线数据）。
func registerChartRoutes(mux *http.ServeMux, platform *service.Platform) {
	// GET /api/v1/chart/annotations — 图表标注数据
	mux.HandleFunc("/api/v1/chart/annotations", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		symbol := service.NormalizeSymbol(r.URL.Query().Get("symbol"))
		from, _ := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
		to, _ := strconv.ParseInt(r.URL.Query().Get("to"), 10, 64)
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		writeJSON(w, http.StatusOK, platform.ListAnnotations(symbol, from, to, limit))
	})

	// GET /api/v1/chart/candles — K 线数据（支持 symbol/resolution/from/to/limit 参数）
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

		candles, err := platform.CandleSeries(symbol, resolution, from, to, limit)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"symbol":     symbol,
			"resolution": resolution,
			"candles":    candles,
		})
	})

	mux.HandleFunc("/api/v1/chart/indicators", func(w http.ResponseWriter, r *http.Request) {
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
		result, err := platform.CandleIndicators(symbol, resolution, from, to, limit)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
}
