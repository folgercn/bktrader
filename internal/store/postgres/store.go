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

func (s *Store) GetRuntimePolicy() (domain.RuntimePolicy, bool, error) {
	var item domain.RuntimePolicy
	err := s.db.QueryRow(`
		select trade_tick_freshness_seconds, order_book_freshness_seconds, signal_bar_freshness_seconds, runtime_quiet_seconds, paper_start_readiness_timeout_seconds, updated_at
		from runtime_policies
		where id = 1
	`).Scan(
		&item.TradeTickFreshnessSeconds,
		&item.OrderBookFreshnessSeconds,
		&item.SignalBarFreshnessSeconds,
		&item.RuntimeQuietSeconds,
		&item.PaperStartReadinessTimeoutSecs,
		&item.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.RuntimePolicy{}, false, nil
		}
		return domain.RuntimePolicy{}, false, err
	}
	return item, true, nil
}

func (s *Store) UpsertRuntimePolicy(policy domain.RuntimePolicy) (domain.RuntimePolicy, error) {
	policy.UpdatedAt = time.Now().UTC()
	_, err := s.db.Exec(`
		insert into runtime_policies (
			id,
			trade_tick_freshness_seconds,
			order_book_freshness_seconds,
			signal_bar_freshness_seconds,
			runtime_quiet_seconds,
			paper_start_readiness_timeout_seconds,
			updated_at
		)
		values (1, $1, $2, $3, $4, $5, $6)
		on conflict (id) do update set
			trade_tick_freshness_seconds = excluded.trade_tick_freshness_seconds,
			order_book_freshness_seconds = excluded.order_book_freshness_seconds,
			signal_bar_freshness_seconds = excluded.signal_bar_freshness_seconds,
			runtime_quiet_seconds = excluded.runtime_quiet_seconds,
			paper_start_readiness_timeout_seconds = excluded.paper_start_readiness_timeout_seconds,
			updated_at = excluded.updated_at
	`,
		policy.TradeTickFreshnessSeconds,
		policy.OrderBookFreshnessSeconds,
		policy.SignalBarFreshnessSeconds,
		policy.RuntimeQuietSeconds,
		policy.PaperStartReadinessTimeoutSecs,
		policy.UpdatedAt,
	)
	if err != nil {
		return domain.RuntimePolicy{}, err
	}
	return policy, nil
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

func (s *Store) UpdateStrategyParameters(strategyID string, parameters map[string]any) (map[string]any, error) {
	items, err := s.ListStrategies()
	if err != nil {
		return nil, err
	}

	var current domain.StrategyVersion
	var strategy map[string]any
	found := false
	for _, item := range items {
		if fmt.Sprint(item["id"]) != strategyID {
			continue
		}
		current, _ = item["currentVersion"].(domain.StrategyVersion)
		strategy = item
		found = true
		break
	}
	if !found {
		return nil, fmt.Errorf("strategy not found: %s", strategyID)
	}

	raw, _ := json.Marshal(parameters)
	if _, err := s.db.Exec(`
		update strategy_versions
		set parameters = $2
		where id = $1
	`, current.ID, raw); err != nil {
		return nil, err
	}

	current.Parameters = parameters
	strategy["currentVersion"] = current
	return strategy, nil
}

func (s *Store) ListAccounts() ([]domain.Account, error) {
	rows, err := s.db.Query(`select id, name, mode, exchange, status, metadata, created_at from accounts order by created_at asc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.Account{}
	for rows.Next() {
		var item domain.Account
		var metadataRaw []byte
		if err := rows.Scan(&item.ID, &item.Name, &item.Mode, &item.Exchange, &item.Status, &metadataRaw, &item.CreatedAt); err != nil {
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

func (s *Store) CreateAccount(name, mode, exchange string) (domain.Account, error) {
	item := domain.Account{
		ID:        fmt.Sprintf("account-%d", time.Now().UTC().UnixNano()),
		Name:      name,
		Mode:      mode,
		Exchange:  exchange,
		Status:    accountStatusForMode(mode),
		Metadata:  map[string]any{},
		CreatedAt: time.Now().UTC(),
	}
	raw, _ := json.Marshal(item.Metadata)

	_, err := s.db.Exec(`
		insert into accounts (id, name, mode, exchange, status, metadata, created_at)
		values ($1, $2, $3, $4, $5, $6, $7)
	`, item.ID, item.Name, item.Mode, item.Exchange, item.Status, raw, item.CreatedAt)
	return item, err
}

func (s *Store) GetAccount(accountID string) (domain.Account, error) {
	var item domain.Account
	var metadataRaw []byte
	err := s.db.QueryRow(`
		select id, name, mode, exchange, status, metadata, created_at
		from accounts
		where id = $1
	`, accountID).Scan(&item.ID, &item.Name, &item.Mode, &item.Exchange, &item.Status, &metadataRaw, &item.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.Account{}, fmt.Errorf("account not found: %s", accountID)
		}
		return domain.Account{}, err
	}
	item.Metadata = map[string]any{}
	if len(metadataRaw) > 0 {
		_ = json.Unmarshal(metadataRaw, &item.Metadata)
	}
	return item, nil
}

func (s *Store) UpdateAccount(account domain.Account) (domain.Account, error) {
	if account.Metadata == nil {
		account.Metadata = map[string]any{}
	}
	raw, _ := json.Marshal(account.Metadata)
	_, err := s.db.Exec(`
		update accounts
		set name = $2,
			mode = $3,
			exchange = $4,
			status = $5,
			metadata = $6
		where id = $1
	`, account.ID, account.Name, account.Mode, account.Exchange, account.Status, raw)
	if err != nil {
		return domain.Account{}, err
	}
	return s.GetAccount(account.ID)
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
			item              domain.Order
			strategyVersionID sql.NullString
			metadataRaw       []byte
		)
		if err := rows.Scan(&item.ID, &item.AccountID, &strategyVersionID, &item.Symbol, &item.Side, &item.Type, &item.Status, &item.Quantity, &item.Price, &metadataRaw, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.StrategyVersionID = strategyVersionID.String
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

func (s *Store) UpdateOrder(order domain.Order) (domain.Order, error) {
	if order.Metadata == nil {
		order.Metadata = map[string]any{}
	}
	raw, _ := json.Marshal(order.Metadata)
	_, err := s.db.Exec(`
		update orders
		set account_id = $2,
			strategy_version_id = $3,
			symbol = $4,
			side = $5,
			type = $6,
			status = $7,
			quantity = $8,
			price = $9,
			metadata = $10
		where id = $1
	`, order.ID, order.AccountID, nullIfEmpty(order.StrategyVersionID), order.Symbol, order.Side, order.Type, order.Status, order.Quantity, order.Price, raw)
	return order, err
}

func (s *Store) ListFills() ([]domain.Fill, error) {
	rows, err := s.db.Query(`
		select id, order_id, price, quantity, fee, created_at
		from fills order by created_at asc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.Fill{}
	for rows.Next() {
		var item domain.Fill
		if err := rows.Scan(&item.ID, &item.OrderID, &item.Price, &item.Quantity, &item.Fee, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateFill(fill domain.Fill) (domain.Fill, error) {
	fill.ID = fmt.Sprintf("fill-%d", time.Now().UTC().UnixNano())
	fill.CreatedAt = time.Now().UTC()

	_, err := s.db.Exec(`
		insert into fills (id, order_id, price, quantity, fee, created_at)
		values ($1, $2, $3, $4, $5, $6)
	`, fill.ID, fill.OrderID, fill.Price, fill.Quantity, fill.Fee, fill.CreatedAt)
	return fill, err
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
		var (
			item              domain.Position
			strategyVersionID sql.NullString
		)
		if err := rows.Scan(&item.ID, &item.AccountID, &strategyVersionID, &item.Symbol, &item.Side, &item.Quantity, &item.EntryPrice, &item.MarkPrice, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.StrategyVersionID = strategyVersionID.String
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) FindPosition(accountID, symbol string) (domain.Position, bool, error) {
	var item domain.Position
	var strategyVersionID sql.NullString
	err := s.db.QueryRow(`
		select id, account_id, strategy_version_id, symbol, side, quantity, entry_price, mark_price, updated_at
		from positions
		where account_id = $1 and symbol = $2
		order by updated_at desc
		limit 1
	`, accountID, symbol).Scan(&item.ID, &item.AccountID, &strategyVersionID, &item.Symbol, &item.Side, &item.Quantity, &item.EntryPrice, &item.MarkPrice, &item.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.Position{}, false, nil
		}
		return domain.Position{}, false, err
	}
	item.StrategyVersionID = strategyVersionID.String
	return item, true, nil
}

func (s *Store) SavePosition(position domain.Position) (domain.Position, error) {
	now := time.Now().UTC()
	position.UpdatedAt = now
	if position.ID == "" {
		position.ID = fmt.Sprintf("position-%d", now.UnixNano())
		_, err := s.db.Exec(`
			insert into positions (id, account_id, strategy_version_id, symbol, side, quantity, entry_price, mark_price, updated_at)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, position.ID, position.AccountID, nullIfEmpty(position.StrategyVersionID), position.Symbol, position.Side, position.Quantity, position.EntryPrice, position.MarkPrice, position.UpdatedAt)
		return position, err
	}

	_, err := s.db.Exec(`
		update positions
		set account_id = $2,
			strategy_version_id = $3,
			symbol = $4,
			side = $5,
			quantity = $6,
			entry_price = $7,
			mark_price = $8,
			updated_at = $9
		where id = $1
	`, position.ID, position.AccountID, nullIfEmpty(position.StrategyVersionID), position.Symbol, position.Side, position.Quantity, position.EntryPrice, position.MarkPrice, position.UpdatedAt)
	return position, err
}

func (s *Store) DeletePosition(positionID string) error {
	_, err := s.db.Exec(`delete from positions where id = $1`, positionID)
	return err
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

func (s *Store) UpdateBacktest(backtest domain.BacktestRun) (domain.BacktestRun, error) {
	parametersRaw, _ := json.Marshal(backtest.Parameters)
	summaryRaw, _ := json.Marshal(backtest.ResultSummary)
	_, err := s.db.Exec(`
		update backtest_runs
		set strategy_version_id = $2,
			status = $3,
			parameters = $4,
			result_summary = $5,
			created_at = $6
		where id = $1
	`, backtest.ID, backtest.StrategyVersionID, backtest.Status, parametersRaw, summaryRaw, backtest.CreatedAt)
	if err != nil {
		return domain.BacktestRun{}, err
	}
	return backtest, nil
}

func (s *Store) ListPaperSessions() ([]domain.PaperSession, error) {
	rows, err := s.db.Query(`
		select id, account_id, strategy_id, status, start_equity, state, created_at
		from paper_sessions order by created_at asc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.PaperSession{}
	for rows.Next() {
		var item domain.PaperSession
		var stateRaw []byte
		if err := rows.Scan(&item.ID, &item.AccountID, &item.StrategyID, &item.Status, &item.StartEquity, &stateRaw, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.State = map[string]any{}
		if len(stateRaw) > 0 {
			_ = json.Unmarshal(stateRaw, &item.State)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetPaperSession(sessionID string) (domain.PaperSession, error) {
	var item domain.PaperSession
	var stateRaw []byte
	err := s.db.QueryRow(`
		select id, account_id, strategy_id, status, start_equity, state, created_at
		from paper_sessions
		where id = $1
	`, sessionID).Scan(&item.ID, &item.AccountID, &item.StrategyID, &item.Status, &item.StartEquity, &stateRaw, &item.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.PaperSession{}, fmt.Errorf("paper session not found: %s", sessionID)
		}
		return domain.PaperSession{}, err
	}
	item.State = map[string]any{}
	if len(stateRaw) > 0 {
		_ = json.Unmarshal(stateRaw, &item.State)
	}
	return item, nil
}

func (s *Store) CreatePaperSession(accountID, strategyID string, startEquity float64) (domain.PaperSession, error) {
	item := domain.PaperSession{
		ID:          fmt.Sprintf("paper-session-%d", time.Now().UTC().UnixNano()),
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
	stateRaw, _ := json.Marshal(item.State)

	_, err := s.db.Exec(`
		insert into paper_sessions (id, account_id, strategy_id, status, start_equity, state, created_at)
		values ($1, $2, $3, $4, $5, $6, $7)
	`, item.ID, item.AccountID, item.StrategyID, item.Status, item.StartEquity, stateRaw, item.CreatedAt)
	return item, err
}

func (s *Store) UpdatePaperSessionStatus(sessionID, status string) (domain.PaperSession, error) {
	_, err := s.db.Exec(`update paper_sessions set status = $2 where id = $1`, sessionID, status)
	if err != nil {
		return domain.PaperSession{}, err
	}
	return s.GetPaperSession(sessionID)
}

func (s *Store) UpdatePaperSessionState(sessionID string, state map[string]any) (domain.PaperSession, error) {
	stateRaw, _ := json.Marshal(state)
	_, err := s.db.Exec(`update paper_sessions set state = $2 where id = $1`, sessionID, stateRaw)
	if err != nil {
		return domain.PaperSession{}, err
	}
	return s.GetPaperSession(sessionID)
}

func (s *Store) ListAccountEquitySnapshots(accountID string) ([]domain.AccountEquitySnapshot, error) {
	rows, err := s.db.Query(`
		select id, account_id, start_equity, realized_pnl, unrealized_pnl, fees, net_equity, exposure_notional, open_position_count, created_at
		from account_equity_snapshots
		where account_id = $1
		order by created_at asc
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.AccountEquitySnapshot{}
	for rows.Next() {
		var item domain.AccountEquitySnapshot
		if err := rows.Scan(&item.ID, &item.AccountID, &item.StartEquity, &item.RealizedPnL, &item.UnrealizedPnL, &item.Fees, &item.NetEquity, &item.ExposureNotional, &item.OpenPositionCount, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateAccountEquitySnapshot(snapshot domain.AccountEquitySnapshot) (domain.AccountEquitySnapshot, error) {
	snapshot.ID = fmt.Sprintf("equity-snapshot-%d", time.Now().UTC().UnixNano())
	snapshot.CreatedAt = time.Now().UTC()
	_, err := s.db.Exec(`
		insert into account_equity_snapshots (
			id, account_id, start_equity, realized_pnl, unrealized_pnl, fees, net_equity, exposure_notional, open_position_count, created_at
		) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, snapshot.ID, snapshot.AccountID, snapshot.StartEquity, snapshot.RealizedPnL, snapshot.UnrealizedPnL, snapshot.Fees, snapshot.NetEquity, snapshot.ExposureNotional, snapshot.OpenPositionCount, snapshot.CreatedAt)
	return snapshot, err
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func accountStatusForMode(mode string) string {
	if mode == "LIVE" {
		return "PENDING_SETUP"
	}
	return "READY"
}
