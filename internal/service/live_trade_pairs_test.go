package service

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestListLiveTradePairsClassifiesTrailingStopExitAsTSL(t *testing.T) {
	platform, session := newLiveTradePairTestPlatform(t)
	pair := createClosedLiveTradePairFixture(t, platform, session, liveTradeFixture{
		entryPrice:        100,
		exitPrice:         112,
		quantity:          2,
		entryFee:          0.2,
		exitFee:           0.3,
		exitReason:        "SL",
		targetPriceSource: "trailing-stop",
	})

	if got := pair.ExitClassifier; got != "TSL" {
		t.Fatalf("expected TSL classifier, got %s", got)
	}
	if got := pair.ExitVerdict; got != "normal" {
		t.Fatalf("expected normal verdict, got %s", got)
	}
	assertTradePairFloat(t, pair.RealizedPnL, 24)
	assertTradePairFloat(t, pair.Fees, 0.5)
	assertTradePairFloat(t, pair.NetPnL, 23.5)
}

func TestListLiveTradePairsClassifiesTakeProfitExitAsTP(t *testing.T) {
	platform, session := newLiveTradePairTestPlatform(t)
	pair := createClosedLiveTradePairFixture(t, platform, session, liveTradeFixture{
		entryPrice:        200,
		exitPrice:         215,
		quantity:          1.5,
		entryFee:          0.15,
		exitFee:           0.2,
		exitReason:        "PT",
		targetPriceSource: "structure-profit",
	})

	if got := pair.ExitClassifier; got != "TP" {
		t.Fatalf("expected TP classifier, got %s", got)
	}
	if got := pair.ExitVerdict; got != "normal" {
		t.Fatalf("expected normal verdict, got %s", got)
	}
	assertTradePairFloat(t, pair.RealizedPnL, 22.5)
	assertTradePairFloat(t, pair.NetPnL, 22.15)
}

func TestListLiveTradePairsClassifiesInitialStopExitAsSL(t *testing.T) {
	platform, session := newLiveTradePairTestPlatform(t)
	pair := createClosedLiveTradePairFixture(t, platform, session, liveTradeFixture{
		entryPrice:        150,
		exitPrice:         140,
		quantity:          1.2,
		entryFee:          0.12,
		exitFee:           0.18,
		exitReason:        "SL",
		targetPriceSource: "initial-stop",
	})

	if got := pair.ExitClassifier; got != "SL" {
		t.Fatalf("expected SL classifier, got %s", got)
	}
	if got := pair.ExitVerdict; got != "normal" {
		t.Fatalf("expected normal verdict, got %s", got)
	}
	assertTradePairFloat(t, pair.RealizedPnL, -12)
	assertTradePairFloat(t, pair.NetPnL, -12.3)
}

func TestListLiveTradePairsReturnsOpenTradeWithUnrealizedPnLAndAggregatedEntries(t *testing.T) {
	platform, session := newLiveTradePairTestPlatform(t)
	entryAt := time.Date(2026, 4, 22, 6, 0, 0, 0, time.UTC)

	firstEntry := createLiveTradePairOrder(t, platform, session, tradePairOrderFixture{
		side:       "BUY",
		reason:     "initial",
		quantity:   0.001,
		price:      100,
		fee:        0.01,
		createdAt:  entryAt,
		fillAt:     entryAt.Add(2 * time.Second),
		reduceOnly: false,
	})
	secondEntry := createLiveTradePairOrder(t, platform, session, tradePairOrderFixture{
		side:       "BUY",
		reason:     "sl-reentry",
		quantity:   0.002,
		price:      105,
		fee:        0.02,
		createdAt:  entryAt.Add(5 * time.Minute),
		fillAt:     entryAt.Add(5*time.Minute + 2*time.Second),
		reduceOnly: false,
	})
	_ = firstEntry
	_ = secondEntry

	if _, err := platform.store.SavePosition(domain.Position{
		AccountID:         session.AccountID,
		StrategyVersionID: "strategy-version-test",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.003,
		EntryPrice:        (100*0.001 + 105*0.002) / 0.003,
		MarkPrice:         110,
		UpdatedAt:         entryAt.Add(10 * time.Minute),
	}); err != nil {
		t.Fatalf("save position: %v", err)
	}

	items, err := platform.ListLiveTradePairs(domain.LiveTradePairQuery{
		LiveSessionID: session.ID,
		Status:        "open",
		Limit:         10,
	})
	if err != nil {
		t.Fatalf("list live trade pairs: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 open pair, got %d", len(items))
	}
	pair := items[0]
	if got := pair.Status; got != "open" {
		t.Fatalf("expected open status, got %s", got)
	}
	if got := pair.Side; got != "LONG" {
		t.Fatalf("expected LONG side, got %s", got)
	}
	if got := len(pair.EntryOrderIDs); got != 2 {
		t.Fatalf("expected 2 aggregated entry orders, got %d", got)
	}
	assertTradePairFloat(t, pair.EntryQuantity, 0.003)
	assertTradePairFloat(t, pair.OpenQuantity, 0.003)
	assertTradePairFloat(t, pair.EntryAvgPrice, (100*0.001+105*0.002)/0.003)
	assertTradePairFloat(t, pair.UnrealizedPnL, (110-pair.EntryAvgPrice)*0.003)
	assertTradePairFloat(t, pair.Fees, 0.03)
	assertTradePairFloat(t, pair.NetPnL, pair.UnrealizedPnL-0.03)
}

func TestListLiveTradePairsFallsBackWhenTelemetryTablesAreUnavailable(t *testing.T) {
	baseStore := memory.NewStore()
	platform := NewPlatform(&testMissingLiveTradePairTelemetryStore{Store: baseStore})
	account, err := platform.CreateAccount("Live Trade Pair", "LIVE", "binance-futures")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	session, err := platform.CreateLiveSession("", account.ID, "strategy-bk-1d", map[string]any{
		"symbol": "BTCUSDT",
	})
	if err != nil {
		t.Fatalf("create live session: %v", err)
	}

	entryAt := time.Date(2026, 4, 22, 6, 0, 0, 0, time.UTC)
	exitAt := entryAt.Add(30 * time.Minute)
	createLiveTradePairOrder(t, platform, session, tradePairOrderFixture{
		side:       "BUY",
		reason:     "initial",
		quantity:   1,
		price:      100,
		fee:        0.1,
		createdAt:  entryAt,
		fillAt:     entryAt.Add(2 * time.Second),
		reduceOnly: false,
	})
	createLiveTradePairOrder(t, platform, session, tradePairOrderFixture{
		side:       "SELL",
		reason:     "PT",
		quantity:   1,
		price:      110,
		fee:        0.1,
		createdAt:  exitAt,
		fillAt:     exitAt.Add(2 * time.Second),
		reduceOnly: true,
	})

	items, err := platform.ListLiveTradePairs(domain.LiveTradePairQuery{
		LiveSessionID: session.ID,
		Status:        "closed",
		Limit:         10,
	})
	if err != nil {
		t.Fatalf("list live trade pairs: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 closed pair, got %d", len(items))
	}
	pair := items[0]
	if got := pair.ExitClassifier; got != "TP" {
		t.Fatalf("expected TP classifier, got %s", got)
	}
	if got := pair.ExitVerdict; got != "normal" {
		t.Fatalf("expected normal verdict, got %s", got)
	}
	assertTradePairFloat(t, pair.RealizedPnL, 10)
	assertTradePairFloat(t, pair.Fees, 0.2)
	assertTradePairFloat(t, pair.NetPnL, 9.8)
}

func TestListLiveTradePairsScopesTelemetryQueriesToPairOrders(t *testing.T) {
	baseStore := memory.NewStore()
	store := &testScopedLiveTradePairTelemetryStore{Store: baseStore}
	platform := NewPlatform(store)
	account, err := platform.CreateAccount("Live Trade Pair", "LIVE", "binance-futures")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	session, err := platform.CreateLiveSession("", account.ID, "strategy-bk-1d", map[string]any{
		"symbol": "BTCUSDT",
	})
	if err != nil {
		t.Fatalf("create live session: %v", err)
	}

	pair := createClosedLiveTradePairFixture(t, platform, session, liveTradeFixture{
		entryPrice:        100,
		exitPrice:         112,
		quantity:          2,
		entryFee:          0.2,
		exitFee:           0.3,
		exitReason:        "SL",
		targetPriceSource: "trailing-stop",
	})

	if got := pair.ExitClassifier; got != "TSL" {
		t.Fatalf("expected TSL classifier, got %s", got)
	}
	if store.usedLiveSessionTelemetryQuery {
		t.Fatalf("expected trade pair telemetry lookup to avoid full live-session query")
	}
	if store.usedBulkSnapshotTelemetryQuery {
		t.Fatalf("expected trade pair snapshot lookup to avoid bulk live-session query")
	}
}

type liveTradeFixture struct {
	entryPrice        float64
	exitPrice         float64
	quantity          float64
	entryFee          float64
	exitFee           float64
	exitReason        string
	targetPriceSource string
}

type tradePairOrderFixture struct {
	side              string
	reason            string
	quantity          float64
	price             float64
	fee               float64
	createdAt         time.Time
	fillAt            time.Time
	reduceOnly        bool
	decisionEventID   string
	targetPriceSource string
}

type liveTradePairTargetedQueryStore struct {
	*memory.Store
	decisionQueries []domain.StrategyDecisionEventQuery
	snapshotQueries []domain.PositionAccountSnapshotQuery
}

func (s *liveTradePairTargetedQueryStore) QueryStrategyDecisionEvents(query domain.StrategyDecisionEventQuery) ([]domain.StrategyDecisionEvent, error) {
	s.decisionQueries = append(s.decisionQueries, query)
	if query.DecisionEventID == "" {
		return nil, fmt.Errorf("expected targeted decision event query")
	}
	return s.Store.QueryStrategyDecisionEvents(query)
}

func (s *liveTradePairTargetedQueryStore) QueryPositionAccountSnapshots(query domain.PositionAccountSnapshotQuery) ([]domain.PositionAccountSnapshot, error) {
	s.snapshotQueries = append(s.snapshotQueries, query)
	if query.OrderID == "" {
		return nil, fmt.Errorf("expected targeted position snapshot query")
	}
	return s.Store.QueryPositionAccountSnapshots(query)
}

func newLiveTradePairTestPlatform(t *testing.T) (*Platform, domain.LiveSession) {
	t.Helper()
	platform := NewPlatform(memory.NewStore())
	account, err := platform.CreateAccount("Live Trade Pair", "LIVE", "binance-futures")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	session, err := platform.CreateLiveSession("", account.ID, "strategy-bk-1d", map[string]any{
		"symbol": "BTCUSDT",
	})
	if err != nil {
		t.Fatalf("create live session: %v", err)
	}
	return platform, session
}

func createClosedLiveTradePairFixture(t *testing.T, platform *Platform, session domain.LiveSession, fixture liveTradeFixture) domain.LiveTradePair {
	t.Helper()
	entryAt := time.Date(2026, 4, 22, 6, 0, 0, 0, time.UTC)
	exitAt := entryAt.Add(30 * time.Minute)
	exitDecisionEventID := "decision-event-exit-" + normalizeStrategyReasonTag(fixture.exitReason)

	createLiveTradePairOrder(t, platform, session, tradePairOrderFixture{
		side:       "BUY",
		reason:     "initial",
		quantity:   fixture.quantity,
		price:      fixture.entryPrice,
		fee:        fixture.entryFee,
		createdAt:  entryAt,
		fillAt:     entryAt.Add(2 * time.Second),
		reduceOnly: false,
	})
	exitOrder := createLiveTradePairOrder(t, platform, session, tradePairOrderFixture{
		side:              "SELL",
		reason:            fixture.exitReason,
		quantity:          fixture.quantity,
		price:             fixture.exitPrice,
		fee:               fixture.exitFee,
		createdAt:         exitAt,
		fillAt:            exitAt.Add(2 * time.Second),
		reduceOnly:        true,
		decisionEventID:   exitDecisionEventID,
		targetPriceSource: fixture.targetPriceSource,
	})

	if _, err := platform.store.CreateStrategyDecisionEvent(domain.StrategyDecisionEvent{
		ID:            exitDecisionEventID,
		LiveSessionID: session.ID,
		AccountID:     session.AccountID,
		StrategyID:    session.StrategyID,
		Symbol:        "BTCUSDT",
		Action:        "advance-plan",
		Reason:        fixture.exitReason,
		SignalKind:    "risk-exit",
		EventTime:     exitAt.UTC(),
		DecisionMetadata: map[string]any{
			"signalBarDecision": map[string]any{
				"targetPriceSource": fixture.targetPriceSource,
			},
		},
	}); err != nil {
		t.Fatalf("create strategy decision event: %v", err)
	}
	if _, err := platform.store.CreatePositionAccountSnapshot(domain.PositionAccountSnapshot{
		LiveSessionID:    session.ID,
		DecisionEventID:  exitDecisionEventID,
		OrderID:          exitOrder.ID,
		AccountID:        session.AccountID,
		StrategyID:       session.StrategyID,
		Symbol:           "BTCUSDT",
		Trigger:          "order-filled",
		PositionFound:    false,
		PositionQuantity: 0,
		EventTime:        exitAt.Add(3 * time.Second).UTC(),
	}); err != nil {
		t.Fatalf("create position snapshot: %v", err)
	}
	if _, err := platform.store.CreateOrderCloseVerification(domain.OrderCloseVerification{
		LiveSessionID:        session.ID,
		OrderID:              exitOrder.ID,
		DecisionEventID:      exitDecisionEventID,
		AccountID:            session.AccountID,
		StrategyID:           session.StrategyID,
		Symbol:               "BTCUSDT",
		VerifiedClosed:       true,
		RemainingPositionQty: 0,
		VerificationSource:   "ws-sync",
		EventTime:            exitAt.Add(3 * time.Second).UTC(),
	}); err != nil {
		t.Fatalf("create order close verification: %v", err)
	}

	items, err := platform.ListLiveTradePairs(domain.LiveTradePairQuery{
		LiveSessionID: session.ID,
		Status:        "closed",
		Limit:         10,
	})
	if err != nil {
		t.Fatalf("list live trade pairs: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 closed pair, got %d", len(items))
	}
	return items[0]
}

func createLiveTradePairOrder(t *testing.T, platform *Platform, session domain.LiveSession, fixture tradePairOrderFixture) domain.Order {
	t.Helper()
	role := "entry"
	if fixture.reduceOnly {
		role = "exit"
	}
	orderMeta := map[string]any{
		"liveSessionId": session.ID,
		"executionMode": "live",
		"executionProposal": map[string]any{
			"role":     role,
			"reason":   fixture.reason,
			"side":     fixture.side,
			"symbol":   "BTCUSDT",
			"quantity": fixture.quantity,
			"status":   "dispatchable",
			"metadata": map[string]any{
				"targetPriceSource": fixture.targetPriceSource,
			},
		},
	}
	if fixture.decisionEventID != "" {
		orderMeta["decisionEventId"] = fixture.decisionEventID
		mapValue(orderMeta["executionProposal"])["decisionEventId"] = fixture.decisionEventID
	}

	order, err := platform.store.CreateOrder(domain.Order{
		AccountID:         session.AccountID,
		StrategyVersionID: "strategy-version-test",
		Symbol:            "BTCUSDT",
		Side:              fixture.side,
		Type:              "MARKET",
		Status:            "FILLED",
		Quantity:          fixture.quantity,
		Price:             fixture.price,
		ReduceOnly:        fixture.reduceOnly,
		Metadata:          orderMeta,
		CreatedAt:         fixture.createdAt.UTC(),
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if _, err := platform.store.CreateFill(domain.Fill{
		OrderID:           order.ID,
		Price:             fixture.price,
		Quantity:          fixture.quantity,
		Fee:               fixture.fee,
		ExchangeTradeTime: timePointer(fixture.fillAt.UTC()),
		CreatedAt:         fixture.fillAt.UTC(),
	}); err != nil {
		t.Fatalf("create fill: %v", err)
	}
	return order
}

func assertTradePairFloat(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected %.12f, got %.12f", want, got)
	}
}

func timePointer(value time.Time) *time.Time {
	return &value
}

type testMissingLiveTradePairTelemetryStore struct {
	*memory.Store
}

func (s *testMissingLiveTradePairTelemetryStore) QueryStrategyDecisionEvents(domain.StrategyDecisionEventQuery) ([]domain.StrategyDecisionEvent, error) {
	return nil, fmt.Errorf(`pq: relation "strategy_decision_events" does not exist (SQLSTATE 42P01)`)
}

func (s *testMissingLiveTradePairTelemetryStore) QueryPositionAccountSnapshots(domain.PositionAccountSnapshotQuery) ([]domain.PositionAccountSnapshot, error) {
	return nil, fmt.Errorf(`pq: relation "position_account_snapshots" does not exist (SQLSTATE 42P01)`)
}

type testScopedLiveTradePairTelemetryStore struct {
	*memory.Store
	usedLiveSessionTelemetryQuery  bool
	usedBulkSnapshotTelemetryQuery bool
}

func (s *testScopedLiveTradePairTelemetryStore) QueryStrategyDecisionEvents(query domain.StrategyDecisionEventQuery) ([]domain.StrategyDecisionEvent, error) {
	if query.LiveSessionID != "" {
		s.usedLiveSessionTelemetryQuery = true
		return nil, fmt.Errorf("unexpected full live session telemetry query")
	}
	return s.Store.QueryStrategyDecisionEvents(query)
}

func (s *testScopedLiveTradePairTelemetryStore) QueryPositionAccountSnapshots(query domain.PositionAccountSnapshotQuery) ([]domain.PositionAccountSnapshot, error) {
	if query.LiveSessionID != "" && query.OrderID == "" {
		s.usedBulkSnapshotTelemetryQuery = true
		return nil, fmt.Errorf("unexpected bulk snapshot telemetry query")
	}
	return s.Store.QueryPositionAccountSnapshots(query)
}

func TestListLiveTradePairsUsesLatestVerificationRecord(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	account, err := platform.CreateAccount("Live Trade Pair", "LIVE", "binance-futures")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	session, err := platform.CreateLiveSession("", account.ID, "strategy-bk-1d", map[string]any{
		"symbol": "BTCUSDT",
	})
	if err != nil {
		t.Fatalf("create live session: %v", err)
	}

	exitAt := time.Date(2026, 4, 22, 6, 30, 0, 0, time.UTC)
	pair := createClosedLiveTradePairFixture(t, platform, session, liveTradeFixture{
		entryPrice:        100,
		exitPrice:         110,
		quantity:          1,
		entryFee:          0.1,
		exitFee:           0.1,
		exitReason:        "PT",
		targetPriceSource: "take-profit",
	})

	// Add an older optimistic verification (ws-sync says it's closed)
	_, _ = platform.store.CreateOrderCloseVerification(domain.OrderCloseVerification{
		LiveSessionID:        session.ID,
		OrderID:              pair.ExitOrderIDs[0],
		AccountID:            session.AccountID,
		StrategyID:           session.StrategyID,
		Symbol:               "BTCUSDT",
		VerifiedClosed:       true,
		RemainingPositionQty: 0,
		VerificationSource:   "ws-sync",
		EventTime:            exitAt.Add(2 * time.Second),
	})

	// Add a newer pessimistic verification (reconcile says it's NOT closed)
	_, _ = platform.store.CreateOrderCloseVerification(domain.OrderCloseVerification{
		LiveSessionID:        session.ID,
		OrderID:              pair.ExitOrderIDs[0],
		AccountID:            session.AccountID,
		StrategyID:           session.StrategyID,
		Symbol:               "BTCUSDT",
		VerifiedClosed:       false,
		RemainingPositionQty: 0.5, // Some residual left
		VerificationSource:   "reconcile",
		EventTime:            exitAt.Add(10 * time.Second),
	})

	items, err := platform.ListLiveTradePairs(domain.LiveTradePairQuery{
		LiveSessionID: session.ID,
		Status:        "closed",
		Limit:         10,
	})
	if err != nil {
		t.Fatalf("list live trade pairs: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 closed pair, got %d", len(items))
	}
	if got := items[0].ExitVerdict; got != "mismatch" {
		t.Fatalf("expected mismatch verdict due to latest reconcile event, got %s", got)
	}
}
