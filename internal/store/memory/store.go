package memory

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

type Store struct {
	mu sync.RWMutex

	strategies       map[string]domain.Strategy
	strategyVersion  map[string]domain.StrategyVersion
	accounts         map[string]domain.Account
	orders           map[string]domain.Order
	fills            map[string]domain.Fill
	positions        map[string]domain.Position
	backtests        map[string]domain.BacktestRun
	paperSessions    map[string]domain.PaperSession
	liveSessions     map[string]domain.LiveSession
	equitySnapshots  map[string][]domain.AccountEquitySnapshot
	decisionEvents   []domain.StrategyDecisionEvent
	executionEvents  []domain.OrderExecutionEvent
	liveSnapshots    []domain.PositionAccountSnapshot
	marketBars       map[string]domain.MarketBar
	signalSources    []map[string]any
	annotations      []domain.ChartAnnotation
	runtimePolicy    *domain.RuntimePolicy
	notificationAcks map[string]domain.NotificationAck
	telegramConfig   *domain.TelegramConfig
	deliveries       map[string]domain.NotificationDelivery

	sequence int64
}

func NewStore() *Store {
	now := time.Now().UTC()
	store := &Store{
		strategies:      make(map[string]domain.Strategy),
		strategyVersion: make(map[string]domain.StrategyVersion),
		accounts:        make(map[string]domain.Account),
		orders:          make(map[string]domain.Order),
		fills:           make(map[string]domain.Fill),
		positions:       make(map[string]domain.Position),
		backtests:       make(map[string]domain.BacktestRun),
		paperSessions:   make(map[string]domain.PaperSession),
		liveSessions:    make(map[string]domain.LiveSession),
		equitySnapshots: make(map[string][]domain.AccountEquitySnapshot),
		marketBars:      make(map[string]domain.MarketBar),
		signalSources: []map[string]any{
			{
				"id":          "signal-source-bk-1d",
				"name":        "BK 1D ATR Reentry",
				"type":        "internal-strategy",
				"status":      "ACTIVE",
				"dedupeKey":   "symbol+strategyVersion+reason+bar",
				"description": "1D signal / 1m execution strategy feed.",
			},
		},
		annotations: []domain.ChartAnnotation{
			{
				ID:     "anno-1",
				Source: "backtest",
				Type:   "entry_long",
				Symbol: "BTCUSDT",
				Time:   time.Date(2024, 2, 5, 14, 21, 0, 0, time.UTC),
				Price:  43125.0,
				Label:  "SL-Reentry",
			},
			{
				ID:     "anno-2",
				Source: "backtest",
				Type:   "exit_tp",
				Symbol: "BTCUSDT",
				Time:   time.Date(2024, 2, 17, 10, 12, 0, 0, time.UTC),
				Price:  52520.0,
				Label:  "PT",
			},
		},
		notificationAcks: make(map[string]domain.NotificationAck),
		deliveries:       make(map[string]domain.NotificationDelivery),
	}

	strategy := domain.Strategy{
		ID:          "strategy-bk-1d",
		Name:        "BK 1D Zero Initial",
		Status:      "ACTIVE",
		Description: "1D signal / 1m execution with zero initial risk and ATR protection.",
		CreatedAt:   now,
	}
	version := domain.StrategyVersion{
		ID:                 "strategy-version-bk-1d-v010",
		StrategyID:         strategy.ID,
		Version:            "v0.1.0",
		SignalTimeframe:    "1D",
		ExecutionTimeframe: "1m",
		Parameters: map[string]any{
			"strategyEngine":                  "bk-default",
			"max_trades_per_bar":              3,
			"reentry_size_schedule":           []float64{0.20, 0.10},
			"stop_mode":                       "atr",
			"stop_loss_atr":                   0.05,
			"profit_protect_atr":              1.0,
			"trailing_stop_atr":               0.3,
			"delayed_trailing_activation_atr": 0.5,
			"long_reentry_atr":                0.1,
			"short_reentry_atr":               0.0,
			"tradingFeeBps":                   10.0,
			"fundingRateBps":                  0.0,
			"fundingIntervalHours":            8,
		},
		CreatedAt: now,
	}
	store.strategies[strategy.ID] = strategy
	store.strategyVersion[version.ID] = version

	live := domain.Account{
		ID:       "live-main",
		Name:     "Live Main",
		Mode:     "LIVE",
		Exchange: "binance-futures",
		Status:   "PENDING_SETUP",
		Metadata: map[string]any{
			"liveBinding": map[string]any{
				"adapterKey":     "binance-futures",
				"feeSource":      "exchange",
				"fundingSource":  "exchange",
				"connectionMode": "disabled",
			},
		},
		CreatedAt: now,
	}
	store.accounts[live.ID] = live

	backtest := domain.BacktestRun{
		ID:                "backtest-20260403-001",
		StrategyVersionID: version.ID,
		Status:            "COMPLETED",
		Parameters:        version.Parameters,
		ResultSummary: map[string]any{
			"return":       1.51,
			"maxDrawdown":  -0.0055,
			"tradePairs":   1098,
			"sampleWindow": "2020-01-01 ~ 2026-02-28",
		},
		CreatedAt: now,
	}
	store.backtests[backtest.ID] = backtest

	store.liveSessions["live-session-main"] = domain.LiveSession{
		ID:         "live-session-main",
		AccountID:  live.ID,
		StrategyID: strategy.ID,
		Status:     "READY",
		State: map[string]any{
			"runner":       "strategy-engine",
			"dispatchMode": "manual-review",
			"planIndex":    0,
		},
		CreatedAt: now,
	}
	return store
}

func (s *Store) nextID(prefix string) string {
	s.sequence++
	return fmt.Sprintf("%s-%d", prefix, s.sequence)
}

func (s *Store) SignalSources() []map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]map[string]any, 0, len(s.signalSources))
	for _, item := range s.signalSources {
		out = append(out, item)
	}
	return out
}

func (s *Store) GetRuntimePolicy() (domain.RuntimePolicy, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.runtimePolicy == nil {
		return domain.RuntimePolicy{}, false, nil
	}
	return *s.runtimePolicy, true, nil
}

func (s *Store) UpsertRuntimePolicy(policy domain.RuntimePolicy) (domain.RuntimePolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	policy.UpdatedAt = time.Now().UTC()
	copyPolicy := policy
	s.runtimePolicy = &copyPolicy
	return policy, nil
}

func (s *Store) ListNotificationAcks() ([]domain.NotificationAck, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.NotificationAck, 0, len(s.notificationAcks))
	for _, item := range s.notificationAcks {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	return items, nil
}

func (s *Store) UpsertNotificationAck(id string) (domain.NotificationAck, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	item := domain.NotificationAck{
		ID:        id,
		AckedAt:   now,
		UpdatedAt: now,
	}
	s.notificationAcks[id] = item
	return item, nil
}

func (s *Store) DeleteNotificationAck(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.notificationAcks, id)
	return nil
}

func (s *Store) GetTelegramConfig() (domain.TelegramConfig, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.telegramConfig == nil {
		return domain.TelegramConfig{}, false, nil
	}
	return *s.telegramConfig, true, nil
}

func (s *Store) UpsertTelegramConfig(config domain.TelegramConfig) (domain.TelegramConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	config.UpdatedAt = time.Now().UTC()
	copyConfig := config
	s.telegramConfig = &copyConfig
	return config, nil
}

func (s *Store) ListNotificationDeliveries() ([]domain.NotificationDelivery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.NotificationDelivery, 0, len(s.deliveries))
	for _, item := range s.deliveries {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	return items, nil
}

func (s *Store) UpsertNotificationDelivery(notificationID, channel, status, lastError string) (domain.NotificationDelivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	key := notificationID + "::" + channel
	item := domain.NotificationDelivery{
		NotificationID: notificationID,
		Channel:        channel,
		Status:         status,
		LastError:      lastError,
		AttemptedAt:    now,
		UpdatedAt:      now,
	}
	if strings.EqualFold(status, "sent") {
		item.SentAt = now
	}
	s.deliveries[key] = item
	return item, nil
}

func (s *Store) ListStrategies() ([]map[string]any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	strategies := make([]domain.Strategy, 0, len(s.strategies))
	for _, strategy := range s.strategies {
		strategies = append(strategies, strategy)
	}
	sort.Slice(strategies, func(i, j int) bool { return strategies[i].CreatedAt.Before(strategies[j].CreatedAt) })

	result := make([]map[string]any, 0, len(strategies))
	for _, strategy := range strategies {
		item := map[string]any{
			"id":          strategy.ID,
			"name":        strategy.Name,
			"status":      strategy.Status,
			"description": strategy.Description,
			"createdAt":   strategy.CreatedAt,
		}
		for _, version := range s.strategyVersion {
			if version.StrategyID == strategy.ID {
				item["currentVersion"] = version
			}
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *Store) CreateStrategy(name, description string, parameters map[string]any) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	id := s.nextID("strategy")
	versionID := s.nextID("strategy-version")
	strategy := domain.Strategy{
		ID:          id,
		Name:        name,
		Status:      "DRAFT",
		Description: description,
		CreatedAt:   now,
	}
	version := domain.StrategyVersion{
		ID:                 versionID,
		StrategyID:         id,
		Version:            "v0.1.0",
		SignalTimeframe:    "1D",
		ExecutionTimeframe: "1m",
		Parameters:         parameters,
		CreatedAt:          now,
	}
	s.strategies[id] = strategy
	s.strategyVersion[versionID] = version

	return map[string]any{
		"id":             strategy.ID,
		"name":           strategy.Name,
		"status":         strategy.Status,
		"description":    strategy.Description,
		"createdAt":      strategy.CreatedAt,
		"currentVersion": version,
	}, nil
}

func (s *Store) UpdateStrategyParameters(strategyID string, parameters map[string]any) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	strategy, ok := s.strategies[strategyID]
	if !ok {
		return nil, fmt.Errorf("strategy not found: %s", strategyID)
	}

	var current domain.StrategyVersion
	found := false
	for _, version := range s.strategyVersion {
		if version.StrategyID != strategyID {
			continue
		}
		if !found || version.CreatedAt.After(current.CreatedAt) {
			current = version
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("strategy version not found: %s", strategyID)
	}
	current.Parameters = parameters
	s.strategyVersion[current.ID] = current

	return map[string]any{
		"id":             strategy.ID,
		"name":           strategy.Name,
		"status":         strategy.Status,
		"description":    strategy.Description,
		"createdAt":      strategy.CreatedAt,
		"currentVersion": current,
	}, nil
}

func (s *Store) ListAccounts() ([]domain.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.Account, 0, len(s.accounts))
	for _, item := range s.accounts {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items, nil
}

func (s *Store) CreateAccount(name, mode, exchange string) (domain.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	account := domain.Account{
		ID:        s.nextID("account"),
		Name:      name,
		Mode:      mode,
		Exchange:  exchange,
		Status:    accountStatusForMode(mode),
		Metadata:  map[string]any{},
		CreatedAt: time.Now().UTC(),
	}
	s.accounts[account.ID] = account
	return account, nil
}

func (s *Store) GetAccount(accountID string) (domain.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	account, ok := s.accounts[accountID]
	if !ok {
		return domain.Account{}, fmt.Errorf("account not found: %s", accountID)
	}
	return account, nil
}

func (s *Store) UpdateAccount(account domain.Account) (domain.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.accounts[account.ID]; !ok {
		return domain.Account{}, fmt.Errorf("account not found: %s", account.ID)
	}
	if account.Metadata == nil {
		account.Metadata = map[string]any{}
	}
	s.accounts[account.ID] = account
	return account, nil
}

func (s *Store) ListOrders() ([]domain.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.Order, 0, len(s.orders))
	for _, item := range s.orders {
		item.NormalizeExecutionFlags()
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items, nil
}

func (s *Store) CreateOrder(order domain.Order) (domain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	order.NormalizeExecutionFlags()
	order.ID = s.nextID("order")
	order.Status = "NEW"
	order.CreatedAt = time.Now().UTC()
	if order.Metadata == nil {
		order.Metadata = map[string]any{}
	}
	s.orders[order.ID] = order
	return order, nil
}

func (s *Store) UpdateOrder(order domain.Order) (domain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	order.NormalizeExecutionFlags()
	existing, ok := s.orders[order.ID]
	if !ok {
		return domain.Order{}, fmt.Errorf("order not found: %s", order.ID)
	}
	order.CreatedAt = existing.CreatedAt
	s.orders[order.ID] = order
	return order, nil
}

func (s *Store) ListFills() ([]domain.Fill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.Fill, 0, len(s.fills))
	for _, item := range s.fills {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items, nil
}

func (s *Store) TotalFilledQuantityForOrder(orderID string) (float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := 0.0
	for _, item := range s.fills {
		if item.OrderID != orderID {
			continue
		}
		total += item.Quantity
	}
	return total, nil
}

func (s *Store) CreateFill(fill domain.Fill) (domain.Fill, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(fill.ExchangeTradeID) != "" {
		for _, item := range s.fills {
			if item.OrderID == fill.OrderID && strings.EqualFold(strings.TrimSpace(item.ExchangeTradeID), strings.TrimSpace(fill.ExchangeTradeID)) {
				return item, nil
			}
		}
	} else {
		fill.DedupFingerprint = strings.TrimSpace(fill.DedupFingerprint)
		if fill.DedupFingerprint == "" {
			fill.DedupFingerprint = fill.FallbackFingerprint()
		}
		for _, item := range s.fills {
			if item.OrderID == fill.OrderID && item.DedupFingerprint != "" && item.DedupFingerprint == fill.DedupFingerprint {
				return item, nil
			}
		}
	}
	fill.ID = s.nextID("fill")
	fill.CreatedAt = time.Now().UTC()
	if fill.ExchangeTradeTime != nil {
		resolved := fill.ExchangeTradeTime.UTC()
		fill.ExchangeTradeTime = &resolved
	}
	s.fills[fill.ID] = fill
	return fill, nil
}

func (s *Store) ListPositions() ([]domain.Position, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.Position, 0, len(s.positions))
	for _, item := range s.positions {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.Before(items[j].UpdatedAt) })
	return items, nil
}

func (s *Store) FindPosition(accountID, symbol string) (domain.Position, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.positions {
		if item.AccountID == accountID && item.Symbol == symbol {
			return item, true, nil
		}
	}
	return domain.Position{}, false, nil
}

func (s *Store) SavePosition(position domain.Position) (domain.Position, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if position.ID == "" {
		position.ID = s.nextID("position")
	}
	position.UpdatedAt = time.Now().UTC()
	s.positions[position.ID] = position
	return position, nil
}

func (s *Store) DeletePosition(positionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.positions, positionID)
	return nil
}

func (s *Store) ListBacktests() ([]domain.BacktestRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.BacktestRun, 0, len(s.backtests))
	for _, item := range s.backtests {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items, nil
}

func (s *Store) CreateBacktest(strategyVersionID string, parameters map[string]any) (domain.BacktestRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	backtest := domain.BacktestRun{
		ID:                s.nextID("backtest"),
		StrategyVersionID: strategyVersionID,
		Status:            "QUEUED",
		Parameters:        parameters,
		ResultSummary: map[string]any{
			"return":      0,
			"maxDrawdown": 0,
			"tradePairs":  0,
		},
		CreatedAt: time.Now().UTC(),
	}
	s.backtests[backtest.ID] = backtest
	return backtest, nil
}

func (s *Store) UpdateBacktest(backtest domain.BacktestRun) (domain.BacktestRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backtests[backtest.ID] = backtest
	return backtest, nil
}

func (s *Store) ListPaperSessions() ([]domain.PaperSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.PaperSession, 0, len(s.paperSessions))
	for _, item := range s.paperSessions {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items, nil
}

func (s *Store) GetPaperSession(sessionID string) (domain.PaperSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.paperSessions[sessionID]
	if !ok {
		return domain.PaperSession{}, fmt.Errorf("paper session not found: %s", sessionID)
	}
	return item, nil
}

func (s *Store) CreatePaperSession(accountID, strategyID string, startEquity float64) (domain.PaperSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextID("paper-session")
	item := domain.PaperSession{
		ID:          id,
		AccountID:   accountID,
		StrategyID:  strategyID,
		Status:      "READY",
		StartEquity: startEquity,
		State: map[string]any{
			"runner":      "strategy-engine",
			"runtimeMode": "canonical-strategy-engine",
			"planIndex":   0,
		},
		CreatedAt: time.Now().UTC(),
	}
	s.paperSessions[id] = item
	return item, nil
}

func (s *Store) UpdatePaperSessionStatus(sessionID, status string) (domain.PaperSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.paperSessions[sessionID]
	if !ok {
		return domain.PaperSession{}, fmt.Errorf("paper session not found: %s", sessionID)
	}
	item.Status = status
	s.paperSessions[sessionID] = item
	return item, nil
}

func (s *Store) UpdatePaperSessionState(sessionID string, state map[string]any) (domain.PaperSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.paperSessions[sessionID]
	if !ok {
		return domain.PaperSession{}, fmt.Errorf("paper session not found: %s", sessionID)
	}
	item.State = state
	s.paperSessions[sessionID] = item
	return item, nil
}

func (s *Store) ListLiveSessions() ([]domain.LiveSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.LiveSession, 0, len(s.liveSessions))
	for _, item := range s.liveSessions {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items, nil
}

func (s *Store) GetLiveSession(sessionID string) (domain.LiveSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.liveSessions[sessionID]
	if !ok {
		return domain.LiveSession{}, fmt.Errorf("live session not found: %s", sessionID)
	}
	return item, nil
}

func (s *Store) CreateLiveSession(accountID, strategyID string) (domain.LiveSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextID("live-session")
	item := domain.LiveSession{
		ID:         id,
		AccountID:  accountID,
		StrategyID: strategyID,
		Status:     "READY",
		State: map[string]any{
			"runner":       "strategy-engine",
			"dispatchMode": "manual-review",
			"planIndex":    0,
		},
		CreatedAt: time.Now().UTC(),
	}
	s.liveSessions[id] = item
	return item, nil
}

func (s *Store) UpdateLiveSession(session domain.LiveSession) (domain.LiveSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.liveSessions[session.ID]; !ok {
		return domain.LiveSession{}, fmt.Errorf("live session not found: %s", session.ID)
	}
	s.liveSessions[session.ID] = session
	return session, nil
}

func (s *Store) DeleteLiveSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.liveSessions[sessionID]; !ok {
		return fmt.Errorf("live session not found: %s", sessionID)
	}
	delete(s.liveSessions, sessionID)
	return nil
}

func (s *Store) UpdateLiveSessionStatus(sessionID, status string) (domain.LiveSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.liveSessions[sessionID]
	if !ok {
		return domain.LiveSession{}, fmt.Errorf("live session not found: %s", sessionID)
	}
	item.Status = status
	s.liveSessions[sessionID] = item
	return item, nil
}

func (s *Store) UpdateLiveSessionState(sessionID string, state map[string]any) (domain.LiveSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.liveSessions[sessionID]
	if !ok {
		return domain.LiveSession{}, fmt.Errorf("live session not found: %s", sessionID)
	}
	item.State = state
	s.liveSessions[sessionID] = item
	return item, nil
}

func (s *Store) ListAccountEquitySnapshots(accountID string) ([]domain.AccountEquitySnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := append([]domain.AccountEquitySnapshot(nil), s.equitySnapshots[accountID]...)
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items, nil
}

func (s *Store) CreateAccountEquitySnapshot(snapshot domain.AccountEquitySnapshot) (domain.AccountEquitySnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot.ID = s.nextID("equity-snapshot")
	snapshot.CreatedAt = time.Now().UTC()
	s.equitySnapshots[snapshot.AccountID] = append(s.equitySnapshots[snapshot.AccountID], snapshot)
	return snapshot, nil
}

func (s *Store) ListStrategyDecisionEvents(liveSessionID string) ([]domain.StrategyDecisionEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.StrategyDecisionEvent, 0, len(s.decisionEvents))
	for _, item := range s.decisionEvents {
		if liveSessionID != "" && item.LiveSessionID != liveSessionID {
			continue
		}
		items = append(items, cloneJSONValue(item))
	}
	sort.Slice(items, func(i, j int) bool {
		return domain.EventLessAsc(items[i].EventTime, items[i].RecordedAt, items[i].ID, items[j].EventTime, items[j].RecordedAt, items[j].ID)
	})
	return items, nil
}

func (s *Store) CreateStrategyDecisionEvent(event domain.StrategyDecisionEvent) (domain.StrategyDecisionEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if event.ID == "" {
		event.ID = s.nextID("strategy-decision-event")
	}
	if event.EventTime.IsZero() {
		event.EventTime = time.Now().UTC()
	}
	if event.RecordedAt.IsZero() {
		event.RecordedAt = time.Now().UTC()
	}
	event = cloneJSONValue(event)
	s.decisionEvents = append(s.decisionEvents, event)
	return cloneJSONValue(event), nil
}

func (s *Store) ListOrderExecutionEvents(orderID string) ([]domain.OrderExecutionEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.OrderExecutionEvent, 0, len(s.executionEvents))
	for _, item := range s.executionEvents {
		if orderID != "" && item.OrderID != orderID {
			continue
		}
		items = append(items, cloneJSONValue(item))
	}
	sort.Slice(items, func(i, j int) bool {
		return domain.EventLessAsc(items[i].EventTime, items[i].RecordedAt, items[i].ID, items[j].EventTime, items[j].RecordedAt, items[j].ID)
	})
	return items, nil
}

func (s *Store) CreateOrderExecutionEvent(event domain.OrderExecutionEvent) (domain.OrderExecutionEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if event.ID == "" {
		event.ID = s.nextID("order-execution-event")
	}
	if event.EventTime.IsZero() {
		event.EventTime = time.Now().UTC()
	}
	if event.RecordedAt.IsZero() {
		event.RecordedAt = time.Now().UTC()
	}
	event = cloneJSONValue(event)
	s.executionEvents = append(s.executionEvents, event)
	return cloneJSONValue(event), nil
}

func (s *Store) ListPositionAccountSnapshots(accountID string) ([]domain.PositionAccountSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.PositionAccountSnapshot, 0, len(s.liveSnapshots))
	for _, item := range s.liveSnapshots {
		if accountID != "" && item.AccountID != accountID {
			continue
		}
		items = append(items, cloneJSONValue(item))
	}
	sort.Slice(items, func(i, j int) bool {
		return domain.EventLessAsc(items[i].EventTime, items[i].RecordedAt, items[i].ID, items[j].EventTime, items[j].RecordedAt, items[j].ID)
	})
	return items, nil
}

func (s *Store) CreatePositionAccountSnapshot(snapshot domain.PositionAccountSnapshot) (domain.PositionAccountSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if snapshot.ID == "" {
		snapshot.ID = s.nextID("position-account-snapshot")
	}
	if snapshot.EventTime.IsZero() {
		snapshot.EventTime = time.Now().UTC()
	}
	if snapshot.RecordedAt.IsZero() {
		snapshot.RecordedAt = time.Now().UTC()
	}
	snapshot = cloneJSONValue(snapshot)
	s.liveSnapshots = append(s.liveSnapshots, snapshot)
	return cloneJSONValue(snapshot), nil
}

func (s *Store) QueryStrategyDecisionEvents(query domain.StrategyDecisionEventQuery) ([]domain.StrategyDecisionEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.StrategyDecisionEvent, 0, len(s.decisionEvents))
	for _, item := range s.decisionEvents {
		if strings.TrimSpace(query.LiveSessionID) != "" && item.LiveSessionID != strings.TrimSpace(query.LiveSessionID) {
			continue
		}
		if strings.TrimSpace(query.AccountID) != "" && item.AccountID != strings.TrimSpace(query.AccountID) {
			continue
		}
		if strings.TrimSpace(query.StrategyID) != "" && item.StrategyID != strings.TrimSpace(query.StrategyID) {
			continue
		}
		if strings.TrimSpace(query.RuntimeSessionID) != "" && item.RuntimeSessionID != strings.TrimSpace(query.RuntimeSessionID) {
			continue
		}
		if strings.TrimSpace(query.DecisionEventID) != "" && item.ID != strings.TrimSpace(query.DecisionEventID) {
			continue
		}
		if !query.From.IsZero() && item.EventTime.Before(query.From.UTC()) {
			continue
		}
		if !query.To.IsZero() && item.EventTime.After(query.To.UTC()) {
			continue
		}
		if query.Before != nil && !domain.EventBeforeCursor(item.EventTime, item.RecordedAt, item.ID, *query.Before) {
			continue
		}
		items = append(items, cloneJSONValue(item))
	}
	sort.SliceStable(items, func(i, j int) bool {
		return domain.EventLessDesc(items[i].EventTime, items[i].RecordedAt, items[i].ID, items[j].EventTime, items[j].RecordedAt, items[j].ID)
	})
	if query.Limit > 0 && len(items) > query.Limit {
		items = items[:query.Limit]
	}
	return items, nil
}

func (s *Store) QueryOrderExecutionEvents(query domain.OrderExecutionEventQuery) ([]domain.OrderExecutionEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.OrderExecutionEvent, 0, len(s.executionEvents))
	for _, item := range s.executionEvents {
		if strings.TrimSpace(query.AccountID) != "" && item.AccountID != strings.TrimSpace(query.AccountID) {
			continue
		}
		if strings.TrimSpace(query.StrategyID) != "" && stringValue(item.Metadata["strategyId"]) != strings.TrimSpace(query.StrategyID) {
			continue
		}
		if strings.TrimSpace(query.RuntimeSessionID) != "" && item.RuntimeSessionID != strings.TrimSpace(query.RuntimeSessionID) {
			continue
		}
		if strings.TrimSpace(query.LiveSessionID) != "" && item.LiveSessionID != strings.TrimSpace(query.LiveSessionID) {
			continue
		}
		if strings.TrimSpace(query.OrderID) != "" && item.OrderID != strings.TrimSpace(query.OrderID) {
			continue
		}
		if strings.TrimSpace(query.DecisionEventID) != "" && item.DecisionEventID != strings.TrimSpace(query.DecisionEventID) {
			continue
		}
		if !query.From.IsZero() && item.EventTime.Before(query.From.UTC()) {
			continue
		}
		if !query.To.IsZero() && item.EventTime.After(query.To.UTC()) {
			continue
		}
		if query.Before != nil && !domain.EventBeforeCursor(item.EventTime, item.RecordedAt, item.ID, *query.Before) {
			continue
		}
		items = append(items, cloneJSONValue(item))
	}
	sort.SliceStable(items, func(i, j int) bool {
		return domain.EventLessDesc(items[i].EventTime, items[i].RecordedAt, items[i].ID, items[j].EventTime, items[j].RecordedAt, items[j].ID)
	})
	if query.Limit > 0 && len(items) > query.Limit {
		items = items[:query.Limit]
	}
	return items, nil
}

func (s *Store) QueryPositionAccountSnapshots(query domain.PositionAccountSnapshotQuery) ([]domain.PositionAccountSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.PositionAccountSnapshot, 0, len(s.liveSnapshots))
	for _, item := range s.liveSnapshots {
		if strings.TrimSpace(query.AccountID) != "" && item.AccountID != strings.TrimSpace(query.AccountID) {
			continue
		}
		if strings.TrimSpace(query.StrategyID) != "" && item.StrategyID != strings.TrimSpace(query.StrategyID) {
			continue
		}
		if strings.TrimSpace(query.LiveSessionID) != "" && item.LiveSessionID != strings.TrimSpace(query.LiveSessionID) {
			continue
		}
		if strings.TrimSpace(query.OrderID) != "" && item.OrderID != strings.TrimSpace(query.OrderID) {
			continue
		}
		if strings.TrimSpace(query.DecisionEventID) != "" && item.DecisionEventID != strings.TrimSpace(query.DecisionEventID) {
			continue
		}
		if !query.From.IsZero() && item.EventTime.Before(query.From.UTC()) {
			continue
		}
		if !query.To.IsZero() && item.EventTime.After(query.To.UTC()) {
			continue
		}
		if query.Before != nil && !domain.EventBeforeCursor(item.EventTime, item.RecordedAt, item.ID, *query.Before) {
			continue
		}
		items = append(items, cloneJSONValue(item))
	}
	sort.SliceStable(items, func(i, j int) bool {
		return domain.EventLessDesc(items[i].EventTime, items[i].RecordedAt, items[i].ID, items[j].EventTime, items[j].RecordedAt, items[j].ID)
	})
	if query.Limit > 0 && len(items) > query.Limit {
		items = items[:query.Limit]
	}
	return items, nil
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func (s *Store) ListMarketBars(exchange, symbol, timeframe string, from, to int64, limit int) ([]domain.MarketBar, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.MarketBar, 0)
	normalizedExchange := strings.ToUpper(strings.TrimSpace(exchange))
	normalizedSymbol := strings.ToUpper(strings.TrimSpace(symbol))
	normalizedTimeframe := strings.ToLower(strings.TrimSpace(timeframe))
	var startTime time.Time
	var endTime time.Time
	if from > 0 {
		startTime = time.Unix(from, 0).UTC()
	}
	if to > 0 {
		endTime = time.Unix(to, 0).UTC()
	}
	for _, item := range s.marketBars {
		if normalizedExchange != "" && strings.ToUpper(strings.TrimSpace(item.Exchange)) != normalizedExchange {
			continue
		}
		if normalizedSymbol != "" && strings.ToUpper(strings.TrimSpace(item.Symbol)) != normalizedSymbol {
			continue
		}
		if normalizedTimeframe != "" && strings.ToLower(strings.TrimSpace(item.Timeframe)) != normalizedTimeframe {
			continue
		}
		if !startTime.IsZero() && item.OpenTime.Before(startTime) {
			continue
		}
		if !endTime.IsZero() && item.OpenTime.After(endTime) {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].OpenTime.Before(items[j].OpenTime) })
	if limit > 0 && len(items) > limit {
		items = items[len(items)-limit:]
	}
	return items, nil
}

func (s *Store) UpsertMarketBars(bars []domain.MarketBar) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for _, item := range bars {
		if strings.TrimSpace(item.Exchange) == "" || strings.TrimSpace(item.Symbol) == "" || strings.TrimSpace(item.Timeframe) == "" || item.OpenTime.IsZero() {
			continue
		}
		if item.ID == "" {
			item.ID = marketBarMemoryKey(item.Exchange, item.Symbol, item.Timeframe, item.OpenTime)
		}
		if item.UpdatedAt.IsZero() {
			item.UpdatedAt = now
		}
		s.marketBars[item.ID] = item
	}
	return nil
}

func marketBarMemoryKey(exchange, symbol, timeframe string, openTime time.Time) string {
	return strings.ToUpper(strings.TrimSpace(exchange)) + "|" +
		strings.ToUpper(strings.TrimSpace(symbol)) + "|" +
		strings.ToLower(strings.TrimSpace(timeframe)) + "|" +
		openTime.UTC().Format(time.RFC3339)
}

func cloneJSONValue[T any](value T) T {
	raw, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var cloned T
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return value
	}
	return cloned
}

func accountStatusForMode(mode string) string {
	if mode == "LIVE" {
		return "PENDING_SETUP"
	}
	return "READY"
}

func (s *Store) ListAnnotations(symbol string) []domain.ChartAnnotation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.ChartAnnotation, 0, len(s.annotations))
	for _, item := range s.annotations {
		if symbol == "" || item.Symbol == symbol {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Time.Before(items[j].Time) })
	return items
}
