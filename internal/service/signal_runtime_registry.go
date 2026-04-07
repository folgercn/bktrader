package service

import (
	"fmt"
	"slices"
	"strings"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

type SignalRuntimeAdapter interface {
	Key() string
	Describe() map[string]any
	Supports(source SignalSourceProvider) bool
	BuildSubscription(source domain.SignalSourceDefinition, binding domain.AccountSignalBinding) map[string]any
}

type staticSignalRuntimeAdapter struct {
	key          string
	name         string
	transport    string
	exchange     string
	streamTypes  []string
	environments []string
	metadata     map[string]any
}

func (a staticSignalRuntimeAdapter) Key() string {
	return strings.ToLower(strings.TrimSpace(a.key))
}

func (a staticSignalRuntimeAdapter) Describe() map[string]any {
	return map[string]any{
		"key":          a.Key(),
		"name":         a.name,
		"transport":    a.transport,
		"exchange":     a.exchange,
		"streamTypes":  a.streamTypes,
		"environments": a.environments,
		"metadata":     cloneMetadata(a.metadata),
	}
}

func (a staticSignalRuntimeAdapter) Supports(source SignalSourceProvider) bool {
	definition := source.Describe()
	if a.transport != "" && a.transport != definition.Transport {
		return false
	}
	if a.exchange != "" && !strings.EqualFold(a.exchange, definition.Exchange) {
		return false
	}
	if len(a.streamTypes) > 0 && !slices.Contains(a.streamTypes, definition.StreamType) {
		return false
	}
	return true
}

func (a staticSignalRuntimeAdapter) BuildSubscription(source domain.SignalSourceDefinition, binding domain.AccountSignalBinding) map[string]any {
	channel := binding.Symbol
	switch source.Exchange {
	case "BINANCE":
		switch source.StreamType {
		case "signal_bar":
			interval := normalizeSignalBarInterval(firstNonEmpty(strings.TrimSpace(stringValue(binding.Options["timeframe"])), "1d"))
			channel = strings.ToLower(binding.Symbol) + "@kline_" + interval
		case "trade_tick":
			channel = strings.ToLower(binding.Symbol) + "@trade"
		case "order_book":
			levels := maxIntValue(binding.Options["levels"], 20)
			updateSpeed := firstNonEmpty(strings.TrimSpace(stringValue(binding.Options["updateSpeed"])), "100ms")
			channel = strings.ToLower(binding.Symbol) + "@depth" + fmt.Sprintf("%d", levels) + "@" + updateSpeed
		}
	case "OKX":
		switch source.StreamType {
		case "trade_tick":
			channel = "trades:" + binding.Symbol
		case "order_book":
			levels := maxIntValue(binding.Options["levels"], 20)
			channel = fmt.Sprintf("books%d:%s", levels, binding.Symbol)
		}
	case "INTERNAL":
		channel = source.StreamType + ":" + binding.Symbol
	}
	return map[string]any{
		"adapterKey":  a.Key(),
		"sourceKey":   source.Key,
		"exchange":    source.Exchange,
		"streamType":  source.StreamType,
		"role":        binding.Role,
		"symbol":      binding.Symbol,
		"channel":     channel,
		"options":     cloneMetadata(binding.Options),
		"transport":   source.Transport,
		"environment": firstNonEmpty(firstString(source.Environments), "live"),
	}
}

func (p *Platform) registerSignalRuntimeAdapter(adapter SignalRuntimeAdapter) {
	if adapter == nil {
		return
	}
	if p.signalAdapters == nil {
		p.signalAdapters = make(map[string]SignalRuntimeAdapter)
	}
	p.signalAdapters[strings.ToLower(strings.TrimSpace(adapter.Key()))] = adapter
}

func (p *Platform) registerBuiltInSignalRuntimeAdapters() {
	p.registerSignalRuntimeAdapter(staticSignalRuntimeAdapter{
		key:          "binance-market-ws",
		name:         "Binance Market Data WebSocket",
		transport:    "websocket",
		exchange:     "BINANCE",
		streamTypes:  []string{"signal_bar", "trade_tick", "order_book"},
		environments: []string{"paper", "live"},
		metadata: map[string]any{
			"supportsCombinedStreams": true,
			"marketDataAuth":          false,
		},
	})
	p.registerSignalRuntimeAdapter(staticSignalRuntimeAdapter{
		key:          "okx-market-ws",
		name:         "OKX Market Data WebSocket",
		transport:    "websocket",
		exchange:     "OKX",
		streamTypes:  []string{"signal_bar", "trade_tick", "order_book"},
		environments: []string{"paper", "live"},
		metadata: map[string]any{
			"supportsCombinedStreams": true,
			"marketDataAuth":          false,
		},
	})
	p.registerSignalRuntimeAdapter(staticSignalRuntimeAdapter{
		key:          "replay-file-loader",
		name:         "Replay File Loader",
		transport:    "file",
		exchange:     "INTERNAL",
		streamTypes:  []string{"replay_tick", "minute_bar"},
		environments: []string{"backtest"},
		metadata: map[string]any{
			"streaming": true,
		},
	})
}

func (p *Platform) SignalRuntimeAdapters() []map[string]any {
	items := make([]map[string]any, 0, len(p.signalAdapters))
	for _, adapter := range p.signalAdapters {
		items = append(items, adapter.Describe())
	}
	slices.SortFunc(items, func(a, b map[string]any) int {
		return strings.Compare(stringValue(a["key"]), stringValue(b["key"]))
	})
	return items
}

func normalizeSignalBarInterval(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "1d", "d", "1day":
		return "1d"
	case "4h", "240", "4hour":
		return "4h"
	default:
		return value
	}
}

func (p *Platform) resolveSignalRuntimeAdapterForSource(sourceKey string) (SignalRuntimeAdapter, error) {
	provider, ok := p.signalSources[normalizeSignalSourceKey(sourceKey)]
	if !ok {
		return nil, fmt.Errorf("signal source not registered: %s", sourceKey)
	}
	for _, adapter := range p.signalAdapters {
		if adapter.Supports(provider) {
			return adapter, nil
		}
	}
	return nil, fmt.Errorf("no runtime adapter available for signal source: %s", sourceKey)
}

func (p *Platform) BuildSignalRuntimePlan(accountID, strategyID string) (map[string]any, error) {
	account, err := p.store.GetAccount(accountID)
	if err != nil {
		return nil, err
	}

	accountBindings, err := p.ListAccountSignalBindings(accountID)
	if err != nil {
		return nil, err
	}
	strategyBindings, err := p.ListStrategySignalBindings(strategyID)
	if err != nil {
		return nil, err
	}

	accountIndex := map[string]domain.AccountSignalBinding{}
	for _, binding := range accountBindings {
		accountIndex[signalBindingMatchKey(binding.SourceKey, binding.Role, binding.Symbol)] = binding
	}

	required := make([]map[string]any, 0, len(strategyBindings))
	matched := make([]map[string]any, 0, len(strategyBindings))
	missing := make([]map[string]any, 0)
	subscriptions := make([]map[string]any, 0, len(strategyBindings))
	for _, binding := range strategyBindings {
		item := bindingToMap(binding)
		if adapter, err := p.resolveSignalRuntimeAdapterForSource(binding.SourceKey); err == nil {
			item["runtimeAdapterKey"] = stringValue(adapter.Describe()["key"])
		}
		required = append(required, item)

		key := signalBindingMatchKey(binding.SourceKey, binding.Role, binding.Symbol)
		accountBinding, ok := accountIndex[key]
		if !ok {
			missing = append(missing, item)
			continue
		}

		runtimeAdapter, err := p.resolveSignalRuntimeAdapterForSource(binding.SourceKey)
		matchedItem := map[string]any{
			"strategyBinding": bindingToMap(binding),
			"accountBinding":  bindingToMap(accountBinding),
		}
		if err != nil {
			matchedItem["status"] = "MISSING_ADAPTER"
			matchedItem["error"] = err.Error()
		} else {
			matchedItem["status"] = "READY"
			matchedItem["runtimeAdapter"] = runtimeAdapter.Describe()
			source := p.signalSources[normalizeSignalSourceKey(binding.SourceKey)].Describe()
			subscription := runtimeAdapter.BuildSubscription(source, accountBinding)
			subscription["environment"] = normalizeEnvironmentFromAccountMode(account.Mode)
			subscription["accountMode"] = account.Mode
			matchedItem["subscription"] = subscription
			subscriptions = append(subscriptions, subscription)
		}
		matched = append(matched, matchedItem)
	}

	extra := make([]map[string]any, 0)
	requiredKeys := map[string]struct{}{}
	for _, binding := range strategyBindings {
		requiredKeys[signalBindingMatchKey(binding.SourceKey, binding.Role, binding.Symbol)] = struct{}{}
	}
	for _, binding := range accountBindings {
		key := signalBindingMatchKey(binding.SourceKey, binding.Role, binding.Symbol)
		if _, ok := requiredKeys[key]; ok {
			continue
		}
		item := bindingToMap(binding)
		if adapter, err := p.resolveSignalRuntimeAdapterForSource(binding.SourceKey); err == nil {
			item["runtimeAdapterKey"] = stringValue(adapter.Describe()["key"])
		}
		extra = append(extra, item)
	}

	triggerReady := true
	for _, binding := range strategyBindings {
		if binding.Role != "trigger" {
			continue
		}
		if _, ok := accountIndex[signalBindingMatchKey(binding.SourceKey, binding.Role, binding.Symbol)]; !ok {
			triggerReady = false
			break
		}
	}

	return map[string]any{
		"accountId":            accountID,
		"strategyId":           strategyID,
		"accountMode":          account.Mode,
		"accountExchange":      account.Exchange,
		"requiredBindings":     required,
		"matchedBindings":      matched,
		"missingBindings":      missing,
		"extraAccountBindings": extra,
		"subscriptions":        subscriptions,
		"ready":                len(missing) == 0 && triggerReady,
		"notes": []string{
			"策略绑定定义所需输入源，账户绑定定义实际订阅的市场流。",
			"trigger 源缺失会直接阻断 paper/live 的实时触发。",
			"feature 源缺失不会阻断最小运行，但会让依赖盘口特征的策略不可用。",
		},
	}, nil
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func signalBindingMatchKey(sourceKey, role, symbol string) string {
	return normalizeSignalSourceKey(sourceKey) + "|" + normalizeSignalSourceRole(role) + "|" + NormalizeSymbol(symbol)
}
