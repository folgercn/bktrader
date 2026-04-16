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
