package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestUpdateRuntimeHealthSummaryResetsDailyBucket(t *testing.T) {
	originalLocal := time.Local
	loc := time.FixedZone("UTC+8", 8*60*60)
	time.Local = loc
	defer func() {
		time.Local = originalLocal
	}()

	state := map[string]any{}
	updateRuntimeHealthSummary(state, map[string]any{
		"streamType": "trade_tick",
		"symbol":     "BTCUSDT",
		"price":      64000.5,
	}, time.Date(2026, 4, 14, 9, 0, 0, 0, loc))
	updateRuntimeHealthSummary(state, map[string]any{
		"streamType": "trade_tick",
		"symbol":     "BTCUSDT",
		"price":      64010.5,
	}, time.Date(2026, 4, 15, 9, 0, 0, 0, loc))

	health := mapValue(state["healthSummary"])
	tradeTick := mapValue(health["tradeTick"])
	today := mapValue(tradeTick["today"])
	if got := stringValue(today["day"]); got != "2026-04-15" {
		t.Fatalf("expected latest local day 2026-04-15, got %s", got)
	}
	if got := maxIntValue(today["eventCount"], 0); got != 1 {
		t.Fatalf("expected daily event count reset to 1, got %d", got)
	}
	if got := NormalizeSymbol(stringValue(tradeTick["lastSymbol"])); got != "BTCUSDT" {
		t.Fatalf("expected last symbol BTCUSDT, got %s", got)
	}
	if got := parseFloatValue(tradeTick["lastPrice"]); got != 64010.5 {
		t.Fatalf("expected last price 64010.5, got %v", got)
	}
}

func TestRecordStrategyDecisionHealthTracksOrderBookUsage(t *testing.T) {
	state := map[string]any{}
	eventTime := time.Date(2026, 4, 14, 10, 30, 0, 0, time.UTC)
	recordStrategyDecisionHealth(state, StrategySignalDecision{
		Action: "wait",
		Reason: "spread-too-wide",
		Metadata: map[string]any{
			"decisionState": "waiting-price",
			"signalKind":    "initial-entry-watch",
			"bestBid":       100.0,
			"bestAsk":       100.1,
			"spreadBps":     10.0,
			"bookImbalance": 0.25,
			"liquidityBias": "ask-heavy",
			"marketPrice":   100.05,
		},
	}, eventTime)

	health := mapValue(state["healthSummary"])
	ingress := mapValue(health["strategyIngress"])
	today := mapValue(ingress["today"])
	if got := maxIntValue(today["evaluatedCount"], 0); got != 1 {
		t.Fatalf("expected evaluatedCount 1, got %d", got)
	}
	if got := maxIntValue(today["waitCount"], 0); got != 1 {
		t.Fatalf("expected waitCount 1, got %d", got)
	}
	if got := maxIntValue(today["orderBookEvaluatedCount"], 0); got != 1 {
		t.Fatalf("expected orderBookEvaluatedCount 1, got %d", got)
	}
	if got := maxIntValue(today["orderBookBlockedCount"], 0); got != 1 {
		t.Fatalf("expected orderBookBlockedCount 1, got %d", got)
	}
	if got := parseFloatValue(today["avgSpreadBps"]); got != 10.0 {
		t.Fatalf("expected avgSpreadBps 10.0, got %v", got)
	}
	if got := stringValue(ingress["lastLiquidityBias"]); got != "ask-heavy" {
		t.Fatalf("expected liquidity bias ask-heavy, got %s", got)
	}
}

func TestAccountSyncHealthTracksGapAndFailures(t *testing.T) {
	account := domain.Account{
		Metadata: map[string]any{
			"liveSyncSnapshot": map[string]any{
				"syncStatus":     "SYNCED",
				"source":         "binance-rest-account-v3",
				"positionCount":  2,
				"openOrderCount": 1,
			},
		},
	}
	prevSuccessAt := time.Date(2026, 4, 14, 9, 0, 0, 0, time.UTC)
	syncedAt := prevSuccessAt.Add(45 * time.Second)
	updateAccountSyncSuccessHealth(&account, syncedAt, prevSuccessAt)
	updateAccountSyncFailureHealth(&account, syncedAt.Add(30*time.Second), errors.New("sync failed"))

	health := mapValue(account.Metadata["healthSummary"])
	accountSync := mapValue(health["accountSync"])
	today := mapValue(accountSync["today"])
	if got := maxIntValue(today["syncCount"], 0); got != 1 {
		t.Fatalf("expected syncCount 1, got %d", got)
	}
	if got := maxIntValue(today["errorCount"], 0); got != 1 {
		t.Fatalf("expected errorCount 1, got %d", got)
	}
	if got := maxIntValue(accountSync["lastSyncGapSeconds"], 0); got != 45 {
		t.Fatalf("expected lastSyncGapSeconds 45, got %d", got)
	}
	if got := maxIntValue(accountSync["consecutiveErrorCount"], 0); got != 1 {
		t.Fatalf("expected consecutiveErrorCount 1, got %d", got)
	}
	if got := stringValue(accountSync["lastError"]); got != "sync failed" {
		t.Fatalf("expected lastError sync failed, got %s", got)
	}
}

func TestStrategyEvaluationQuietUsesLatestTrigger(t *testing.T) {
	platform := &Platform{
		runtimePolicy: RuntimePolicy{
			StrategyEvaluationQuietSeconds: 15,
		},
	}
	now := time.Now().UTC()
	state := map[string]any{
		"healthSummary": map[string]any{
			"strategyIngress": map[string]any{
				"lastTriggeredAt": now.Add(-20 * time.Second).Format(time.RFC3339),
			},
		},
	}
	if !platform.strategyEvaluationQuiet(state) {
		t.Fatal("expected strategy evaluation to be quiet when trigger is stale and no evaluation exists")
	}
	state["lastStrategyEvaluationAt"] = now.Add(-5 * time.Second).Format(time.RFC3339)
	if platform.strategyEvaluationQuiet(state) {
		t.Fatal("expected strategy evaluation quiet to clear once evaluation catches up")
	}
	state["healthSummary"] = map[string]any{
		"strategyIngress": map[string]any{
			"lastTriggeredAt":  now.Add(-20 * time.Second).Format(time.RFC3339),
			"lastEvaluationAt": now.Add(-40 * time.Second).Format(time.RFC3339),
		},
	}
	state["lastStrategyEvaluationAt"] = now.Add(-5 * time.Second).Format(time.RFC3339)
	if platform.strategyEvaluationQuiet(state) {
		t.Fatal("expected newer lastStrategyEvaluationAt to suppress quiet alert even when healthSummary lags")
	}
}

func TestLiveAccountSyncStaleAndRefreshThreshold(t *testing.T) {
	platform := &Platform{
		runtimePolicy: RuntimePolicy{
			LiveAccountSyncFreshnessSecs: 60,
		},
	}
	now := time.Now().UTC()
	account := domain.Account{
		ID: "live-1",
		Metadata: map[string]any{
			"healthSummary": map[string]any{
				"accountSync": map[string]any{
					"lastSuccessAt": now.Add(-61 * time.Second).Format(time.RFC3339),
				},
			},
			"lastLiveSyncAt": now.Add(-61 * time.Second).Format(time.RFC3339),
		},
	}
	stale, ageSeconds := platform.liveAccountSyncStale(account, now)
	if !stale {
		t.Fatal("expected live account sync to be stale")
	}
	if ageSeconds < 61 {
		t.Fatalf("expected stale age at least 61 seconds, got %d", ageSeconds)
	}
	if !platform.shouldRefreshLiveAccountSync(account, now) {
		t.Fatal("expected live account refresh to be requested once threshold is exceeded")
	}
	account.Metadata["lastLiveSyncAt"] = now.Add(-30 * time.Second).Format(time.RFC3339)
	account.Metadata["healthSummary"] = map[string]any{
		"accountSync": map[string]any{
			"lastSuccessAt": now.Add(-30 * time.Second).Format(time.RFC3339),
		},
	}
	stale, _ = platform.liveAccountSyncStale(account, now)
	if stale {
		t.Fatal("expected live account sync to be fresh under threshold")
	}
	if platform.shouldRefreshLiveAccountSync(account, now) {
		t.Fatal("did not expect live account refresh under threshold")
	}
}

func TestLiveAccountSyncStaleAlertGraceSuppressesThresholdFlap(t *testing.T) {
	platform := &Platform{
		runtimePolicy: RuntimePolicy{
			LiveAccountSyncFreshnessSecs: 60,
		},
	}
	now := time.Now().UTC()
	account := domain.Account{
		ID: "live-1",
		Metadata: map[string]any{
			"healthSummary": map[string]any{
				"accountSync": map[string]any{
					"lastSuccessAt": now.Add(-61 * time.Second).Format(time.RFC3339),
				},
			},
			"lastLiveSyncAt": now.Add(-61 * time.Second).Format(time.RFC3339),
		},
	}

	stale, ageSeconds := platform.liveAccountSyncStale(account, now)
	if !stale {
		t.Fatal("expected health stale state once freshness threshold is exceeded")
	}
	if staleness, _ := platform.liveAccountSyncStaleness(account, now); staleness != liveAccountSyncSoftStale {
		t.Fatalf("expected soft stale inside alert grace, got %s", staleness)
	}
	alertStale, alertAgeSeconds := platform.liveAccountSyncStaleForAlert(account, now)
	if alertStale {
		t.Fatalf("expected alert grace to suppress threshold flap, age=%d", alertAgeSeconds)
	}
	if alertAgeSeconds != ageSeconds {
		t.Fatalf("expected alert age to preserve stale age, got %d want %d", alertAgeSeconds, ageSeconds)
	}

	account.Metadata["healthSummary"] = map[string]any{
		"accountSync": map[string]any{
			"lastSuccessAt": now.Add(-91 * time.Second).Format(time.RFC3339),
			"lastAttemptAt": now.Add(-5 * time.Second).Format(time.RFC3339),
		},
	}
	account.Metadata["lastLiveSyncAt"] = now.Add(-91 * time.Second).Format(time.RFC3339)
	alertStale, alertAgeSeconds = platform.liveAccountSyncStaleForAlert(account, now)
	if alertStale {
		t.Fatalf("expected recent sync attempt to suppress hard stale alert, age=%d", alertAgeSeconds)
	}

	account.Metadata["healthSummary"] = map[string]any{
		"accountSync": map[string]any{
			"lastSuccessAt": now.Add(-91 * time.Second).Format(time.RFC3339),
			"lastAttemptAt": now.Add(-45 * time.Second).Format(time.RFC3339),
		},
	}
	alertStale, alertAgeSeconds = platform.liveAccountSyncStaleForAlert(account, now)
	if !alertStale {
		t.Fatalf("expected alert after grace elapses, age=%d", alertAgeSeconds)
	}
	if staleness, _ := platform.liveAccountSyncStaleness(account, now); staleness != liveAccountSyncHardStale {
		t.Fatalf("expected hard stale after alert grace elapses, got %s", staleness)
	}
}

func TestLiveAccountSyncNeverSucceededIsHardStale(t *testing.T) {
	platform := &Platform{
		runtimePolicy: RuntimePolicy{
			LiveAccountSyncFreshnessSecs: 60,
		},
	}
	now := time.Now().UTC()
	account := domain.Account{
		ID:        "live-1",
		CreatedAt: now.Add(-5 * time.Second),
		Metadata:  map[string]any{},
	}

	staleness, ageSeconds := platform.liveAccountSyncStaleness(account, now)
	if staleness != liveAccountSyncHardStale {
		t.Fatalf("expected never-synced account to be hard stale, got %s", staleness)
	}
	if ageSeconds < 5 {
		t.Fatalf("expected age to use account creation time, got %d", ageSeconds)
	}
	alertStale, _ := platform.liveAccountSyncStaleForAlert(account, now)
	if !alertStale {
		t.Fatal("expected never-synced account to alert immediately")
	}
}

func TestUpdateRuntimePolicyAllowsDisablingHealthThresholds(t *testing.T) {
	platform := NewPlatform(memory.NewStore())

	updated, err := platform.UpdateRuntimePolicy(RuntimePolicy{
		TradeTickFreshnessSeconds:      15,
		OrderBookFreshnessSeconds:      10,
		SignalBarFreshnessSeconds:      30,
		RuntimeQuietSeconds:            30,
		StrategyEvaluationQuietSeconds: 0,
		LiveAccountSyncFreshnessSecs:   0,
		PaperStartReadinessTimeoutSecs: 5,
	})
	if err != nil {
		t.Fatalf("update runtime policy failed: %v", err)
	}
	if updated.StrategyEvaluationQuietSeconds != 0 {
		t.Fatalf("expected strategy evaluation quiet threshold to allow zero, got %d", updated.StrategyEvaluationQuietSeconds)
	}
	if updated.LiveAccountSyncFreshnessSecs != 0 {
		t.Fatalf("expected live account sync freshness threshold to allow zero, got %d", updated.LiveAccountSyncFreshnessSecs)
	}
}

func TestListAlertsShowsAuthoritativeSyncWarningForLocalRecoveryFallback(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}

	state := cloneMetadata(session.State)
	state["protectionRecoveryStatus"] = "unprotected-open-position"
	state["positionRecoveryStatus"] = "unprotected-open-position"
	state["protectionRecoveryAuthoritative"] = false
	state["protectionRecoverySource"] = "platform-live-reconciliation"
	state["lastProtectionRecoveryAt"] = time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC).Format(time.RFC3339)
	session.State = state
	if _, err := platform.store.UpdateLiveSession(session); err != nil {
		t.Fatalf("update live session failed: %v", err)
	}

	alerts, err := platform.ListAlerts()
	if err != nil {
		t.Fatalf("list alerts failed: %v", err)
	}

	foundSyncWarning := false
	for _, alert := range alerts {
		if alert.ID == "live-recovery-awaiting-authoritative-sync-"+session.ID {
			foundSyncWarning = true
			if alert.Level != "warning" {
				t.Fatalf("expected warning level, got %s", alert.Level)
			}
			if !strings.Contains(alert.Detail, "看门狗自动平仓已暂停") {
				t.Fatalf("expected pause detail in alert, got %s", alert.Detail)
			}
		}
		if alert.ID == "live-unprotected-position-"+session.ID {
			t.Fatal("expected local fallback recovery to suppress authoritative unprotected-position alert")
		}
	}
	if !foundSyncWarning {
		t.Fatal("expected authoritative sync warning alert to be present")
	}
}

func TestListAlertsUsesStableLiveUnprotectedPositionID(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}

	state := cloneMetadata(session.State)
	state["symbol"] = "BTCUSDT"
	state["protectionRecoveryStatus"] = "unprotected-open-position"
	state["positionRecoveryStatus"] = "unprotected-open-position"
	state["protectionRecoveryAuthoritative"] = true
	state["lastProtectionRecoveryAt"] = time.Date(2026, 4, 28, 1, 0, 0, 0, time.UTC).Format(time.RFC3339)
	session.State = state
	if _, err := platform.store.UpdateLiveSession(session); err != nil {
		t.Fatalf("update live session failed: %v", err)
	}

	alerts, err := platform.ListAlerts()
	if err != nil {
		t.Fatalf("list alerts failed: %v", err)
	}

	expectedID := "live-unprotected-position-" + session.ID
	found := false
	for _, alert := range alerts {
		if alert.Title != "恢复持仓无保护" {
			continue
		}
		found = true
		if alert.ID != expectedID {
			t.Fatalf("expected stable session-scoped alert ID %s, got %s", expectedID, alert.ID)
		}
		if strings.Contains(alert.ID, "BTCUSDT") {
			t.Fatalf("expected alert ID not to include symbol, got %s", alert.ID)
		}
	}
	if !found {
		t.Fatal("expected unprotected-position alert to be present")
	}
}

func TestListAlertsShowsCriticalLiveExitDispatchFailure(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         session.AccountID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "SHORT",
		Quantity:          0.013,
		EntryPrice:        77830.1,
		MarkPrice:         78168.3,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	state := cloneMetadata(session.State)
	state["symbol"] = "BTCUSDT"
	state["lastDispatchedOrderId"] = "order-rejected-exit-1"
	state["lastDispatchedOrderStatus"] = "REJECTED"
	state["lastDispatchRejectedStatus"] = "REJECTED"
	state["lastDispatchRejectedAt"] = time.Date(2026, 4, 23, 6, 11, 54, 0, time.UTC).Format(time.RFC3339)
	state["lastAutoDispatchError"] = "reduce-only order quantity 0.013000000000 is below minQty 0.000100000000 for BTCUSDT"
	state["lastDispatchedIntent"] = map[string]any{
		"role":       "exit",
		"reason":     "SL",
		"signalKind": "risk-exit",
		"symbol":     "BTCUSDT",
		"reduceOnly": true,
	}
	state["lastExecutionDispatch"] = map[string]any{
		"status":           "REJECTED",
		"symbol":           "BTCUSDT",
		"side":             "BUY",
		"reduceOnly":       true,
		"reason":           "SL",
		"signalKind":       "risk-exit",
		"orderType":        "MARKET",
		"quantity":         0.013,
		"price":            78168.3,
		"failed":           true,
		"orderId":          "order-rejected-exit-1",
		"executionProfile": "exit",
	}
	session.State = state
	session.Status = "RUNNING"
	if _, err := platform.store.UpdateLiveSession(session); err != nil {
		t.Fatalf("update live session failed: %v", err)
	}

	alerts, err := platform.ListAlerts()
	if err != nil {
		t.Fatalf("list alerts failed: %v", err)
	}

	found := false
	for _, alert := range alerts {
		if alert.ID != "live-exit-dispatch-failure-"+session.ID {
			continue
		}
		found = true
		if alert.Level != "critical" {
			t.Fatalf("expected critical alert level, got %s", alert.Level)
		}
		if !strings.Contains(alert.Detail, "自动平仓派单失败") || !strings.Contains(alert.Detail, "REJECTED") {
			t.Fatalf("expected reject failure detail, got %s", alert.Detail)
		}
		if stringValue(alert.Metadata["liveSessionId"]) != session.ID {
			t.Fatalf("expected liveSessionId metadata %s, got %#v", session.ID, alert.Metadata)
		}
	}
	if !found {
		t.Fatal("expected live exit dispatch failure alert to be present")
	}

	notifications, err := platform.ListNotifications(false)
	if err != nil {
		t.Fatalf("list notifications failed: %v", err)
	}
	foundNotification := false
	for _, item := range notifications {
		if item.ID != "live-exit-dispatch-failure-"+session.ID {
			continue
		}
		foundNotification = true
		if got := stringValue(item.Metadata["telegramStatus"]); got != "pending" {
			t.Fatalf("expected telegramStatus=pending, got %s", got)
		}
	}
	if !foundNotification {
		t.Fatal("expected live exit dispatch failure notification to be present")
	}
}

func TestListAlertsIgnoresPendingLiveExitDispatch(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         session.AccountID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "SHORT",
		Quantity:          0.013,
		EntryPrice:        77830.1,
		MarkPrice:         78168.3,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}

	state := cloneMetadata(session.State)
	state["symbol"] = "BTCUSDT"
	state["lastDispatchedOrderId"] = "order-pending-exit-1"
	state["lastDispatchedOrderStatus"] = "ACCEPTED"
	state["lastDispatchedIntent"] = map[string]any{
		"role":       "exit",
		"reason":     "SL",
		"signalKind": "risk-exit",
		"symbol":     "BTCUSDT",
		"reduceOnly": true,
	}
	state["lastExecutionDispatch"] = map[string]any{
		"status":           "ACCEPTED",
		"symbol":           "BTCUSDT",
		"side":             "BUY",
		"reduceOnly":       true,
		"reason":           "SL",
		"signalKind":       "risk-exit",
		"orderType":        "MARKET",
		"quantity":         0.013,
		"price":            78168.3,
		"failed":           false,
		"orderId":          "order-pending-exit-1",
		"executionProfile": "exit",
	}
	session.State = state
	session.Status = "RUNNING"
	if _, err := platform.store.UpdateLiveSession(session); err != nil {
		t.Fatalf("update live session failed: %v", err)
	}

	alerts, err := platform.ListAlerts()
	if err != nil {
		t.Fatalf("list alerts failed: %v", err)
	}
	for _, alert := range alerts {
		if alert.ID == "live-exit-dispatch-failure-"+session.ID {
			t.Fatalf("expected pending accepted exit dispatch not to raise failure alert: %#v", alert)
		}
	}
}

func TestLiveExitDispatchFailureLogDedupeIsBounded(t *testing.T) {
	liveExitDispatchFailureLogDedupe.Lock()
	liveExitDispatchFailureLogDedupe.entries = map[string]time.Time{}
	liveExitDispatchFailureLogDedupe.Unlock()

	base := time.Date(2026, 4, 23, 7, 55, 0, 0, time.UTC)
	if !shouldLogLiveExitDispatchFailure("same-failure", base) {
		t.Fatal("expected first failure signature to log")
	}
	if shouldLogLiveExitDispatchFailure("same-failure", base.Add(time.Minute)) {
		t.Fatal("expected duplicate failure signature inside TTL to be suppressed")
	}
	if !shouldLogLiveExitDispatchFailure("same-failure", base.Add(liveExitDispatchFailureLogDedupeTTL+time.Second)) {
		t.Fatal("expected failure signature to log again after TTL")
	}

	liveExitDispatchFailureLogDedupe.Lock()
	liveExitDispatchFailureLogDedupe.entries = map[string]time.Time{}
	liveExitDispatchFailureLogDedupe.Unlock()

	for i := 0; i < liveExitDispatchFailureLogDedupeMaxEntries+25; i++ {
		if !shouldLogLiveExitDispatchFailure(fmt.Sprintf("failure-%03d", i), base.Add(time.Duration(i)*time.Second)) {
			t.Fatalf("expected unique signature %d to log", i)
		}
	}
	liveExitDispatchFailureLogDedupe.Lock()
	entryCount := len(liveExitDispatchFailureLogDedupe.entries)
	liveExitDispatchFailureLogDedupe.Unlock()
	if entryCount > liveExitDispatchFailureLogDedupeMaxEntries {
		t.Fatalf("expected bounded dedupe cache <= %d entries, got %d", liveExitDispatchFailureLogDedupeMaxEntries, entryCount)
	}
}

func TestLoadPersistedRuntimePolicyKeepsDisabledHealthThresholds(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	updated, err := platform.UpdateRuntimePolicy(RuntimePolicy{
		TradeTickFreshnessSeconds:      15,
		OrderBookFreshnessSeconds:      10,
		SignalBarFreshnessSeconds:      30,
		RuntimeQuietSeconds:            30,
		StrategyEvaluationQuietSeconds: 0,
		LiveAccountSyncFreshnessSecs:   0,
		PaperStartReadinessTimeoutSecs: 5,
	})
	if err != nil {
		t.Fatalf("update runtime policy failed: %v", err)
	}

	reloaded := NewPlatform(store)
	if err := reloaded.LoadPersistedRuntimePolicy(); err != nil {
		t.Fatalf("load persisted runtime policy failed: %v", err)
	}
	current := reloaded.RuntimePolicy()
	if current.StrategyEvaluationQuietSeconds != 0 {
		t.Fatalf("expected persisted strategy evaluation quiet threshold 0, got %d", current.StrategyEvaluationQuietSeconds)
	}
	if current.LiveAccountSyncFreshnessSecs != 0 {
		t.Fatalf("expected persisted live account sync freshness threshold 0, got %d", current.LiveAccountSyncFreshnessSecs)
	}
	if current.UpdatedAt.IsZero() {
		t.Fatal("expected persisted runtime policy updatedAt to be populated")
	}

	snapshot, err := reloaded.HealthSnapshot()
	if err != nil {
		t.Fatalf("build health snapshot failed: %v", err)
	}
	if snapshot.RuntimePolicy.StrategyEvaluationQuietSeconds != 0 {
		t.Fatalf("expected health snapshot strategy evaluation quiet threshold 0, got %d", snapshot.RuntimePolicy.StrategyEvaluationQuietSeconds)
	}
	if snapshot.RuntimePolicy.LiveAccountSyncFreshnessSecs != 0 {
		t.Fatalf("expected health snapshot live account sync freshness threshold 0, got %d", snapshot.RuntimePolicy.LiveAccountSyncFreshnessSecs)
	}
	if snapshot.RuntimePolicy.UpdatedAt != updated.UpdatedAt {
		t.Fatalf("expected health snapshot updatedAt %s, got %s", updated.UpdatedAt.Format(time.RFC3339), snapshot.RuntimePolicy.UpdatedAt.Format(time.RFC3339))
	}

	staleState := map[string]any{
		"healthSummary": map[string]any{
			"strategyIngress": map[string]any{
				"lastTriggeredAt": time.Now().UTC().Add(-5 * time.Minute).Format(time.RFC3339),
			},
		},
	}
	if reloaded.strategyEvaluationQuiet(staleState) {
		t.Fatal("expected strategy evaluation quiet to stay disabled when threshold is 0")
	}

	account := domain.Account{
		CreatedAt: time.Now().UTC().Add(-10 * time.Minute),
		Metadata: map[string]any{
			"lastLiveSyncAt": time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339),
			"healthSummary": map[string]any{
				"accountSync": map[string]any{
					"lastSuccessAt": time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339),
				},
			},
		},
	}
	if stale, _ := reloaded.liveAccountSyncStale(account, time.Now().UTC()); stale {
		t.Fatal("expected live account sync stale to stay disabled when threshold is 0")
	}
	if reloaded.shouldRefreshLiveAccountSync(account, time.Now().UTC()) {
		t.Fatal("expected live account refresh to stay disabled when threshold is 0")
	}
}

func TestHealthSnapshotAggregatesBackendHealthState(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	now := time.Now().UTC()
	policy := platform.RuntimePolicy()
	policy.UpdatedAt = now
	platform.SetRuntimePolicy(policy)

	account, err := platform.store.GetAccount("live-main")
	if err != nil {
		t.Fatalf("get live account failed: %v", err)
	}
	account.Status = "CONFIGURED"
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["lastLiveSyncAt"] = now.Add(-20 * time.Second).Format(time.RFC3339)
	account.Metadata["healthSummary"] = map[string]any{
		"accountSync": map[string]any{
			"lastSuccessAt":         now.Add(-20 * time.Second).Format(time.RFC3339),
			"lastStatus":            "SYNCED",
			"consecutiveErrorCount": 0,
		},
	}
	if _, err := platform.store.UpdateAccount(account); err != nil {
		t.Fatalf("update live account failed: %v", err)
	}

	if _, err := platform.store.UpdateLiveSessionStatus("live-session-main", "RUNNING"); err != nil {
		t.Fatalf("update live session status failed: %v", err)
	}
	liveState := map[string]any{
		"signalRuntimeSessionId":           "runtime-1",
		"lastSignalRuntimeEventAt":         now.Add(-5 * time.Second).Format(time.RFC3339),
		"lastStrategyEvaluationAt":         now.Add(-3 * time.Second).Format(time.RFC3339),
		"lastStrategyEvaluationStatus":     "evaluated",
		"lastSyncedOrderStatus":            "FILLED",
		"lastStrategyEvaluationSourceGate": map[string]any{"ready": true},
		"healthSummary": map[string]any{
			"strategyIngress": map[string]any{
				"lastTriggeredAt":  now.Add(-5 * time.Second).Format(time.RFC3339),
				"lastEvaluationAt": now.Add(-3 * time.Second).Format(time.RFC3339),
			},
			"execution": map[string]any{
				"lastDispatchAt":     now.Add(-2 * time.Second).Format(time.RFC3339),
				"lastProposalStatus": "dispatchable",
			},
		},
	}
	if _, err := platform.store.UpdateLiveSessionState("live-session-main", liveState); err != nil {
		t.Fatalf("update live session state failed: %v", err)
	}
	stoppedSession, err := platform.CreateLiveSession("", "live-main", "strategy-bk-1d", map[string]any{
		"symbol":          "BTCUSDT",
		"signalTimeframe": "1d",
	})
	if err != nil {
		t.Fatalf("create stopped live session failed: %v", err)
	}
	if _, err := platform.store.UpdateLiveSessionStatus(stoppedSession.ID, "STOPPED"); err != nil {
		t.Fatalf("update stopped live session status failed: %v", err)
	}
	if _, err := platform.store.UpdateLiveSessionState(stoppedSession.ID, map[string]any{
		"lastSignalRuntimeEventAt": now.Add(-1 * time.Minute).Format(time.RFC3339),
		"healthSummary": map[string]any{
			"strategyIngress": map[string]any{
				"lastTriggeredAt": now.Add(-1 * time.Minute).Format(time.RFC3339),
			},
		},
	}); err != nil {
		t.Fatalf("update stopped live session state failed: %v", err)
	}

	paperAccount, err := platform.CreateAccount("Paper", "PAPER", "binance-futures")
	if err != nil {
		t.Fatalf("create paper account failed: %v", err)
	}
	paperSession, err := platform.store.CreatePaperSession(paperAccount.ID, "strategy-bk-1d", 1000)
	if err != nil {
		t.Fatalf("create paper session failed: %v", err)
	}
	paperState := map[string]any{
		"lastStrategyEvaluationAt":     now.Add(-4 * time.Second).Format(time.RFC3339),
		"lastStrategyEvaluationStatus": "waiting-decision",
		"healthSummary": map[string]any{
			"strategyIngress": map[string]any{
				"lastTriggeredAt": now.Add(-6 * time.Second).Format(time.RFC3339),
			},
		},
	}
	if _, err := platform.store.UpdatePaperSessionState(paperSession.ID, paperState); err != nil {
		t.Fatalf("update paper session state failed: %v", err)
	}

	platform.signalSessions["runtime-1"] = domain.SignalRuntimeSession{
		ID:         "runtime-1",
		AccountID:  "live-main",
		StrategyID: "strategy-bk-1d",
		Status:     "RUNNING",
		Transport:  "websocket",
		State: map[string]any{
			"health":          "healthy",
			"lastEventAt":     now.Add(-1 * time.Second).Format(time.RFC3339),
			"lastHeartbeatAt": now.Add(-1 * time.Second).Format(time.RFC3339),
			"healthSummary": map[string]any{
				"tradeTick": map[string]any{
					"lastPrice": 68000.0,
				},
				"orderBook": map[string]any{
					"lastBestBid": 67999.0,
					"lastBestAsk": 68001.0,
				},
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	snapshot, err := platform.HealthSnapshot()
	if err != nil {
		t.Fatalf("build health snapshot failed: %v", err)
	}
	if len(snapshot.LiveAccounts) != 1 {
		t.Fatalf("expected 1 live account snapshot, got %d", len(snapshot.LiveAccounts))
	}
	if len(snapshot.RuntimeSessions) != 1 {
		t.Fatalf("expected 1 runtime snapshot, got %d", len(snapshot.RuntimeSessions))
	}
	if len(snapshot.LiveSessions) != 2 {
		t.Fatalf("expected 2 live session snapshots, got %d", len(snapshot.LiveSessions))
	}
	if len(snapshot.PaperSessions) != 1 {
		t.Fatalf("expected 1 paper session snapshot, got %d", len(snapshot.PaperSessions))
	}
	if snapshot.RuntimePolicy.UpdatedAt.IsZero() {
		t.Fatal("expected runtime policy updatedAt to be populated")
	}
	if got := snapshot.LiveAccounts[0].RuntimeSessionCount; got != 1 {
		t.Fatalf("expected runtime session count 1, got %d", got)
	}
	if got := snapshot.RuntimeSessions[0].Health; got != "healthy" {
		t.Fatalf("expected runtime health healthy, got %s", got)
	}
	if got := parseFloatValue(snapshot.RuntimeSessions[0].TradeTick["lastPrice"]); got != 68000.0 {
		t.Fatalf("expected trade tick last price 68000, got %v", got)
	}
	liveSnapshotsByID := make(map[string]domain.PlatformHealthStrategySessionSnapshot, len(snapshot.LiveSessions))
	for _, item := range snapshot.LiveSessions {
		liveSnapshotsByID[item.ID] = item
	}
	if got := liveSnapshotsByID["live-session-main"].RuntimeSessionID; got != "runtime-1" {
		t.Fatalf("expected live session runtime id runtime-1, got %s", got)
	}
	if liveSnapshotsByID[stoppedSession.ID].EvaluationQuiet {
		t.Fatal("expected stopped live session to suppress evaluationQuiet")
	}
	if got := snapshot.PaperSessions[0].Mode; got != "PAPER" {
		t.Fatalf("expected paper session mode PAPER, got %s", got)
	}
}

func TestLiveSessionEvaluationQuietMatchesAlertsAndSnapshot(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	now := time.Now().UTC()

	session, err := platform.store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["healthSummary"] = map[string]any{
		"strategyIngress": map[string]any{
			"lastTriggeredAt": now.Add(-30 * time.Second).Format(time.RFC3339),
		},
	}
	state["lastSignalRuntimeEventAt"] = now.Add(-30 * time.Second).Format(time.RFC3339)
	session.State = state
	session.Status = "RUNNING"
	if _, err := platform.store.UpdateLiveSession(session); err != nil {
		t.Fatalf("update live session failed: %v", err)
	}

	alerts, err := platform.ListAlerts()
	if err != nil {
		t.Fatalf("list alerts failed: %v", err)
	}
	foundQuietAlert := false
	for _, alert := range alerts {
		if alert.ID == "live-strategy-eval-quiet-"+session.ID {
			foundQuietAlert = true
			break
		}
	}
	if !foundQuietAlert {
		t.Fatal("expected running live session quiet alert to be present")
	}

	snapshot, err := platform.HealthSnapshot()
	if err != nil {
		t.Fatalf("build health snapshot failed: %v", err)
	}
	liveSnapshotsByID := make(map[string]domain.PlatformHealthStrategySessionSnapshot, len(snapshot.LiveSessions))
	for _, item := range snapshot.LiveSessions {
		liveSnapshotsByID[item.ID] = item
	}
	if !liveSnapshotsByID[session.ID].EvaluationQuiet {
		t.Fatal("expected running live session snapshot to report evaluationQuiet")
	}

	session.Status = "STOPPED"
	if _, err := platform.store.UpdateLiveSession(session); err != nil {
		t.Fatalf("stop live session failed: %v", err)
	}

	alerts, err = platform.ListAlerts()
	if err != nil {
		t.Fatalf("list alerts after stop failed: %v", err)
	}
	for _, alert := range alerts {
		if alert.ID == "live-strategy-eval-quiet-"+session.ID {
			t.Fatal("expected stopped live session quiet alert to be suppressed")
		}
	}

	snapshot, err = platform.HealthSnapshot()
	if err != nil {
		t.Fatalf("build health snapshot after stop failed: %v", err)
	}
	liveSnapshotsByID = make(map[string]domain.PlatformHealthStrategySessionSnapshot, len(snapshot.LiveSessions))
	for _, item := range snapshot.LiveSessions {
		liveSnapshotsByID[item.ID] = item
	}
	if liveSnapshotsByID[session.ID].EvaluationQuiet {
		t.Fatal("expected stopped live session snapshot to suppress evaluationQuiet")
	}
}

func TestListAlertsShowsStuckLiveControlPending(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	now := time.Now().UTC()
	state := cloneMetadata(session.State)
	state["desiredStatus"] = "RUNNING"
	state["actualStatus"] = "STARTING"
	state["lastControlAction"] = "start"
	state["controlRequestId"] = "control-request-stuck"
	state["controlVersion"] = 7
	state["controlRequestedAt"] = now.Add(-10 * time.Minute).Format(time.RFC3339)
	state["lastControlUpdateAt"] = now.Add(-3 * time.Minute).Format(time.RFC3339)
	session.State = state
	session.Status = "STOPPED"
	if _, err := platform.store.UpdateLiveSession(session); err != nil {
		t.Fatalf("update live session failed: %v", err)
	}

	alerts, err := platform.ListAlerts()
	if err != nil {
		t.Fatalf("list alerts failed: %v", err)
	}
	found := false
	for _, alert := range alerts {
		if alert.ID != "live-control-pending-"+session.ID {
			continue
		}
		found = true
		if alert.Level != "warning" {
			t.Fatalf("expected warning level, got %s", alert.Level)
		}
		if got := stringValue(alert.Metadata["controlRequestId"]); got != "control-request-stuck" {
			t.Fatalf("expected request id metadata, got %s", got)
		}
		if got := liveSessionControlVersionKey(alert.Metadata, "controlVersion"); got != 7 {
			t.Fatalf("expected control version 7, got %d", got)
		}
		if got := stringValue(alert.Metadata["actualStatus"]); got != "STARTING" {
			t.Fatalf("expected actual STARTING metadata, got %s", got)
		}
		pending, _ := toFloat64(alert.Metadata["pendingSeconds"])
		if int64(pending) < int64(liveSessionControlPendingAlertThreshold.Seconds()) {
			t.Fatalf("expected pending seconds over threshold, got %.0f", pending)
		}
	}
	if !found {
		t.Fatal("expected live control pending alert")
	}
}

func TestListAlertsSuppressesFreshOrErrorLiveControlPending(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	now := time.Now().UTC()
	state := cloneMetadata(session.State)
	state["desiredStatus"] = "STOPPED"
	state["actualStatus"] = "STOPPING"
	state["lastControlAction"] = "stop"
	state["controlRequestId"] = "control-request-fresh"
	state["controlVersion"] = 9
	state["controlRequestedAt"] = now.Add(-30 * time.Second).Format(time.RFC3339)
	state["lastControlUpdateAt"] = now.Add(-10 * time.Second).Format(time.RFC3339)
	session.State = state
	session.Status = "RUNNING"
	if _, err := platform.store.UpdateLiveSession(session); err != nil {
		t.Fatalf("update live session failed: %v", err)
	}

	alerts, err := platform.ListAlerts()
	if err != nil {
		t.Fatalf("list fresh alerts failed: %v", err)
	}
	for _, alert := range alerts {
		if alert.ID == "live-control-pending-"+session.ID {
			t.Fatal("expected fresh pending control alert to be suppressed")
		}
	}

	state["actualStatus"] = "ERROR"
	state["lastControlErrorAt"] = now.Add(-10 * time.Minute).Format(time.RFC3339)
	state["lastControlError"] = "adapter unavailable"
	session.State = state
	if _, err := platform.store.UpdateLiveSession(session); err != nil {
		t.Fatalf("update error live session failed: %v", err)
	}
	alerts, err = platform.ListAlerts()
	if err != nil {
		t.Fatalf("list error alerts failed: %v", err)
	}
	for _, alert := range alerts {
		if alert.ID == "live-control-pending-"+session.ID {
			t.Fatal("expected ERROR control state not to emit pending alert")
		}
	}
}

func TestListAlertsShowsOrphanLiveControlActiveRequest(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	session, err := platform.store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	now := time.Now().UTC()
	state := cloneMetadata(session.State)
	state["desiredStatus"] = "RUNNING"
	state["actualStatus"] = "RUNNING"
	state["controlRequestId"] = "control-request-orphan"
	state["controlVersion"] = 13
	state["activeControlRequestId"] = "control-request-orphan"
	state["activeControlVersion"] = 13
	state["controlRequestedAt"] = now.Add(-5 * time.Minute).Format(time.RFC3339)
	state["lastControlUpdateAt"] = now.Add(-4 * time.Minute).Format(time.RFC3339)
	session.State = state
	session.Status = "RUNNING"
	if _, err := platform.store.UpdateLiveSession(session); err != nil {
		t.Fatalf("update live session failed: %v", err)
	}

	alerts, err := platform.ListAlerts()
	if err != nil {
		t.Fatalf("list alerts failed: %v", err)
	}
	for _, alert := range alerts {
		if alert.ID != "live-control-active-request-"+session.ID {
			continue
		}
		if got := stringValue(alert.Metadata["reason"]); got != "orphan" {
			t.Fatalf("expected orphan metadata, got %s", got)
		}
		if got := stringValue(alert.Metadata["activeControlRequestId"]); got != "control-request-orphan" {
			t.Fatalf("expected active request metadata, got %s", got)
		}
		return
	}
	t.Fatal("expected orphan live control active request alert")
}
