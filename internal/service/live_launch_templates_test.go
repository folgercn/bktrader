package service

import (
	"testing"

	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestLiveLaunchTemplatesExposeEightBinanceTestnetVariants(t *testing.T) {
	platform := NewPlatform(memory.NewStore())

	templates, err := platform.LiveLaunchTemplates()
	if err != nil {
		t.Fatalf("list live launch templates failed: %v", err)
	}
	if len(templates) != 8 {
		t.Fatalf("expected 8 launch templates, got %d", len(templates))
	}

	expected := map[string]struct {
		symbol           string
		timeframe        string
		quantity         float64
		researchBaseline bool
	}{
		"binance-testnet-btc-5m":  {symbol: "BTCUSDT", timeframe: "5m", quantity: 0.002},
		"binance-testnet-btc-15m": {symbol: "BTCUSDT", timeframe: "15m", quantity: 0.002, researchBaseline: true},
		"binance-testnet-btc-30m": {symbol: "BTCUSDT", timeframe: "30m", quantity: 0.002, researchBaseline: true},
		"binance-testnet-btc-4h":  {symbol: "BTCUSDT", timeframe: "4h", quantity: 0.002},
		"binance-testnet-btc-1d":  {symbol: "BTCUSDT", timeframe: "1d", quantity: 0.002},
		"binance-testnet-eth-5m":  {symbol: "ETHUSDT", timeframe: "5m", quantity: 0.1},
		"binance-testnet-eth-4h":  {symbol: "ETHUSDT", timeframe: "4h", quantity: 0.1},
		"binance-testnet-eth-1d":  {symbol: "ETHUSDT", timeframe: "1d", quantity: 0.1},
	}

	for _, item := range templates {
		want, ok := expected[item.Key]
		if !ok {
			t.Fatalf("unexpected template key %s", item.Key)
		}
		if item.StrategyID != "strategy-bk-1d" {
			t.Fatalf("expected strategy-bk-1d, got %s", item.StrategyID)
		}
		if item.Symbol != want.symbol {
			t.Fatalf("expected symbol %s for %s, got %s", want.symbol, item.Key, item.Symbol)
		}
		if item.SignalTimeframe != want.timeframe {
			t.Fatalf("expected timeframe %s for %s, got %s", want.timeframe, item.Key, item.SignalTimeframe)
		}
		if item.DefaultDispatchMode != "manual-review" {
			t.Fatalf("expected default dispatchMode manual-review, got %s", item.DefaultDispatchMode)
		}
		if len(item.DispatchModeOptions) != 2 || item.DispatchModeOptions[0] != "manual-review" || item.DispatchModeOptions[1] != "auto-dispatch" {
			t.Fatalf("expected dispatch mode options [manual-review auto-dispatch], got %#v", item.DispatchModeOptions)
		}
		if stringValue(item.AccountBinding["adapterKey"]) != "binance-futures" {
			t.Fatalf("expected binance-futures account binding, got %#v", item.AccountBinding)
		}
		if stringValue(item.AccountBinding["executionMode"]) != "rest" {
			t.Fatalf("expected executionMode=rest, got %#v", item.AccountBinding)
		}
		if !boolValue(item.AccountBinding["sandbox"]) {
			t.Fatalf("expected sandbox=true, got %#v", item.AccountBinding)
		}
		if len(item.StrategySignalBindings) != 3 {
			t.Fatalf("expected 3 strategy bindings for %s, got %#v", item.Key, item.StrategySignalBindings)
		}
		if len(item.LaunchPayload.StrategySignalBindings) != 3 {
			t.Fatalf("expected launch payload strategy bindings for %s, got %#v", item.Key, item.LaunchPayload.StrategySignalBindings)
		}
		if item.TriggerSourceKey != "binance-trade-tick" {
			t.Fatalf("expected trade tick trigger source, got %s", item.TriggerSourceKey)
		}
		if item.FeatureSourceKey != "binance-order-book" {
			t.Fatalf("expected order book feature source, got %s", item.FeatureSourceKey)
		}
		if !item.LaunchPayload.MirrorStrategySignals || !item.LaunchPayload.StartRuntime || !item.LaunchPayload.StartSession {
			t.Fatalf("expected template launch flags to all be true: %#v", item.LaunchPayload)
		}
		if got := stringValue(item.LaunchPayload.LiveSessionOverrides["symbol"]); got != want.symbol {
			t.Fatalf("expected launch symbol %s, got %s", want.symbol, got)
		}
		if got := stringValue(item.LaunchPayload.LiveSessionOverrides["signalTimeframe"]); got != want.timeframe {
			t.Fatalf("expected launch timeframe %s, got %s", want.timeframe, got)
		}
		if got := stringValue(item.LaunchPayload.LiveSessionOverrides["executionDataSource"]); got != "tick" {
			t.Fatalf("expected executionDataSource=tick, got %s", got)
		}
		if got := stringValue(item.LaunchPayload.LiveSessionOverrides["executionStrategy"]); got != "book-aware-v1" {
			t.Fatalf("expected executionStrategy=book-aware-v1, got %s", got)
		}
		if got := parseFloatValue(item.LaunchPayload.LiveSessionOverrides["executionEntryMaxSlippageBps"]); got != 8 {
			t.Fatalf("expected executionEntryMaxSlippageBps=8 for %s, got %v", item.Key, got)
		}
		if _, ok := item.LaunchPayload.LiveSessionOverrides["dispatchMode"]; ok {
			t.Fatalf("expected dispatchMode to stay configurable and not be hardcoded in launch payload: %#v", item.LaunchPayload.LiveSessionOverrides)
		}
		if got := parseFloatValue(item.LaunchPayload.LiveSessionOverrides["defaultOrderQuantity"]); got != want.quantity {
			t.Fatalf("expected defaultOrderQuantity=%v, got %v", want.quantity, got)
		}
		if want.researchBaseline {
			if got := stringValue(item.LaunchPayload.LiveSessionOverrides["positionSizingMode"]); got != "reentry_size_schedule" {
				t.Fatalf("expected positionSizingMode=reentry_size_schedule for %s, got %s", item.Key, got)
			}
			if got := stringValue(item.LaunchPayload.LiveSessionOverrides["strategyEngine"]); got == "" {
				t.Fatalf("expected explicit strategyEngine for %s", item.Key)
			}
			if !boolValue(item.LaunchPayload.LiveSessionOverrides["dir2_zero_initial"]) {
				t.Fatalf("expected dir2_zero_initial=true for %s", item.Key)
			}
			if got := stringValue(item.LaunchPayload.LiveSessionOverrides["zero_initial_mode"]); got != "reentry_window" {
				t.Fatalf("expected zero_initial_mode=reentry_window for %s, got %s", item.Key, got)
			}
			if got := stringValue(item.LaunchPayload.LiveSessionOverrides["stop_mode"]); got != "atr" {
				t.Fatalf("expected stop_mode=atr for %s, got %s", item.Key, got)
			}
			if got := parseFloatValue(item.LaunchPayload.LiveSessionOverrides["stop_loss_atr"]); got != 0.3 {
				t.Fatalf("expected stop_loss_atr=0.3 for %s, got %v", item.Key, got)
			}
			if got := parseFloatValue(item.LaunchPayload.LiveSessionOverrides["profit_protect_atr"]); got != 1.0 {
				t.Fatalf("expected profit_protect_atr=1.0 for %s, got %v", item.Key, got)
			}
			if got := parseFloatValue(item.LaunchPayload.LiveSessionOverrides["trailing_stop_atr"]); got != 0.3 {
				t.Fatalf("expected trailing_stop_atr=0.3 for %s, got %v", item.Key, got)
			}
			if got := parseFloatValue(item.LaunchPayload.LiveSessionOverrides["delayed_trailing_activation_atr"]); got != 0.5 {
				t.Fatalf("expected delayed_trailing_activation_atr=0.5 for %s, got %v", item.Key, got)
			}
			if got := parseFloatValue(item.LaunchPayload.LiveSessionOverrides["long_reentry_atr"]); got != 0.1 {
				t.Fatalf("expected long_reentry_atr=0.1 for %s, got %v", item.Key, got)
			}
			if got := parseFloatValue(item.LaunchPayload.LiveSessionOverrides["short_reentry_atr"]); got != 0.0 {
				t.Fatalf("expected short_reentry_atr=0.0 for %s, got %v", item.Key, got)
			}
			if got := maxIntValue(item.LaunchPayload.LiveSessionOverrides["max_trades_per_bar"], 0); got != 1 {
				t.Fatalf("expected max_trades_per_bar=1 for %s, got %d", item.Key, got)
			}
			schedule := normalizeBacktestFloatSlice(item.LaunchPayload.LiveSessionOverrides["reentry_size_schedule"], nil)
			if len(schedule) != 2 || schedule[0] != 0.20 || schedule[1] != 0.10 {
				t.Fatalf("expected reentry_size_schedule [0.20, 0.10] for %s, got %#v", item.Key, item.LaunchPayload.LiveSessionOverrides["reentry_size_schedule"])
			}
		}
		if item.LaunchPayload.LaunchTemplateKey != item.Key {
			t.Fatalf("expected launch template key %s, got %s", item.Key, item.LaunchPayload.LaunchTemplateKey)
		}
	}
}

func TestLiveLaunchTemplatesIncludeIdempotentFrontendWorkflow(t *testing.T) {
	platform := NewPlatform(memory.NewStore())

	templates, err := platform.LiveLaunchTemplates()
	if err != nil {
		t.Fatalf("list live launch templates failed: %v", err)
	}
	if len(templates) == 0 {
		t.Fatal("expected launch templates")
	}

	steps := templates[0].Steps
	if len(steps) != 2 {
		t.Fatalf("expected 2 workflow steps, got %#v", steps)
	}
	if steps[0].PathTemplate != "/api/v1/live/accounts/:accountId/binding" {
		t.Fatalf("unexpected step 1 path: %#v", steps[0])
	}
	if steps[1].PathTemplate != "/api/v1/live/accounts/:accountId/launch" {
		t.Fatalf("unexpected step 2 path: %#v", steps[1])
	}
}

func TestLiveLaunchTemplatesExposeDispatchModeMetadata(t *testing.T) {
	platform := NewPlatform(memory.NewStore())

	templates, err := platform.LiveLaunchTemplates()
	if err != nil {
		t.Fatalf("list live launch templates failed: %v", err)
	}

	for _, item := range templates {
		// 验证模板层级暴露了默认分发模式，供前端 hook 使用，避免前端硬编码安全基线
		if item.DefaultDispatchMode == "" {
			t.Errorf("template %s: expected non-empty DefaultDispatchMode", item.Key)
		}

		// 验证 LaunchPayload 中不应包含强制的 dispatchMode 覆盖
		// 这样前端在调用 launch 接口时，传入的 overrides 才能生效，且前端有权决定回落逻辑
		if _, ok := item.LaunchPayload.LiveSessionOverrides["dispatchMode"]; ok {
			t.Errorf("template %s: LaunchPayload.LiveSessionOverrides should not contain fixed dispatchMode", item.Key)
		}

		// 验证提供了合法的模式选项以便 UI 渲染
		foundManual := false
		foundAuto := false
		for _, opt := range item.DispatchModeOptions {
			if opt == "manual-review" {
				foundManual = true
			}
			if opt == "auto-dispatch" {
				foundAuto = true
			}
		}
		if !foundManual || !foundAuto {
			t.Errorf("template %s: missing required dispatch mode options, got %#v", item.Key, item.DispatchModeOptions)
		}
	}
}
