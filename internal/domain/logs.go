package domain

import "time"

// EventCursor 定义事件查询游标的排序锚点。
type EventCursor struct {
	EventTime  time.Time
	RecordedAt time.Time
	ID         string
}

// StrategyDecisionEventQuery 定义策略决策事件的筛选与分页参数。
type StrategyDecisionEventQuery struct {
	AccountID        string
	StrategyID       string
	RuntimeSessionID string
	LiveSessionID    string
	DecisionEventID  string
	DecisionEventIDs []string
	From             time.Time
	To               time.Time
	Before           *EventCursor
	Limit            int
}

// OrderExecutionEventQuery 定义订单执行事件的筛选与分页参数。
type OrderExecutionEventQuery struct {
	AccountID        string
	StrategyID       string
	RuntimeSessionID string
	LiveSessionID    string
	OrderID          string
	DecisionEventID  string
	From             time.Time
	To               time.Time
	Before           *EventCursor
	Limit            int
}

// PositionAccountSnapshotQuery 定义仓位账户快照的筛选与分页参数。
type PositionAccountSnapshotQuery struct {
	AccountID       string
	StrategyID      string
	LiveSessionID   string
	OrderID         string
	DecisionEventID string
	From            time.Time
	To              time.Time
	Before          *EventCursor
	Limit           int
}

// NormalizeEventRecordedAt 统一补齐事件排序用的 recordedAt 锚点。
func NormalizeEventRecordedAt(recordedAt, eventTime time.Time) time.Time {
	if recordedAt.IsZero() {
		return eventTime.UTC()
	}
	return recordedAt.UTC()
}

// EventLessDesc 判断左事件是否应在倒序（最新优先）结果中排在右事件前。
func EventLessDesc(leftTime, leftRecordedAt time.Time, leftID string, rightTime, rightRecordedAt time.Time, rightID string) bool {
	leftTime = leftTime.UTC()
	rightTime = rightTime.UTC()
	switch {
	case leftTime.After(rightTime):
		return true
	case leftTime.Before(rightTime):
		return false
	}
	leftRecordedAt = NormalizeEventRecordedAt(leftRecordedAt, leftTime)
	rightRecordedAt = NormalizeEventRecordedAt(rightRecordedAt, rightTime)
	switch {
	case leftRecordedAt.After(rightRecordedAt):
		return true
	case leftRecordedAt.Before(rightRecordedAt):
		return false
	default:
		return leftID > rightID
	}
}

// EventLessAsc 判断左事件是否应在正序（最旧优先）结果中排在右事件前。
func EventLessAsc(leftTime, leftRecordedAt time.Time, leftID string, rightTime, rightRecordedAt time.Time, rightID string) bool {
	return EventLessDesc(rightTime, rightRecordedAt, rightID, leftTime, leftRecordedAt, leftID)
}

// EventBeforeCursor 判断事件是否位于游标之前。
func EventBeforeCursor(eventTime, recordedAt time.Time, id string, cursor EventCursor) bool {
	eventTime = eventTime.UTC()
	recordedAt = NormalizeEventRecordedAt(recordedAt, eventTime)
	cursorRecordedAt := NormalizeEventRecordedAt(cursor.RecordedAt, cursor.EventTime)
	switch {
	case eventTime.Before(cursor.EventTime.UTC()):
		return true
	case eventTime.After(cursor.EventTime.UTC()):
		return false
	}
	switch {
	case recordedAt.Before(cursorRecordedAt):
		return true
	case recordedAt.After(cursorRecordedAt):
		return false
	default:
		return id < cursor.ID
	}
}
