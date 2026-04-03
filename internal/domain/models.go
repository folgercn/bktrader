package domain

import "time"

type Strategy struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
}

type StrategyVersion struct {
	ID                 string         `json:"id"`
	StrategyID         string         `json:"strategyId"`
	Version            string         `json:"version"`
	SignalTimeframe    string         `json:"signalTimeframe"`
	ExecutionTimeframe string         `json:"executionTimeframe"`
	Parameters         map[string]any `json:"parameters"`
	CreatedAt          time.Time      `json:"createdAt"`
}

type Signal struct {
	ID                string         `json:"id"`
	StrategyVersionID string         `json:"strategyVersionId"`
	Symbol            string         `json:"symbol"`
	Side              string         `json:"side"`
	Reason            string         `json:"reason"`
	Metadata          map[string]any `json:"metadata"`
	CreatedAt         time.Time      `json:"createdAt"`
}

type Account struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Mode      string    `json:"mode"`
	Exchange  string    `json:"exchange"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

type PaperSession struct {
	ID          string         `json:"id"`
	AccountID   string         `json:"accountId"`
	StrategyID  string         `json:"strategyId"`
	Status      string         `json:"status"`
	StartEquity float64        `json:"startEquity"`
	State       map[string]any `json:"state"`
	CreatedAt   time.Time      `json:"createdAt"`
}

type AccountSummary struct {
	AccountID         string    `json:"accountId"`
	AccountName       string    `json:"accountName"`
	Mode              string    `json:"mode"`
	Exchange          string    `json:"exchange"`
	Status            string    `json:"status"`
	StartEquity       float64   `json:"startEquity"`
	RealizedPnL       float64   `json:"realizedPnl"`
	UnrealizedPnL     float64   `json:"unrealizedPnl"`
	Fees              float64   `json:"fees"`
	NetEquity         float64   `json:"netEquity"`
	ExposureNotional  float64   `json:"exposureNotional"`
	OpenPositionCount int       `json:"openPositionCount"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type AccountEquitySnapshot struct {
	ID                string    `json:"id"`
	AccountID         string    `json:"accountId"`
	StartEquity       float64   `json:"startEquity"`
	RealizedPnL       float64   `json:"realizedPnl"`
	UnrealizedPnL     float64   `json:"unrealizedPnl"`
	Fees              float64   `json:"fees"`
	NetEquity         float64   `json:"netEquity"`
	ExposureNotional  float64   `json:"exposureNotional"`
	OpenPositionCount int       `json:"openPositionCount"`
	CreatedAt         time.Time `json:"createdAt"`
}

type Order struct {
	ID                string         `json:"id"`
	AccountID         string         `json:"accountId"`
	StrategyVersionID string         `json:"strategyVersionId"`
	Symbol            string         `json:"symbol"`
	Side              string         `json:"side"`
	Type              string         `json:"type"`
	Status            string         `json:"status"`
	Quantity          float64        `json:"quantity"`
	Price             float64        `json:"price"`
	Metadata          map[string]any `json:"metadata"`
	CreatedAt         time.Time      `json:"createdAt"`
}

type Fill struct {
	ID        string    `json:"id"`
	OrderID   string    `json:"orderId"`
	Price     float64   `json:"price"`
	Quantity  float64   `json:"quantity"`
	Fee       float64   `json:"fee"`
	CreatedAt time.Time `json:"createdAt"`
}

type Position struct {
	ID                string    `json:"id"`
	AccountID         string    `json:"accountId"`
	StrategyVersionID string    `json:"strategyVersionId"`
	Symbol            string    `json:"symbol"`
	Side              string    `json:"side"`
	Quantity          float64   `json:"quantity"`
	EntryPrice        float64   `json:"entryPrice"`
	MarkPrice         float64   `json:"markPrice"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type BacktestRun struct {
	ID                string         `json:"id"`
	StrategyVersionID string         `json:"strategyVersionId"`
	Status            string         `json:"status"`
	Parameters        map[string]any `json:"parameters"`
	ResultSummary     map[string]any `json:"resultSummary"`
	CreatedAt         time.Time      `json:"createdAt"`
}

type ChartAnnotation struct {
	ID       string         `json:"id"`
	Source   string         `json:"source"`
	Type     string         `json:"type"`
	Symbol   string         `json:"symbol"`
	Time     time.Time      `json:"time"`
	Price    float64        `json:"price"`
	Label    string         `json:"label"`
	Metadata map[string]any `json:"metadata"`
}
