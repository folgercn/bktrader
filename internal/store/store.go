// Package store 定义平台的存储层接口。
// 所有具体实现（内存/PostgreSQL）必须实现 Repository 接口。
package store

import "github.com/wuyaocheng/bktrader/internal/domain"

// Repository 定义平台的数据持久化接口。
// 支持可插拔的后端实现（memory/postgres），通过 STORE_BACKEND 配置切换。
type Repository interface {
	// --- 策略管理 ---

	// ListStrategies 获取所有策略（含当前版本信息）。
	ListStrategies() ([]map[string]any, error)
	// CreateStrategy 创建新策略。
	CreateStrategy(name, description string, parameters map[string]any) (map[string]any, error)
	// UpdateStrategyParameters 更新策略当前版本参数。
	UpdateStrategyParameters(strategyID string, parameters map[string]any) (map[string]any, error)

	// --- 账户管理 ---

	// ListAccounts 获取所有账户。
	ListAccounts() ([]domain.Account, error)
	// GetAccount 根据 ID 获取单个账户。
	GetAccount(accountID string) (domain.Account, error)
	// CreateAccount 创建新账户。
	CreateAccount(name, mode, exchange string) (domain.Account, error)
	// UpdateAccount 更新账户信息（状态、metadata 等）。
	UpdateAccount(account domain.Account) (domain.Account, error)

	// --- 订单管理 ---

	// ListOrders 获取所有订单。
	ListOrders() ([]domain.Order, error)
	// CreateOrder 创建新订单。
	CreateOrder(order domain.Order) (domain.Order, error)
	// UpdateOrder 更新订单信息（状态、价格、metadata 等）。
	UpdateOrder(order domain.Order) (domain.Order, error)

	// --- 成交管理 ---

	// ListFills 获取所有成交记录。
	ListFills() ([]domain.Fill, error)
	// TotalFilledQuantityForOrder 返回指定订单已落账成交数量总和。
	TotalFilledQuantityForOrder(orderID string) (float64, error)
	// CreateFill 创建新成交记录。
	CreateFill(fill domain.Fill) (domain.Fill, error)

	// --- 持仓管理 ---

	// ListPositions 获取所有持仓。
	ListPositions() ([]domain.Position, error)
	// FindPosition 查找指定账户和交易对的持仓。
	FindPosition(accountID, symbol string) (domain.Position, bool, error)
	// SavePosition 创建或更新持仓。
	SavePosition(position domain.Position) (domain.Position, error)
	// DeletePosition 删除持仓（全部平仓时调用）。
	DeletePosition(positionID string) error

	// --- 回测管理 ---

	// ListBacktests 获取所有回测记录。
	ListBacktests() ([]domain.BacktestRun, error)
	// CreateBacktest 创建新回测运行记录。
	CreateBacktest(strategyVersionID string, parameters map[string]any) (domain.BacktestRun, error)
	// UpdateBacktest 更新回测运行记录状态和结果。
	UpdateBacktest(backtest domain.BacktestRun) (domain.BacktestRun, error)

	// --- 模拟交易会话 ---

	// ListPaperSessions 获取所有模拟交易会话。
	ListPaperSessions() ([]domain.PaperSession, error)
	// GetPaperSession 根据 ID 获取单个模拟交易会话。
	GetPaperSession(sessionID string) (domain.PaperSession, error)
	// CreatePaperSession 创建新模拟交易会话。
	CreatePaperSession(accountID, strategyID string, startEquity float64) (domain.PaperSession, error)
	// UpdatePaperSessionStatus 更新会话状态（RUNNING/STOPPED）。
	UpdatePaperSessionStatus(sessionID, status string) (domain.PaperSession, error)
	// UpdatePaperSessionState 更新会话运行时状态（ledgerIndex 等）。
	UpdatePaperSessionState(sessionID string, state map[string]any) (domain.PaperSession, error)

	// --- 实盘策略会话 ---

	// ListLiveSessions 获取所有实盘策略会话。
	ListLiveSessions() ([]domain.LiveSession, error)
	// GetLiveSession 根据 ID 获取单个实盘策略会话。
	GetLiveSession(sessionID string) (domain.LiveSession, error)
	// CreateLiveSession 创建新实盘策略会话。
	CreateLiveSession(accountID, strategyID string) (domain.LiveSession, error)
	// UpdateLiveSession 更新指定实盘策略会话的账户、策略或状态载荷。
	UpdateLiveSession(session domain.LiveSession) (domain.LiveSession, error)
	// DeleteLiveSession 删除指定实盘策略会话。
	DeleteLiveSession(sessionID string) error
	// UpdateLiveSessionStatus 更新实盘策略会话状态。
	UpdateLiveSessionStatus(sessionID, status string) (domain.LiveSession, error)
	// UpdateLiveSessionState 更新实盘策略会话运行时状态。
	UpdateLiveSessionState(sessionID string, state map[string]any) (domain.LiveSession, error)

	// --- 净值快照 ---

	// ListAccountEquitySnapshots 获取指定账户的净值快照序列。
	ListAccountEquitySnapshots(accountID string) ([]domain.AccountEquitySnapshot, error)
	// CreateAccountEquitySnapshot 创建新的净值快照。
	CreateAccountEquitySnapshot(snapshot domain.AccountEquitySnapshot) (domain.AccountEquitySnapshot, error)

	// --- Live 决策 / 执行 / 快照采集 ---

	// ListStrategyDecisionEvents 获取指定 live session 的策略决策事件；sessionID 为空时返回全部。
	ListStrategyDecisionEvents(liveSessionID string) ([]domain.StrategyDecisionEvent, error)
	// CreateStrategyDecisionEvent 创建新的策略决策事件。
	CreateStrategyDecisionEvent(event domain.StrategyDecisionEvent) (domain.StrategyDecisionEvent, error)
	// ListOrderExecutionEvents 获取指定订单的执行事件；orderID 为空时返回全部。
	ListOrderExecutionEvents(orderID string) ([]domain.OrderExecutionEvent, error)
	// CreateOrderExecutionEvent 创建新的订单执行事件。
	CreateOrderExecutionEvent(event domain.OrderExecutionEvent) (domain.OrderExecutionEvent, error)
	// ListPositionAccountSnapshots 获取指定账户的仓位/账户快照；accountID 为空时返回全部。
	ListPositionAccountSnapshots(accountID string) ([]domain.PositionAccountSnapshot, error)
	// CreatePositionAccountSnapshot 创建新的仓位/账户快照。
	CreatePositionAccountSnapshot(snapshot domain.PositionAccountSnapshot) (domain.PositionAccountSnapshot, error)

	// --- 市场 Bar 缓存 ---

	// ListMarketBars 获取指定交易所/交易对/周期的缓存 K 线。
	ListMarketBars(exchange, symbol, timeframe string, from, to int64, limit int) ([]domain.MarketBar, error)
	// UpsertMarketBars 批量写入或更新市场 K 线缓存。
	UpsertMarketBars(bars []domain.MarketBar) error

	// --- 平台运行配置 ---

	// GetRuntimePolicy 获取持久化的运行时阈值配置。
	GetRuntimePolicy() (domain.RuntimePolicy, bool, error)
	// UpsertRuntimePolicy 创建或更新运行时阈值配置。
	UpsertRuntimePolicy(policy domain.RuntimePolicy) (domain.RuntimePolicy, error)

	// --- 通知中心 ---

	// ListNotificationAcks 获取所有已确认通知键。
	ListNotificationAcks() ([]domain.NotificationAck, error)
	// UpsertNotificationAck 创建或更新时间已确认通知键。
	UpsertNotificationAck(id string) (domain.NotificationAck, error)
	// DeleteNotificationAck 取消已确认通知键。
	DeleteNotificationAck(id string) error

	// GetTelegramConfig 获取持久化的 Telegram 通知配置。
	GetTelegramConfig() (domain.TelegramConfig, bool, error)
	// UpsertTelegramConfig 创建或更新 Telegram 通知配置。
	UpsertTelegramConfig(config domain.TelegramConfig) (domain.TelegramConfig, error)
	// ListNotificationDeliveries 获取通知发送记录。
	ListNotificationDeliveries() ([]domain.NotificationDelivery, error)
	// UpsertNotificationDelivery 创建或更新通知发送记录。
	UpsertNotificationDelivery(notificationID, channel, status, lastError string) (domain.NotificationDelivery, error)
}
