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

const (
	btc30mBaselinePlusT3ReentryMinStopBps       = 4.0
	btc30mBaselinePlusT3ReentryATRPercentileGTE = 10.0
)

func (p *Platform) LiveLaunchTemplates() ([]LiveLaunchTemplate, error) {
	strategyID, strategyName, strategyVersionID, strategyEngine, err := p.resolvePrimaryLiveTemplateStrategy()
	if err != nil {
		return nil, err
	}
	enhancedStrategyID := ""
	enhancedStrategyName := ""
	enhancedStrategyVersionID := ""
	hasEnhancedTemplate := false
	if id, name, versionID, _, err := p.resolveLiveTemplateStrategy("strategy-bk-btc-30m-enhanced"); err == nil {
		enhancedStrategyID = id
		enhancedStrategyName = name
		enhancedStrategyVersionID = versionID
		hasEnhancedTemplate = true
	}

	t3EnhancedStrategyID := ""
	t3EnhancedStrategyName := ""
	t3EnhancedStrategyVersionID := ""
	hasT3EnhancedTemplate := false
	if id, name, versionID, _, err := p.resolveLiveTemplateStrategy("strategy-bk-btc-30m-enhanced-t3"); err == nil {
		t3EnhancedStrategyID = id
		t3EnhancedStrategyName = name
		t3EnhancedStrategyVersionID = versionID
		hasT3EnhancedTemplate = true
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
			"executionEntryMaxBookAgeMs":              500,
			"executionEntryMinTopBookCoverage":        0.5,
			"executionEntryMaxSourceDivergenceBps":    8,
			"executionEntryWideSpreadMode":            "limit-maker",
			"executionEntryRestingTimeoutSeconds":     15,
			"executionEntryTimeoutFallbackOrderType":  "MARKET",
			"executionPTExitOrderType":                "LIMIT",
			"executionPTExitTimeInForce":              "GTX",
			"executionPTExitPostOnly":                 true,
			"executionPTExitWideSpreadMode":           "limit-maker",
			"executionPTExitTimeoutFallbackOrderType": "MARKET",
			"executionSLExitOrderType":                "MARKET",
			"executionSLExitMaxSpreadBps":             8,
			"executionSLMaxSlippageBps":               8,
			"executionSLExitTimeoutFallbackOrderType": "MARKET",
			"dispatchCooldownSeconds":                 30,
		}
		baselineNotes := []string{}
		if applyResearchBaseline {
			for key, value := range liveIntradayResearchBaselineOverrides(strategyEngine) {
				liveOverrides[key] = value
			}
			baselineNotes = append(baselineNotes,
				"该模板已显式固化 intraday research baseline：dir2 zero initial + reentry_window + reentry_size_schedule=[0.20, 0.10] + max_trades_per_bar=2。",
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
	buildLegacyEnhancedTemplate := func() LiveLaunchTemplate {
		item := buildTemplate("BTCUSDT", "30m", 0.002, false)
		item.Key = "binance-testnet-btc-30m-enhanced"
		item.Name = "Binance Testnet BTCUSDT 30m T2-only 0.5bps"
		item.Description = "BTCUSDT 30m legacy 增强策略：live_intrabar_sma5_original_t2_breakout_tol_0p5bps。"
		item.StrategyID = enhancedStrategyID
		item.StrategyName = enhancedStrategyName
		item.StrategyVersionID = enhancedStrategyVersionID
		for key, value := range liveBTC30mLegacyEnhancedOverrides() {
			item.LaunchPayload.LiveSessionOverrides[key] = value
		}
		item.LaunchPayload.StrategyID = enhancedStrategyID
		item.LaunchPayload.LaunchTemplateKey = item.Key
		item.LaunchPayload.LaunchTemplateName = item.Name
		item.Notes = append([]string{
			fmt.Sprintf("legacy 增强模板继续使用 %s（strategyEngine=%s），用于复现现有 T2-only live 行为。", enhancedStrategyName, bkLiveIntrabarSMA5T2Only0p5BpsEngineKey),
			"策略口径：SMA5 intrabar hard filter + original_t2 breakout + T2 shape tolerance 0.5 bps；t3_swing breakout 不在该 legacy 模板启用。",
			"低波动 reentry gate 仍为 reentry_min_stop_bps=6 与 reentry_atr_percentile_gte=25，仅过滤 Zero-Initial/SL/PT reentry 开仓，不过滤止损出场。",
			"模板仍保持 dispatchMode 由前端提交前选择，默认展示为 manual-review。",
		}, item.Notes...)
		return item
	}
	buildBaselinePlusT3EnhancedTemplate := func() LiveLaunchTemplate {
		item := buildTemplate("BTCUSDT", "30m", 0.002, false)
		item.Key = "binance-testnet-btc-30m-baseline-plus-t3-enhanced"
		item.Name = "Binance Testnet BTCUSDT 30m Baseline+T3 Enhanced"
		item.Description = "BTCUSDT 30m 新增强策略：live_intrabar_sma5_baseline_plus_t3_enhanced。"
		item.StrategyID = t3EnhancedStrategyID
		item.StrategyName = t3EnhancedStrategyName
		item.StrategyVersionID = t3EnhancedStrategyVersionID
		for key, value := range liveBTC30mBaselinePlusT3EnhancedOverrides() {
			item.LaunchPayload.LiveSessionOverrides[key] = value
		}
		item.LaunchPayload.StrategyID = t3EnhancedStrategyID
		item.LaunchPayload.LaunchTemplateKey = item.Key
		item.LaunchPayload.LaunchTemplateName = item.Name
		item.Notes = append([]string{
			fmt.Sprintf("Baseline+T3 增强模板单独使用 %s（strategyEngine=%s），旧 T2-only 模板 key %s 保留兼容。", t3EnhancedStrategyName, bkLiveIntrabarSMA5BaselinePlusT3EnhancedEngineKey, "binance-testnet-btc-30m-enhanced"),
			"策略口径：SMA5 intrabar hard filter + baseline_plus_t3 breakout，original_t2 与 t3_swing 均可锁定 reentry window。",
			"baseline breakout gate 固化为 min_atr_percentile=25 与 min_sma_atr_separation=0.1；legacy reentry gate 保留但放松为 reentry_min_stop_bps=4 与 reentry_atr_percentile_gte=10，仅过滤 entry/reentry，不过滤止损出场。",
			"模板仍保持 dispatchMode 由前端提交前选择，默认展示为 manual-review。",
		}, item.Notes...)
		return item
	}

	templates := []LiveLaunchTemplate{
		buildTemplate("BTCUSDT", "5m", 0.002, false),
		buildTemplate("BTCUSDT", "15m", 0.002, true),
		buildTemplate("BTCUSDT", "30m", 0.002, true),
	}
	if hasEnhancedTemplate {
		templates = append(templates, buildLegacyEnhancedTemplate())
	}
	if hasT3EnhancedTemplate {
		templates = append(templates, buildBaselinePlusT3EnhancedTemplate())
	}
	templates = append(templates,
		buildTemplate("BTCUSDT", "4h", 0.002, false),
		buildTemplate("BTCUSDT", "1d", 0.002, false),
		buildTemplate("ETHUSDT", "5m", 0.100, false),
		buildTemplate("ETHUSDT", "4h", 0.100, false),
		buildTemplate("ETHUSDT", "1d", 0.100, false),
	)
	return templates, nil
}

func liveLaunchTemplateDispatchModeOptions() []string {
	return []string{"manual-review", liveLaunchTemplateAutoDispatchMode()}
}

func liveLaunchTemplateAutoDispatchMode() string {
	return strings.Join([]string{"auto", "dispatch"}, "-")
}

func liveIntradayResearchBaselineTemplateKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "binance-testnet-btc-15m", "binance-testnet-btc-30m", "binance-testnet-btc-30m-baseline-plus-t3-enhanced":
		return true
	default:
		return false
	}
}

func liveIntradayResearchBaselineOverrides(strategyEngine string) map[string]any {
	return map[string]any{
		"strategyEngine":                       firstNonEmpty(strategyEngine, "bk-default"),
		"positionSizingMode":                   "reentry_size_schedule",
		"dir2_zero_initial":                    domain.ResearchBaselineDir2ZeroInitial,
		"zero_initial_mode":                    domain.ResearchBaselineZeroInitialMode,
		"stop_mode":                            "atr",
		"stop_loss_atr":                        0.3,
		"profit_protect_atr":                   1.0,
		"trailing_stop_atr":                    0.3,
		"delayed_trailing_activation_atr":      0.5,
		"long_reentry_atr":                     0.1,
		"short_reentry_atr":                    0.0,
		"max_trades_per_bar":                   domain.ResearchBaselineMaxTradesPerBar,
		"reentry_size_schedule":                domain.ResearchBaselineReentrySizeSchedule(),
		"executionEntryMaxSlippageBps":         8,
		"executionEntryMaxBookAgeMs":           500,
		"executionEntryMinTopBookCoverage":     0.5,
		"executionEntryMaxSourceDivergenceBps": 8,
		"executionSLExitMaxSpreadBps":          8,
		"executionSLMaxSlippageBps":            8,
	}
}

func liveBTC30mLegacyEnhancedOverrides() map[string]any {
	overrides := liveIntradayResearchBaselineOverrides(bkLiveIntrabarSMA5T2Only0p5BpsEngineKey)
	overrides["max_trades_per_bar"] = domain.ResearchBaselineMaxTradesPerBar
	overrides["stop_loss_atr"] = 0.3
	overrides["sl_reentry_min_delay_seconds"] = 60
	overrides["breakout_shape"] = "original_t2"
	overrides["breakout_shape_tolerance_bps"] = defaultT2BreakoutShapeToleranceBps
	overrides["use_sma5_intraday_structure"] = true
	overrides["reentry_min_stop_bps"] = 6.0
	overrides["reentry_atr_percentile_gte"] = 25.0
	return overrides
}

func liveBTC30mBaselinePlusT3EnhancedOverrides() map[string]any {
	overrides := liveIntradayResearchBaselineOverrides(bkLiveIntrabarSMA5BaselinePlusT3EnhancedEngineKey)
	overrides["max_trades_per_bar"] = domain.ResearchBaselineMaxTradesPerBar
	overrides["stop_loss_atr"] = 0.3
	overrides["sl_reentry_min_delay_seconds"] = 60
	overrides["breakout_shape"] = "baseline_plus_t3"
	overrides["breakout_shape_tolerance_bps"] = defaultT2BreakoutShapeToleranceBps
	overrides["use_sma5_intraday_structure"] = true
	overrides["min_atr_percentile"] = 25.0
	overrides["min_sma_atr_separation"] = 0.1
	overrides["quality_filter_shapes"] = []string{"original_t2", "t3_swing"}
	overrides["reentry_min_stop_bps"] = btc30mBaselinePlusT3ReentryMinStopBps
	overrides["reentry_atr_percentile_gte"] = btc30mBaselinePlusT3ReentryATRPercentileGTE
	return overrides
}

func (p *Platform) resolvePrimaryLiveTemplateStrategy() (string, string, string, string, error) {
	preferred := []string{"strategy-bk-1d"}
	for _, strategyID := range preferred {
		if strategyID, strategyName, versionID, strategyEngine, err := p.resolveLiveTemplateStrategy(strategyID); err == nil {
			return strategyID, strategyName, versionID, strategyEngine, nil
		}
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

func (p *Platform) resolveLiveTemplateStrategy(strategyID string) (string, string, string, string, error) {
	strategy, err := p.GetStrategy(strategyID)
	if err != nil {
		return "", "", "", "", err
	}
	return liveTemplateStrategyMetadata(strategy)
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
