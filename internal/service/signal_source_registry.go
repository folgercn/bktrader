package service

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

type SignalSourceProvider interface {
	Key() string
	Describe() domain.SignalSourceDefinition
}

type staticSignalSourceProvider struct {
	definition domain.SignalSourceDefinition
}

func (p staticSignalSourceProvider) Key() string {
	return normalizeSignalSourceKey(p.definition.Key)
}

func (p staticSignalSourceProvider) Describe() domain.SignalSourceDefinition {
	definition := p.definition
	definition.Key = normalizeSignalSourceKey(definition.Key)
	definition.Exchange = normalizeSignalSourceExchange(definition.Exchange)
	if definition.Status == "" {
		definition.Status = "ACTIVE"
	}
	return definition
}

func normalizeSignalSourceKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeSignalSourceExchange(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "INTERNAL"
	}
	return strings.ToUpper(value)
}

func normalizeSignalSourceRole(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "", "trigger":
		return "trigger"
	case "feature":
		return "feature"
	default:
		return value
	}
}

func normalizeEnvironmentFromAccountMode(mode string) string {
	switch strings.ToUpper(strings.TrimSpace(mode)) {
	case "LIVE":
		return "live"
	case "PAPER":
		return "paper"
	default:
		return strings.ToLower(strings.TrimSpace(mode))
	}
}

func (p *Platform) registerBuiltInSignalSources() {
	register := func(provider SignalSourceProvider) {
		p.signalSources[provider.Key()] = provider
	}

	register(staticSignalSourceProvider{definition: domain.SignalSourceDefinition{
		Key:          "binance-trade-tick",
		Name:         "Binance Futures Trade Tick",
		Exchange:     "BINANCE",
		StreamType:   "trade_tick",
		Transport:    "websocket",
		Status:       "ACTIVE",
		Roles:        []string{"trigger"},
		Environments: []string{"paper", "live"},
		SymbolScope:  "multi_symbol",
		Description:  "逐笔成交 tick 流，适合作为模拟盘和实盘的主触发源。",
		Metadata: map[string]any{
			"stream":            "aggTrade/trade",
			"triggerLatency":    "low",
			"supportsArbitrage": true,
		},
	}})
	register(staticSignalSourceProvider{definition: domain.SignalSourceDefinition{
		Key:          "binance-order-book",
		Name:         "Binance Futures Order Book",
		Exchange:     "BINANCE",
		StreamType:   "order_book",
		Transport:    "websocket",
		Status:       "ACTIVE",
		Roles:        []string{"feature"},
		Environments: []string{"paper", "live"},
		SymbolScope:  "multi_symbol",
		Description:  "订单簿深度流，用于后续盘口特征、滑点和套利研究。",
		Metadata: map[string]any{
			"stream":         "depth/bookTicker",
			"supportsLevels": []int{1, 5, 10, 20, 50},
		},
	}})
	register(staticSignalSourceProvider{definition: domain.SignalSourceDefinition{
		Key:          "okx-trade-tick",
		Name:         "OKX Trade Tick",
		Exchange:     "OKX",
		StreamType:   "trade_tick",
		Transport:    "websocket",
		Status:       "ACTIVE",
		Roles:        []string{"trigger"},
		Environments: []string{"paper", "live"},
		SymbolScope:  "multi_symbol",
		Description:  "OKX 逐笔成交 tick 流，可独立绑定到 OKX 账户，或用于跨市场套利观测。",
		Metadata: map[string]any{
			"stream":            "trades",
			"supportsArbitrage": true,
		},
	}})
	register(staticSignalSourceProvider{definition: domain.SignalSourceDefinition{
		Key:          "okx-order-book",
		Name:         "OKX Order Book",
		Exchange:     "OKX",
		StreamType:   "order_book",
		Transport:    "websocket",
		Status:       "ACTIVE",
		Roles:        []string{"feature"},
		Environments: []string{"paper", "live"},
		SymbolScope:  "multi_symbol",
		Description:  "OKX 深度盘口流，用于 order book 特征和跨市场价差研究。",
		Metadata: map[string]any{
			"stream":         "books/book5",
			"supportsLevels": []int{1, 5, 10, 50},
		},
	}})
	register(staticSignalSourceProvider{definition: domain.SignalSourceDefinition{
		Key:          "replay-tick-archive",
		Name:         "Replay Tick Archive",
		Exchange:     "INTERNAL",
		StreamType:   "replay_tick",
		Transport:    "file",
		Status:       "ACTIVE",
		Roles:        []string{"trigger", "feature"},
		Environments: []string{"backtest"},
		SymbolScope:  "multi_symbol",
		Description:  "回测使用的逐笔 archive 重放源，不直接服务实盘触发。",
		Metadata: map[string]any{
			"dataDirConfig": "TICK_DATA_DIR",
		},
	}})
	register(staticSignalSourceProvider{definition: domain.SignalSourceDefinition{
		Key:          "replay-minute-bars",
		Name:         "Replay Minute Bars",
		Exchange:     "INTERNAL",
		StreamType:   "minute_bar",
		Transport:    "file",
		Status:       "ACTIVE",
		Roles:        []string{"trigger"},
		Environments: []string{"backtest"},
		SymbolScope:  "multi_symbol",
		Description:  "回测使用的 1min 执行代理数据源。",
		Metadata: map[string]any{
			"dataDirConfig": "MINUTE_DATA_DIR",
		},
	}})
}

func (p *Platform) SignalSources() []domain.SignalSourceDefinition {
	items := make([]domain.SignalSourceDefinition, 0, len(p.signalSources))
	for _, provider := range p.signalSources {
		items = append(items, provider.Describe())
	}
	slices.SortFunc(items, func(a, b domain.SignalSourceDefinition) int {
		return strings.Compare(a.Key, b.Key)
	})
	return items
}

func (p *Platform) SignalSourceCatalog() map[string]any {
	sources := p.SignalSources()
	grouped := map[string][]domain.SignalSourceDefinition{}
	for _, source := range sources {
		for _, env := range source.Environments {
			grouped[env] = append(grouped[env], source)
		}
	}
	return map[string]any{
		"sources": sources,
		"notes": []string{
			"账户级信号源绑定支持多源并行，可同时绑定交易触发源和盘口特征源。",
			"paper/live 应优先绑定交易所 trade tick；order book 建议作为 feature 源单独接入。",
			"跨市场套利可在单账户或多账户上并行绑定 Binance / OKX 等多个来源。",
		},
		"byEnvironment": grouped,
	}
}

func (p *Platform) SignalSourceTypes() []map[string]any {
	return []map[string]any{
		{
			"streamType":    "trade_tick",
			"primaryRole":   "trigger",
			"description":   "逐笔成交流，适合作为 paper/live 的策略触发源。",
			"typicalInputs": []string{"price", "quantity", "side", "tradeId", "eventTime"},
		},
		{
			"streamType":    "order_book",
			"primaryRole":   "feature",
			"description":   "盘口深度流，适合订单簿特征、流动性和套利研究。",
			"typicalInputs": []string{"bid", "ask", "levels", "updateId", "eventTime"},
		},
		{
			"streamType":    "minute_bar",
			"primaryRole":   "trigger",
			"description":   "回测使用的分钟级执行代理源。",
			"typicalInputs": []string{"open", "high", "low", "close", "volume"},
		},
		{
			"streamType":    "replay_tick",
			"primaryRole":   "trigger",
			"description":   "回测使用的逐笔 archive 重放源。",
			"typicalInputs": []string{"timestamp", "price", "quantity", "side"},
		},
	}
}

func (p *Platform) BindAccountSignalSource(accountID string, payload map[string]any) (domain.Account, error) {
	account, err := p.store.GetAccount(accountID)
	if err != nil {
		return domain.Account{}, err
	}

	sourceKey := normalizeSignalSourceKey(stringValue(payload["sourceKey"]))
	if sourceKey == "" {
		return domain.Account{}, fmt.Errorf("sourceKey is required")
	}
	provider, ok := p.signalSources[sourceKey]
	if !ok {
		return domain.Account{}, fmt.Errorf("signal source not registered: %s", sourceKey)
	}
	source := provider.Describe()

	environment := normalizeEnvironmentFromAccountMode(account.Mode)
	if !slices.Contains(source.Environments, environment) {
		return domain.Account{}, fmt.Errorf("signal source %s does not support %s accounts", source.Key, account.Mode)
	}

	role := normalizeSignalSourceRole(stringValue(payload["role"]))
	if !slices.Contains(source.Roles, role) {
		return domain.Account{}, fmt.Errorf("signal source %s does not support role %s", source.Key, role)
	}

	symbol := NormalizeSymbol(stringValue(payload["symbol"]))
	options := cloneMetadata(metadataValue(payload["options"]))
	if options == nil {
		options = map[string]any{}
	}

	binding := domain.AccountSignalBinding{
		ID:         fmt.Sprintf("signal-binding-%d", time.Now().UnixNano()),
		AccountID:  account.ID,
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

	account.Metadata = cloneMetadata(account.Metadata)
	if account.Metadata == nil {
		account.Metadata = map[string]any{}
	}
	existing := resolveSignalBindings(account)
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
	account.Metadata["signalBindings"] = bindings
	return p.store.UpdateAccount(account)
}

func (p *Platform) ListAccountSignalBindings(accountID string) ([]domain.AccountSignalBinding, error) {
	account, err := p.store.GetAccount(accountID)
	if err != nil {
		return nil, err
	}
	raw := resolveSignalBindings(account)
	items := make([]domain.AccountSignalBinding, 0, len(raw))
	for _, binding := range raw {
		items = append(items, domain.AccountSignalBinding{
			ID:         stringValue(binding["id"]),
			AccountID:  account.ID,
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
	slices.SortFunc(items, func(a, b domain.AccountSignalBinding) int {
		if cmp := strings.Compare(a.Role, b.Role); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(a.Exchange, b.Exchange); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.Symbol, b.Symbol)
	})
	return items, nil
}

func resolveSignalBindings(account domain.Account) []map[string]any {
	if account.Metadata == nil {
		return nil
	}
	value, ok := account.Metadata["signalBindings"]
	if !ok {
		return nil
	}
	switch items := value.(type) {
	case []map[string]any:
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			out = append(out, cloneMetadata(item))
		}
		return out
	case []any:
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if binding, ok := item.(map[string]any); ok {
				out = append(out, cloneMetadata(binding))
			}
		}
		return out
	default:
		return nil
	}
}

func resolveStrategySignalBindings(parameters map[string]any) []map[string]any {
	if parameters == nil {
		return nil
	}
	value, ok := parameters["signalBindings"]
	if !ok {
		return nil
	}
	switch items := value.(type) {
	case []map[string]any:
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			out = append(out, cloneMetadata(item))
		}
		return out
	case []any:
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if binding, ok := item.(map[string]any); ok {
				out = append(out, cloneMetadata(binding))
			}
		}
		return out
	default:
		return nil
	}
}

func bindingToMap(binding domain.AccountSignalBinding) map[string]any {
	return map[string]any{
		"id":         binding.ID,
		"accountId":  binding.AccountID,
		"sourceKey":  binding.SourceKey,
		"sourceName": binding.SourceName,
		"exchange":   binding.Exchange,
		"role":       binding.Role,
		"streamType": binding.StreamType,
		"symbol":     binding.Symbol,
		"status":     binding.Status,
		"options":    cloneMetadata(binding.Options),
		"createdAt":  binding.CreatedAt,
	}
}

func metadataValue(value any) map[string]any {
	if value == nil {
		return nil
	}
	item, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return cloneMetadata(item)
}

func timeValue(value any) time.Time {
	switch v := value.(type) {
	case time.Time:
		return v
	case string:
		if parsed, err := time.Parse(time.RFC3339, v); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
