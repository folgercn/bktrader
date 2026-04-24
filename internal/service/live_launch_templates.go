package service

import (
	"fmt"
	"strings"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

type LiveLaunchTemplateStep struct {
	Key          string `json:"key"`
	Method       string `json:"method"`
	PathTemplate string `json:"pathTemplate"`
	PayloadRef   string `json:"payloadRef"`
	Description  string `json:"description"`
}

type LiveLaunchTemplate struct {
	Key                    string                   `json:"key"`
	Name                   string                   `json:"name"`
	Description            string                   `json:"description"`
	Symbol                 string                   `json:"symbol"`
	SignalTimeframe        string                   `json:"signalTimeframe"`
	DefaultDispatchMode    string                   `json:"defaultDispatchMode"`
	DispatchModeOptions    []string                 `json:"dispatchModeOptions"`
	TriggerSourceKey       string                   `json:"triggerSourceKey"`
	FeatureSourceKey       string                   `json:"featureSourceKey"`
	StrategyID             string                   `json:"strategyId"`
	StrategyName           string                   `json:"strategyName"`
	StrategyVersionID      string                   `json:"strategyVersionId,omitempty"`
	AccountRequirements    map[string]any           `json:"accountRequirements"`
	AccountBinding         map[string]any           `json:"accountBinding"`
	StrategySignalBindings []map[string]any         `json:"strategySignalBindings"`
	LaunchPayload          LiveLaunchOptions        `json:"launchPayload"`
	Steps                  []LiveLaunchTemplateStep `json:"steps"`
	Notes                  []string                 `json:"notes"`
}

func (p *Platform) LiveLaunchTemplates() ([]LiveLaunchTemplate, error) {
	strategyID, strategyName, strategyVersionID, strategyEngine, err := p.resolvePrimaryLiveTemplateStrategy()
	if err != nil {
		return nil, err
	}

	baseBinding := map[string]any{
		"adapterKey":    "binance-futures",
		"positionMode":  "ONE_WAY",
		"marginMode":    "CROSSED",
		"sandbox":       true,
		"executionMode": "rest",
		"credentialRefs": map[string]any{
			"apiKeyRef":    "BINANCE_TESTNET_API_KEY",
			"apiSecretRef": "BINANCE_TESTNET_API_SECRET",
		},
	}

	steps := []LiveLaunchTemplateStep{
		{
			Key:          "bind-account",
			Method:       "POST",
			PathTemplate: "/api/v1/live/accounts/:accountId/binding",
			PayloadRef:   "accountBinding",
			Description:  "确保账户被绑定为 Binance Futures testnet REST 账户。",
		},
		{
			Key:          "launch-live-flow",
			Method:       "POST",
			PathTemplate: "/api/v1/live/accounts/:accountId/launch",
			PayloadRef:   "launchPayload",
			Description:  "独占替换当前模板绑定，刷新 runtime 订阅，并按 symbol + timeframe 创建或复用 live session。",
		},
	}

	buildTemplate := func(symbol, timeframe string, quantity float64, applyResearchBaseline bool) LiveLaunchTemplate {
		signalBindings := []map[string]any{
			{
				"sourceKey": "binance-kline",
				"role":      "signal",
				"symbol":    symbol,
				"options": map[string]any{
					"timeframe": timeframe,
				},
			},
			{
				"sourceKey": "binance-trade-tick",
				"role":      "trigger",
				"symbol":    symbol,
			},
			{
				"sourceKey": "binance-order-book",
				"role":      "feature",
				"symbol":    symbol,
			},
		}
		liveOverrides := map[string]any{
			"symbol":                                  symbol,
			"signalTimeframe":                         timeframe,
			"executionDataSource":                     "tick",
			"positionSizingMode":                      "fixed_quantity",
			"defaultOrderQuantity":                    quantity,
			"executionStrategy":                       "book-aware-v1",
			"executionEntryOrderType":                 "MARKET",
			"executionEntryMaxSpreadBps":              8,
			"executionEntryMaxSlippageBps":            8,
			"executionEntryWideSpreadMode":            "limit-maker",
			"executionEntryRestingTimeoutSeconds":     15,
			"executionEntryTimeoutFallbackOrderType":  "MARKET",
			"executionPTExitOrderType":                "LIMIT",
			"executionPTExitTimeInForce":              "GTX",
			"executionPTExitPostOnly":                 true,
			"executionPTExitWideSpreadMode":           "limit-maker",
			"executionPTExitTimeoutFallbackOrderType": "MARKET",
			"executionSLExitOrderType":                "MARKET",
			"executionSLExitMaxSpreadBps":             999,
			"executionSLExitTimeoutFallbackOrderType": "MARKET",
			"dispatchCooldownSeconds":                 30,
		}
		baselineNotes := []string{}
		if applyResearchBaseline {
			for key, value := range liveIntradayResearchBaselineOverrides(strategyEngine) {
				liveOverrides[key] = value
			}
			baselineNotes = append(baselineNotes,
				"该模板已显式固化 intraday research baseline：dir2 zero initial + reentry_window + reentry_size_schedule=[0.20, 0.10] + max_trades_per_bar=1。",
				"非 1d 周期默认使用 canonical SMA5 hard filter；止损与移动止损参数分别固定为 stop_loss_atr=0.3、trailing_stop_atr=0.3、profit_protect_atr=1.0、delayed_trailing_activation_atr=0.5。",
				"当前 research baseline 只提升 BTCUSDT 15m/30m；BTCUSDT 5m 暂保留为通用模板，因为现阶段 5m 对执行摩擦更敏感，尚不作为默认 intraday baseline 候选。",
			)
		}
		key := fmt.Sprintf("binance-testnet-%s-%s", strings.ToLower(symbol[:3]), strings.ToLower(timeframe))
		quantityNote := fmt.Sprintf("默认下单量 %.3f 用于尽量避免 Binance testnet 最小名义价值拦截。", quantity)
		notes := []string{
			fmt.Sprintf("当前主策略使用 %s（strategyEngine=%s）。", strategyName, firstNonEmpty(strategyEngine, "bk-default")),
			"signal 绑定使用 Binance 原生 kline；trigger 绑定使用 Binance trade tick；feature 绑定使用 Binance order book。",
			"点击模板会独占替换该策略当前模板绑定，不再在旧模板之上继续叠加 symbol / timeframe。",
			quantityNote,
			"模板里只有 dispatchMode 需要前端在提交前注入；其余 launch 参数保持固定。",
			"launch 会在安全前提下刷新 account + strategy 级 runtime 订阅，live session 仍按 symbol + signalTimeframe 分开创建或复用。",
		}
		notes = append(notes, baselineNotes...)
		return LiveLaunchTemplate{
			Key:                 key,
			Name:                fmt.Sprintf("Binance Testnet %s %s", symbol, timeframe),
			Description:         fmt.Sprintf("%s %s 策略信号 + trade tick 触发 + order book feature 的一键启动模板。", symbol, timeframe),
			Symbol:              symbol,
			SignalTimeframe:     timeframe,
			DefaultDispatchMode: "manual-review",
			DispatchModeOptions: liveLaunchTemplateDispatchModeOptions(),
			TriggerSourceKey:    "binance-trade-tick",
			FeatureSourceKey:    "binance-order-book",
			StrategyID:          strategyID,
			StrategyName:        strategyName,
			StrategyVersionID:   strategyVersionID,
			AccountRequirements: map[string]any{
				"mode":     "LIVE",
				"exchange": "binance-futures",
				"sandbox":  true,
			},
			AccountBinding:         cloneMetadata(baseBinding),
			StrategySignalBindings: cloneMetadataList(signalBindings),
			LaunchPayload: LiveLaunchOptions{
				StrategyID:             strategyID,
				Binding:                cloneMetadata(baseBinding),
				StrategySignalBindings: cloneMetadataList(signalBindings),
				LiveSessionOverrides:   cloneMetadata(liveOverrides),
				LaunchTemplateKey:      key,
				LaunchTemplateName:     fmt.Sprintf("Binance Testnet %s %s", symbol, timeframe),
				MirrorStrategySignals:  true,
				StartRuntime:           true,
				StartSession:           true,
			},
			Steps: cloneLiveLaunchTemplateSteps(steps),
			Notes: notes,
		}
	}

	return []LiveLaunchTemplate{
		buildTemplate("BTCUSDT", "5m", 0.002, false),
		buildTemplate("BTCUSDT", "15m", 0.002, true),
		buildTemplate("BTCUSDT", "30m", 0.002, true),
		buildTemplate("BTCUSDT", "4h", 0.002, false),
		buildTemplate("BTCUSDT", "1d", 0.002, false),
		buildTemplate("ETHUSDT", "5m", 0.100, false),
		buildTemplate("ETHUSDT", "4h", 0.100, false),
		buildTemplate("ETHUSDT", "1d", 0.100, false),
	}, nil
}

func liveLaunchTemplateDispatchModeOptions() []string {
	return []string{"manual-review", liveLaunchTemplateAutoDispatchMode()}
}

func liveLaunchTemplateAutoDispatchMode() string {
	return strings.Join([]string{"auto", "dispatch"}, "-")
}

func liveIntradayResearchBaselineTemplateKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "binance-testnet-btc-15m", "binance-testnet-btc-30m":
		return true
	default:
		return false
	}
}

func liveIntradayResearchBaselineOverrides(strategyEngine string) map[string]any {
	return map[string]any{
		"strategyEngine":                  firstNonEmpty(strategyEngine, "bk-default"),
		"positionSizingMode":              "reentry_size_schedule",
		"dir2_zero_initial":               domain.ResearchBaselineDir2ZeroInitial,
		"zero_initial_mode":               domain.ResearchBaselineZeroInitialMode,
		"stop_mode":                       "atr",
		"stop_loss_atr":                   0.3,
		"profit_protect_atr":              1.0,
		"trailing_stop_atr":               0.3,
		"delayed_trailing_activation_atr": 0.5,
		"long_reentry_atr":                0.1,
		"short_reentry_atr":               0.0,
		"max_trades_per_bar":              1,
		"reentry_size_schedule":           domain.ResearchBaselineReentrySizeSchedule(),
		"executionEntryMaxSlippageBps":    8,
	}
}

func (p *Platform) resolvePrimaryLiveTemplateStrategy() (string, string, string, string, error) {
	preferred := []string{"strategy-bk-1d"}
	for _, strategyID := range preferred {
		strategy, err := p.GetStrategy(strategyID)
		if err != nil {
			continue
		}
		return liveTemplateStrategyMetadata(strategy)
	}
	strategies, err := p.ListStrategies()
	if err != nil {
		return "", "", "", "", err
	}
	for _, strategy := range strategies {
		if strings.EqualFold(stringValue(strategy["status"]), "ACTIVE") {
			return liveTemplateStrategyMetadata(strategy)
		}
	}
	if len(strategies) > 0 {
		return liveTemplateStrategyMetadata(strategies[0])
	}
	return "", "", "", "", fmt.Errorf("no strategy available for live launch templates")
}

func liveTemplateStrategyMetadata(strategy map[string]any) (string, string, string, string, error) {
	if strategy == nil {
		return "", "", "", "", fmt.Errorf("strategy metadata is empty")
	}
	strategyID := strings.TrimSpace(stringValue(strategy["id"]))
	if strategyID == "" {
		return "", "", "", "", fmt.Errorf("strategy metadata missing id")
	}
	strategyName := strings.TrimSpace(stringValue(strategy["name"]))
	versionID := ""
	strategyEngine := ""
	switch current := strategy["currentVersion"].(type) {
	case domain.StrategyVersion:
		versionID = current.ID
		strategyEngine = stringValue(current.Parameters["strategyEngine"])
	case map[string]any:
		versionID = stringValue(current["id"])
		strategyEngine = stringValue(mapValue(current["parameters"])["strategyEngine"])
	}
	return strategyID, strategyName, versionID, strategyEngine, nil
}

func cloneMetadataList(items []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, cloneMetadata(item))
	}
	return out
}

func cloneLiveLaunchTemplateSteps(items []LiveLaunchTemplateStep) []LiveLaunchTemplateStep {
	out := make([]LiveLaunchTemplateStep, len(items))
	copy(out, items)
	return out
}
