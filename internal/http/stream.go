package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/service"
)

func registerStreamRoutes(mux *http.ServeMux, platform *service.Platform, cfg config.Config) {
	mux.HandleFunc("/api/v1/stream/dashboard", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "streaming unsupported")
			return
		}

		if cfg.AuthEnabled {
			token := strings.TrimSpace(r.URL.Query().Get("token"))
			if token == "" {
				writeError(w, http.StatusUnauthorized, "stream token required")
				return
			}
			claims, err := parseAuthToken(cfg, token)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid stream token")
				return
			}
			if claims.Scope != "dashboard_stream" {
				writeError(w, http.StatusForbidden, "invalid token scope")
				return
			}
		}

		broker := platform.DashboardBroker()
		if broker == nil {
			writeError(w, http.StatusInternalServerError, "dashboard broker not initialized")
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")



		subID, ch := broker.Subscribe(64)
		defer broker.Unsubscribe(subID)

		_, _ = fmt.Fprint(w, ": connected\n\n")
		flusher.Flush()

		// Push initial snapshot immediately
		broker.PushInitialSnapshot(subID)

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case event, ok := <-ch:
				if !ok {
					return
				}
				if err := writeDashboardSSEMessage(w, flusher, event); err != nil {
					return
				}
			case <-ticker.C:
				if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	})
}

func writeDashboardSSEMessage(w http.ResponseWriter, flusher http.Flusher, event service.DashboardEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, payload); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
