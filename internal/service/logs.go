package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
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
