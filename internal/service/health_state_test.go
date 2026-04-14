package service

import (
	"errors"
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

func TestHealthSnapshotAggregatesBackendHealthState(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	now := time.Now().UTC()

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
	if len(snapshot.LiveSessions) != 1 {
		t.Fatalf("expected 1 live session snapshot, got %d", len(snapshot.LiveSessions))
	}
	if len(snapshot.PaperSessions) != 1 {
		t.Fatalf("expected 1 paper session snapshot, got %d", len(snapshot.PaperSessions))
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
	if got := snapshot.LiveSessions[0].RuntimeSessionID; got != "runtime-1" {
		t.Fatalf("expected live session runtime id runtime-1, got %s", got)
	}
	if got := snapshot.PaperSessions[0].Mode; got != "PAPER" {
		t.Fatalf("expected paper session mode PAPER, got %s", got)
	}
}
