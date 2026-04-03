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

	strategies      map[string]domain.Strategy
	strategyVersion map[string]domain.StrategyVersion
	accounts        map[string]domain.Account
	orders          map[string]domain.Order
	positions       map[string]domain.Position
	backtests       map[string]domain.BacktestRun
	paperSessions   map[string]map[string]any
	signalSources   []map[string]any
	annotations     []domain.ChartAnnotation

	sequence int64
}

func NewStore() *Store {
	now := time.Now().UTC()
	store := &Store{
		strategies:      make(map[string]domain.Strategy),
		strategyVersion: make(map[string]domain.StrategyVersion),
		accounts:        make(map[string]domain.Account),
		orders:          make(map[string]domain.Order),
		positions:       make(map[string]domain.Position),
		backtests:       make(map[string]domain.BacktestRun),
		paperSessions:   make(map[string]map[string]any),
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
			"maxTradesPerBar":  3,
			"reentrySizes":     []float64{0.10, 0.20},
			"stopMode":         "atr",
			"stopLossATR":      0.05,
			"profitProtectATR": 1.0,
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
		CreatedAt: now,
	}
	live := domain.Account{
		ID:        "live-main",
		Name:      "Live Main",
		Mode:      "LIVE",
		Exchange:  "binance-futures",
		Status:    "PENDING_SETUP",
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

	store.paperSessions["paper-session-main"] = map[string]any{
		"id":          "paper-session-main",
		"accountId":   paper.ID,
		"strategyId":  strategy.ID,
		"status":      "RUNNING",
		"startEquity": 100000.0,
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

func (s *Store) ListStrategies() []map[string]any {
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
	return result
}

func (s *Store) CreateStrategy(name, description string, parameters map[string]any) map[string]any {
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
	}
}

func (s *Store) ListAccounts() []domain.Account {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.Account, 0, len(s.accounts))
	for _, item := range s.accounts {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (s *Store) CreateAccount(name, mode, exchange string) domain.Account {
	s.mu.Lock()
	defer s.mu.Unlock()
	account := domain.Account{
		ID:        s.nextID("account"),
		Name:      name,
		Mode:      mode,
		Exchange:  exchange,
		Status:    "READY",
		CreatedAt: time.Now().UTC(),
	}
	s.accounts[account.ID] = account
	return account
}

func (s *Store) ListOrders() []domain.Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.Order, 0, len(s.orders))
	for _, item := range s.orders {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (s *Store) CreateOrder(order domain.Order) domain.Order {
	s.mu.Lock()
	defer s.mu.Unlock()
	order.ID = s.nextID("order")
	order.Status = "NEW"
	order.CreatedAt = time.Now().UTC()
	if order.Metadata == nil {
		order.Metadata = map[string]any{}
	}
	s.orders[order.ID] = order
	return order
}

func (s *Store) ListPositions() []domain.Position {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.Position, 0, len(s.positions))
	for _, item := range s.positions {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.Before(items[j].UpdatedAt) })
	return items
}

func (s *Store) ListBacktests() []domain.BacktestRun {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.BacktestRun, 0, len(s.backtests))
	for _, item := range s.backtests {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (s *Store) CreateBacktest(strategyVersionID string, parameters map[string]any) domain.BacktestRun {
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
	return backtest
}

func (s *Store) ListPaperSessions() []map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]map[string]any, 0, len(s.paperSessions))
	for _, item := range s.paperSessions {
		items = append(items, item)
	}
	return items
}

func (s *Store) CreatePaperSession(accountID, strategyID string, startEquity float64) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextID("paper-session")
	item := map[string]any{
		"id":          id,
		"accountId":   accountID,
		"strategyId":  strategyID,
		"status":      "RUNNING",
		"startEquity": startEquity,
	}
	s.paperSessions[id] = item
	return item
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
