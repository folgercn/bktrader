package service

import (
	"fmt"
	"slices"
	"strings"
	"time"

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
	parameters["strategyEngine"] = normalizeStrategyEngineKey(stringValue(parameters["strategyEngine"]))
	return p.store.CreateStrategy(name, description, parameters)
}

func (p *Platform) UpdateStrategyParameters(strategyID string, parameters map[string]any) (map[string]any, error) {
	if parameters == nil {
		parameters = map[string]any{}
	}
	parameters["strategyEngine"] = normalizeStrategyEngineKey(stringValue(parameters["strategyEngine"]))
	return p.store.UpdateStrategyParameters(strategyID, parameters)
}

func (p *Platform) GetStrategy(strategyID string) (map[string]any, error) {
	items, err := p.store.ListStrategies()
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if stringValue(item["id"]) == strategyID {
			return item, nil
		}
	}
	return nil, fmt.Errorf("strategy not found: %s", strategyID)
}

func (p *Platform) StrategyEngines() []map[string]any {
	items := make([]map[string]any, 0, len(p.strategyEngines))
	for _, engine := range p.strategyEngines {
		items = append(items, engine.Describe())
	}
	return items
}

func (p *Platform) BindStrategySignalSource(strategyID string, payload map[string]any) (map[string]any, error) {
	strategy, err := p.GetStrategy(strategyID)
	if err != nil {
		return nil, err
	}
	currentVersion, ok := strategy["currentVersion"].(domain.StrategyVersion)
	if !ok {
		return nil, fmt.Errorf("strategy %s has no current version", strategyID)
	}

	sourceKey := normalizeSignalSourceKey(stringValue(payload["sourceKey"]))
	if sourceKey == "" {
		return nil, fmt.Errorf("sourceKey is required")
	}
	provider, ok := p.signalSources[sourceKey]
	if !ok {
		return nil, fmt.Errorf("signal source not registered: %s", sourceKey)
	}
	source := provider.Describe()

	role := normalizeSignalSourceRole(stringValue(payload["role"]))
	if !slices.Contains(source.Roles, role) {
		return nil, fmt.Errorf("signal source %s does not support role %s", source.Key, role)
	}

	symbol := NormalizeSymbol(stringValue(payload["symbol"]))
	options := cloneMetadata(metadataValue(payload["options"]))
	if options == nil {
		options = map[string]any{}
	}

	parameters := cloneMetadata(currentVersion.Parameters)
	if parameters == nil {
		parameters = map[string]any{}
	}
	existing := resolveStrategySignalBindings(parameters)
	binding := domain.AccountSignalBinding{
		ID:         fmt.Sprintf("strategy-signal-binding-%d", time.Now().UnixNano()),
		AccountID:  strategyID,
		SourceKey:  source.Key,
		SourceName: source.Name,
		Exchange:   source.Exchange,
		Role:       role,
		StreamType: source.StreamType,
		Symbol:     symbol,
		Status:     "ACTIVE",
		Options:    options,
		CreatedAt:  time.Now().UTC(),
	}

	bindings := make([]map[string]any, 0, len(existing)+1)
	replaced := false
	for _, item := range existing {
		if normalizeSignalSourceKey(stringValue(item["sourceKey"])) == source.Key &&
			normalizeSignalSourceRole(stringValue(item["role"])) == role &&
			NormalizeSymbol(stringValue(item["symbol"])) == symbol {
			bindings = append(bindings, bindingToMap(binding))
			replaced = true
			continue
		}
		bindings = append(bindings, cloneMetadata(item))
	}
	if !replaced {
		bindings = append(bindings, bindingToMap(binding))
	}
	parameters["signalBindings"] = bindings
	parameters["strategyEngine"] = normalizeStrategyEngineKey(stringValue(parameters["strategyEngine"]))
	return p.store.UpdateStrategyParameters(strategyID, parameters)
}

func (p *Platform) UnbindStrategySignalSource(strategyID string, bindingID string) (map[string]any, bool, error) {
	strategy, err := p.GetStrategy(strategyID)
	if err != nil {
		return nil, err
	}
	currentVersion, ok := strategy["currentVersion"].(domain.StrategyVersion)
	if !ok {
		return nil, fmt.Errorf("strategy %s has no current version", strategyID)
	}
	parameters := cloneMetadata(currentVersion.Parameters)
	if parameters == nil {
		parameters = map[string]any{}
	}
	existing := resolveStrategySignalBindings(parameters)
	bindings := make([]map[string]any, 0, len(existing))
	found := false
	for _, item := range existing {
		if stringValue(item["id"]) == bindingID {
			found = true
			continue
		}
		bindings = append(bindings, cloneMetadata(item))
	}
	if !found {
		return strategy, false, nil
	}
	parameters["signalBindings"] = bindings
	parameters["strategyEngine"] = normalizeStrategyEngineKey(stringValue(parameters["strategyEngine"]))
	updated, err := p.store.UpdateStrategyParameters(strategyID, parameters)
	return updated, true, err
}


func (p *Platform) ListStrategySignalBindings(strategyID string) ([]domain.AccountSignalBinding, error) {
	strategy, err := p.GetStrategy(strategyID)
	if err != nil {
		return nil, err
	}
	currentVersion, ok := strategy["currentVersion"].(domain.StrategyVersion)
	if !ok {
		return nil, fmt.Errorf("strategy %s has no current version", strategyID)
	}
	raw := resolveStrategySignalBindings(currentVersion.Parameters)
	items := make([]domain.AccountSignalBinding, 0, len(raw))
	for _, binding := range raw {
		items = append(items, domain.AccountSignalBinding{
			ID:         stringValue(binding["id"]),
			AccountID:  strategyID,
			SourceKey:  normalizeSignalSourceKey(stringValue(binding["sourceKey"])),
			SourceName: stringValue(binding["sourceName"]),
			Exchange:   normalizeSignalSourceExchange(stringValue(binding["exchange"])),
			Role:       normalizeSignalSourceRole(stringValue(binding["role"])),
			StreamType: stringValue(binding["streamType"]),
			Symbol:     NormalizeSymbol(stringValue(binding["symbol"])),
			Status:     firstNonEmpty(stringValue(binding["status"]), "ACTIVE"),
			Options:    cloneMetadata(metadataValue(binding["options"])),
			CreatedAt:  timeValue(binding["createdAt"]),
		})
	}
	return items, nil
}

// --- 账户管理服务方法 ---

// ListAccounts 获取所有账户列表。
func (p *Platform) ListAccounts() ([]domain.Account, error) {
	return p.store.ListAccounts()
}

// GetAccount 获取单个账户。
func (p *Platform) GetAccount(accountID string) (domain.Account, error) {
	return p.store.GetAccount(accountID)
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
		if liveSummary, ok := buildLiveAccountSummaryFromSnapshot(account); ok {
			summaries = append(summaries, liveSummary)
			continue
		}
		summaries = append(summaries, buildAccountSummary(account, positions, startEquityByAccount, states, feesByAccount))
	}
	return summaries, nil
}

func buildLiveAccountSummaryFromSnapshot(account domain.Account) (domain.AccountSummary, bool) {
	if !strings.EqualFold(account.Mode, "LIVE") || account.Metadata == nil {
		return domain.AccountSummary{}, false
	}
	snapshot := mapValue(account.Metadata["liveSyncSnapshot"])
	if len(snapshot) == 0 || !strings.EqualFold(stringValue(snapshot["syncStatus"]), "SYNCED") {
		return domain.AccountSummary{}, false
	}
	updatedAt := time.Now().UTC()
	if raw := stringValue(account.Metadata["lastLiveSyncAt"]); raw != "" {
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			updatedAt = parsed
		}
	}
	startEquity := parseFloatValue(snapshot["totalWalletBalance"]) - parseFloatValue(snapshot["totalUnrealizedProfit"])
	return domain.AccountSummary{
		AccountID:         account.ID,
		AccountName:       account.Name,
		Mode:              account.Mode,
		Exchange:          account.Exchange,
		Status:            account.Status,
		StartEquity:       startEquity,
		RealizedPnL:       0,
		UnrealizedPnL:     parseFloatValue(snapshot["totalUnrealizedProfit"]),
		Fees:              0,
		NetEquity:         parseFloatValue(snapshot["totalMarginBalance"]),
		AvailableBalance:  parseFloatValue(snapshot["availableBalance"]),
		WalletBalance:     parseFloatValue(snapshot["totalWalletBalance"]),
		MarginBalance:     parseFloatValue(snapshot["totalMarginBalance"]),
		ExposureNotional:  sumSnapshotPositionNotional(snapshot["positions"]),
		OpenPositionCount: len(metadataList(snapshot["positions"])),
		UpdatedAt:         updatedAt,
	}, true
}

func sumSnapshotPositionNotional(value any) float64 {
	items := metadataList(value)
	total := 0.0
	for _, item := range items {
		notional := parseFloatValue(item["notional"])
		if notional < 0 {
			notional = -notional
		}
		total += notional
	}
	return total
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

// GetBacktest 根据 ID 获取单个回测记录。
func (p *Platform) GetBacktest(backtestID string) (domain.BacktestRun, error) {
	items, err := p.store.ListBacktests()
	if err != nil {
		return domain.BacktestRun{}, err
	}
	for _, item := range items {
		if item.ID == backtestID {
			return item, nil
		}
	}
	return domain.BacktestRun{}, fmt.Errorf("backtest not found: %s", backtestID)
}

// CreateBacktest 创建新的回测运行记录。
func (p *Platform) CreateBacktest(strategyVersionID string, parameters map[string]any) (domain.BacktestRun, error) {
	normalized, err := NormalizeBacktestParameters(parameters)
	if err != nil {
		return domain.BacktestRun{}, err
	}
	executionSource := stringValue(normalized["executionDataSource"])
	symbol := stringValue(normalized["symbol"])
	if !p.hasExecutionDataset(executionSource, symbol) {
		return domain.BacktestRun{}, fmt.Errorf("no %s dataset found for symbol %s", executionSource, symbol)
	}
	backtest, err := p.store.CreateBacktest(strategyVersionID, normalized)
	if err != nil {
		return domain.BacktestRun{}, err
	}
	backtest = p.runBacktestSkeleton(backtest)
	return p.store.UpdateBacktest(backtest)
}

func (p *Platform) BacktestOptions() map[string]any {
	tickDatasets := p.discoverExecutionDatasets("tick")
	minuteDatasets := p.discoverExecutionDatasets("1min")

	tickAvailability := "missing"
	if len(tickDatasets) > 0 {
		tickAvailability = "available"
	}

	minuteAvailability := "missing"
	if len(minuteDatasets) > 0 {
		minuteAvailability = "available"
	}

	return map[string]any{
		"signalTimeframes":           []string{"4h", "1d"},
		"executionDataSources":       []string{"tick", "1min"},
		"defaultSignalTimeframe":     "1d",
		"defaultExecutionDataSource": "tick",
		"supportedSymbols": map[string]any{
			"tick": extractDatasetSymbols(tickDatasets),
			"1min": extractDatasetSymbols(minuteDatasets),
		},
		"schema": map[string]any{
			"tick": map[string]any{
				"requiredColumns":  []string{"id", "price", "qty", "quoteQty", "time", "isBuyerMaker", "isBestMatch"},
				"optionalColumns":  []string{"timestamp", "quantity", "side"},
				"filenameExamples": []string{"BTC_tick_Clean.csv", "ETH_tick.csv", "BTCUSDT-trades-2020-01/BTCUSDT-trades-2020-01.csv"},
			},
			"1min": map[string]any{
				"requiredColumns":  []string{"timestamp", "open", "high", "low", "close"},
				"optionalColumns":  []string{"volume"},
				"filenameExamples": []string{"BTC_1min_Clean.csv", "ETH_1min.csv"},
			},
		},
		"dataDirectories": map[string]any{
			"tick": p.tickDataDir,
			"1min": p.minuteDataDir,
		},
		"availability": map[string]any{
			"tick": tickAvailability,
			"1min": minuteAvailability,
		},
		"datasets": map[string]any{
			"tick": tickDatasets,
			"1min": minuteDatasets,
		},
		"notes": []string{
			"4h 和 1d 用于策略信号周期。",
			"执行层测试可选 tick 或 1min。",
			"回测模块聚焦单一执行源回放，不做 tick 与 1min 的结果对比分析。",
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

	symbol := normalizeBacktestSymbol(stringValue(normalized["symbol"]))
	if symbol == "" {
		symbol = "BTCUSDT"
	}

	from := strings.TrimSpace(stringValue(normalized["from"]))
	if from != "" {
		if _, err := time.Parse(time.RFC3339, from); err != nil {
			return nil, fmt.Errorf("invalid from: must be RFC3339")
		}
	}

	to := strings.TrimSpace(stringValue(normalized["to"]))
	if to != "" {
		if _, err := time.Parse(time.RFC3339, to); err != nil {
			return nil, fmt.Errorf("invalid to: must be RFC3339")
		}
	}
	if from != "" && to != "" {
		fromTime, _ := time.Parse(time.RFC3339, from)
		toTime, _ := time.Parse(time.RFC3339, to)
		if toTime.Before(fromTime) {
			return nil, fmt.Errorf("invalid range: to must be greater than or equal to from")
		}
	}

	normalized["signalTimeframe"] = signalTimeframe
	normalized["executionDataSource"] = executionDataSource
	normalized["symbol"] = symbol
	normalized["strategyEngine"] = normalizeStrategyEngineKey(stringValue(normalized["strategyEngine"]))
	if feeBps := parseFloatValue(normalized["tradingFeeBps"]); feeBps >= 0 {
		normalized["tradingFeeBps"] = feeBps
	} else {
		normalized["tradingFeeBps"] = 10.0
	}
	normalized["fundingRateBps"] = parseFloatValue(normalized["fundingRateBps"])
	normalized["fundingIntervalHours"] = maxIntValue(normalized["fundingIntervalHours"], 8)
	if from != "" {
		normalized["from"] = from
	}
	if to != "" {
		normalized["to"] = to
	}
	normalized["executionTimeframe"] = executionDataSource
	normalized["backtestMode"] = fmt.Sprintf("%s->%s", signalTimeframe, executionDataSource)
	return normalized, nil
}

func extractDatasetSymbols(datasets []executionDatasetDescriptor) []string {
	seen := map[string]struct{}{}
	symbols := make([]string, 0, len(datasets))
	for _, dataset := range datasets {
		if dataset.Symbol == "" {
			continue
		}
		if _, ok := seen[dataset.Symbol]; ok {
			continue
		}
		seen[dataset.Symbol] = struct{}{}
		symbols = append(symbols, dataset.Symbol)
	}
	return symbols
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
