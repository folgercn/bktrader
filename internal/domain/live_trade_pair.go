package domain

import "time"

// LiveTradePair 描述同一 live session 内一笔完整或进行中的开平仓回合。
type LiveTradePair struct {
	ID             string     `json:"id"`
	LiveSessionID  string     `json:"liveSessionId"`
	AccountID      string     `json:"accountId"`
	StrategyID     string     `json:"strategyId"`
	Symbol         string     `json:"symbol"`
	Status         string     `json:"status"` // open / closed
	Side           string     `json:"side"`   // LONG / SHORT
	EntryOrderIDs  []string   `json:"entryOrderIds"`
	ExitOrderIDs   []string   `json:"exitOrderIds,omitempty"`
	EntryAt        time.Time  `json:"entryAt"`
	ExitAt         *time.Time `json:"exitAt,omitempty"`
	EntryAvgPrice  float64    `json:"entryAvgPrice"`
	ExitAvgPrice   float64    `json:"exitAvgPrice"`
	EntryQuantity  float64    `json:"entryQuantity"`
	ExitQuantity   float64    `json:"exitQuantity"`
	OpenQuantity   float64    `json:"openQuantity"`
	EntryReason    string     `json:"entryReason,omitempty"`
	ExitReason     string     `json:"exitReason,omitempty"`
	ExitClassifier string     `json:"exitClassifier,omitempty"` // SL / TSL / TP / manual / recovery
	ExitVerdict    string     `json:"exitVerdict"`              // open / normal / recovery-close / orphan-exit / mismatch
	RealizedPnL    float64    `json:"realizedPnl"`
	UnrealizedPnL  float64    `json:"unrealizedPnl"`
	Fees           float64    `json:"fees"`
	NetPnL         float64    `json:"netPnl"`
	EntryFillCount int        `json:"entryFillCount"`
	ExitFillCount  int        `json:"exitFillCount"`
	Notes          []string   `json:"notes,omitempty"`
}

// LiveTradePairQuery 定义 live trade pair 聚合接口的过滤条件。
type LiveTradePairQuery struct {
	LiveSessionID string
	Status        string
	Limit         int
}
