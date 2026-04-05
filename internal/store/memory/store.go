package memory

import (
	"fmt"
	"sort"
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
	equitySnapshots  map[string][]domain.AccountEquitySnapshot
	signalSources    []map[string]any
	annotations      []domain.ChartAnnotation
	runtimePolicy    *domain.RuntimePolicy
	notificationAcks map[string]domain.NotificationAck
	telegramConfig   *domain.TelegramConfig

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
		equitySnapshots: make(map[string][]domain.AccountEquitySnapshot),
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
			"strategyEngine":       "bk-default",
			"maxTradesPerBar":      3,
			"reentrySizes":         []float64{0.10, 0.20},
			"stopMode":             "atr",
			"stopLossATR":          0.05,
			"profitProtectATR":     1.0,
			"tradingFeeBps":        10.0,
			"fundingRateBps":       0.0,
			"fundingIntervalHours": 8,
		},
		CreatedAt: now,
	}
	store.strategies[strategy.ID] = strategy
	store.strategyVersion[version.ID] = version

	paper := domain.Account{
		ID:        "paper-main",
		Name:      "Paper Main",
		Mode:      "PAPER",
		Exchange:  "binance-futures",
		Status:    "READY",
		Metadata:  map[string]any{},
		CreatedAt: now,
	}
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
	store.accounts[paper.ID] = paper
	store.accounts[live.ID] = live

	order := domain.Order{
		ID:                "sample-order-1",
		AccountID:         paper.ID,
		StrategyVersionID: version.ID,
		Symbol:            "BTCUSDT",
		Side:              "BUY",
		Type:              "MARKET",
		Status:            "FILLED",
		Quantity:          0.01,
		Price:             68000.0,
		Metadata: map[string]any{
			"source":      "paper-trading",
			"entryReason": "SL-Reentry",
		},
		CreatedAt: now,
	}
	store.orders[order.ID] = order

	fill := domain.Fill{
		ID:        "sample-fill-1",
		OrderID:   order.ID,
		Price:     68000.0,
		Quantity:  0.01,
		Fee:       0.68,
		CreatedAt: now,
	}
	store.fills[fill.ID] = fill

	position := domain.Position{
		ID:                "position-paper-btcusdt",
		AccountID:         paper.ID,
		StrategyVersionID: version.ID,
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.01,
		EntryPrice:        68000.0,
		MarkPrice:         68420.0,
		UpdatedAt:         now,
	}
	store.positions[position.ID] = position

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

	store.paperSessions["paper-session-main"] = domain.PaperSession{
		ID:          "paper-session-main",
		AccountID:   paper.ID,
		StrategyID:  strategy.ID,
		Status:      "READY",
		StartEquity: 100000.0,
		State: map[string]any{
			"runner":      "strategy-engine",
			"runtimeMode": "canonical-strategy-engine",
			"planIndex":   0,
		},
		CreatedAt: now,
	}
	store.equitySnapshots[paper.ID] = []domain.AccountEquitySnapshot{
		{
			ID:                "equity-snapshot-main",
			AccountID:         paper.ID,
			StartEquity:       100000.0,
			RealizedPnL:       0,
			UnrealizedPnL:     4.2,
			Fees:              0.68,
			NetEquity:         100003.52,
			ExposureNotional:  684.2,
			OpenPositionCount: 1,
			CreatedAt:         now,
		},
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
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items, nil
}

func (s *Store) CreateOrder(order domain.Order) (domain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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

func (s *Store) CreateFill(fill domain.Fill) (domain.Fill, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fill.ID = s.nextID("fill")
	fill.CreatedAt = time.Now().UTC()
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
