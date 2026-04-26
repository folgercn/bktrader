package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/logging"
	"github.com/wuyaocheng/bktrader/internal/service"
)

// NewRouter 创建并注册所有 HTTP 路由，统一包裹鉴权、CORS 和请求日志中间件。
func NewRouter(cfg config.Config, platform *service.Platform) http.Handler {
	mux := http.NewServeMux()

	// 健康检查端点
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"app":    cfg.AppName,
			"env":    cfg.Environment,
			"time":   time.Now().UTC(),
		})
	})

	registerAuthRoutes(mux, cfg)
	registerLogRoutes(mux, platform)
	registerStreamRoutes(mux, platform, cfg)

	// 系统概览端点
	mux.HandleFunc("/api/v1/overview", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"modules": []string{
				"signal-sources",
				"strategies",
				"accounts",
				"orders",
				"positions",
				"live-monitoring",
				"backtests",
				"chart-feed",
			},
			"notes": "Phase 1 MVP API，可插拔存储、实盘/测试网执行流、账户净值快照、CRUD 风格接口、TradingView 图表脚手架。",
		})
	})

	// 注册各模块路由
	registerSignalRoutes(mux, platform, cfg)
	registerStrategyRoutes(mux, platform)
	registerAccountRoutes(mux, platform)
	registerLiveRoutes(mux, platform, cfg)
	registerLiveRecoveryRoutes(mux, platform)
	registerOrderRoutes(mux, platform)
	registerBacktestRoutes(mux, platform)
	registerChartRoutes(mux, platform)

	// 依次包裹中间件：CORS -> 请求日志 -> 路由
	var handler http.Handler = mux
	handler = authMiddleware(cfg, handler)
	handler = corsMiddleware(handler)
	handler = requestLogMiddleware(handler)
	return handler
}

// corsMiddleware 为所有请求添加 CORS 响应头，允许前端跨域访问。
// 开发环境下允许所有来源，生产环境应在反向代理层控制。
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Expose-Headers", "Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// 预检请求直接返回 204
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// statusRecorder 包装 ResponseWriter 以捕获响应状态码，用于请求日志。
type statusRecorder struct {
	http.ResponseWriter
	statusCode  int
	bytesWrite  int
	wroteHeader bool
}

// WriteHeader 覆写以记录状态码。
func (sr *statusRecorder) WriteHeader(code int) {
	sr.statusCode = code
	sr.wroteHeader = true
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Write(payload []byte) (int, error) {
	if !sr.wroteHeader {
		sr.WriteHeader(sr.statusCode)
	}
	size, err := sr.ResponseWriter.Write(payload)
	sr.bytesWrite += size
	return size, err
}

func (sr *statusRecorder) Flush() {
	if flusher, ok := sr.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// requestLogMiddleware 记录每个 HTTP 请求的方法、路径、状态码和耗时。
func requestLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		logger := slog.Default().With(
			"component", "http",
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)

		defer func() {
			panicMessage := ""
			stackTrace := ""
			if recovered := recover(); recovered != nil {
				recorder.statusCode = http.StatusInternalServerError
				if !recorder.wroteHeader {
					writeError(recorder, http.StatusInternalServerError, "internal server error")
				}
				panicMessage = fmt.Sprint(recovered)
				stackTrace = string(debug.Stack())
				logger.Error("http request panicked",
					"panic", recovered,
					"stack", stackTrace,
				)
			}

			message := "http request completed"
			switch {
			case recorder.statusCode >= 500:
				message = "http request failed"
			case recorder.statusCode >= 400:
				message = "http request rejected"
			}

			logger.Log(context.Background(), logging.HTTPLevel(recorder.statusCode), message,
				"status", recorder.statusCode,
				"duration_ms", time.Since(start).Milliseconds(),
				"bytes_written", recorder.bytesWrite,
				"content_length", r.ContentLength,
			)
			logging.RecordHTTPRequest(logging.HTTPRequestLogEntry{
				Level:         logging.HTTPLevel(recorder.statusCode).String(),
				Message:       message,
				Method:        r.Method,
				Path:          r.URL.Path,
				Query:         r.URL.RawQuery,
				RemoteAddr:    r.RemoteAddr,
				UserAgent:     r.UserAgent(),
				Status:        recorder.statusCode,
				DurationMs:    time.Since(start).Milliseconds(),
				BytesWritten:  recorder.bytesWrite,
				ContentLength: r.ContentLength,
				PanicMessage:  panicMessage,
				Stack:         stackTrace,
				CreatedAt:     time.Now().UTC(),
				Attributes: map[string]any{
					"component": "http",
				},
			})
		}()

		next.ServeHTTP(recorder, r)
	})
}

// writeJSON 序列化 payload 为 JSON 并返回给客户端。
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// decodeJSON 从请求体中解码 JSON 到 target 结构。
func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return errors.New("request body is required")
	}
	return json.Unmarshal(body, target)
}

// writeError 返回统一格式的错误 JSON 响应。
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"error": message,
	})
}

func queryFlagEnabled(r *http.Request, key string) bool {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return false
	}
	enabled, err := strconv.ParseBool(raw)
	return err == nil && enabled
}
