// Package domain 定义 bkTrader 平台的核心领域模型。
// 所有数据结构与具体存储层无关，可在内存和数据库间通用。
package domain

import (
	"strconv"
	"strings"
	"time"
)

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

// SignalRuntimeSession 表示一个账户+策略组合下的信号源运行时会话。
// 首期以骨架形式保存订阅计划、连接状态和最近事件摘要。
type SignalRuntimeSession struct {
	ID              string         `json:"id"`
	AccountID       string         `json:"accountId"`
	StrategyID      string         `json:"strategyId"`
	Status          string         `json:"status"` // READY / RUNNING / STOPPED / ERROR
	RuntimeAdapter  string         `json:"runtimeAdapter"`
	Transport       string         `json:"transport"`
	SubscriptionCnt int            `json:"subscriptionCount"`
	State           map[string]any `json:"state,omitempty"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}

const (
	RuntimeLeaseResourceSignalRuntimeSession = "signal-runtime-session"
	RuntimeLeaseResourceLiveSession          = "live-session"
	RuntimeLeaseResourceAccountSync          = "account-sync"
)

// RuntimeLease records which runner currently owns a runtime resource.
type RuntimeLease struct {
	ResourceType string    `json:"resourceType"`
	ResourceID   string    `json:"resourceId"`
	OwnerID      string    `json:"ownerId"`
	ExpiresAt    time.Time `json:"expiresAt"`
	AcquiredAt   time.Time `json:"acquiredAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type RuntimeLeaseAcquireRequest struct {
	ResourceType string
	ResourceID   string
	OwnerID      string
	TTL          time.Duration
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

// LiveSession 实盘策略会话，绑定 LIVE 账户和策略，管理运行状态与实时评估上下文。
type LiveSession struct {
	ID         string         `json:"id"`
	Alias      string         `json:"alias"`
	AccountID  string         `json:"accountId"`
	StrategyID string         `json:"strategyId"`
	Status     string         `json:"status"` // READY / RUNNING / STOPPED / BLOCKED
	State      map[string]any `json:"state"`
	CreatedAt  time.Time      `json:"createdAt"`
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
	AvailableBalance  float64   `json:"availableBalance"`  // 可用余额（live/testnet 优先来自交易所快照）
	WalletBalance     float64   `json:"walletBalance"`     // 钱包余额
	MarginBalance     float64   `json:"marginBalance"`     // 保证金余额
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

// AccountEquitySnapshotQuery limits account equity time-series reads.
type AccountEquitySnapshotQuery struct {
	AccountID string
	From      time.Time
	To        time.Time
	Limit     int
}

// MarketBar 市场 K 线缓存，按交易所/交易对/周期/开盘时间唯一。
type MarketBar struct {
	ID        string    `json:"id"`
	Exchange  string    `json:"exchange"`
	Symbol    string    `json:"symbol"`
	Timeframe string    `json:"timeframe"`
	OpenTime  time.Time `json:"openTime"`
	CloseTime time.Time `json:"closeTime"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
	IsClosed  bool      `json:"isClosed"`
	Source    string    `json:"source"`
	UpdatedAt time.Time `json:"updatedAt"`
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
	ReduceOnly        bool           `json:"reduceOnly,omitempty"`
	ClosePosition     bool           `json:"closePosition,omitempty"`
	Metadata          map[string]any `json:"metadata"` // 扩展信息（执行模式、来源等）
	CreatedAt         time.Time      `json:"createdAt"`
}

// OrderQuery 定义订单查询条件。
type OrderQuery struct {
	LiveSessionID      string
	AccountID          string
	Symbols            []string
	Statuses           []string
	ExcludeStatuses    []string
	MetadataBoolEquals map[string]bool
	Limit              int
	Offset             int
}

// Fill 成交记录，每笔成交关联一个订单。
type Fill struct {
	ID                string     `json:"id"`
	OrderID           string     `json:"orderId"`
	ExchangeTradeID   string     `json:"exchangeTradeId,omitempty"`
	ExchangeTradeTime *time.Time `json:"exchangeTradeTime,omitempty"`
	DedupFingerprint  string     `json:"-"`
	// Source is an internal reconciliation source stored in fill_source, not exposed by fill JSON.
	Source    string    `json:"source,omitempty"`
	Price     float64   `json:"price"`
	Quantity  float64   `json:"quantity"`
	Fee       float64   `json:"fee"` // 手续费
	CreatedAt time.Time `json:"createdAt"`
}

func (f Fill) FallbackFingerprint() string {
	tradeTime := ""
	if f.ExchangeTradeTime != nil && !f.ExchangeTradeTime.IsZero() {
		tradeTime = f.ExchangeTradeTime.UTC().Format(time.RFC3339Nano)
	}
	return strings.Join([]string{
		strconv.FormatFloat(f.Price, 'f', -1, 64),
		strconv.FormatFloat(f.Quantity, 'f', -1, 64),
		strconv.FormatFloat(f.Fee, 'f', -1, 64),
		tradeTime,
	}, "|")
}

// FillQuery 定义成交记录查询条件。
type FillQuery struct {
	OrderIDs []string
	Limit    int
	Offset   int
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

// PositionQuery 定义持仓查询条件。
type PositionQuery struct {
	AccountID string
	Symbol    string
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
// signalTimeframe 表示策略信号周期，例如 5m / 4h / 1d。
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

// RuntimePolicy 保存平台运行期告警与 readiness 阈值配置。
type RuntimePolicy struct {
	TradeTickFreshnessSeconds      int       `json:"tradeTickFreshnessSeconds"`
	OrderBookFreshnessSeconds      int       `json:"orderBookFreshnessSeconds"`
	SignalBarFreshnessSeconds      int       `json:"signalBarFreshnessSeconds"`
	RuntimeQuietSeconds            int       `json:"runtimeQuietSeconds"`
	StrategyEvaluationQuietSeconds int       `json:"strategyEvaluationQuietSeconds"`
	LiveAccountSyncFreshnessSecs   int       `json:"liveAccountSyncFreshnessSeconds"`
	PaperStartReadinessTimeoutSecs int       `json:"paperStartReadinessTimeoutSeconds"`
	WSHandshakeTimeoutSeconds      int       `json:"wsHandshakeTimeoutSeconds"`
	WSReadStaleTimeoutSeconds      int       `json:"wsReadStaleTimeoutSeconds"`
	WSPingIntervalSeconds          int       `json:"wsPingIntervalSeconds"`
	WSPassiveCloseTimeoutSeconds   int       `json:"wsPassiveCloseTimeoutSeconds"`
	WSReconnectBackoffs            []int     `json:"wsReconnectBackoffs"`
	WSReconnectRecoveryBackoffs    []int     `json:"wsReconnectRecoveryBackoffs"`
	RESTLimiterRPS                 int       `json:"restLimiterRps"`
	RESTLimiterBurst               int       `json:"restLimiterBurst"`
	RESTBackoffSeconds             int       `json:"restBackoffSeconds"`
	LiveMarketCacheTTLMinutes      int       `json:"liveMarketCacheTTLMinutes"`
	TelegramHTTPTimeoutSeconds     int       `json:"telegramHTTPTimeoutSeconds"`
	BinanceRecvWindowMs            int       `json:"binanceRecvWindowMs"`
	LiveSignalWarmWindowDays       int       `json:"liveSignalWarmWindowDays"`
	LiveFastSignalWarmWindowDays   int       `json:"liveFastSignalWarmWindowDays"`
	LiveMinuteWarmWindowDays       int       `json:"liveMinuteWarmWindowDays"`
	UpdatedAt                      time.Time `json:"updatedAt"`
}

// StrategyDecisionEvent 记录 live 运行时的策略评估输入、决策和执行意图。
type StrategyDecisionEvent struct {
	ID                string         `json:"id"`
	LiveSessionID     string         `json:"liveSessionId"`
	RuntimeSessionID  string         `json:"runtimeSessionId,omitempty"`
	AccountID         string         `json:"accountId"`
	StrategyID        string         `json:"strategyId"`
	StrategyVersionID string         `json:"strategyVersionId,omitempty"`
	Symbol            string         `json:"symbol"`
	TriggerType       string         `json:"triggerType,omitempty"`
	Action            string         `json:"action"`
	Reason            string         `json:"reason"`
	SignalKind        string         `json:"signalKind,omitempty"`
	DecisionState     string         `json:"decisionState,omitempty"`
	IntentSignature   string         `json:"intentSignature,omitempty"`
	SourceGateReady   bool           `json:"sourceGateReady"`
	MissingCount      int            `json:"missingCount"`
	StaleCount        int            `json:"staleCount"`
	EventTime         time.Time      `json:"eventTime"`
	RecordedAt        time.Time      `json:"recordedAt"`
	TriggerSummary    map[string]any `json:"triggerSummary,omitempty"`
	SourceGate        map[string]any `json:"sourceGate,omitempty"`
	SourceStates      map[string]any `json:"sourceStates,omitempty"`
	SignalBarStates   map[string]any `json:"signalBarStates,omitempty"`
	PositionSnapshot  map[string]any `json:"positionSnapshot,omitempty"`
	DecisionMetadata  map[string]any `json:"decisionMetadata,omitempty"`
	SignalIntent      map[string]any `json:"signalIntent,omitempty"`
	ExecutionProposal map[string]any `json:"executionProposal,omitempty"`
	EvaluationContext map[string]any `json:"evaluationContext,omitempty"`
}

// OrderExecutionEvent 记录 live 订单在提交、同步、成交等生命周期中的结构化执行数据。
type OrderExecutionEvent struct {
	ID                string         `json:"id"`
	OrderID           string         `json:"orderId"`
	ExchangeOrderID   string         `json:"exchangeOrderId,omitempty"`
	LiveSessionID     string         `json:"liveSessionId,omitempty"`
	DecisionEventID   string         `json:"decisionEventId,omitempty"`
	RuntimeSessionID  string         `json:"runtimeSessionId,omitempty"`
	AccountID         string         `json:"accountId"`
	StrategyVersionID string         `json:"strategyVersionId,omitempty"`
	Symbol            string         `json:"symbol"`
	Side              string         `json:"side"`
	OrderType         string         `json:"orderType"`
	EventType         string         `json:"eventType"`
	Status            string         `json:"status"`
	ExecutionStrategy string         `json:"executionStrategy,omitempty"`
	ExecutionDecision string         `json:"executionDecision,omitempty"`
	ExecutionMode     string         `json:"executionMode,omitempty"`
	Quantity          float64        `json:"quantity"`
	Price             float64        `json:"price"`
	ExpectedPrice     float64        `json:"expectedPrice"`
	PriceDriftBps     float64        `json:"priceDriftBps"`
	RawQuantity       float64        `json:"rawQuantity"`
	NormalizedQty     float64        `json:"normalizedQuantity"`
	RawPriceReference float64        `json:"rawPriceReference"`
	NormalizedPrice   float64        `json:"normalizedPrice"`
	SpreadBps         float64        `json:"spreadBps"`
	BookImbalance     float64        `json:"bookImbalance"`
	SubmitLatencyMs   int            `json:"submitLatencyMs"`
	SyncLatencyMs     int            `json:"syncLatencyMs"`
	FillLatencyMs     int            `json:"fillLatencyMs"`
	EventTime         time.Time      `json:"eventTime"`
	RecordedAt        time.Time      `json:"recordedAt"`
	Fallback          bool           `json:"fallback"`
	PostOnly          bool           `json:"postOnly"`
	ReduceOnly        bool           `json:"reduceOnly"`
	Failed            bool           `json:"failed"`
	Error             string         `json:"error,omitempty"`
	RuntimePreflight  map[string]any `json:"runtimePreflight,omitempty"`
	DispatchSummary   map[string]any `json:"dispatchSummary,omitempty"`
	AdapterSubmission map[string]any `json:"adapterSubmission,omitempty"`
	AdapterSync       map[string]any `json:"adapterSync,omitempty"`
	Normalization     map[string]any `json:"normalization,omitempty"`
	SymbolRules       map[string]any `json:"symbolRules,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
}

// PositionAccountSnapshot 记录 live session 在关键事件后的仓位和账户状态快照。
type PositionAccountSnapshot struct {
	ID                string         `json:"id"`
	LiveSessionID     string         `json:"liveSessionId"`
	DecisionEventID   string         `json:"decisionEventId,omitempty"`
	OrderID           string         `json:"orderId,omitempty"`
	AccountID         string         `json:"accountId"`
	StrategyID        string         `json:"strategyId"`
	Symbol            string         `json:"symbol"`
	Trigger           string         `json:"trigger"`
	IntentSignature   string         `json:"intentSignature,omitempty"`
	PositionFound     bool           `json:"positionFound"`
	PositionSide      string         `json:"positionSide,omitempty"`
	PositionQuantity  float64        `json:"positionQuantity"`
	EntryPrice        float64        `json:"entryPrice"`
	MarkPrice         float64        `json:"markPrice"`
	NetEquity         float64        `json:"netEquity"`
	AvailableBalance  float64        `json:"availableBalance"`
	MarginBalance     float64        `json:"marginBalance"`
	WalletBalance     float64        `json:"walletBalance"`
	ExposureNotional  float64        `json:"exposureNotional"`
	OpenPositionCount int            `json:"openPositionCount"`
	SyncStatus        string         `json:"syncStatus,omitempty"`
	EventTime         time.Time      `json:"eventTime"`
	RecordedAt        time.Time      `json:"recordedAt"`
	PositionSnapshot  map[string]any `json:"positionSnapshot,omitempty"`
	LivePositionState map[string]any `json:"livePositionState,omitempty"`
	AccountSnapshot   map[string]any `json:"accountSnapshot,omitempty"`
	AccountSummary    map[string]any `json:"accountSummary,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
}

// PlatformAlert 是统一告警中心消费的聚合告警记录。
type PlatformAlert struct {
	ID               string         `json:"id"`
	Scope            string         `json:"scope"` // paper / live / runtime
	Level            string         `json:"level"` // critical / warning / info
	Title            string         `json:"title"`
	Detail           string         `json:"detail"`
	AccountID        string         `json:"accountId,omitempty"`
	AccountName      string         `json:"accountName,omitempty"`
	StrategyID       string         `json:"strategyId,omitempty"`
	StrategyName     string         `json:"strategyName,omitempty"`
	PaperSessionID   string         `json:"paperSessionId,omitempty"`
	RuntimeSessionID string         `json:"runtimeSessionId,omitempty"`
	Anchor           string         `json:"anchor,omitempty"`
	EventTime        time.Time      `json:"eventTime"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type PlatformHealthAlertCounts struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
	Info     int `json:"info"`
}

type PlatformHealthAccountSnapshot struct {
	ID                      string         `json:"id"`
	Name                    string         `json:"name"`
	Exchange                string         `json:"exchange"`
	Status                  string         `json:"status"`
	LastLiveSyncAt          string         `json:"lastLiveSyncAt"`
	SyncAgeSeconds          int            `json:"syncAgeSeconds"`
	SyncStale               bool           `json:"syncStale"`
	RuntimeSessionCount     int            `json:"runtimeSessionCount"`
	RunningLiveSessionCount int            `json:"runningLiveSessionCount"`
	AccountSync             map[string]any `json:"accountSync,omitempty"`
}

type PlatformHealthRuntimeSessionSnapshot struct {
	ID              string         `json:"id"`
	AccountID       string         `json:"accountId"`
	StrategyID      string         `json:"strategyId"`
	StrategyName    string         `json:"strategyName,omitempty"`
	Status          string         `json:"status"`
	Transport       string         `json:"transport"`
	Health          string         `json:"health"`
	LastEventAt     string         `json:"lastEventAt,omitempty"`
	LastHeartbeatAt string         `json:"lastHeartbeatAt,omitempty"`
	Quiet           bool           `json:"quiet"`
	TradeTick       map[string]any `json:"tradeTick,omitempty"`
	OrderBook       map[string]any `json:"orderBook,omitempty"`
}

type PlatformHealthStrategySessionSnapshot struct {
	ID                           string         `json:"id"`
	Mode                         string         `json:"mode"`
	AccountID                    string         `json:"accountId"`
	StrategyID                   string         `json:"strategyId"`
	StrategyName                 string         `json:"strategyName,omitempty"`
	Status                       string         `json:"status"`
	RuntimeSessionID             string         `json:"runtimeSessionId,omitempty"`
	LastSignalRuntimeEventAt     string         `json:"lastSignalRuntimeEventAt,omitempty"`
	LastStrategyEvaluationAt     string         `json:"lastStrategyEvaluationAt,omitempty"`
	LastStrategyEvaluationStatus string         `json:"lastStrategyEvaluationStatus,omitempty"`
	LastSyncedOrderStatus        string         `json:"lastSyncedOrderStatus,omitempty"`
	EvaluationQuiet              bool           `json:"evaluationQuiet"`
	StrategyIngress              map[string]any `json:"strategyIngress,omitempty"`
	Execution                    map[string]any `json:"execution,omitempty"`
	SourceGate                   map[string]any `json:"sourceGate,omitempty"`
}

type PlatformHealthSnapshot struct {
	GeneratedAt     time.Time                               `json:"generatedAt"`
	Status          string                                  `json:"status"`
	AlertCounts     PlatformHealthAlertCounts               `json:"alertCounts"`
	RuntimePolicy   RuntimePolicy                           `json:"runtimePolicy"`
	LiveControl     map[string]any                          `json:"liveControl,omitempty"`
	LiveAccounts    []PlatformHealthAccountSnapshot         `json:"liveAccounts"`
	RuntimeSessions []PlatformHealthRuntimeSessionSnapshot  `json:"runtimeSessions"`
	LiveSessions    []PlatformHealthStrategySessionSnapshot `json:"liveSessions"`
	PaperSessions   []PlatformHealthStrategySessionSnapshot `json:"paperSessions"`
}

// NotificationAck 保存用户已确认的通知键。
type NotificationAck struct {
	ID        string    `json:"id"`
	AckedAt   time.Time `json:"ackedAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// PlatformNotification 是平台内通知中心使用的聚合对象。
type PlatformNotification struct {
	ID        string         `json:"id"`
	Status    string         `json:"status"` // active / acked
	AckedAt   *time.Time     `json:"ackedAt,omitempty"`
	Alert     PlatformAlert  `json:"alert"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

// TelegramConfig 保存 Telegram 通知通道配置。
type TelegramConfig struct {
	Enabled                       bool      `json:"enabled"`
	BotToken                      string    `json:"botToken"`
	ChatID                        string    `json:"chatId"`
	SendLevels                    []string  `json:"sendLevels"`
	TradeEventsEnabled            bool      `json:"tradeEventsEnabled"`
	PositionReportEnabled         bool      `json:"positionReportEnabled"`
	PositionReportIntervalMinutes int       `json:"positionReportIntervalMinutes"`
	UpdatedAt                     time.Time `json:"updatedAt"`
}

// NotificationDelivery 记录通知通过外部通道的发送结果。
type NotificationDelivery struct {
	NotificationID string         `json:"notificationId"`
	Channel        string         `json:"channel"` // telegram
	Status         string         `json:"status"`  // sent / failed
	Metadata       map[string]any `json:"metadata,omitempty"`
	LastError      string         `json:"lastError,omitempty"`
	AttemptedAt    time.Time      `json:"attemptedAt"`
	SentAt         time.Time      `json:"sentAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}
