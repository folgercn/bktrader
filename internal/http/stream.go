package http

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"sort"
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
		streamLogger := slog.Default().With(
			"component", "http.dashboard_stream",
			"subscriber_id", subID,
			"remote_addr", r.RemoteAddr,
		)
		streamStats := newDashboardStreamStats()
		defer streamStats.logSummary(streamLogger, "closed", true)

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
				payloadBytes, wireBytes, err := writeDashboardSSEMessage(w, flusher, event)
				if err != nil {
					return
				}
				streamStats.record(event, payloadBytes, wireBytes)
				streamStats.logSummary(streamLogger, "interval", false)
			case <-ticker.C:
				if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	})
}

func writeDashboardSSEMessage(w http.ResponseWriter, flusher http.Flusher, event service.DashboardEvent) (int, int, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return 0, 0, err
	}
	message := fmt.Sprintf("event: %s\ndata: %s\n\n", event.Type, payload)
	written, err := fmt.Fprint(w, message)
	if err != nil {
		return len(payload), written, err
	}
	flusher.Flush()
	return len(payload), written, nil
}

type dashboardStreamEventStats struct {
	Events       int64
	PayloadBytes int64
	WireBytes    int64
	PayloadItems int64
}

type dashboardStreamStats struct {
	startedAt time.Time
	lastLogAt time.Time
	interval  map[string]dashboardStreamEventStats
	total     map[string]dashboardStreamEventStats
}

func newDashboardStreamStats() *dashboardStreamStats {
	now := time.Now()
	return &dashboardStreamStats{
		startedAt: now,
		lastLogAt: now,
		interval:  make(map[string]dashboardStreamEventStats),
		total:     make(map[string]dashboardStreamEventStats),
	}
}

func (s *dashboardStreamStats) record(event service.DashboardEvent, payloadBytes, wireBytes int) {
	key := strings.TrimSpace(event.Type)
	if key == "" {
		key = "unknown"
	}
	itemCount := dashboardPayloadItemCount(event.Payload)
	incrementDashboardStreamStats(s.interval, key, payloadBytes, wireBytes, itemCount)
	incrementDashboardStreamStats(s.total, key, payloadBytes, wireBytes, itemCount)
}

func incrementDashboardStreamStats(stats map[string]dashboardStreamEventStats, key string, payloadBytes, wireBytes, itemCount int) {
	item := stats[key]
	item.Events++
	item.PayloadBytes += int64(payloadBytes)
	item.WireBytes += int64(wireBytes)
	item.PayloadItems += int64(itemCount)
	stats[key] = item
}

func (s *dashboardStreamStats) logSummary(logger *slog.Logger, reason string, force bool) {
	now := time.Now()
	if !force && now.Sub(s.lastLogAt) < 15*time.Second {
		return
	}
	target := s.interval
	if force {
		target = s.total
	}
	if len(target) == 0 {
		s.lastLogAt = now
		return
	}
	for _, eventType := range sortedDashboardStreamEventTypes(target) {
		item := target[eventType]
		logger.Info("dashboard stream payload summary",
			"reason", reason,
			"event_type", eventType,
			"events", item.Events,
			"payload_bytes", item.PayloadBytes,
			"wire_bytes", item.WireBytes,
			"payload_items", item.PayloadItems,
			"elapsed_ms", now.Sub(s.startedAt).Milliseconds(),
		)
	}
	s.interval = make(map[string]dashboardStreamEventStats)
	s.lastLogAt = now
}

func sortedDashboardStreamEventTypes(stats map[string]dashboardStreamEventStats) []string {
	items := make([]string, 0, len(stats))
	for key := range stats {
		items = append(items, key)
	}
	sort.Strings(items)
	return items
}

func dashboardPayloadItemCount(payload any) int {
	if payload == nil {
		return 0
	}
	value := reflect.ValueOf(payload)
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return 0
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return value.Len()
	default:
		return 1
	}
}
