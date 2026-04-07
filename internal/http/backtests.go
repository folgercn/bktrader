package http

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"

	"github.com/wuyaocheng/bktrader/internal/service"
)

// registerBacktestRoutes 注册回测管理相关路由。
func registerBacktestRoutes(mux *http.ServeMux, platform *service.Platform) {
	mux.HandleFunc("/api/v1/backtests/options", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, platform.BacktestOptions())
	})

	// GET|POST /api/v1/backtests — 回测记录列表/创建回测
	mux.HandleFunc("/api/v1/backtests", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			items, err := platform.ListBacktests()
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, items)
		case http.MethodPost:
			var payload struct {
				StrategyVersionID string         `json:"strategyVersionId"`
				Parameters        map[string]any `json:"parameters"`
			}
			if err := decodeJSON(r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			if err := service.ValidateRequired(map[string]string{"strategyVersionId": payload.StrategyVersionID}, "strategyVersionId"); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			item, err := platform.CreateBacktest(payload.StrategyVersionID, payload.Parameters)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, item)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/backtests/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/backtests/")
		if !strings.HasSuffix(path, "/execution-trades.csv") {
			writeError(w, http.StatusNotFound, "backtest export not found")
			return
		}
		backtestID := strings.TrimSuffix(path, "/execution-trades.csv")
		backtestID = strings.Trim(backtestID, "/")
		if backtestID == "" {
			writeError(w, http.StatusBadRequest, "backtest id is required")
			return
		}

		item, err := platform.GetBacktest(backtestID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		trades, ok := item.ResultSummary["executionTrades"].([]any)
		if !ok {
			if typed, okTyped := item.ResultSummary["executionTrades"].([]map[string]any); okTyped {
				writeExecutionTradesCSV(w, backtestID, typed)
				return
			}
			writeExecutionTradesCSV(w, backtestID, []map[string]any{})
			return
		}

		rows := make([]map[string]any, 0, len(trades))
		for _, trade := range trades {
			if row, okRow := trade.(map[string]any); okRow {
				rows = append(rows, row)
			}
		}
		writeExecutionTradesCSV(w, backtestID, rows)
	})
}

func writeExecutionTradesCSV(w http.ResponseWriter, backtestID string, trades []map[string]any) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+backtestID+`-execution-trades.csv"`)
	w.WriteHeader(http.StatusOK)

	writer := csv.NewWriter(w)
	defer writer.Flush()

	_ = writer.Write([]string{
		"source",
		"side",
		"quantity",
		"entry_target",
		"stop_loss",
		"take_profit",
		"entry_time",
		"entry_price",
		"exit_time",
		"exit_price",
		"exit_type",
		"realized_pnl",
		"processed_bars",
		"status",
	})

	for _, trade := range trades {
		_ = writer.Write([]string{
			stringValue(trade["source"]),
			stringValue(trade["side"]),
			stringValue(trade["quantity"]),
			stringValue(trade["entryTarget"]),
			stringValue(trade["stopLoss"]),
			stringValue(trade["takeProfit"]),
			stringValue(trade["entryTime"]),
			stringValue(trade["entryPrice"]),
			stringValue(trade["exitTime"]),
			stringValue(trade["exitPrice"]),
			stringValue(trade["exitType"]),
			stringValue(trade["realizedPnL"]),
			stringValue(trade["processedBars"]),
			stringValue(trade["status"]),
		})
	}
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
