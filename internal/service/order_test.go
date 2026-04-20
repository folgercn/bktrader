package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"strings"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestBuildLiveSyncSettlementKeepsExchangeTradeIDEmptyWithoutRealTradeID(t *testing.T) {
	order := domain.Order{ID: "order-1", Symbol: "BTCUSDT"}

	fills, _, _ := buildLiveSyncSettlement(order, LiveOrderSync{
		Status: "FILLED",
		Fills: []LiveFillReport{{
			Price:    68000,
			Quantity: 0.1,
			Fee:      1.23,
			Metadata: map[string]any{
				"exchangeOrderId": "exchange-order-1",
				"tradeTime":       "2026-04-17T12:36:00Z",
			},
		}},
	})

	if len(fills) != 1 {
		t.Fatalf("expected one fill, got %d", len(fills))
	}
	if got := fills[0].ExchangeTradeID; got != "" {
		t.Fatalf("expected missing real trade id to stay empty, got %q", got)
	}
	if fills[0].ExchangeTradeTime == nil || fills[0].ExchangeTradeTime.Format(time.RFC3339) != "2026-04-17T12:36:00Z" {
		t.Fatalf("expected exchange trade time to be captured, got %#v", fills[0].ExchangeTradeTime)
	}
}

func TestFinalizeExecutedOrderSkipsDuplicateExchangeTradeIDFills(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)

	account, err := store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}

	order, err := store.CreateOrder(domain.Order{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "BUY",
		Type:              "MARKET",
		Quantity:          0.1,
		Price:             68000,
		Metadata: map[string]any{
			"executionMode": "live",
			"orderLifecycle": map[string]any{
				"submitted": true,
				"accepted":  true,
				"synced":    true,
				"filled":    false,
			},
			"exchangeOrderId": "exchange-order-1",
		},
	})
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	fill := domain.Fill{
		OrderID:         order.ID,
		ExchangeTradeID: "trade-1",
		Price:           68000,
		Quantity:        0.1,
		Fee:             1.23,
	}

	filledOrder, err := platform.finalizeExecutedOrder(account, order, []domain.Fill{fill})
	if err != nil {
		t.Fatalf("first finalize failed: %v", err)
	}
	if _, err := platform.finalizeExecutedOrder(account, filledOrder, []domain.Fill{fill}); err != nil {
		t.Fatalf("second finalize failed: %v", err)
	}

	fills, err := store.ListFills()
	if err != nil {
		t.Fatalf("list fills failed: %v", err)
	}
	orderFillCount := 0
	for _, item := range fills {
		if item.OrderID == order.ID {
			orderFillCount++
		}
	}
	if orderFillCount != 1 {
		t.Fatalf("expected duplicate sync to keep one fill, got %d", orderFillCount)
	}

	position, found, err := store.FindPosition(account.ID, "BTCUSDT")
	if err != nil {
		t.Fatalf("find position failed: %v", err)
	}
	if !found {
		t.Fatal("expected filled order to create a position")
	}
	if position.Quantity != 0.1 {
		t.Fatalf("expected duplicate sync to keep position quantity at 0.1, got %v", position.Quantity)
	}
}

func TestFinalizeExecutedOrderSkipsDuplicateFallbackFillsWithoutExchangeTradeID(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)

	account, err := store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}

	order, err := store.CreateOrder(domain.Order{
		AccountID: account.ID,
		Symbol:    "BTCUSDT",
		Side:      "BUY",
		Type:      "MARKET",
		Quantity:  0.1,
		Price:     68000,
		Metadata:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	tradeTime := time.Date(2026, 4, 17, 12, 36, 0, 0, time.UTC)
	fill := domain.Fill{
		OrderID:           order.ID,
		Price:             68000,
		Quantity:          0.1,
		Fee:               1.23,
		ExchangeTradeTime: &tradeTime,
	}

	filledOrder, err := platform.finalizeExecutedOrder(account, order, []domain.Fill{fill})
	if err != nil {
		t.Fatalf("first finalize failed: %v", err)
	}
	if _, err := platform.finalizeExecutedOrder(account, filledOrder, []domain.Fill{fill}); err != nil {
		t.Fatalf("second finalize failed: %v", err)
	}

	fills, err := store.ListFills()
	if err != nil {
		t.Fatalf("list fills failed: %v", err)
	}
	orderFillCount := 0
	for _, item := range fills {
		if item.OrderID == order.ID {
			orderFillCount++
		}
	}
	if orderFillCount != 1 {
		t.Fatalf("expected duplicate fallback sync to keep one fill, got %d", orderFillCount)
	}
}

func TestFinalizeExecutedOrderUsesExchangeTradeTimeForLastFilledAt(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)

	account, _ := store.GetAccount("live-main")
	order, err := store.CreateOrder(domain.Order{
		AccountID: account.ID,
		Symbol:    "BTCUSDT",
		Side:      "BUY",
		Type:      "MARKET",
		Quantity:  0.1,
		Price:     68000,
		Metadata:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	tradeTime := time.Date(2026, 4, 17, 12, 36, 0, 0, time.UTC)
	filledOrder, err := platform.finalizeExecutedOrder(account, order, []domain.Fill{{
		OrderID:           order.ID,
		Price:             68000,
		Quantity:          0.1,
		Fee:               1.23,
		ExchangeTradeTime: &tradeTime,
	}})
	if err != nil {
		t.Fatalf("finalize failed: %v", err)
	}

	if got := stringValue(filledOrder.Metadata["lastFilledAt"]); got != tradeTime.Format(time.RFC3339) {
		t.Fatalf("expected lastFilledAt to use exchange trade time, got %q", got)
	}
}

func TestClosePositionAllowsLiveManualCloseWithoutRuntimeSession(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	platform.registerLiveAdapter(testLiveAccountSyncAdapter{key: "test-manual-close"})

	account, err := platform.BindLiveAccount("live-main", map[string]any{
		"adapterKey": "test-manual-close",
	})
	if err != nil {
		t.Fatalf("bind live account failed: %v", err)
	}

	position, err := store.SavePosition(domain.Position{
		AccountID: account.ID,
		Symbol:    "BTCUSDT",
		Side:      "LONG",
		Quantity:  0.25,
		MarkPrice: 68100,
	})
	if err != nil {
		t.Fatalf("save live position failed: %v", err)
	}

	order, err := platform.ClosePosition(position.ID)
	if err != nil {
		t.Fatalf("expected live manual close to bypass runtime session checks, got %v", err)
	}
	if got := boolValue(order.Metadata["skipRuntimeCheck"]); !got {
		t.Fatal("expected live manual close order to set skipRuntimeCheck")
	}
	if got := order.Status; got != "ACCEPTED" {
		t.Fatalf("expected live manual close order to be accepted, got %s", got)
	}
	if got := stringValue(order.Metadata["runtimeSessionId"]); got != "" {
		t.Fatalf("expected bypassed live manual close to avoid linking a runtime session, got %s", got)
	}
}

func TestCreateLiveOrderImmediateFilledSubmissionSettlesReduceOnlyExit(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	tradeTime := time.Date(2026, 4, 20, 12, 33, 23, 0, time.UTC)
	syncCalls := 0
	platform.registerLiveAdapter(testLiveAccountSyncAdapter{
		key: "test-immediate-filled",
		submitOrderFunc: func(domain.Account, domain.Order, map[string]any) (LiveOrderSubmission, error) {
			return LiveOrderSubmission{
				Status:          "FILLED",
				ExchangeOrderID: "exchange-order-1",
				AcceptedAt:      tradeTime.Format(time.RFC3339),
				Metadata: map[string]any{
					"adapterMode":   "test",
					"executionMode": "rest",
					"executedQty":   0.002,
					"avgPrice":      75399.42,
					"updateTime":    tradeTime.Format(time.RFC3339),
				},
			}, nil
		},
		syncOrderFunc: func(domain.Account, domain.Order, map[string]any) (LiveOrderSync, error) {
			syncCalls++
			return LiveOrderSync{
				Status:   "FILLED",
				SyncedAt: tradeTime.Format(time.RFC3339),
				Fills: []LiveFillReport{{
					Price:    75399.42,
					Quantity: 0.002,
					Fee:      0.06,
					Metadata: map[string]any{
						"source":          "exchange-sync",
						"exchangeOrderId": "exchange-order-1",
						"tradeId":         "trade-1",
						"tradeTime":       tradeTime.Format(time.RFC3339),
					},
				}},
				Terminal:   true,
				FeeSource:  "exchange",
				FundingSrc: "exchange",
			}, nil
		},
	})

	account, err := platform.BindLiveAccount("live-main", map[string]any{
		"adapterKey": "test-immediate-filled",
	})
	if err != nil {
		t.Fatalf("bind live account failed: %v", err)
	}

	if _, err := store.SavePosition(domain.Position{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        75405,
		MarkPrice:         75399.42,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	order, err := platform.CreateOrder(domain.Order{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "SELL",
		Type:              "MARKET",
		Quantity:          0.002,
		Price:             75399.42,
		ReduceOnly:        true,
		Metadata: map[string]any{
			"skipRuntimeCheck": true,
			"executionProposal": map[string]any{
				"role":       "exit",
				"signalKind": "risk-exit",
				"reason":     "SL",
			},
		},
	})
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	if syncCalls != 1 {
		t.Fatalf("expected immediate FILLED submission to force one sync, got %d", syncCalls)
	}
	if got := order.Status; got != "FILLED" {
		t.Fatalf("expected order to stay FILLED after settlement, got %s", got)
	}
	if got := parseFloatValue(order.Metadata["filledQuantity"]); got != 0.002 {
		t.Fatalf("expected filledQuantity 0.002, got %v", got)
	}
	if _, found, err := store.FindPosition(account.ID, "BTCUSDT"); err != nil {
		t.Fatalf("find position failed: %v", err)
	} else if found {
		t.Fatal("expected reduce-only FILLED exit to clear the local position")
	}

	fills, err := store.ListFills()
	if err != nil {
		t.Fatalf("list fills failed: %v", err)
	}
	orderFillCount := 0
	for _, item := range fills {
		if item.OrderID != order.ID {
			continue
		}
		orderFillCount++
		if item.Fee != 0.06 {
			t.Fatalf("expected synced fee 0.06, got %v", item.Fee)
		}
	}
	if orderFillCount != 1 {
		t.Fatalf("expected one fill for settled immediate-FILLED order, got %d", orderFillCount)
	}
}

func TestRecoveredPassiveCloseExecutionBoundaryAllowsValidHedgeClose(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	adapter := &recordingLiveExecutionAdapter{key: "test-recovered-close-valid"}
	platform.registerLiveAdapter(adapter)

	account, err := platform.BindLiveAccount("live-main", map[string]any{
		"adapterKey":    adapter.key,
		"executionMode": "mock",
		"positionMode":  "HEDGE",
	})
	if err != nil {
		t.Fatalf("bind live account failed: %v", err)
	}
	if _, err := store.SavePosition(domain.Position{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.25,
		MarkPrice:         68100,
	}); err != nil {
		t.Fatalf("save live position failed: %v", err)
	}

	order, err := platform.CreateOrder(domain.Order{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "SELL",
		Type:              "MARKET",
		Quantity:          0.25,
		ReduceOnly:        true,
		Metadata: map[string]any{
			"skipRuntimeCheck": true,
			"executionProposal": map[string]any{
				"role":       "exit",
				"side":       "SELL",
				"symbol":     "BTCUSDT",
				"signalKind": "recovery-watchdog",
				"metadata": map[string]any{
					"recoveryTriggered": true,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create recovered close order failed: %v", err)
	}
	if adapter.submitCount != 1 {
		t.Fatalf("expected adapter submit to be called once, got %d", adapter.submitCount)
	}
	if got := stringValue(adapter.lastOrder.Metadata["executionBoundaryClass"]); got != liveExecutionBoundaryClassRecoveredPassiveClose {
		t.Fatalf("expected recovered-passive-close classification, got %s", got)
	}
	if got := stringValue(adapter.lastOrder.Metadata["positionSide"]); got != "LONG" {
		t.Fatalf("expected hedge recovered close to submit positionSide LONG, got %s", got)
	}
	if got := order.Status; got != "ACCEPTED" {
		t.Fatalf("expected recovered close order to be accepted, got %s", got)
	}
}

func TestRecoveredPassiveCloseExecutionBoundaryBlocksMissingReduceOnly(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	adapter := &recordingLiveExecutionAdapter{key: "test-recovered-close-missing-reduce-only"}
	platform.registerLiveAdapter(adapter)

	account, err := platform.BindLiveAccount("live-main", map[string]any{
		"adapterKey":    adapter.key,
		"executionMode": "mock",
		"positionMode":  "ONE_WAY",
	})
	if err != nil {
		t.Fatalf("bind live account failed: %v", err)
	}
	if _, err := store.SavePosition(domain.Position{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.25,
		MarkPrice:         68100,
	}); err != nil {
		t.Fatalf("save live position failed: %v", err)
	}

	order, err := platform.CreateOrder(domain.Order{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "SELL",
		Type:              "MARKET",
		Quantity:          0.25,
		Metadata: map[string]any{
			"skipRuntimeCheck": true,
			"executionProposal": map[string]any{
				"role":       "exit",
				"side":       "SELL",
				"symbol":     "BTCUSDT",
				"signalKind": "recovery-watchdog",
				"metadata": map[string]any{
					"recoveryTriggered": true,
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected recovered close without reduceOnly to be blocked")
	}
	if adapter.submitCount != 0 {
		t.Fatalf("expected adapter submit not to be called, got %d", adapter.submitCount)
	}
	if got := order.Status; got != "REJECTED" {
		t.Fatalf("expected blocked recovered close order to be marked REJECTED, got %s", got)
	}
}

func TestRecoveredPassiveCloseExecutionBoundaryBlocksInvalidSideAndHedgePayload(t *testing.T) {
	tests := []struct {
		name         string
		side         string
		positionSide string
		wantErr      string
	}{
		{
			name:    "wrong side",
			side:    "BUY",
			wantErr: "does not reduce LONG position",
		},
		{
			name:         "wrong hedge positionSide",
			side:         "SELL",
			positionSide: "SHORT",
			wantErr:      "does not match LONG position",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := memory.NewStore()
			platform := NewPlatform(store)
			adapter := &recordingLiveExecutionAdapter{key: "test-recovered-close-" + strings.ReplaceAll(tt.name, " ", "-")}
			platform.registerLiveAdapter(adapter)

			account, err := platform.BindLiveAccount("live-main", map[string]any{
				"adapterKey":    adapter.key,
				"executionMode": "mock",
				"positionMode":  "HEDGE",
			})
			if err != nil {
				t.Fatalf("bind live account failed: %v", err)
			}
			if _, err := store.SavePosition(domain.Position{
				AccountID:         account.ID,
				StrategyVersionID: "strategy-version-bk-1d-v010",
				Symbol:            "BTCUSDT",
				Side:              "LONG",
				Quantity:          0.25,
				MarkPrice:         68100,
			}); err != nil {
				t.Fatalf("save live position failed: %v", err)
			}
			binding := resolveLiveBinding(account)
			order, err := platform.prepareLiveOrderForSubmission(account, domain.Order{
				AccountID:         account.ID,
				StrategyVersionID: "strategy-version-bk-1d-v010",
				Symbol:            "BTCUSDT",
				Side:              tt.side,
				Type:              "MARKET",
				Quantity:          0.25,
				ReduceOnly:        true,
				Metadata: map[string]any{
					"skipRuntimeCheck": true,
					"positionSide":     tt.positionSide,
					"executionProposal": map[string]any{
						"role":       "exit",
						"side":       tt.side,
						"symbol":     "BTCUSDT",
						"signalKind": "recovery-watchdog",
						"metadata": map[string]any{
							"recoveryTriggered": true,
						},
					},
				},
			}, binding)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
			if got := stringValue(order.Metadata["executionBoundaryClass"]); got != liveExecutionBoundaryClassRecoveredPassiveClose {
				t.Fatalf("expected recovered-passive-close classification, got %s", got)
			}
		})
	}
}

func TestRecoveredPassiveCloseExecutionBoundaryLeavesNormalEntryFlowIntact(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	adapter := &recordingLiveExecutionAdapter{key: "test-normal-entry"}
	platform.registerLiveAdapter(adapter)

	account, err := platform.BindLiveAccount("live-main", map[string]any{
		"adapterKey":    adapter.key,
		"executionMode": "mock",
		"positionMode":  "ONE_WAY",
	})
	if err != nil {
		t.Fatalf("bind live account failed: %v", err)
	}

	order, err := platform.CreateOrder(domain.Order{
		AccountID:         account.ID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "BUY",
		Type:              "MARKET",
		Quantity:          0.25,
		Metadata: map[string]any{
			"skipRuntimeCheck": true,
		},
	})
	if err != nil {
		t.Fatalf("create normal entry order failed: %v", err)
	}
	if adapter.submitCount != 1 {
		t.Fatalf("expected adapter submit to be called once, got %d", adapter.submitCount)
	}
	if got := stringValue(adapter.lastOrder.Metadata["executionBoundaryClass"]); got != liveExecutionBoundaryClassNormalEntry {
		t.Fatalf("expected normal-entry classification, got %s", got)
	}
	if got := stringValue(adapter.lastOrder.Metadata["positionSide"]); got != "" {
		t.Fatalf("expected normal entry not to inject positionSide, got %s", got)
	}
	if got := order.Status; got != "ACCEPTED" {
		t.Fatalf("expected normal entry order to be accepted, got %s", got)
	}
}

func TestShouldSendBinanceReduceOnlyFlagRespectsPositionMode(t *testing.T) {
	order := domain.Order{ReduceOnly: true}
	if !shouldSendBinanceReduceOnlyFlag(map[string]any{"positionMode": "ONE_WAY"}, order) {
		t.Fatal("expected reduceOnly flag to be sent in ONE_WAY mode")
	}
	if shouldSendBinanceReduceOnlyFlag(map[string]any{"positionMode": "HEDGE"}, order) {
		t.Fatal("expected reduceOnly flag to be omitted in HEDGE mode")
	}
}

func TestResolveBinancePositionSideForSubmissionOmitsOneWayBoth(t *testing.T) {
	order := domain.Order{Metadata: map[string]any{"positionSide": "BOTH"}}
	if got := resolveBinancePositionSideForSubmission(map[string]any{"positionMode": "ONE_WAY"}, order); got != "" {
		t.Fatalf("expected ONE_WAY BOTH to be omitted, got %s", got)
	}
	if got := resolveBinancePositionSideForSubmission(map[string]any{"positionMode": "HEDGE"}, domain.Order{
		Metadata: map[string]any{"positionSide": "LONG"},
	}); got != "LONG" {
		t.Fatalf("expected hedge positionSide LONG to be preserved, got %s", got)
	}
}

func TestSubmitRESTOrderRecoveredPassiveClosePayloadMatchesBinanceModes(t *testing.T) {
	tests := []struct {
		name                  string
		positionMode          string
		side                  string
		positionSide          string
		reduceOnly            bool
		wantPositionSide      string
		wantReduceOnlyPresent bool
	}{
		{
			name:                  "hedge long close",
			positionMode:          "HEDGE",
			side:                  "SELL",
			positionSide:          "LONG",
			reduceOnly:            true,
			wantPositionSide:      "LONG",
			wantReduceOnlyPresent: false,
		},
		{
			name:                  "one way close",
			positionMode:          "ONE_WAY",
			side:                  "SELL",
			positionSide:          "BOTH",
			reduceOnly:            true,
			wantPositionSide:      "",
			wantReduceOnlyPresent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedForm neturl.Values
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/fapi/v1/exchangeInfo":
					_ = json.NewEncoder(w).Encode(map[string]any{
						"symbols": []map[string]any{{
							"symbol": "BTCUSDT",
							"filters": []map[string]any{
								{"filterType": "PRICE_FILTER", "tickSize": "0.1"},
								{"filterType": "LOT_SIZE", "stepSize": "0.001", "minQty": "0.001", "maxQty": "1000"},
								{"filterType": "MIN_NOTIONAL", "notional": "100"},
							},
						}},
					})
				case "/fapi/v1/order":
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Fatalf("read order body failed: %v", err)
					}
					capturedForm, err = neturl.ParseQuery(string(body))
					if err != nil {
						t.Fatalf("parse order form failed: %v", err)
					}
					_ = json.NewEncoder(w).Encode(map[string]any{
						"status":        "NEW",
						"orderId":       12345,
						"clientOrderId": "order-test",
						"updateTime":    time.Now().UTC().UnixMilli(),
					})
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			t.Setenv("TEST_BINANCE_KEY", "key")
			t.Setenv("TEST_BINANCE_SECRET", "secret")

			binanceSymbolRulesCacheMu.Lock()
			binanceSymbolRulesCache = map[string]binanceSymbolRules{}
			binanceSymbolRulesCacheMu.Unlock()

			adapter := binanceFuturesLiveAdapter{}
			_, err := adapter.submitRESTOrder(domain.Account{Exchange: "binance-futures"}, domain.Order{
				ID:         "order-test",
				AccountID:  "live-main",
				Symbol:     "BTCUSDT",
				Side:       tt.side,
				Type:       "MARKET",
				Quantity:   0.25,
				ReduceOnly: tt.reduceOnly,
				Metadata: map[string]any{
					"positionSide": tt.positionSide,
				},
			}, map[string]any{
				"executionMode": "rest",
				"positionMode":  tt.positionMode,
				"restBaseUrl":   server.URL,
				"recvWindowMs":  5000,
				"credentialRefs": map[string]any{
					"apiKeyRef":    "TEST_BINANCE_KEY",
					"apiSecretRef": "TEST_BINANCE_SECRET",
				},
			})
			if err != nil {
				t.Fatalf("submit REST order failed: %v", err)
			}
			if got := capturedForm.Get("side"); got != tt.side {
				t.Fatalf("expected side %s, got %s", tt.side, got)
			}
			if got := capturedForm.Get("positionSide"); got != tt.wantPositionSide {
				t.Fatalf("expected positionSide %q, got %q", tt.wantPositionSide, got)
			}
			_, hasReduceOnly := capturedForm["reduceOnly"]
			if hasReduceOnly != tt.wantReduceOnlyPresent {
				t.Fatalf("expected reduceOnly present=%t, form=%v", tt.wantReduceOnlyPresent, capturedForm)
			}
		})
	}
}

func TestFinalizeExecutedOrderFallsBackToNowWhenExchangeTradeTimeMissing(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)

	account, _ := store.GetAccount("live-main")
	order, err := store.CreateOrder(domain.Order{
		AccountID: account.ID,
		Symbol:    "BTCUSDT",
		Side:      "BUY",
		Type:      "MARKET",
		Quantity:  0.1,
		Price:     68000,
		Metadata:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	before := time.Now().UTC().Add(-time.Second)
	filledOrder, err := platform.finalizeExecutedOrder(account, order, []domain.Fill{{
		OrderID:  order.ID,
		Price:    68000,
		Quantity: 0.1,
		Fee:      1.23,
	}})
	after := time.Now().UTC().Add(time.Second)
	if err != nil {
		t.Fatalf("finalize failed: %v", err)
	}

	got := parseOptionalRFC3339(stringValue(filledOrder.Metadata["lastFilledAt"]))
	if got.IsZero() || got.Before(before) || got.After(after) {
		t.Fatalf("expected lastFilledAt to fall back to now, got %v", got)
	}
}

func TestFinalizeExecutedOrderKeepsLastFilledAtOnDuplicateSync(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)

	account, _ := store.GetAccount("live-main")
	order, err := store.CreateOrder(domain.Order{
		AccountID: account.ID,
		Symbol:    "BTCUSDT",
		Side:      "BUY",
		Type:      "MARKET",
		Quantity:  0.1,
		Price:     68000,
		Metadata:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	firstTradeTime := time.Date(2026, 4, 17, 12, 36, 0, 0, time.UTC)
	fill := domain.Fill{
		OrderID:           order.ID,
		Price:             68000,
		Quantity:          0.1,
		Fee:               1.23,
		ExchangeTradeTime: &firstTradeTime,
	}
	filledOrder, err := platform.finalizeExecutedOrder(account, order, []domain.Fill{fill})
	if err != nil {
		t.Fatalf("first finalize failed: %v", err)
	}

	filledOrder, err = platform.finalizeExecutedOrder(account, filledOrder, []domain.Fill{fill})
	if err != nil {
		t.Fatalf("second finalize failed: %v", err)
	}

	if got := stringValue(filledOrder.Metadata["lastFilledAt"]); got != firstTradeTime.Format(time.RFC3339) {
		t.Fatalf("expected duplicate sync to keep original lastFilledAt, got %q", got)
	}
}

type recordingLiveExecutionAdapter struct {
	key         string
	submitCount int
	lastOrder   domain.Order
}

func (a *recordingLiveExecutionAdapter) Key() string {
	return a.key
}

func (a *recordingLiveExecutionAdapter) Describe() map[string]any {
	return map[string]any{"key": a.key}
}

func (a *recordingLiveExecutionAdapter) ValidateAccountConfig(map[string]any) error {
	return nil
}

func (a *recordingLiveExecutionAdapter) SubmitOrder(_ domain.Account, order domain.Order, binding map[string]any) (LiveOrderSubmission, error) {
	a.submitCount++
	a.lastOrder = order
	return LiveOrderSubmission{
		Status:          "ACCEPTED",
		ExchangeOrderID: "exchange-order-1",
		AcceptedAt:      time.Now().UTC().Format(time.RFC3339),
		Metadata: map[string]any{
			"adapterMode":   "recording",
			"executionMode": stringValue(binding["executionMode"]),
			"positionMode":  stringValue(binding["positionMode"]),
			"positionSide":  stringValue(order.Metadata["positionSide"]),
		},
	}, nil
}

func (a *recordingLiveExecutionAdapter) SyncOrder(domain.Account, domain.Order, map[string]any) (LiveOrderSync, error) {
	return LiveOrderSync{}, nil
}

func (a *recordingLiveExecutionAdapter) CancelOrder(domain.Account, domain.Order, map[string]any) (LiveOrderSync, error) {
	return LiveOrderSync{}, nil
}
