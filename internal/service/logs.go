package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/logging"
)

const (
	defaultLogEventLimit = 100
	maxLogEventLimit     = 200
)

// UnifiedLogEvent 表示后端统一聚合后的业务事件记录。
type UnifiedLogEvent struct {
	ID               string         `json:"id"`
	Source           string         `json:"source"`
	Type             string         `json:"type"`
	Level            string         `json:"level"`
	Title            string         `json:"title"`
	Message          string         `json:"message"`
	EventTime        time.Time      `json:"eventTime"`
	RecordedAt       time.Time      `json:"recordedAt"`
	AccountID        string         `json:"accountId,omitempty"`
	StrategyID       string         `json:"strategyId,omitempty"`
	RuntimeSessionID string         `json:"runtimeSessionId,omitempty"`
	LiveSessionID    string         `json:"liveSessionId,omitempty"`
	OrderID          string         `json:"orderId,omitempty"`
	DecisionEventID  string         `json:"decisionEventId,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	Payload          any            `json:"payload,omitempty"`
}

// UnifiedLogEventPage 表示业务事件查询分页结果。
type UnifiedLogEventPage struct {
	Items      []UnifiedLogEvent `json:"items"`
	NextCursor string            `json:"nextCursor,omitempty"`
}

// UnifiedLogEventQuery 定义统一业务事件接口的过滤条件。
type UnifiedLogEventQuery struct {
	Type             string
	Level            string
	AccountID        string
	StrategyID       string
	RuntimeSessionID string
	LiveSessionID    string
	OrderID          string
	DecisionEventID  string
	Cursor           string
	From             time.Time
	To               time.Time
	Limit            int
}

// LiveControlMetrics aggregates the bounded live control audit history into an
// operator-facing summary. It is intentionally derived from state.controlEvents
// so Phase 4 observability does not introduce a new persistence surface.
type LiveControlMetrics struct {
	GeneratedAt                 time.Time                         `json:"generatedAt"`
	TotalEvents                 int                               `json:"totalEvents"`
	Requests                    int                               `json:"requests"`
	RunnerPickups               int                               `json:"runnerPickups"`
	Succeeded                   int                               `json:"succeeded"`
	Failed                      int                               `json:"failed"`
	StaleDiscarded              int                               `json:"staleDiscarded"`
	CurrentPending              int                               `json:"currentPending"`
	CurrentErrors               int                               `json:"currentErrors"`
	CurrentMaxPendingPickupMs   int64                             `json:"currentMaxPendingPickupMs,omitempty"`
	StaleActiveControlRequests  int                               `json:"staleActiveControlRequests,omitempty"`
	OrphanActiveControlRequests int                               `json:"orphanActiveControlRequests,omitempty"`
	Scanner                     LiveControlScannerStatus          `json:"scanner"`
	Latency                     LiveControlLatencyMetrics         `json:"latency"`
	ByErrorCode                 map[string]int                    `json:"byErrorCode,omitempty"`
	ByAccount                   map[string]LiveControlMetricGroup `json:"byAccount,omitempty"`
	ByStrategy                  map[string]LiveControlMetricGroup `json:"byStrategy,omitempty"`
	ByLiveSession               map[string]LiveControlMetricGroup `json:"byLiveSession,omitempty"`
}

type LiveControlLatencyMetrics struct {
	PickupMs   LiveControlLatencyStats `json:"pickupMs"`
	SuccessMs  LiveControlLatencyStats `json:"successMs"`
	FailureMs  LiveControlLatencyStats `json:"failureMs"`
	TerminalMs LiveControlLatencyStats `json:"terminalMs"`
}

type LiveControlLatencyStats struct {
	Count   int     `json:"count"`
	Min     int64   `json:"min,omitempty"`
	Max     int64   `json:"max,omitempty"`
	Average float64 `json:"average,omitempty"`
}

type LiveControlMetricGroup struct {
	Total          int            `json:"total"`
	Requests       int            `json:"requests"`
	RunnerPickups  int            `json:"runnerPickups"`
	Succeeded      int            `json:"succeeded"`
	Failed         int            `json:"failed"`
	StaleDiscarded int            `json:"staleDiscarded"`
	ErrorCodes     map[string]int `json:"errorCodes,omitempty"`
}

type logEventCursor struct {
	EventTime  string `json:"eventTime"`
	RecordedAt string `json:"recordedAt"`
	ID         string `json:"id"`
}

type strategyDecisionEventQueryReader interface {
	QueryStrategyDecisionEvents(query domain.StrategyDecisionEventQuery) ([]domain.StrategyDecisionEvent, error)
}

type orderExecutionEventQueryReader interface {
	QueryOrderExecutionEvents(query domain.OrderExecutionEventQuery) ([]domain.OrderExecutionEvent, error)
}

type positionAccountSnapshotQueryReader interface {
	QueryPositionAccountSnapshots(query domain.PositionAccountSnapshotQuery) ([]domain.PositionAccountSnapshot, error)
}

func (p *Platform) SubscribeLogStream(buffer int) (int, <-chan logging.StreamMessage) {
	if p.logBroker == nil {
		p.logBroker = logging.NewBroker()
	}
	return p.logBroker.Subscribe(buffer)
}

func (p *Platform) UnsubscribeLogStream(id int) {
	if p.logBroker == nil {
		return
	}
	p.logBroker.Unsubscribe(id)
}

func (p *Platform) publishLogEvent(item UnifiedLogEvent) {
	if p.logBroker == nil {
		return
	}
	p.logBroker.Publish(logging.StreamMessage{
		ID:         item.ID,
		Source:     item.Source,
		Type:       item.Type,
		Level:      item.Level,
		EventTime:  item.EventTime,
		RecordedAt: item.RecordedAt,
		Payload:    item,
	})
}

func (p *Platform) ListLogEvents(query UnifiedLogEventQuery) (UnifiedLogEventPage, error) {
	normalizedType := normalizeUnifiedLogEventType(query.Type)
	levelFilter := normalizeUnifiedLogLevel(query.Level)
	cursor, err := decodeLogEventCursor(query.Cursor)
	if err != nil {
		return UnifiedLogEventPage{}, err
	}
	limit := normalizeUnifiedLogLimit(query.Limit)
	fetchLimit := limit + 1
	if levelFilter != "" {
		fetchLimit = min(limit*3+1, 600)
	}

	var merged []UnifiedLogEvent
	if normalizedType == "" || normalizedType == "strategy-decision" {
		items, err := p.queryStrategyDecisionEvents(domain.StrategyDecisionEventQuery{
			AccountID:        strings.TrimSpace(query.AccountID),
			StrategyID:       strings.TrimSpace(query.StrategyID),
			RuntimeSessionID: strings.TrimSpace(query.RuntimeSessionID),
			LiveSessionID:    strings.TrimSpace(query.LiveSessionID),
			DecisionEventID:  strings.TrimSpace(query.DecisionEventID),
			From:             query.From,
			To:               query.To,
			Before:           cursor,
			Limit:            fetchLimit,
		})
		if err != nil {
			return UnifiedLogEventPage{}, err
		}
		for _, item := range items {
			event := strategyDecisionToUnifiedLogEvent(item)
			if matchesUnifiedLogLevel(event.Level, levelFilter) {
				merged = append(merged, event)
			}
		}
	}
	if normalizedType == "" || normalizedType == "order-execution" {
		items, err := p.queryOrderExecutionEvents(domain.OrderExecutionEventQuery{
			AccountID:        strings.TrimSpace(query.AccountID),
			StrategyID:       strings.TrimSpace(query.StrategyID),
			RuntimeSessionID: strings.TrimSpace(query.RuntimeSessionID),
			LiveSessionID:    strings.TrimSpace(query.LiveSessionID),
			OrderID:          strings.TrimSpace(query.OrderID),
			DecisionEventID:  strings.TrimSpace(query.DecisionEventID),
			From:             query.From,
			To:               query.To,
			Before:           cursor,
			Limit:            fetchLimit,
		})
		if err != nil {
			return UnifiedLogEventPage{}, err
		}
		for _, item := range items {
			event := orderExecutionToUnifiedLogEvent(item)
			if matchesUnifiedLogLevel(event.Level, levelFilter) {
				merged = append(merged, event)
			}
		}
	}
	if normalizedType == "" || normalizedType == "position-account-snapshot" {
		items, err := p.queryPositionAccountSnapshots(domain.PositionAccountSnapshotQuery{
			AccountID:       strings.TrimSpace(query.AccountID),
			StrategyID:      strings.TrimSpace(query.StrategyID),
			LiveSessionID:   strings.TrimSpace(query.LiveSessionID),
			OrderID:         strings.TrimSpace(query.OrderID),
			DecisionEventID: strings.TrimSpace(query.DecisionEventID),
			From:            query.From,
			To:              query.To,
			Before:          cursor,
			Limit:           fetchLimit,
		})
		if err != nil {
			return UnifiedLogEventPage{}, err
		}
		for _, item := range items {
			event := positionSnapshotToUnifiedLogEvent(item)
			if matchesUnifiedLogLevel(event.Level, levelFilter) {
				merged = append(merged, event)
			}
		}
	}
	if normalizedType == "" || normalizedType == "live-control" {
		items, err := p.queryLiveSessionControlEvents(query, cursor, fetchLimit)
		if err != nil {
			return UnifiedLogEventPage{}, err
		}
		for _, event := range items {
			if matchesUnifiedLogLevel(event.Level, levelFilter) {
				merged = append(merged, event)
			}
		}
	}

	sort.SliceStable(merged, func(i, j int) bool {
		return unifiedLogEventLess(merged[i], merged[j])
	})
	page := UnifiedLogEventPage{Items: merged}
	if len(merged) > limit {
		page.NextCursor = encodeLogEventCursor(merged[limit-1])
		page.Items = slices.Clone(merged[:limit])
	}
	return page, nil
}

func (p *Platform) queryStrategyDecisionEvents(query domain.StrategyDecisionEventQuery) ([]domain.StrategyDecisionEvent, error) {
	if reader, ok := p.store.(strategyDecisionEventQueryReader); ok {
		return reader.QueryStrategyDecisionEvents(query)
	}
	items, err := p.store.ListStrategyDecisionEvents("")
	if err != nil {
		return nil, err
	}
	filtered := make([]domain.StrategyDecisionEvent, 0, len(items))

	decisionEventIDMap := make(map[string]struct{})
	for _, id := range query.DecisionEventIDs {
		decisionEventIDMap[strings.TrimSpace(id)] = struct{}{}
	}

	for _, item := range items {
		if strings.TrimSpace(query.LiveSessionID) != "" && item.LiveSessionID != strings.TrimSpace(query.LiveSessionID) {
			continue
		}
		if strings.TrimSpace(query.AccountID) != "" && item.AccountID != strings.TrimSpace(query.AccountID) {
			continue
		}
		if strings.TrimSpace(query.StrategyID) != "" && item.StrategyID != strings.TrimSpace(query.StrategyID) {
			continue
		}
		if strings.TrimSpace(query.RuntimeSessionID) != "" && item.RuntimeSessionID != strings.TrimSpace(query.RuntimeSessionID) {
			continue
		}
		if strings.TrimSpace(query.DecisionEventID) != "" && item.ID != strings.TrimSpace(query.DecisionEventID) {
			continue
		}
		if len(query.DecisionEventIDs) > 0 {
			if _, ok := decisionEventIDMap[item.ID]; !ok {
				continue
			}
		}
		if !query.From.IsZero() && item.EventTime.Before(query.From.UTC()) {
			continue
		}
		if !query.To.IsZero() && item.EventTime.After(query.To.UTC()) {
			continue
		}
		if query.Before != nil && !domain.EventBeforeCursor(item.EventTime, item.RecordedAt, item.ID, *query.Before) {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return domain.EventLessDesc(filtered[i].EventTime, filtered[i].RecordedAt, filtered[i].ID, filtered[j].EventTime, filtered[j].RecordedAt, filtered[j].ID)
	})
	return limitStrategyDecisionEvents(filtered, query.Limit), nil
}

func (p *Platform) queryOrderExecutionEvents(query domain.OrderExecutionEventQuery) ([]domain.OrderExecutionEvent, error) {
	if reader, ok := p.store.(orderExecutionEventQueryReader); ok {
		return reader.QueryOrderExecutionEvents(query)
	}
	items, err := p.store.ListOrderExecutionEvents("")
	if err != nil {
		return nil, err
	}
	filtered := make([]domain.OrderExecutionEvent, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(query.AccountID) != "" && item.AccountID != strings.TrimSpace(query.AccountID) {
			continue
		}
		if strings.TrimSpace(query.StrategyID) != "" && stringValue(item.Metadata["strategyId"]) != strings.TrimSpace(query.StrategyID) {
			continue
		}
		if strings.TrimSpace(query.RuntimeSessionID) != "" && item.RuntimeSessionID != strings.TrimSpace(query.RuntimeSessionID) {
			continue
		}
		if strings.TrimSpace(query.LiveSessionID) != "" && item.LiveSessionID != strings.TrimSpace(query.LiveSessionID) {
			continue
		}
		if strings.TrimSpace(query.OrderID) != "" && item.OrderID != strings.TrimSpace(query.OrderID) {
			continue
		}
		if strings.TrimSpace(query.DecisionEventID) != "" && item.DecisionEventID != strings.TrimSpace(query.DecisionEventID) {
			continue
		}
		if !query.From.IsZero() && item.EventTime.Before(query.From.UTC()) {
			continue
		}
		if !query.To.IsZero() && item.EventTime.After(query.To.UTC()) {
			continue
		}
		if query.Before != nil && !domain.EventBeforeCursor(item.EventTime, item.RecordedAt, item.ID, *query.Before) {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return domain.EventLessDesc(filtered[i].EventTime, filtered[i].RecordedAt, filtered[i].ID, filtered[j].EventTime, filtered[j].RecordedAt, filtered[j].ID)
	})
	return limitOrderExecutionEvents(filtered, query.Limit), nil
}

func (p *Platform) queryPositionAccountSnapshots(query domain.PositionAccountSnapshotQuery) ([]domain.PositionAccountSnapshot, error) {
	if reader, ok := p.store.(positionAccountSnapshotQueryReader); ok {
		return reader.QueryPositionAccountSnapshots(query)
	}
	items, err := p.store.ListPositionAccountSnapshots("")
	if err != nil {
		return nil, err
	}
	filtered := make([]domain.PositionAccountSnapshot, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(query.AccountID) != "" && item.AccountID != strings.TrimSpace(query.AccountID) {
			continue
		}
		if strings.TrimSpace(query.StrategyID) != "" && item.StrategyID != strings.TrimSpace(query.StrategyID) {
			continue
		}
		if strings.TrimSpace(query.LiveSessionID) != "" && item.LiveSessionID != strings.TrimSpace(query.LiveSessionID) {
			continue
		}
		if strings.TrimSpace(query.OrderID) != "" && item.OrderID != strings.TrimSpace(query.OrderID) {
			continue
		}
		if strings.TrimSpace(query.DecisionEventID) != "" && item.DecisionEventID != strings.TrimSpace(query.DecisionEventID) {
			continue
		}
		if !query.From.IsZero() && item.EventTime.Before(query.From.UTC()) {
			continue
		}
		if !query.To.IsZero() && item.EventTime.After(query.To.UTC()) {
			continue
		}
		if query.Before != nil && !domain.EventBeforeCursor(item.EventTime, item.RecordedAt, item.ID, *query.Before) {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return domain.EventLessDesc(filtered[i].EventTime, filtered[i].RecordedAt, filtered[i].ID, filtered[j].EventTime, filtered[j].RecordedAt, filtered[j].ID)
	})
	return limitPositionSnapshots(filtered, query.Limit), nil
}

func (p *Platform) queryLiveSessionControlEvents(query UnifiedLogEventQuery, cursor *domain.EventCursor, limit int) ([]UnifiedLogEvent, error) {
	sessions, err := p.store.ListLiveSessions()
	if err != nil {
		return nil, err
	}
	filtered := make([]UnifiedLogEvent, 0)
	for _, session := range sessions {
		if strings.TrimSpace(query.AccountID) != "" && session.AccountID != strings.TrimSpace(query.AccountID) {
			continue
		}
		if strings.TrimSpace(query.StrategyID) != "" && session.StrategyID != strings.TrimSpace(query.StrategyID) {
			continue
		}
		if strings.TrimSpace(query.LiveSessionID) != "" && session.ID != strings.TrimSpace(query.LiveSessionID) {
			continue
		}
		for _, entry := range metadataList(session.State[liveSessionControlEventStateKey]) {
			event, ok := liveSessionControlToUnifiedLogEvent(session, entry)
			if !ok {
				continue
			}
			if !query.From.IsZero() && event.EventTime.Before(query.From.UTC()) {
				continue
			}
			if !query.To.IsZero() && event.EventTime.After(query.To.UTC()) {
				continue
			}
			if cursor != nil && !domain.EventBeforeCursor(event.EventTime, event.RecordedAt, event.ID, *cursor) {
				continue
			}
			filtered = append(filtered, event)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return unifiedLogEventLess(filtered[i], filtered[j])
	})
	if limit > 0 && len(filtered) > limit {
		return slices.Clone(filtered[:limit]), nil
	}
	return filtered, nil
}

func (p *Platform) LiveControlMetrics(query UnifiedLogEventQuery) (LiveControlMetrics, error) {
	sessions, err := p.store.ListLiveSessions()
	if err != nil {
		return LiveControlMetrics{}, err
	}
	metrics := LiveControlMetrics{
		GeneratedAt:   time.Now().UTC(),
		Scanner:       p.LiveSessionControlScannerStatus(),
		ByErrorCode:   make(map[string]int),
		ByAccount:     make(map[string]LiveControlMetricGroup),
		ByStrategy:    make(map[string]LiveControlMetricGroup),
		ByLiveSession: make(map[string]LiveControlMetricGroup),
	}
	for _, session := range sessions {
		if !liveControlMetricsSessionMatches(session, query) {
			continue
		}
		if liveSessionControlPending(session.State) {
			metrics.CurrentPending++
		}
		if pendingMs, ok := liveSessionControlPendingPickupMs(session.State, metrics.GeneratedAt); ok && pendingMs > metrics.CurrentMaxPendingPickupMs {
			metrics.CurrentMaxPendingPickupMs = pendingMs
		}
		staleActive, orphanActive := liveSessionControlActiveRequestAnomalies(session.State)
		if staleActive {
			metrics.StaleActiveControlRequests++
		}
		if orphanActive {
			metrics.OrphanActiveControlRequests++
		}
		if strings.EqualFold(stringValue(session.State["actualStatus"]), "ERROR") {
			metrics.CurrentErrors++
		}
		for _, entry := range metadataList(session.State[liveSessionControlEventStateKey]) {
			event, ok := liveSessionControlToUnifiedLogEvent(session, entry)
			if !ok {
				continue
			}
			if !query.From.IsZero() && event.EventTime.Before(query.From.UTC()) {
				continue
			}
			if !query.To.IsZero() && event.EventTime.After(query.To.UTC()) {
				continue
			}
			metrics.recordLiveControlEvent(event)
		}
	}
	return metrics, nil
}

func liveControlMetricsSessionMatches(session domain.LiveSession, query UnifiedLogEventQuery) bool {
	if strings.TrimSpace(query.AccountID) != "" && session.AccountID != strings.TrimSpace(query.AccountID) {
		return false
	}
	if strings.TrimSpace(query.StrategyID) != "" && session.StrategyID != strings.TrimSpace(query.StrategyID) {
		return false
	}
	if strings.TrimSpace(query.LiveSessionID) != "" && session.ID != strings.TrimSpace(query.LiveSessionID) {
		return false
	}
	return true
}

func liveSessionControlPending(state map[string]any) bool {
	actual := strings.ToUpper(strings.TrimSpace(stringValue(state["actualStatus"])))
	if actual == "ERROR" {
		return false
	}
	if actual == "STARTING" || actual == "STOPPING" {
		return true
	}
	desired := strings.ToUpper(strings.TrimSpace(stringValue(state["desiredStatus"])))
	return desired != "" && actual != "" && desired != actual
}

func liveSessionControlPendingPickupMs(state map[string]any, now time.Time) (int64, bool) {
	desired := strings.ToUpper(strings.TrimSpace(stringValue(state["desiredStatus"])))
	actual := strings.ToUpper(strings.TrimSpace(stringValue(state["actualStatus"])))
	if desired == "" || actual == "" || desired == actual || actual == "ERROR" || actual == "STARTING" || actual == "STOPPING" {
		return 0, false
	}
	requestedAt, ok := parseLiveSessionControlTime(state["controlRequestedAt"])
	if !ok {
		return 0, false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if now.Before(requestedAt) {
		return 0, true
	}
	return now.UTC().Sub(requestedAt.UTC()).Milliseconds(), true
}

func liveSessionControlActiveRequestAnomalies(state map[string]any) (stale bool, orphan bool) {
	activeID := strings.TrimSpace(stringValue(state["activeControlRequestId"]))
	activeVersion := liveSessionControlVersionKey(state, "activeControlVersion")
	if activeID == "" && activeVersion == 0 {
		return false, false
	}
	currentID := strings.TrimSpace(stringValue(state["controlRequestId"]))
	currentVersion := liveSessionControlVersion(state)
	if activeID == "" || activeVersion == 0 || activeID != currentID || activeVersion != currentVersion {
		return true, false
	}
	actual := strings.ToUpper(strings.TrimSpace(stringValue(state["actualStatus"])))
	if actual != "STARTING" && actual != "STOPPING" {
		return false, true
	}
	return false, false
}

func (m *LiveControlMetrics) recordLiveControlEvent(event UnifiedLogEvent) {
	m.TotalEvents++
	phase := strings.ToLower(strings.TrimSpace(stringValue(event.Metadata["phase"])))
	errorCode := strings.ToUpper(strings.TrimSpace(stringValue(event.Metadata["errorCode"])))
	latencyMs, hasLatency := int64MetadataValue(event.Metadata["latencyMs"])

	switch phase {
	case "request_accepted", "legacy_request_initialized":
		m.Requests++
	case "runner_picked_up":
		m.RunnerPickups++
		if hasLatency {
			m.Latency.PickupMs.Add(latencyMs)
		}
	case "succeeded":
		m.Succeeded++
		if hasLatency {
			m.Latency.SuccessMs.Add(latencyMs)
			m.Latency.TerminalMs.Add(latencyMs)
		}
	case "failed":
		m.Failed++
		if errorCode != "" {
			m.ByErrorCode[errorCode]++
		}
		if hasLatency {
			m.Latency.FailureMs.Add(latencyMs)
			m.Latency.TerminalMs.Add(latencyMs)
		}
	case "stale_update_discarded":
		m.StaleDiscarded++
	}
	m.recordLiveControlGroup(m.ByAccount, event.AccountID, phase, errorCode)
	m.recordLiveControlGroup(m.ByStrategy, event.StrategyID, phase, errorCode)
	m.recordLiveControlGroup(m.ByLiveSession, event.LiveSessionID, phase, errorCode)
}

func (m *LiveControlMetrics) recordLiveControlGroup(groups map[string]LiveControlMetricGroup, key, phase, errorCode string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	group := groups[key]
	group.Total++
	switch phase {
	case "request_accepted", "legacy_request_initialized":
		group.Requests++
	case "runner_picked_up":
		group.RunnerPickups++
	case "succeeded":
		group.Succeeded++
	case "failed":
		group.Failed++
		if errorCode != "" {
			if group.ErrorCodes == nil {
				group.ErrorCodes = make(map[string]int)
			}
			group.ErrorCodes[errorCode]++
		}
	case "stale_update_discarded":
		group.StaleDiscarded++
	}
	groups[key] = group
}

func (s *LiveControlLatencyStats) Add(value int64) {
	if value < 0 {
		return
	}
	if s.Count == 0 || value < s.Min {
		s.Min = value
	}
	if value > s.Max {
		s.Max = value
	}
	s.Average = ((s.Average * float64(s.Count)) + float64(value)) / float64(s.Count+1)
	s.Count++
}

func int64MetadataValue(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case uint:
		if uint64(v) > math.MaxInt64 {
			return 0, false
		}
		return int64(v), true
	case uint8:
		return int64(v), true
	case uint16:
		return int64(v), true
	case uint32:
		return int64(v), true
	case uint64:
		if v > math.MaxInt64 {
			return 0, false
		}
		return int64(v), true
	case float32:
		return int64(v), true
	case float64:
		return int64(v), true
	case string:
		parsed, err := time.ParseDuration(strings.TrimSpace(v))
		if err == nil {
			return parsed.Milliseconds(), true
		}
		numeric, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return numeric, err == nil
	default:
		return 0, false
	}
}

func strategyDecisionToUnifiedLogEvent(item domain.StrategyDecisionEvent) UnifiedLogEvent {
	level := "info"
	if !item.SourceGateReady || item.MissingCount > 0 || item.StaleCount > 0 {
		level = "warning"
	}
	if strings.Contains(strings.ToLower(strings.TrimSpace(item.Reason)), "error") || strings.Contains(strings.ToLower(strings.TrimSpace(item.DecisionState)), "error") {
		level = "critical"
	}
	return UnifiedLogEvent{
		ID:               item.ID,
		Source:           "event",
		Type:             "strategy-decision",
		Level:            level,
		Title:            firstNonEmpty(strings.TrimSpace(item.Action), "strategy decision"),
		Message:          firstNonEmpty(strings.TrimSpace(item.Reason), "strategy evaluation completed"),
		EventTime:        item.EventTime.UTC(),
		RecordedAt:       domain.NormalizeEventRecordedAt(item.RecordedAt, item.EventTime),
		AccountID:        item.AccountID,
		StrategyID:       item.StrategyID,
		RuntimeSessionID: item.RuntimeSessionID,
		LiveSessionID:    item.LiveSessionID,
		DecisionEventID:  item.ID,
		Metadata: map[string]any{
			"triggerType":     item.TriggerType,
			"signalKind":      item.SignalKind,
			"decisionState":   item.DecisionState,
			"sourceGateReady": item.SourceGateReady,
			"missingCount":    item.MissingCount,
			"staleCount":      item.StaleCount,
			"symbol":          item.Symbol,
		},
		Payload: item,
	}
}

func orderExecutionToUnifiedLogEvent(item domain.OrderExecutionEvent) UnifiedLogEvent {
	level := "info"
	status := strings.ToLower(strings.TrimSpace(item.Status))
	if item.Failed || strings.TrimSpace(item.Error) != "" || slices.Contains([]string{"failed", "error", "rejected", "expired"}, status) {
		level = "critical"
	} else if item.Fallback || slices.Contains([]string{"canceled", "cancelled"}, status) {
		level = "warning"
	}
	strategyID := stringValue(item.Metadata["strategyId"])
	return UnifiedLogEvent{
		ID:               item.ID,
		Source:           "event",
		Type:             "order-execution",
		Level:            level,
		Title:            firstNonEmpty(strings.TrimSpace(item.EventType), "order execution"),
		Message:          firstNonEmpty(strings.TrimSpace(item.Status), strings.TrimSpace(item.Error), "order execution updated"),
		EventTime:        item.EventTime.UTC(),
		RecordedAt:       domain.NormalizeEventRecordedAt(item.RecordedAt, item.EventTime),
		AccountID:        item.AccountID,
		StrategyID:       strategyID,
		RuntimeSessionID: item.RuntimeSessionID,
		LiveSessionID:    item.LiveSessionID,
		OrderID:          item.OrderID,
		DecisionEventID:  item.DecisionEventID,
		Metadata: map[string]any{
			"eventType":         item.EventType,
			"status":            item.Status,
			"symbol":            item.Symbol,
			"side":              item.Side,
			"orderType":         item.OrderType,
			"executionStrategy": item.ExecutionStrategy,
			"fallback":          item.Fallback,
			"failed":            item.Failed,
		},
		Payload: item,
	}
}

func positionSnapshotToUnifiedLogEvent(item domain.PositionAccountSnapshot) UnifiedLogEvent {
	level := "info"
	if strings.Contains(strings.ToLower(strings.TrimSpace(item.SyncStatus)), "error") || strings.Contains(strings.ToLower(strings.TrimSpace(item.SyncStatus)), "fail") {
		level = "warning"
	}
	return UnifiedLogEvent{
		ID:              item.ID,
		Source:          "event",
		Type:            "position-account-snapshot",
		Level:           level,
		Title:           firstNonEmpty(strings.TrimSpace(item.Trigger), "position/account snapshot"),
		Message:         firstNonEmpty(strings.TrimSpace(item.SyncStatus), "snapshot recorded"),
		EventTime:       item.EventTime.UTC(),
		RecordedAt:      domain.NormalizeEventRecordedAt(item.RecordedAt, item.EventTime),
		AccountID:       item.AccountID,
		StrategyID:      item.StrategyID,
		LiveSessionID:   item.LiveSessionID,
		OrderID:         item.OrderID,
		DecisionEventID: item.DecisionEventID,
		Metadata: map[string]any{
			"symbol":            item.Symbol,
			"trigger":           item.Trigger,
			"syncStatus":        item.SyncStatus,
			"positionFound":     item.PositionFound,
			"openPositionCount": item.OpenPositionCount,
		},
		Payload: item,
	}
}

func liveSessionControlToUnifiedLogEvent(session domain.LiveSession, entry map[string]any) (UnifiedLogEvent, bool) {
	id := strings.TrimSpace(stringValue(entry["id"]))
	if id == "" {
		return UnifiedLogEvent{}, false
	}
	eventTime := parseOptionalRFC3339(stringValue(entry["eventTime"]))
	if eventTime.IsZero() {
		return UnifiedLogEvent{}, false
	}
	recordedAt := parseOptionalRFC3339(stringValue(entry["recordedAt"]))
	recordedAt = domain.NormalizeEventRecordedAt(recordedAt, eventTime)
	phase := strings.TrimSpace(stringValue(entry["phase"]))
	actual := strings.TrimSpace(stringValue(entry["actualStatus"]))
	errorCode := strings.TrimSpace(stringValue(entry["errorCode"]))
	level := "info"
	if errorCode != "" || strings.EqualFold(phase, "failed") || strings.EqualFold(actual, "ERROR") {
		level = "critical"
	} else if strings.EqualFold(phase, "stale_update_discarded") {
		level = "warning"
	}
	metadata := map[string]any{
		"phase":            phase,
		"action":           entry["action"],
		"desiredStatus":    entry["desiredStatus"],
		"actualStatus":     entry["actualStatus"],
		"controlRequestId": entry["controlRequestId"],
		"controlVersion":   entry["controlVersion"],
		"errorCode":        entry["errorCode"],
		"latencyMs":        entry["latencyMs"],
	}
	return UnifiedLogEvent{
		ID:            id,
		Source:        "event",
		Type:          "live-control",
		Level:         level,
		Title:         liveSessionControlEventTitle(phase),
		Message:       liveSessionControlEventMessage(entry),
		EventTime:     eventTime.UTC(),
		RecordedAt:    recordedAt,
		AccountID:     firstNonEmpty(strings.TrimSpace(stringValue(entry["accountId"])), session.AccountID),
		StrategyID:    firstNonEmpty(strings.TrimSpace(stringValue(entry["strategyId"])), session.StrategyID),
		LiveSessionID: firstNonEmpty(strings.TrimSpace(stringValue(entry["liveSessionId"])), session.ID),
		Metadata:      metadata,
		Payload:       entry,
	}, true
}

func liveSessionControlEventTitle(phase string) string {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "request_accepted":
		return "live control request accepted"
	case "legacy_request_initialized":
		return "live control request initialized"
	case "runner_picked_up":
		return "live control picked up"
	case "succeeded":
		return "live control succeeded"
	case "failed":
		return "live control failed"
	case "stale_update_discarded":
		return "live control stale update discarded"
	default:
		return "live control updated"
	}
}

func liveSessionControlEventMessage(entry map[string]any) string {
	action := strings.TrimSpace(stringValue(entry["action"]))
	actual := strings.TrimSpace(stringValue(entry["actualStatus"]))
	if err := strings.TrimSpace(stringValue(entry["error"])); err != "" {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(stringValue(entry["phase"]))) {
	case "request_accepted":
		return firstNonEmpty(action, "control") + " intent accepted"
	case "legacy_request_initialized":
		return firstNonEmpty(action, "control") + " legacy intent initialized"
	case "runner_picked_up":
		return "runner picked up " + firstNonEmpty(action, "control") + " intent"
	case "succeeded":
		return firstNonEmpty(action, "control") + " converged to " + firstNonEmpty(actual, "target status")
	case "stale_update_discarded":
		return "stale runner result discarded"
	default:
		return firstNonEmpty(actual, "live control state updated")
	}
}

func encodeLogEventCursor(item UnifiedLogEvent) string {
	payload, _ := json.Marshal(logEventCursor{
		EventTime:  item.EventTime.UTC().Format(time.RFC3339Nano),
		RecordedAt: item.RecordedAt.UTC().Format(time.RFC3339Nano),
		ID:         item.ID,
	})
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeLogEventCursor(raw string) (*domain.EventCursor, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}
	var cursor logEventCursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}
	eventTime, err := time.Parse(time.RFC3339Nano, cursor.EventTime)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}
	recordedAt, err := time.Parse(time.RFC3339Nano, cursor.RecordedAt)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}
	if strings.TrimSpace(cursor.ID) == "" {
		return nil, fmt.Errorf("invalid cursor")
	}
	return &domain.EventCursor{
		EventTime:  eventTime.UTC(),
		RecordedAt: recordedAt.UTC(),
		ID:         cursor.ID,
	}, nil
}

func normalizeUnifiedLogEventType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "all":
		return ""
	case "decision", "strategy-decision", "strategy_decision":
		return "strategy-decision"
	case "execution", "order-execution", "order_execution":
		return "order-execution"
	case "snapshot", "position-account-snapshot", "position_account_snapshot":
		return "position-account-snapshot"
	case "live-control", "live_control", "control":
		return "live-control"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func normalizeUnifiedLogLevel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "all":
		return ""
	case "warn":
		return "warning"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func matchesUnifiedLogLevel(level, filter string) bool {
	if filter == "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(level), strings.TrimSpace(filter))
}

func normalizeUnifiedLogLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultLogEventLimit
	case limit > maxLogEventLimit:
		return maxLogEventLimit
	default:
		return limit
	}
}

func unifiedLogEventLess(left, right UnifiedLogEvent) bool {
	return domain.EventLessDesc(left.EventTime, left.RecordedAt, left.ID, right.EventTime, right.RecordedAt, right.ID)
}

func limitStrategyDecisionEvents(items []domain.StrategyDecisionEvent, limit int) []domain.StrategyDecisionEvent {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return slices.Clone(items[:limit])
}

func limitOrderExecutionEvents(items []domain.OrderExecutionEvent, limit int) []domain.OrderExecutionEvent {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return slices.Clone(items[:limit])
}

func limitPositionSnapshots(items []domain.PositionAccountSnapshot, limit int) []domain.PositionAccountSnapshot {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return slices.Clone(items[:limit])
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}
