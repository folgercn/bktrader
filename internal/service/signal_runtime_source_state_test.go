package service

import "testing"

func TestFilterSignalRuntimeSourceStatesBySubscriptionsDropsOutOfPlanEntries(t *testing.T) {
	subscriptions := []map[string]any{
		{
			"sourceKey":  "binance-kline",
			"role":       "signal",
			"symbol":     "ETHUSDT",
			"streamType": "signal_bar",
			"options":    map[string]any{"timeframe": "1h"},
		},
		{
			"sourceKey":  "binance-trade-tick",
			"role":       "trigger",
			"symbol":     "ETHUSDT",
			"streamType": "trade_tick",
			"options":    map[string]any{},
		},
		{
			"sourceKey":  "binance-order-book",
			"role":       "feature",
			"symbol":     "ETHUSDT",
			"streamType": "order_book",
			"options":    map[string]any{},
		},
	}
	sourceStates := map[string]any{
		"binance-kline|signal|ETHUSDT|1h": map[string]any{
			"sourceKey":  "binance-kline",
			"role":       "signal",
			"symbol":     "ETHUSDT",
			"timeframe":  "1h",
			"streamType": "signal_bar",
		},
		"binance-trade-tick|trigger|ETHUSDT": map[string]any{
			"sourceKey":  "binance-trade-tick",
			"role":       "trigger",
			"symbol":     "ETHUSDT",
			"streamType": "trade_tick",
		},
		"|trigger|BTCUSDT": map[string]any{
			"symbol": "BTCUSDT",
		},
	}

	filtered := filterSignalRuntimeSourceStatesBySubscriptions(sourceStates, subscriptions)
	if _, ok := filtered["|trigger|BTCUSDT"]; ok {
		t.Fatalf("expected out-of-plan BTC source state to be pruned, got %#v", filtered)
	}
	if _, ok := filtered["binance-kline|signal|ETHUSDT|1h"]; !ok {
		t.Fatalf("expected ETH 1h kline source state to remain, got %#v", filtered)
	}
	if _, ok := filtered["binance-trade-tick|trigger|ETHUSDT"]; !ok {
		t.Fatalf("expected ETH trade tick source state to remain, got %#v", filtered)
	}
}
