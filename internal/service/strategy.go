package service

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

// --- 策略管理服务方法 ---

func strategyLogger(component string, args ...any) *slog.Logger {
	logger := slog.Default()
	if strings.TrimSpace(component) != "" {
		logger = logger.With("component", component)
	}
	if len(args) > 0 {
		logger = logger.With(args...)
	}
	return logger
}

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
	strategy, err := p.store.CreateStrategy(name, description, parameters)
	if err != nil {
		strategyLogger("service.strategy", "strategy_name", name).Error("create strategy failed", "error", err)
		return nil, err
	}
	strategyLogger("service.strategy",
		"strategy_id", stringValue(strategy["id"]),
		"strategy_name", name,
		"strategy_engine", stringValue(parameters["strategyEngine"]),
	).Info("strategy created")
	return strategy, nil
}

func (p *Platform) UpdateStrategyParameters(strategyID string, parameters map[string]any) (map[string]any, error) {
	if parameters == nil {
		parameters = map[string]any{}
	}
	parameters["strategyEngine"] = normalizeStrategyEngineKey(stringValue(parameters["strategyEngine"]))
	updated, err := p.store.UpdateStrategyParameters(strategyID, parameters)
	if err != nil {
		strategyLogger("service.strategy", "strategy_id", strategyID).Error("update strategy parameters failed", "error", err)
		return nil, err
	}
	strategyLogger("service.strategy",
		"strategy_id", strategyID,
		"strategy_engine", stringValue(parameters["strategyEngine"]),
	).Info("strategy parameters updated")
	return updated, nil
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

	parameters := cloneMetadata(currentVersion.Parameters)
	if parameters == nil {
		parameters = map[string]any{}
	}
	existing := resolveStrategySignalBindings(parameters)
	binding, err := p.strategySignalBindingFromPayload(strategyID, payload)
	if err != nil {
		return nil, err
	}

	bindings := make([]map[string]any, 0, len(existing)+1)
	replaced := false
	for _, item := range existing {
		if signalBindingMatches(binding.SourceKey, binding.Role, binding.Symbol, binding.Options, item) {
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
	updated, err := p.store.UpdateStrategyParameters(strategyID, parameters)
	if err != nil {
		strategyLogger("service.strategy", "strategy_id", strategyID).Error("bind strategy signal source failed", "error", err)
		return nil, err
	}
	strategyLogger("service.strategy",
		"strategy_id", strategyID,
		"source_key", binding.SourceKey,
		"role", binding.Role,
		"symbol", binding.Symbol,
	).Info("strategy signal source bound", "replaced_existing", replaced)
	return updated, nil
}

func (p *Platform) replaceStrategySignalSources(strategyID string, payloads []map[string]any) (map[string]any, error) {
	strategy, err := p.GetStrategy(strategyID)
	if err != nil {
		return nil, err
	}
	currentVersion, ok := strategy["currentVersion"].(domain.StrategyVersion)
	if !ok {
		return nil, fmt.Errorf("strategy %s has no current version", strategyID)
	}
	if len(payloads) == 0 {
		return nil, fmt.Errorf("strategy %s requires at least one signal binding", strategyID)
	}

	parameters := cloneMetadata(currentVersion.Parameters)
	if parameters == nil {
		parameters = map[string]any{}
	}
	bindings := make([]map[string]any, 0, len(payloads))
	seen := make(map[string]struct{}, len(payloads))
	for _, payload := range payloads {
		binding, err := p.strategySignalBindingFromPayload(strategyID, payload)
		if err != nil {
			return nil, err
		}
		key := signalBindingKey(binding)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		bindings = append(bindings, bindingToMap(binding))
	}
	if len(bindings) == 0 {
		return nil, fmt.Errorf("strategy %s requires at least one valid signal binding", strategyID)
	}
	parameters["signalBindings"] = bindings
	parameters["strategyEngine"] = normalizeStrategyEngineKey(stringValue(parameters["strategyEngine"]))
	updated, err := p.store.UpdateStrategyParameters(strategyID, parameters)
	if err != nil {
		strategyLogger("service.strategy", "strategy_id", strategyID).Error("replace strategy signal sources failed", "error", err)
		return nil, err
	}
	strategyLogger("service.strategy",
		"strategy_id", strategyID,
		"binding_count", len(bindings),
	).Info("strategy signal sources replaced")
	return updated, nil
}

func (p *Platform) strategySignalBindingFromPayload(strategyID string, payload map[string]any) (domain.AccountSignalBinding, error) {
	sourceKey := normalizeSignalSourceKey(stringValue(payload["sourceKey"]))
	if sourceKey == "" {
		return domain.AccountSignalBinding{}, fmt.Errorf("sourceKey is required")
	}
	provider, ok := p.signalSources[sourceKey]
	if !ok {
		return domain.AccountSignalBinding{}, fmt.Errorf("signal source not registered: %s", sourceKey)
	}
	source := provider.Describe()

	role := normalizeSignalSourceRole(stringValue(payload["role"]))
	if !slices.Contains(source.Roles, role) {
		return domain.AccountSignalBinding{}, fmt.Errorf("signal source %s does not support role %s", source.Key, role)
	}

	symbol := NormalizeSymbol(stringValue(payload["symbol"]))
	options := canonicalizeSignalBindingOptions(source.Key, cloneMetadata(metadataValue(payload["options"])))

	return domain.AccountSignalBinding{
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
	}, nil
}

func (p *Platform) UnbindStrategySignalSource(strategyID string, bindingID string) (map[string]any, bool, error) {
	strategy, err := p.GetStrategy(strategyID)
	if err != nil {
		return nil, false, err
	}
	currentVersion, ok := strategy["currentVersion"].(domain.StrategyVersion)
	if !ok {
		return nil, false, fmt.Errorf("strategy %s has no current version", strategyID)
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
			Options:    canonicalizeSignalBindingOptions(stringValue(binding["sourceKey"]), cloneMetadata(metadataValue(binding["options"]))),
			CreatedAt:  timeValue(binding["createdAt"]),
		})
	}
	slices.SortFunc(items, func(a, b domain.AccountSignalBinding) int {
		if cmp := strings.Compare(a.Role, b.Role); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(a.Exchange, b.Exchange); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(a.Symbol, b.Symbol); cmp != 0 {
			return cmp
		}
		return strings.Compare(signalBindingTimeframe(a.SourceKey, a.Options), signalBindingTimeframe(b.SourceKey, b.Options))
	})
	return items, nil
}

func (p *Platform) GetLiveSession(sessionID string) (domain.LiveSession, error) {
	return p.store.GetLiveSession(sessionID)
}

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
	account, err := p.store.CreateAccount(name, strings.ToUpper(mode), exchange)
	if err != nil {
		strategyLogger("service.account", "account_name", name, "mode", strings.ToUpper(mode)).Error("create account failed", "error", err)
		return domain.Account{}, err
	}
	strategyLogger("service.account",
		"account_id", account.ID,
		"account_name", account.Name,
		"mode", account.Mode,
		"exchange", account.Exchange,
	).Info("account created")
	return account, nil
}

// ListAccountSummaries 汇总所有账户的权益、PnL、费用和敞口信息。
// 遍历订单和成交数据，计算已实现/未实现盈亏。
func (p *Platform) ListAccountSummaries() ([]domain.AccountSummary, error) {
	accounts, err := p.store.ListAccounts()
	if err != nil {
		return nil, err
	}

	var liveSummaries []domain.AccountSummary
	var paperAccounts []domain.Account
	for _, account := range accounts {
		if liveSummary, ok := buildLiveAccountSummaryFromSnapshot(account); ok {
			liveSummaries = append(liveSummaries, liveSummary)
		} else {
			paperAccounts = append(paperAccounts, account)
		}
	}

	if len(paperAccounts) == 0 {
		return liveSummaries, nil
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
	summaries = append(summaries, liveSummaries...)
	for _, account := range paperAccounts {
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
func (p *Platform) ListAccountEquitySnapshots(query domain.AccountEquitySnapshotQuery) ([]domain.AccountEquitySnapshot, error) {
	return p.store.ListAccountEquitySnapshots(query)
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
	applyBacktestParameterAliases(normalized)

	signalTimeframe := normalizeSignalBarInterval(stringValue(normalized["signalTimeframe"]))
	if signalTimeframe == "" {
		signalTimeframe = "1d"
	}
	if signalTimeframe != "5m" && signalTimeframe != "15m" && signalTimeframe != "30m" && signalTimeframe != "4h" && signalTimeframe != "1d" {
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
	dir2ZeroInitial := domain.ResearchBaselineDir2ZeroInitial
	if _, ok := normalized["dir2_zero_initial"]; ok {
		dir2ZeroInitial = boolValue(normalized["dir2_zero_initial"])
	}
	normalized["dir2_zero_initial"] = dir2ZeroInitial
	normalized["zero_initial_mode"] = resolveStrategyZeroInitialMode(dir2ZeroInitial, normalized["zero_initial_mode"])
	normalized["max_trades_per_bar"] = maxIntValue(normalized["max_trades_per_bar"], domain.ResearchBaselineMaxTradesPerBar)
	normalized["reentry_size_schedule"] = normalizeBacktestFloatSlice(normalized["reentry_size_schedule"], domain.ResearchBaselineReentrySizeSchedule())
	stopLossATR := parseFloatValue(normalized["stop_loss_atr"])
	if stopLossATR <= 0 {
		stopLossATR = 0.05
	}
	normalized["stop_loss_atr"] = stopLossATR
	profitProtectATR := parseFloatValue(normalized["profit_protect_atr"])
	if profitProtectATR <= 0 {
		profitProtectATR = 1.0
	}
	normalized["profit_protect_atr"] = profitProtectATR
	normalized["long_reentry_atr"] = parseFloatValue(firstNonNil(normalized["long_reentry_atr"], 0.1))
	normalized["short_reentry_atr"] = parseFloatValue(firstNonNil(normalized["short_reentry_atr"], 0.0))
	if trailingStopATR := parseFloatValue(normalized["trailing_stop_atr"]); trailingStopATR > 0 {
		normalized["trailing_stop_atr"] = trailingStopATR
	}
	if delayedTrailingActivationATR := parseFloatValue(normalized["delayed_trailing_activation_atr"]); delayedTrailingActivationATR > 0 {
		normalized["delayed_trailing_activation_atr"] = delayedTrailingActivationATR
	}
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

func applyBacktestParameterAliases(parameters map[string]any) {
	if parameters == nil {
		return
	}
	aliases := map[string]string{
		"signal_timeframe":             "signalTimeframe",
		"execution_data_source":        "executionDataSource",
		"strategy_engine":              "strategyEngine",
		"dir2ZeroInitial":              "dir2_zero_initial",
		"zeroInitialMode":              "zero_initial_mode",
		"maxTradesPerBar":              "max_trades_per_bar",
		"reentrySizes":                 "reentry_size_schedule",
		"stopLossATR":                  "stop_loss_atr",
		"profitProtectATR":             "profit_protect_atr",
		"fixedSlippage":                "fixed_slippage",
		"trailingStopATR":              "trailing_stop_atr",
		"delayedTrailingActivationATR": "delayed_trailing_activation_atr",
		"longReentryATR":               "long_reentry_atr",
		"shortReentryATR":              "short_reentry_atr",
		"breakoutShape":                "breakout_shape",
		"breakoutShapeToleranceBps":    "breakout_shape_tolerance_bps",
		"reentryMinStopBps":            "reentry_min_stop_bps",
		"reentryATRPercentileGTE":      "reentry_atr_percentile_gte",
		"minStopBps":                   "min_stop_bps",
		"atrPctGTE":                    "atr_pct_gte",
		"minATRPercentile":             "min_atr_percentile",
		"minSMAATRSeparation":          "min_sma_atr_separation",
		"qualityFilterShapes":          "quality_filter_shapes",
	}
	for from, to := range aliases {
		if _, ok := parameters[to]; ok {
			continue
		}
		if value, ok := parameters[from]; ok {
			parameters[to] = value
		}
	}
}

func normalizeBacktestFloatSlice(value any, fallback []float64) []float64 {
	items := []float64{}
	switch raw := value.(type) {
	case []float64:
		items = append(items, raw...)
	case []any:
		for _, item := range raw {
			items = append(items, parseFloatValue(item))
		}
	case []string:
		for _, item := range raw {
			items = append(items, parseFloatValue(item))
		}
	case nil:
	default:
		if single, ok := parseBacktestFloatValue(raw); ok {
			items = append(items, single)
		}
	}
	if len(items) == 0 {
		return append([]float64(nil), fallback...)
	}
	clean := make([]float64, 0, len(items))
	for _, item := range items {
		if item >= 0 {
			clean = append(clean, item)
		}
	}
	if len(clean) == 0 {
		return append([]float64(nil), fallback...)
	}
	return clean
}

func parseBacktestFloatValue(value any) (float64, bool) {
	switch raw := value.(type) {
	case float64:
		return raw, true
	case float32:
		return float64(raw), true
	case int:
		return float64(raw), true
	case int8:
		return float64(raw), true
	case int16:
		return float64(raw), true
	case int32:
		return float64(raw), true
	case int64:
		return float64(raw), true
	case uint:
		return float64(raw), true
	case uint8:
		return float64(raw), true
	case uint16:
		return float64(raw), true
	case uint32:
		return float64(raw), true
	case uint64:
		return float64(raw), true
	case string:
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return 0, false
		}
		return parseFloatValue(trimmed), true
	default:
		return 0, false
	}
}

func firstNonNil(value any, fallback any) any {
	if value == nil {
		return fallback
	}
	return value
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
