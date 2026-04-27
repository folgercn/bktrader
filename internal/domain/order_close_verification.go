package domain

import "time"

// OrderCloseVerification 记录每次 exit order 触发后的平仓核验事实。
type OrderCloseVerification struct {
	ID                   string         `json:"id"`
	LiveSessionID        string         `json:"liveSessionId"`
	OrderID              string         `json:"orderId"`
	DecisionEventID      string         `json:"decisionEventId,omitempty"`
	AccountID            string         `json:"accountId"`
	StrategyID           string         `json:"strategyId"`
	Symbol               string         `json:"symbol"`
	VerifiedClosed       bool           `json:"verifiedClosed"`
	RemainingPositionQty float64        `json:"remainingPositionQty"`
	VerificationSource   string         `json:"verificationSource"` // reconcile / rest-sync / manual-review / initial
	EventTime            time.Time      `json:"eventTime"`
	RecordedAt           time.Time      `json:"recordedAt"`
	Metadata             map[string]any `json:"metadata,omitempty"`
}

// OrderCloseVerificationQuery 定义平仓核验记录的查询条件。
type OrderCloseVerificationQuery struct {
	LiveSessionID string
	OrderID       string
	OrderIDs      []string
	AccountID     string
	StrategyID    string
	Symbol        string
	Limit         int
}
