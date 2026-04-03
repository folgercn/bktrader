package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

type Store struct {
	db *sql.DB
}

func New(dsn string) (*Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) ListStrategies() ([]map[string]any, error) {
	rows, err := s.db.Query(`
		select
			s.id, s.name, s.status, s.description, s.created_at,
			sv.id, sv.version, sv.signal_timeframe, sv.execution_timeframe, sv.parameters, sv.created_at
		from strategies s
		left join strategy_versions sv on sv.strategy_id = s.id
		order by s.created_at asc, sv.created_at desc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type versionRow struct {
		ID                 string
		Version            string
		SignalTimeframe    string
		ExecutionTimeframe string
		Parameters         map[string]any
		CreatedAt          time.Time
	}

	items := map[string]map[string]any{}
	order := make([]string, 0)
	for rows.Next() {
		var (
			id, name, status string
			description      sql.NullString
			createdAt        time.Time
			versionID        sql.NullString
			version          sql.NullString
			signalTF         sql.NullString
			execTF           sql.NullString
			parametersRaw    []byte
			versionCreatedAt sql.NullTime
		)
		if err := rows.Scan(&id, &name, &status, &description, &createdAt, &versionID, &version, &signalTF, &execTF, &parametersRaw, &versionCreatedAt); err != nil {
			return nil, err
		}
		if _, ok := items[id]; !ok {
			items[id] = map[string]any{
				"id":          id,
				"name":        name,
				"status":      status,
				"description": description.String,
				"createdAt":   createdAt,
			}
			order = append(order, id)
		}
		if versionID.Valid {
			parameters := map[string]any{}
			if len(parametersRaw) > 0 {
				_ = json.Unmarshal(parametersRaw, &parameters)
			}
			items[id]["currentVersion"] = domain.StrategyVersion{
				ID:                 versionID.String,
				StrategyID:         id,
				Version:            version.String,
				SignalTimeframe:    signalTF.String,
				ExecutionTimeframe: execTF.String,
				Parameters:         parameters,
				CreatedAt:          versionCreatedAt.Time,
			}
		}
	}

	result := make([]map[string]any, 0, len(order))
	for _, id := range order {
		result = append(result, items[id])
	}
	return result, rows.Err()
}

func (s *Store) CreateStrategy(name, description string, parameters map[string]any) (map[string]any, error) {
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	strategyID := fmt.Sprintf("strategy-%d", now.UnixNano())
	versionID := fmt.Sprintf("strategy-version-%d", now.UnixNano())
	raw, _ := json.Marshal(parameters)

	if _, err := tx.ExecContext(ctx, `
		insert into strategies (id, name, status, description, created_at)
		values ($1, $2, $3, $4, $5)
	`, strategyID, name, "DRAFT", description, now); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		insert into strategy_versions (id, strategy_id, version, signal_timeframe, execution_timeframe, parameters, created_at)
		values ($1, $2, $3, $4, $5, $6, $7)
	`, versionID, strategyID, "v0.1.0", "1D", "1m", raw, now); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return map[string]any{
		"id":          strategyID,
		"name":        name,
		"status":      "DRAFT",
		"description": description,
		"createdAt":   now,
		"currentVersion": domain.StrategyVersion{
			ID:                 versionID,
			StrategyID:         strategyID,
			Version:            "v0.1.0",
			SignalTimeframe:    "1D",
			ExecutionTimeframe: "1m",
			Parameters:         parameters,
			CreatedAt:          now,
		},
	}, nil
}

func (s *Store) ListAccounts() ([]domain.Account, error) {
	rows, err := s.db.Query(`select id, name, mode, exchange, status, created_at from accounts order by created_at asc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.Account{}
	for rows.Next() {
		var item domain.Account
		if err := rows.Scan(&item.ID, &item.Name, &item.Mode, &item.Exchange, &item.Status, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateAccount(name, mode, exchange string) (domain.Account, error) {
	item := domain.Account{
		ID:        fmt.Sprintf("account-%d", time.Now().UTC().UnixNano()),
		Name:      name,
		Mode:      mode,
		Exchange:  exchange,
		Status:    "READY",
		CreatedAt: time.Now().UTC(),
	}

	_, err := s.db.Exec(`
		insert into accounts (id, name, mode, exchange, status, created_at)
		values ($1, $2, $3, $4, $5, $6)
	`, item.ID, item.Name, item.Mode, item.Exchange, item.Status, item.CreatedAt)
	return item, err
}

func (s *Store) ListOrders() ([]domain.Order, error) {
	rows, err := s.db.Query(`
		select id, account_id, strategy_version_id, symbol, side, type, status, quantity, price, metadata, created_at
		from orders order by created_at asc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.Order{}
	for rows.Next() {
		var (
			item        domain.Order
			metadataRaw []byte
		)
		if err := rows.Scan(&item.ID, &item.AccountID, &item.StrategyVersionID, &item.Symbol, &item.Side, &item.Type, &item.Status, &item.Quantity, &item.Price, &metadataRaw, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Metadata = map[string]any{}
		if len(metadataRaw) > 0 {
			_ = json.Unmarshal(metadataRaw, &item.Metadata)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateOrder(order domain.Order) (domain.Order, error) {
	order.ID = fmt.Sprintf("order-%d", time.Now().UTC().UnixNano())
	order.Status = "NEW"
	order.CreatedAt = time.Now().UTC()
	if order.Metadata == nil {
		order.Metadata = map[string]any{}
	}
	raw, _ := json.Marshal(order.Metadata)

	_, err := s.db.Exec(`
		insert into orders (id, account_id, strategy_version_id, symbol, side, type, status, quantity, price, metadata, created_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, order.ID, order.AccountID, nullIfEmpty(order.StrategyVersionID), order.Symbol, order.Side, order.Type, order.Status, order.Quantity, order.Price, raw, order.CreatedAt)
	return order, err
}

func (s *Store) ListPositions() ([]domain.Position, error) {
	rows, err := s.db.Query(`
		select id, account_id, strategy_version_id, symbol, side, quantity, entry_price, mark_price, updated_at
		from positions order by updated_at asc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.Position{}
	for rows.Next() {
		var item domain.Position
		if err := rows.Scan(&item.ID, &item.AccountID, &item.StrategyVersionID, &item.Symbol, &item.Side, &item.Quantity, &item.EntryPrice, &item.MarkPrice, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListBacktests() ([]domain.BacktestRun, error) {
	rows, err := s.db.Query(`
		select id, strategy_version_id, status, parameters, result_summary, created_at
		from backtest_runs order by created_at asc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.BacktestRun{}
	for rows.Next() {
		var (
			item          domain.BacktestRun
			parametersRaw []byte
			summaryRaw    []byte
		)
		if err := rows.Scan(&item.ID, &item.StrategyVersionID, &item.Status, &parametersRaw, &summaryRaw, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Parameters = map[string]any{}
		item.ResultSummary = map[string]any{}
		if len(parametersRaw) > 0 {
			_ = json.Unmarshal(parametersRaw, &item.Parameters)
		}
		if len(summaryRaw) > 0 {
			_ = json.Unmarshal(summaryRaw, &item.ResultSummary)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateBacktest(strategyVersionID string, parameters map[string]any) (domain.BacktestRun, error) {
	item := domain.BacktestRun{
		ID:                fmt.Sprintf("backtest-%d", time.Now().UTC().UnixNano()),
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
	parametersRaw, _ := json.Marshal(item.Parameters)
	summaryRaw, _ := json.Marshal(item.ResultSummary)

	_, err := s.db.Exec(`
		insert into backtest_runs (id, strategy_version_id, status, parameters, result_summary, created_at)
		values ($1, $2, $3, $4, $5, $6)
	`, item.ID, item.StrategyVersionID, item.Status, parametersRaw, summaryRaw, item.CreatedAt)
	return item, err
}

func (s *Store) ListPaperSessions() ([]map[string]any, error) {
	rows, err := s.db.Query(`
		select id, account_id, strategy_id, status, start_equity, created_at
		from paper_sessions order by created_at asc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []map[string]any{}
	for rows.Next() {
		var (
			id, accountID, strategyID, status string
			startEquity                       float64
			createdAt                         time.Time
		)
		if err := rows.Scan(&id, &accountID, &strategyID, &status, &startEquity, &createdAt); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"id":          id,
			"accountId":   accountID,
			"strategyId":  strategyID,
			"status":      status,
			"startEquity": startEquity,
			"createdAt":   createdAt,
		})
	}
	return items, rows.Err()
}

func (s *Store) CreatePaperSession(accountID, strategyID string, startEquity float64) (map[string]any, error) {
	id := fmt.Sprintf("paper-session-%d", time.Now().UTC().UnixNano())
	createdAt := time.Now().UTC()
	item := map[string]any{
		"id":          id,
		"accountId":   accountID,
		"strategyId":  strategyID,
		"status":      "RUNNING",
		"startEquity": startEquity,
		"createdAt":   createdAt,
	}

	_, err := s.db.Exec(`
		insert into paper_sessions (id, account_id, strategy_id, status, start_equity, created_at)
		values ($1, $2, $3, $4, $5, $6)
	`, id, accountID, strategyID, "RUNNING", startEquity, createdAt)
	return item, err
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}
