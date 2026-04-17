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

	buildTemplate := func(symbol, timeframe string, quantity float64) LiveLaunchTemplate {
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
		key := fmt.Sprintf("binance-testnet-%s-%s", strings.ToLower(symbol[:3]), strings.ToLower(timeframe))
		quantityNote := fmt.Sprintf("默认下单量 %.3f 用于尽量避免 Binance testnet 最小名义价值拦截。", quantity)
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
			Notes: []string{
				fmt.Sprintf("当前主策略使用 %s（strategyEngine=%s）。", strategyName, firstNonEmpty(strategyEngine, "bk-default")),
				"signal 绑定使用 Binance 原生 kline；trigger 绑定使用 Binance trade tick；feature 绑定使用 Binance order book。",
				"点击模板会独占替换该策略当前模板绑定，不再在旧模板之上继续叠加 symbol / timeframe。",
				quantityNote,
				"模板里只有 dispatchMode 需要前端在提交前注入；其余 launch 参数保持固定。",
				"launch 会在安全前提下刷新 account + strategy 级 runtime 订阅，live session 仍按 symbol + signalTimeframe 分开创建或复用。",
			},
		}
	}

	return []LiveLaunchTemplate{
		buildTemplate("BTCUSDT", "5m", 0.002),
		buildTemplate("BTCUSDT", "4h", 0.002),
		buildTemplate("BTCUSDT", "1d", 0.002),
		buildTemplate("ETHUSDT", "5m", 0.100),
		buildTemplate("ETHUSDT", "4h", 0.100),
		buildTemplate("ETHUSDT", "1d", 0.100),
	}, nil
}

func liveLaunchTemplateDispatchModeOptions() []string {
	return []string{"manual-review", liveLaunchTemplateAutoDispatchMode()}
}

func liveLaunchTemplateAutoDispatchMode() string {
	return strings.Join([]string{"auto", "dispatch"}, "-")
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
