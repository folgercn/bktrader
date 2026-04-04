// Package domain 定义 bkTrader 平台的核心领域模型。
// 所有数据结构与具体存储层无关，可在内存和数据库间通用。
package domain

import "time"

// Strategy 策略定义，包含名称、状态和描述信息。
type Strategy struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
}

// StrategyVersion 策略版本快照，记录特定版本的参数配置。
type StrategyVersion struct {
	ID                 string         `json:"id"`
	StrategyID         string         `json:"strategyId"`
	Version            string         `json:"version"`
	SignalTimeframe    string         `json:"signalTimeframe"`    // 信号时间框架（如 "1D"）
	ExecutionTimeframe string         `json:"executionTimeframe"` // 执行时间框架（如 "1m"）
	Parameters         map[string]any `json:"parameters"`
	CreatedAt          time.Time      `json:"createdAt"`
}

// Signal 交易信号记录，由策略引擎生成。
type Signal struct {
	ID                string         `json:"id"`
	StrategyVersionID string         `json:"strategyVersionId"`
	Symbol            string         `json:"symbol"`
	Side              string         `json:"side"`   // BUY / SELL
	Reason            string         `json:"reason"` // 信号触发原因
	Metadata          map[string]any `json:"metadata"`
	CreatedAt         time.Time      `json:"createdAt"`
}

// SignalSourceDefinition 描述一个可插拔信号源/市场数据源。
// 它可以承担交易触发源，也可以承担 order book 等特征输入源。
type SignalSourceDefinition struct {
	Key          string         `json:"key"`
	Name         string         `json:"name"`
	Exchange     string         `json:"exchange"`
	StreamType   string         `json:"streamType"`   // trade_tick / order_book / minute_bar / replay_tick
	Transport    string         `json:"transport"`    // websocket / replay / file
	Status       string         `json:"status"`       // ACTIVE / PREVIEW
	Roles        []string       `json:"roles"`        // trigger / feature
	Environments []string       `json:"environments"` // backtest / paper / live
	SymbolScope  string         `json:"symbolScope"`  // single_symbol / multi_symbol / wildcard
	Description  string         `json:"description"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// AccountSignalBinding 描述账户级的信号源绑定。
// 一个账户可以同时绑定多个交易触发源和多个特征源，以支持多市场交易和套利。
type AccountSignalBinding struct {
	ID         string         `json:"id"`
	AccountID  string         `json:"accountId"`
	SourceKey  string         `json:"sourceKey"`
	SourceName string         `json:"sourceName"`
	Exchange   string         `json:"exchange"`
	Role       string         `json:"role"`       // trigger / feature
	StreamType string         `json:"streamType"` // trade_tick / order_book ...
	Symbol     string         `json:"symbol"`
	Status     string         `json:"status"` // ACTIVE / DISABLED
	Options    map[string]any `json:"options,omitempty"`
	CreatedAt  time.Time      `json:"createdAt"`
}

// Account 交易账户，支持 LIVE（实盘）和 PAPER（模拟盘）两种模式。
type Account struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Mode      string         `json:"mode"`     // LIVE / PAPER
	Exchange  string         `json:"exchange"` // 交易所名称
	Status    string         `json:"status"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
}

// PaperSession 模拟交易会话，绑定账户和策略，管理回放状态。
type PaperSession struct {
	ID          string         `json:"id"`
	AccountID   string         `json:"accountId"`
	StrategyID  string         `json:"strategyId"`
	Status      string         `json:"status"`      // CREATED / RUNNING / STOPPED
	StartEquity float64        `json:"startEquity"` // 初始权益
	State       map[string]any `json:"state"`       // 运行时状态（ledgerIndex 等）
	CreatedAt   time.Time      `json:"createdAt"`
}

// AccountSummary 账户汇总视图，包含权益、盈亏和敞口信息。
type AccountSummary struct {
	AccountID         string    `json:"accountId"`
	AccountName       string    `json:"accountName"`
	Mode              string    `json:"mode"`
	Exchange          string    `json:"exchange"`
	Status            string    `json:"status"`
	StartEquity       float64   `json:"startEquity"`       // 初始权益
	RealizedPnL       float64   `json:"realizedPnl"`       // 已实现盈亏
	UnrealizedPnL     float64   `json:"unrealizedPnl"`     // 未实现盈亏
	Fees              float64   `json:"fees"`              // 累计手续费
	NetEquity         float64   `json:"netEquity"`         // 净权益 = 初始 + 已实现 + 未实现 - 费用
	ExposureNotional  float64   `json:"exposureNotional"`  // 风险敞口（名义值）
	OpenPositionCount int       `json:"openPositionCount"` // 持仓数量
	UpdatedAt         time.Time `json:"updatedAt"`
}

// AccountEquitySnapshot 账户净值快照，用于绘制权益曲线。
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

// Order 交易订单，关联账户和策略版本。
type Order struct {
	ID                string         `json:"id"`
	AccountID         string         `json:"accountId"`
	StrategyVersionID string         `json:"strategyVersionId"`
	Symbol            string         `json:"symbol"`
	Side              string         `json:"side"`   // BUY / SELL
	Type              string         `json:"type"`   // MARKET / LIMIT
	Status            string         `json:"status"` // NEW / FILLED / CANCELLED
	Quantity          float64        `json:"quantity"`
	Price             float64        `json:"price"`
	Metadata          map[string]any `json:"metadata"` // 扩展信息（执行模式、来源等）
	CreatedAt         time.Time      `json:"createdAt"`
}

// Fill 成交记录，每笔成交关联一个订单。
type Fill struct {
	ID        string    `json:"id"`
	OrderID   string    `json:"orderId"`
	Price     float64   `json:"price"`
	Quantity  float64   `json:"quantity"`
	Fee       float64   `json:"fee"` // 手续费
	CreatedAt time.Time `json:"createdAt"`
}

// Position 当前持仓，由成交记录聚合得出。
type Position struct {
	ID                string    `json:"id"`
	AccountID         string    `json:"accountId"`
	StrategyVersionID string    `json:"strategyVersionId"`
	Symbol            string    `json:"symbol"`
	Side              string    `json:"side"` // LONG / SHORT
	Quantity          float64   `json:"quantity"`
	EntryPrice        float64   `json:"entryPrice"` // 加权平均入场价
	MarkPrice         float64   `json:"markPrice"`  // 最新标记价格
	UpdatedAt         time.Time `json:"updatedAt"`
}

// BacktestRun 回测运行记录，保存参数和结果摘要。
type BacktestRun struct {
	ID                string         `json:"id"`
	StrategyVersionID string         `json:"strategyVersionId"`
	Status            string         `json:"status"` // PENDING / RUNNING / COMPLETED
	Parameters        map[string]any `json:"parameters"`
	ResultSummary     map[string]any `json:"resultSummary"`
	CreatedAt         time.Time      `json:"createdAt"`
}

// BacktestConfig 是平台级标准化回测配置。
// signalTimeframe 表示策略信号周期，例如 4h / 1d。
// executionDataSource 表示执行层数据源，例如 tick / 1min。
type BacktestConfig struct {
	SignalTimeframe     string         `json:"signalTimeframe"`
	ExecutionDataSource string         `json:"executionDataSource"`
	Symbol              string         `json:"symbol"`
	Metadata            map[string]any `json:"metadata,omitempty"`
}

// ChartAnnotation 图表标注，用于在 TradingView 上渲染交易标记。
type ChartAnnotation struct {
	ID       string         `json:"id"`
	Source   string         `json:"source"` // backtest / live / paper
	Type     string         `json:"type"`   // entry_long / exit_tp / exit_sl 等
	Symbol   string         `json:"symbol"`
	Time     time.Time      `json:"time"`
	Price    float64        `json:"price"`
	Label    string         `json:"label"`
	Metadata map[string]any `json:"metadata"`
}
