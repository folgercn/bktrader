package http

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/logging"
	"github.com/wuyaocheng/bktrader/internal/service"
)

const logStreamPollInterval = 2 * time.Second

type streamSnapshotState struct {
	alertIDs     map[string]struct{}
	timelineKeys map[string]map[string]struct{}
}

type timelineStreamEvent struct {
	ID          string         `json:"id"`
	SessionType string         `json:"sessionType"`
	SessionID   string         `json:"sessionId"`
	AccountID   string         `json:"accountId,omitempty"`
	StrategyID  string         `json:"strategyId,omitempty"`
	Category    string         `json:"category,omitempty"`
	Title       string         `json:"title"`
	EventTime   time.Time      `json:"eventTime"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

func registerLogRoutes(mux *http.ServeMux, platform *service.Platform) {
	mux.HandleFunc("/api/v1/logs/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		query, err := parseUnifiedLogEventQuery(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		page, err := platform.ListLogEvents(query)
		if err != nil {
			if strings.Contains(err.Error(), "cursor") {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, page)
	})

	mux.HandleFunc("/api/v1/logs/system", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		from, err := parseOptionalTimeValue(r.URL.Query().Get("from"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid from")
			return
		}
		to, err := parseOptionalTimeValue(r.URL.Query().Get("to"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid to")
			return
		}
		limit, err := parseOptionalPositiveInt(r.URL.Query().Get("limit"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		page, err := logging.ListSystemLogs(logging.SystemLogQuery{
			Level:     r.URL.Query().Get("level"),
			Component: r.URL.Query().Get("component"),
			From:      from,
			To:        to,
			Cursor:    r.URL.Query().Get("cursor"),
			Limit:     limit,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, page)
	})

	mux.HandleFunc("/api/v1/logs/http", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		from, err := parseOptionalTimeValue(r.URL.Query().Get("from"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid from")
			return
		}
		to, err := parseOptionalTimeValue(r.URL.Query().Get("to"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid to")
			return
		}
		limit, err := parseOptionalPositiveInt(r.URL.Query().Get("limit"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		status, err := parseOptionalPositiveInt(r.URL.Query().Get("status"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid status")
			return
		}
		durationMinMs, err := parseOptionalInt64(r.URL.Query().Get("durationMinMs"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid durationMinMs")
			return
		}
		durationMaxMs, err := parseOptionalInt64(r.URL.Query().Get("durationMaxMs"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid durationMaxMs")
			return
		}
		page, err := logging.ListHTTPRequestLogs(logging.HTTPRequestLogQuery{
			Level:         r.URL.Query().Get("level"),
			Method:        r.URL.Query().Get("method"),
			Path:          r.URL.Query().Get("path"),
			Status:        status,
			DurationMinMs: durationMinMs,
			DurationMaxMs: durationMaxMs,
			From:          from,
			To:            to,
			Cursor:        r.URL.Query().Get("cursor"),
			Limit:         limit,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, page)
	})

	mux.HandleFunc("/api/v1/logs/http/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v1/logs/http/"))
		if id == "" || strings.Contains(id, "/") {
			writeError(w, http.StatusNotFound, "http log not found")
			return
		}
		entry, ok := logging.GetHTTPRequestLog(id)
		if !ok {
			writeError(w, http.StatusNotFound, "http log not found")
			return
		}
		writeJSON(w, http.StatusOK, entry)
	})

	mux.HandleFunc("/api/v1/logs/stream", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "streaming unsupported")
			return
		}

		sourceFilter := parseStreamSourceFilter(r.URL.Query().Get("source"))
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		systemID, systemCh := logging.SystemBroker().Subscribe(64)
		httpID, httpCh := logging.HTTPBroker().Subscribe(64)
		eventID, eventCh := platform.SubscribeLogStream(64)
		defer logging.SystemBroker().Unsubscribe(systemID)
		defer logging.HTTPBroker().Unsubscribe(httpID)
		defer platform.UnsubscribeLogStream(eventID)

		_, _ = fmt.Fprint(w, ": connected\n\n")
		flusher.Flush()

		ticker := time.NewTicker(15 * time.Second)
		pollTicker := time.NewTicker(logStreamPollInterval)
		defer ticker.Stop()
		defer pollTicker.Stop()

		streamState := captureStreamSnapshot(platform)

		for {
			select {
			case <-r.Context().Done():
				return
			case message, ok := <-systemCh:
				if !ok {
					systemCh = nil
					continue
				}
				if !matchesStreamSource(sourceFilter, message.Source) {
					continue
				}
				if err := writeSSEMessage(w, flusher, message); err != nil {
					return
				}
			case message, ok := <-httpCh:
				if !ok {
					httpCh = nil
					continue
				}
				if !matchesStreamSource(sourceFilter, message.Source) {
					continue
				}
				if err := writeSSEMessage(w, flusher, message); err != nil {
					return
				}
			case message, ok := <-eventCh:
				if !ok {
					eventCh = nil
					continue
				}
				if !matchesStreamSource(sourceFilter, message.Source) {
					continue
				}
				if err := writeSSEMessage(w, flusher, message); err != nil {
					return
				}
			case <-ticker.C:
				if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
					return
				}
				flusher.Flush()
			case <-pollTicker.C:
				nextState, messages, err := collectPolledStreamMessages(platform, streamState)
				if err != nil {
					continue
				}
				streamState = nextState
				for _, message := range messages {
					if !matchesStreamSource(sourceFilter, message.Source) {
						continue
					}
					if err := writeSSEMessage(w, flusher, message); err != nil {
						return
					}
				}
			}
		}
	})
}

func parseUnifiedLogEventQuery(r *http.Request) (service.UnifiedLogEventQuery, error) {
	from, err := parseOptionalTimeValue(r.URL.Query().Get("from"))
	if err != nil {
		return service.UnifiedLogEventQuery{}, fmt.Errorf("invalid from")
	}
	to, err := parseOptionalTimeValue(r.URL.Query().Get("to"))
	if err != nil {
		return service.UnifiedLogEventQuery{}, fmt.Errorf("invalid to")
	}
	limit, err := parseOptionalPositiveInt(r.URL.Query().Get("limit"))
	if err != nil {
		return service.UnifiedLogEventQuery{}, fmt.Errorf("invalid limit")
	}
	return service.UnifiedLogEventQuery{
		Type:             r.URL.Query().Get("type"),
		Level:            r.URL.Query().Get("level"),
		AccountID:        r.URL.Query().Get("accountId"),
		StrategyID:       r.URL.Query().Get("strategyId"),
		RuntimeSessionID: r.URL.Query().Get("runtimeSessionId"),
		LiveSessionID:    r.URL.Query().Get("liveSessionId"),
		OrderID:          r.URL.Query().Get("orderId"),
		DecisionEventID:  r.URL.Query().Get("decisionEventId"),
		Cursor:           r.URL.Query().Get("cursor"),
		From:             from,
		To:               to,
		Limit:            limit,
	}, nil
}

func writeSSEMessage(w http.ResponseWriter, flusher http.Flusher, message logging.StreamMessage) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", message.Type, payload); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func parseStreamSourceFilter(raw string) map[string]struct{} {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	out := make(map[string]struct{})
	for _, item := range strings.Split(raw, ",") {
		normalized := strings.ToLower(strings.TrimSpace(item))
		if normalized == "" {
			continue
		}
		out[normalized] = struct{}{}
	}
	return out
}

func matchesStreamSource(filters map[string]struct{}, source string) bool {
	if len(filters) == 0 {
		return true
	}
	_, ok := filters[strings.ToLower(strings.TrimSpace(source))]
	return ok
}

func parseOptionalPositiveInt(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < 0 {
		return 0, fmt.Errorf("invalid integer")
	}
	return value, nil
}

func parseOptionalInt64(raw string) (int64, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("invalid integer")
	}
	return value, nil
}

func parseOptionalTimeValue(raw string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, nil
	}
	if unixValue, err := strconv.ParseInt(value, 10, 64); err == nil {
		switch {
		case len(value) >= 13:
			return time.UnixMilli(unixValue).UTC(), nil
		default:
			return time.Unix(unixValue, 0).UTC(), nil
		}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err == nil {
		return parsed.UTC(), nil
	}
	parsed, err = time.Parse(time.RFC3339Nano, value)
	if err == nil {
		return parsed.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid time")
}

func captureStreamSnapshot(platform *service.Platform) streamSnapshotState {
	state := streamSnapshotState{
		alertIDs:     make(map[string]struct{}),
		timelineKeys: make(map[string]map[string]struct{}),
	}

	if alerts, err := platform.ListAlerts(); err == nil {
		for _, alert := range alerts {
			if strings.TrimSpace(alert.ID) != "" {
				state.alertIDs[alert.ID] = struct{}{}
			}
		}
	}
	for _, item := range snapshotLiveTimelines(platform) {
		registerTimelineKey(state.timelineKeys, item.SessionType, item.SessionID, item.ID)
	}
	for _, item := range snapshotRuntimeTimelines(platform) {
		registerTimelineKey(state.timelineKeys, item.SessionType, item.SessionID, item.ID)
	}
	return state
}

func collectPolledStreamMessages(platform *service.Platform, previous streamSnapshotState) (streamSnapshotState, []logging.StreamMessage, error) {
	next := streamSnapshotState{
		alertIDs:     make(map[string]struct{}),
		timelineKeys: make(map[string]map[string]struct{}),
	}
	messages := make([]logging.StreamMessage, 0)

	alerts, err := platform.ListAlerts()
	if err != nil {
		return previous, nil, err
	}
	newAlerts := make([]domain.PlatformAlert, 0)
	for _, alert := range alerts {
		if strings.TrimSpace(alert.ID) == "" {
			continue
		}
		next.alertIDs[alert.ID] = struct{}{}
		if _, seen := previous.alertIDs[alert.ID]; !seen {
			newAlerts = append(newAlerts, alert)
		}
	}
	sort.SliceStable(newAlerts, func(i, j int) bool {
		if newAlerts[i].EventTime.Equal(newAlerts[j].EventTime) {
			return newAlerts[i].ID < newAlerts[j].ID
		}
		return newAlerts[i].EventTime.Before(newAlerts[j].EventTime)
	})
	for _, alert := range newAlerts {
		messages = append(messages, logging.StreamMessage{
			ID:         alert.ID,
			Source:     "alert",
			Type:       "alert",
			Level:      strings.ToLower(strings.TrimSpace(alert.Level)),
			EventTime:  alert.EventTime.UTC(),
			RecordedAt: alert.EventTime.UTC(),
			Payload:    alert,
		})
	}

	timelineEvents := append(snapshotLiveTimelines(platform), snapshotRuntimeTimelines(platform)...)
	sort.SliceStable(timelineEvents, func(i, j int) bool {
		if timelineEvents[i].EventTime.Equal(timelineEvents[j].EventTime) {
			return timelineEvents[i].ID < timelineEvents[j].ID
		}
		return timelineEvents[i].EventTime.Before(timelineEvents[j].EventTime)
	})
	for _, item := range timelineEvents {
		registerTimelineKey(next.timelineKeys, item.SessionType, item.SessionID, item.ID)
		if hasTimelineKey(previous.timelineKeys, item.SessionType, item.SessionID, item.ID) {
			continue
		}
		messages = append(messages, logging.StreamMessage{
			ID:         item.ID,
			Source:     "timeline",
			Type:       "timeline",
			Level:      timelineLevel(item.Category, item.Title),
			EventTime:  item.EventTime.UTC(),
			RecordedAt: item.EventTime.UTC(),
			Payload:    item,
		})
	}
	return next, messages, nil
}

func snapshotLiveTimelines(platform *service.Platform) []timelineStreamEvent {
	sessions, err := platform.ListLiveSessions()
	if err != nil {
		return nil
	}
	items := make([]timelineStreamEvent, 0)
	for _, session := range sessions {
		items = append(items, timelineEventsFromState("live", session.ID, session.AccountID, session.StrategyID, session.State)...)
	}
	return items
}

func snapshotRuntimeTimelines(platform *service.Platform) []timelineStreamEvent {
	sessions := platform.ListSignalRuntimeSessions()
	items := make([]timelineStreamEvent, 0)
	for _, session := range sessions {
		items = append(items, timelineEventsFromState("runtime", session.ID, session.AccountID, session.StrategyID, session.State)...)
	}
	return items
}

func timelineEventsFromState(sessionType, sessionID, accountID, strategyID string, state map[string]any) []timelineStreamEvent {
	rawItems, ok := state["timeline"].([]any)
	if !ok || len(rawItems) == 0 {
		return nil
	}
	items := make([]timelineStreamEvent, 0, len(rawItems))
	for _, raw := range rawItems {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		eventTime, ok := parseTimelineTime(entry["time"])
		if !ok {
			continue
		}
		metadata := httpLogMapValue(entry["metadata"])
		item := timelineStreamEvent{
			SessionType: sessionType,
			SessionID:   sessionID,
			AccountID:   accountID,
			StrategyID:  strategyID,
			Category:    httpLogStringValue(entry["category"]),
			Title:       httpLogStringValue(entry["title"]),
			EventTime:   eventTime,
			Metadata:    cloneMap(metadata),
		}
		item.ID = buildTimelineEventID(item)
		items = append(items, item)
	}
	return items
}

func buildTimelineEventID(item timelineStreamEvent) string {
	payload, _ := json.Marshal(map[string]any{
		"sessionType": item.SessionType,
		"sessionId":   item.SessionID,
		"category":    item.Category,
		"title":       item.Title,
		"eventTime":   item.EventTime.UTC().Format(time.RFC3339Nano),
		"metadata":    item.Metadata,
	})
	sum := sha1.Sum(payload)
	return fmt.Sprintf("timeline-%s-%s-%x", item.SessionType, item.SessionID, sum[:6])
}

func registerTimelineKey(index map[string]map[string]struct{}, sessionType, sessionID, key string) {
	composite := sessionType + ":" + sessionID
	if _, ok := index[composite]; !ok {
		index[composite] = make(map[string]struct{})
	}
	index[composite][key] = struct{}{}
}

func hasTimelineKey(index map[string]map[string]struct{}, sessionType, sessionID, key string) bool {
	composite := sessionType + ":" + sessionID
	items, ok := index[composite]
	if !ok {
		return false
	}
	_, exists := items[key]
	return exists
}

func parseTimelineTime(value any) (time.Time, bool) {
	text := strings.TrimSpace(httpLogStringValue(value))
	if text == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, text)
	if err == nil {
		return parsed.UTC(), true
	}
	parsed, err = time.Parse(time.RFC3339Nano, text)
	if err == nil {
		return parsed.UTC(), true
	}
	return time.Time{}, false
}

func timelineLevel(category, title string) string {
	normalizedCategory := strings.ToLower(strings.TrimSpace(category))
	normalizedTitle := strings.ToLower(strings.TrimSpace(title))
	switch {
	case normalizedCategory == "error":
		return "critical"
	case strings.Contains(normalizedTitle, "error"), strings.Contains(normalizedTitle, "fail"):
		return "warning"
	default:
		return "info"
	}
}

func httpLogMapValue(value any) map[string]any {
	if item, ok := value.(map[string]any); ok {
		return item
	}
	return nil
}

func httpLogStringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(payload, &out); err != nil {
		return map[string]any{}
	}
	return out
}
