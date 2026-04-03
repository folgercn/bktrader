package service

import (
	"fmt"
	"strings"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

// --- 策略管理服务方法 ---

// ListStrategies 获取所有策略列表。
func (p *Platform) ListStrategies() ([]map[string]any, error) {
	return p.store.ListStrategies()
}

// CreateStrategy 创建新策略，参数为空时初始化为空 map。
func (p *Platform) CreateStrategy(name, description string, parameters map[string]any) (map[string]any, error) {
	if parameters == nil {
		parameters = map[string]any{}
	}
	return p.store.CreateStrategy(name, description, parameters)
}

// --- 账户管理服务方法 ---

// ListAccounts 获取所有账户列表。
func (p *Platform) ListAccounts() ([]domain.Account, error) {
	return p.store.ListAccounts()
}

// CreateAccount 创建新账户，mode 自动转为大写（LIVE / PAPER）。
func (p *Platform) CreateAccount(name, mode, exchange string) (domain.Account, error) {
	return p.store.CreateAccount(name, strings.ToUpper(mode), exchange)
}

// ListAccountSummaries 汇总所有账户的权益、PnL、费用和敞口信息。
// 遍历订单和成交数据，计算已实现/未实现盈亏。
func (p *Platform) ListAccountSummaries() ([]domain.AccountSummary, error) {
	accounts, err := p.store.ListAccounts()
	if err != nil {
		return nil, err
	}
	orders, err := p.store.ListOrders()
	if err != nil {
		return nil, err
	}
	fills, err := p.store.ListFills()
	if err != nil {
		return nil, err
	}
	positions, err := p.store.ListPositions()
	if err != nil {
		return nil, err
	}
	paperSessions, err := p.store.ListPaperSessions()
	if err != nil {
		return nil, err
	}

	// 构建订单 ID 索引，用于关联 fill -> order -> account
	orderByID := make(map[string]domain.Order, len(orders))
	for _, order := range orders {
		orderByID[order.ID] = order
	}

	// 从 paper session 获取每个账户的初始权益
	startEquityByAccount := make(map[string]float64, len(paperSessions))
	for _, session := range paperSessions {
		startEquityByAccount[session.AccountID] = session.StartEquity
	}

	// 计算每个账户+交易对的 PnL 状态
	states := map[string]*pnlState{}
	feesByAccount := map[string]float64{}
	for _, fill := range fills {
		order, ok := orderByID[fill.OrderID]
		if !ok {
			continue
		}
		key := order.AccountID + "|" + order.Symbol
		state := states[key]
		if state == nil {
			state = &pnlState{}
			states[key] = state
		}
		feesByAccount[order.AccountID] += fill.Fee
		applyPnLFill(state, order.Side, fill.Quantity, fill.Price)
	}

	summaries := make([]domain.AccountSummary, 0, len(accounts))
	for _, account := range accounts {
		summaries = append(summaries, buildAccountSummary(account, positions, startEquityByAccount, states, feesByAccount))
	}
	return summaries, nil
}

// ListAccountEquitySnapshots 获取指定账户的净值快照时间序列。
func (p *Platform) ListAccountEquitySnapshots(accountID string) ([]domain.AccountEquitySnapshot, error) {
	return p.store.ListAccountEquitySnapshots(accountID)
}

// captureAccountSnapshot 捕获指定账户的当前净值快照并持久化。
// 在 paper 订单成交等关键事件后调用。
func (p *Platform) captureAccountSnapshot(accountID string) error {
	summaries, err := p.ListAccountSummaries()
	if err != nil {
		return err
	}
	for _, summary := range summaries {
		if summary.AccountID != accountID {
			continue
		}
		_, err := p.store.CreateAccountEquitySnapshot(domain.AccountEquitySnapshot{
			AccountID:         summary.AccountID,
			StartEquity:       summary.StartEquity,
			RealizedPnL:       summary.RealizedPnL,
			UnrealizedPnL:     summary.UnrealizedPnL,
			Fees:              summary.Fees,
			NetEquity:         summary.NetEquity,
			ExposureNotional:  summary.ExposureNotional,
			OpenPositionCount: summary.OpenPositionCount,
		})
		return err
	}
	return nil
}

// --- 回测管理服务方法 ---

// ListBacktests 获取所有回测记录。
func (p *Platform) ListBacktests() ([]domain.BacktestRun, error) {
	return p.store.ListBacktests()
}

// CreateBacktest 创建新的回测运行记录。
func (p *Platform) CreateBacktest(strategyVersionID string, parameters map[string]any) (domain.BacktestRun, error) {
	normalized, err := NormalizeBacktestParameters(parameters)
	if err != nil {
		return domain.BacktestRun{}, err
	}
	backtest, err := p.store.CreateBacktest(strategyVersionID, normalized)
	if err != nil {
		return domain.BacktestRun{}, err
	}
	backtest = p.runBacktestSkeleton(backtest)
	return p.store.UpdateBacktest(backtest)
}

func (p *Platform) BacktestOptions() map[string]any {
	tickAvailability := "missing"
	if _, err := p.loadExecutionDatasetSummary("tick", "BTCUSDT"); err == nil {
		tickAvailability = "available"
	}

	minuteAvailability := "missing"
	if _, err := p.loadExecutionDatasetSummary("1min", "BTCUSDT"); err == nil {
		minuteAvailability = "available"
	}

	return map[string]any{
		"signalTimeframes":           []string{"4h", "1d"},
		"executionDataSources":       []string{"tick", "1min"},
		"defaultSignalTimeframe":     "1d",
		"defaultExecutionDataSource": "1min",
		"dataDirectories": map[string]any{
			"tick": p.tickDataDir,
			"1min": p.minuteDataDir,
		},
		"availability": map[string]any{
			"tick": tickAvailability,
			"1min": minuteAvailability,
		},
		"notes": []string{
			"4h 和 1d 用于策略信号周期。",
			"tick 与 1min 用于执行层回测数据源。",
			"1min 可以作为 tick 的近似替代，但不应和信号周期混淆。",
		},
	}
}

// --- 信号源服务方法 ---

// SignalSources 返回当前已注册的信号源列表（静态数据，后续接入动态注册）。
func (p *Platform) SignalSources() []map[string]any {
	return []map[string]any{
		{
			"id":          "signal-source-bk-1d",
			"name":        "BK 1D ATR Reentry",
			"type":        "internal-strategy",
			"status":      "ACTIVE",
			"dedupeKey":   "symbol+strategyVersion+reason+bar",
			"description": "1D 信号 / 1m 执行策略源。",
		},
	}
}

// --- 通用工具函数 ---

// NormalizeSymbol 标准化交易对符号，默认 BTCUSDT。
func NormalizeSymbol(symbol string) string {
	if symbol == "" {
		return "BTCUSDT"
	}
	return strings.ToUpper(strings.TrimSpace(symbol))
}

// ValidateRequired 校验必填字段是否为空，用于 HTTP handler 的请求验证。
func ValidateRequired(values map[string]string, fields ...string) error {
	for _, field := range fields {
		if strings.TrimSpace(values[field]) == "" {
			return fmt.Errorf("%s is required", field)
		}
	}
	return nil
}

func NormalizeBacktestParameters(parameters map[string]any) (map[string]any, error) {
	normalized := cloneMetadata(parameters)
	if normalized == nil {
		normalized = map[string]any{}
	}

	signalTimeframe := strings.ToLower(strings.TrimSpace(stringValue(normalized["signalTimeframe"])))
	if signalTimeframe == "" {
		signalTimeframe = "1d"
	}
	if signalTimeframe != "4h" && signalTimeframe != "1d" {
		return nil, fmt.Errorf("unsupported signalTimeframe: %s", signalTimeframe)
	}

	executionDataSource := strings.ToLower(strings.TrimSpace(stringValue(normalized["executionDataSource"])))
	if executionDataSource == "" {
		executionDataSource = "1min"
	}
	if executionDataSource != "tick" && executionDataSource != "1min" {
		return nil, fmt.Errorf("unsupported executionDataSource: %s", executionDataSource)
	}

	symbol := strings.ToUpper(strings.TrimSpace(stringValue(normalized["symbol"])))
	if symbol == "" {
		symbol = "BTCUSDT"
	}

	normalized["signalTimeframe"] = signalTimeframe
	normalized["executionDataSource"] = executionDataSource
	normalized["symbol"] = symbol
	normalized["executionTimeframe"] = executionDataSource
	normalized["backtestMode"] = fmt.Sprintf("%s->%s", signalTimeframe, executionDataSource)
	return normalized, nil
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}
