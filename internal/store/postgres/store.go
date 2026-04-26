package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/wuyaocheng/bktrader/internal/domain"
	storepkg "github.com/wuyaocheng/bktrader/internal/store"
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
		select trade_tick_freshness_seconds, order_book_freshness_seconds, signal_bar_freshness_seconds, runtime_quiet_seconds, strategy_evaluation_quiet_seconds, live_account_sync_freshness_seconds, paper_start_readiness_timeout_seconds, updated_at
		from runtime_policies
		where id = 1
	`).Scan(
		&item.TradeTickFreshnessSeconds,
		&item.OrderBookFreshnessSeconds,
		&item.SignalBarFreshnessSeconds,
		&item.RuntimeQuietSeconds,
		&item.StrategyEvaluationQuietSeconds,
		&item.LiveAccountSyncFreshnessSecs,
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
				strategy_evaluation_quiet_seconds,
				live_account_sync_freshness_seconds,
				paper_start_readiness_timeout_seconds,
				updated_at
			)
			values (1, $1, $2, $3, $4, $5, $6, $7, $8)
			on conflict (id) do update set
				trade_tick_freshness_seconds = excluded.trade_tick_freshness_seconds,
				order_book_freshness_seconds = excluded.order_book_freshness_seconds,
				signal_bar_freshness_seconds = excluded.signal_bar_freshness_seconds,
				runtime_quiet_seconds = excluded.runtime_quiet_seconds,
				strategy_evaluation_quiet_seconds = excluded.strategy_evaluation_quiet_seconds,
				live_account_sync_freshness_seconds = excluded.live_account_sync_freshness_seconds,
				paper_start_readiness_timeout_seconds = excluded.paper_start_readiness_timeout_seconds,
				updated_at = excluded.updated_at
		`,
		policy.TradeTickFreshnessSeconds,
		policy.OrderBookFreshnessSeconds,
		policy.SignalBarFreshnessSeconds,
		policy.RuntimeQuietSeconds,
		policy.StrategyEvaluationQuietSeconds,
		policy.LiveAccountSyncFreshnessSecs,
		policy.PaperStartReadinessTimeoutSecs,
		policy.UpdatedAt,
	)
	if err != nil {
		return domain.RuntimePolicy{}, err
	}
	return policy, nil
}

func (s *Store) ListNotificationAcks() ([]domain.NotificationAck, error) {
	rows, err := s.db.Query(`
		select id, acked_at, updated_at
		from notification_acks
		order by updated_at desc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.NotificationAck, 0)
	for rows.Next() {
		var item domain.NotificationAck
		if err := rows.Scan(&item.ID, &item.AckedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UpsertNotificationAck(id string) (domain.NotificationAck, error) {
	item := domain.NotificationAck{
		ID:        id,
		AckedAt:   time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	_, err := s.db.Exec(`
		insert into notification_acks (id, acked_at, updated_at)
		values ($1, $2, $3)
		on conflict (id) do update set
			acked_at = excluded.acked_at,
			updated_at = excluded.updated_at
	`, item.ID, item.AckedAt, item.UpdatedAt)
	if err != nil {
		return domain.NotificationAck{}, err
	}
	return item, nil
}

func (s *Store) DeleteNotificationAck(id string) error {
	_, err := s.db.Exec(`delete from notification_acks where id = $1`, id)
	return err
}

func (s *Store) GetTelegramConfig() (domain.TelegramConfig, bool, error) {
	var item domain.TelegramConfig
	var levelsRaw []byte
	err := s.db.QueryRow(`
		select enabled, bot_token, chat_id, send_levels, trade_events_enabled, position_report_enabled, position_report_interval_minutes, updated_at
		from telegram_configs
		where id = 1
	`).Scan(
		&item.Enabled,
		&item.BotToken,
		&item.ChatID,
		&levelsRaw,
		&item.TradeEventsEnabled,
		&item.PositionReportEnabled,
		&item.PositionReportIntervalMinutes,
		&item.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.TelegramConfig{}, false, nil
		}
		return domain.TelegramConfig{}, false, err
	}
	if len(levelsRaw) > 0 {
		_ = json.Unmarshal(levelsRaw, &item.SendLevels)
	}
	if item.PositionReportIntervalMinutes <= 0 {
		item.PositionReportIntervalMinutes = 30
	}
	return item, true, nil
}

func (s *Store) UpsertTelegramConfig(config domain.TelegramConfig) (domain.TelegramConfig, error) {
	config.UpdatedAt = time.Now().UTC()
	if config.PositionReportIntervalMinutes <= 0 {
		config.PositionReportIntervalMinutes = 30
	}
	raw, _ := json.Marshal(config.SendLevels)
	_, err := s.db.Exec(`
		insert into telegram_configs (id, enabled, bot_token, chat_id, send_levels, trade_events_enabled, position_report_enabled, position_report_interval_minutes, updated_at)
		values (1, $1, $2, $3, $4, $5, $6, $7, $8)
		on conflict (id) do update set
			enabled = excluded.enabled,
			bot_token = excluded.bot_token,
			chat_id = excluded.chat_id,
			send_levels = excluded.send_levels,
			trade_events_enabled = excluded.trade_events_enabled,
			position_report_enabled = excluded.position_report_enabled,
			position_report_interval_minutes = excluded.position_report_interval_minutes,
			updated_at = excluded.updated_at
	`, config.Enabled, config.BotToken, config.ChatID, raw, config.TradeEventsEnabled, config.PositionReportEnabled, config.PositionReportIntervalMinutes, config.UpdatedAt)
	if err != nil {
		return domain.TelegramConfig{}, err
	}
	return config, nil
}

func (s *Store) ListNotificationDeliveries() ([]domain.NotificationDelivery, error) {
	rows, err := s.db.Query(`
		select notification_id, channel, status, last_error, metadata, attempted_at, sent_at, updated_at
		from notification_deliveries
		order by updated_at desc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.NotificationDelivery, 0)
	for rows.Next() {
		var item domain.NotificationDelivery
		var metadataRaw []byte
		if err := rows.Scan(&item.NotificationID, &item.Channel, &item.Status, &item.LastError, &metadataRaw, &item.AttemptedAt, &item.SentAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Metadata = unmarshalJSONMap(metadataRaw)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UpsertNotificationDelivery(notificationID, channel, status, lastError string, metadata map[string]any) (domain.NotificationDelivery, error) {
	item := domain.NotificationDelivery{
		NotificationID: notificationID,
		Channel:        channel,
		Status:         status,
		LastError:      lastError,
		Metadata:       metadata,
		AttemptedAt:    time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	if status == "sent" {
		item.SentAt = item.AttemptedAt
	}
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}

	_, err := s.db.Exec(`
		insert into notification_deliveries (notification_id, channel, status, last_error, metadata, attempted_at, sent_at, updated_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		on conflict (notification_id, channel) do update set
			status = excluded.status,
			last_error = excluded.last_error,
			metadata = excluded.metadata,
			attempted_at = excluded.attempted_at,
			sent_at = excluded.sent_at,
			updated_at = excluded.updated_at
	`, item.NotificationID, item.Channel, item.Status, item.LastError, marshalJSONValue(item.Metadata), item.AttemptedAt, item.SentAt, item.UpdatedAt)
	if err != nil {
		return domain.NotificationDelivery{}, err
	}
	return item, nil
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
		item.NormalizeExecutionFlags()
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListOrdersWithLimit(limit, offset int) ([]domain.Order, error) {
	query := `
		select id, account_id, strategy_version_id, symbol, side, type, status, quantity, price, metadata, created_at
		from orders order by created_at desc
	`
	var rows *sql.Rows
	var err error
	if limit > 0 {
		query += ` limit $1 offset $2`
		rows, err = s.db.Query(query, limit, offset)
	} else {
		if offset > 0 {
			query += ` offset $1`
			rows, err = s.db.Query(query, offset)
		} else {
			rows, err = s.db.Query(query)
		}
	}
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
		item.NormalizeExecutionFlags()
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CountOrders() (int, error) {
	var count int
	err := s.db.QueryRow(`select count(*) from orders`).Scan(&count)
	return count, err
}

func (s *Store) GetOrderByID(orderID string) (domain.Order, error) {
	var (
		item              domain.Order
		strategyVersionID sql.NullString
		metadataRaw       []byte
	)
	err := s.db.QueryRow(`
		select id, account_id, strategy_version_id, symbol, side, type, status, quantity, price, metadata, created_at
		from orders
		where id = $1
	`, orderID).Scan(&item.ID, &item.AccountID, &strategyVersionID, &item.Symbol, &item.Side, &item.Type, &item.Status, &item.Quantity, &item.Price, &metadataRaw, &item.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.Order{}, fmt.Errorf("order not found: %s", orderID)
		}
		return domain.Order{}, err
	}
	item.StrategyVersionID = strategyVersionID.String
	item.Metadata = map[string]any{}
	if len(metadataRaw) > 0 {
		_ = json.Unmarshal(metadataRaw, &item.Metadata)
	}
	item.NormalizeExecutionFlags()
	return item, nil
}

func (s *Store) QueryOrders(query domain.OrderQuery) ([]domain.Order, error) {
	var builder strings.Builder
	builder.WriteString(`
			select id, account_id, strategy_version_id, symbol, side, type, status, quantity, price, metadata, created_at
			from orders
			where 1=1
		`)
	var args []any
	appendQueryCondition(&builder, &args, "metadata->>'liveSessionId' = %s", strings.TrimSpace(query.LiveSessionID))
	appendQueryCondition(&builder, &args, "account_id = %s", strings.TrimSpace(query.AccountID))
	appendStringListCondition(&builder, &args, "upper(symbol)", normalizedUpperList(query.Symbols), true)
	appendStringListCondition(&builder, &args, "upper(status)", normalizedUpperList(query.Statuses), true)
	appendStringListCondition(&builder, &args, "upper(status)", normalizedUpperList(query.ExcludeStatuses), false)
	appendMetadataBoolConditions(&builder, &args, query.MetadataBoolEquals)
	builder.WriteString(` order by created_at asc `)
	appendLimitOffsetCondition(&builder, &args, query.Limit, query.Offset)

	rows, err := s.db.Query(builder.String(), args...)
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
		item.NormalizeExecutionFlags()
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateOrder(order domain.Order) (domain.Order, error) {
	order.NormalizeExecutionFlags()
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
	order.NormalizeExecutionFlags()
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
		select id, order_id, exchange_trade_id, exchange_trade_time, dedup_fallback_fingerprint, price, quantity, fee, created_at
		from fills order by created_at asc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.Fill{}
	for rows.Next() {
		var item domain.Fill
		var exchangeTradeID sql.NullString
		var exchangeTradeTime sql.NullTime
		var fallbackFingerprint sql.NullString
		if err := rows.Scan(&item.ID, &item.OrderID, &exchangeTradeID, &exchangeTradeTime, &fallbackFingerprint, &item.Price, &item.Quantity, &item.Fee, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.ExchangeTradeID = exchangeTradeID.String
		item.DedupFingerprint = fallbackFingerprint.String
		if exchangeTradeTime.Valid {
			parsed := exchangeTradeTime.Time.UTC()
			item.ExchangeTradeTime = &parsed
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListFillsWithLimit(limit, offset int) ([]domain.Fill, error) {
	query := `
		select id, order_id, exchange_trade_id, exchange_trade_time, dedup_fallback_fingerprint, price, quantity, fee, created_at
		from fills order by created_at desc
	`
	var rows *sql.Rows
	var err error
	if limit > 0 {
		query += ` limit $1 offset $2`
		rows, err = s.db.Query(query, limit, offset)
	} else {
		if offset > 0 {
			query += ` offset $1`
			rows, err = s.db.Query(query, offset)
		} else {
			rows, err = s.db.Query(query)
		}
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.Fill{}
	for rows.Next() {
		var item domain.Fill
		var exchangeTradeID sql.NullString
		var exchangeTradeTime sql.NullTime
		var fallbackFingerprint sql.NullString
		if err := rows.Scan(&item.ID, &item.OrderID, &exchangeTradeID, &exchangeTradeTime, &fallbackFingerprint, &item.Price, &item.Quantity, &item.Fee, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.ExchangeTradeID = exchangeTradeID.String
		item.DedupFingerprint = fallbackFingerprint.String
		if exchangeTradeTime.Valid {
			parsed := exchangeTradeTime.Time.UTC()
			item.ExchangeTradeTime = &parsed
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CountFills() (int, error) {
	var count int
	err := s.db.QueryRow(`select count(*) from fills`).Scan(&count)
	return count, err
}

func (s *Store) QueryFills(query domain.FillQuery) ([]domain.Fill, error) {
	sqlQuery := `
		select id, order_id, exchange_trade_id, exchange_trade_time, dedup_fallback_fingerprint, price, quantity, fee, created_at
		from fills
	`
	args := []any{}
	if len(query.OrderIDs) > 0 {
		placeholders := []string{}
		for i, id := range query.OrderIDs {
			placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
			args = append(args, id)
		}
		sqlQuery += fmt.Sprintf(" where order_id in (%s) ", strings.Join(placeholders, ","))
	}
	sqlQuery += ` order by created_at asc `

	rows, err := s.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.Fill{}
	for rows.Next() {
		var item domain.Fill
		var exchangeTradeID sql.NullString
		var exchangeTradeTime sql.NullTime
		var fallbackFingerprint sql.NullString
		if err := rows.Scan(&item.ID, &item.OrderID, &exchangeTradeID, &exchangeTradeTime, &fallbackFingerprint, &item.Price, &item.Quantity, &item.Fee, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.ExchangeTradeID = exchangeTradeID.String
		item.DedupFingerprint = fallbackFingerprint.String
		if exchangeTradeTime.Valid {
			parsed := exchangeTradeTime.Time.UTC()
			item.ExchangeTradeTime = &parsed
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) TotalFilledQuantityForOrder(orderID string) (float64, error) {
	var total sql.NullFloat64
	if err := s.db.QueryRow(`
		select coalesce(sum(quantity), 0)
		from fills
		where order_id = $1
	`, orderID).Scan(&total); err != nil {
		return 0, err
	}
	if !total.Valid {
		return 0, nil
	}
	return total.Float64, nil
}

func (s *Store) CreateFill(fill domain.Fill) (domain.Fill, error) {
	now := time.Now().UTC()
	fill.ID = fmt.Sprintf("fill-%d", now.UnixNano())
	fill.CreatedAt = now
	if strings.TrimSpace(fill.ExchangeTradeID) == "" {
		fill.DedupFingerprint = strings.TrimSpace(fill.DedupFingerprint)
		if fill.DedupFingerprint == "" {
			fill.DedupFingerprint = fill.FallbackFingerprint()
		}
	}

	if strings.TrimSpace(fill.ExchangeTradeID) == "" {
		row := s.db.QueryRow(`
			insert into fills (id, order_id, exchange_trade_id, exchange_trade_time, dedup_fallback_fingerprint, price, quantity, fee, created_at)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			on conflict (order_id, dedup_fallback_fingerprint) do update set
				price = fills.price,
				quantity = fills.quantity,
				fee = fills.fee,
				exchange_trade_time = coalesce(fills.exchange_trade_time, excluded.exchange_trade_time)
			returning id, order_id, exchange_trade_id, exchange_trade_time, dedup_fallback_fingerprint, price, quantity, fee, created_at
		`, fill.ID, fill.OrderID, nullIfEmpty(fill.ExchangeTradeID), fill.ExchangeTradeTime, fill.DedupFingerprint, fill.Price, fill.Quantity, fill.Fee, fill.CreatedAt)
		var (
			exchangeTradeID     sql.NullString
			exchangeTradeTime   sql.NullTime
			fallbackFingerprint sql.NullString
		)
		err := row.Scan(
			&fill.ID,
			&fill.OrderID,
			&exchangeTradeID,
			&exchangeTradeTime,
			&fallbackFingerprint,
			&fill.Price,
			&fill.Quantity,
			&fill.Fee,
			&fill.CreatedAt,
		)
		fill.ExchangeTradeID = exchangeTradeID.String
		fill.DedupFingerprint = fallbackFingerprint.String
		if exchangeTradeTime.Valid {
			parsed := exchangeTradeTime.Time.UTC()
			fill.ExchangeTradeTime = &parsed
		} else {
			fill.ExchangeTradeTime = nil
		}
		return fill, err
	}

	row := s.db.QueryRow(`
		insert into fills (id, order_id, exchange_trade_id, exchange_trade_time, dedup_fallback_fingerprint, price, quantity, fee, created_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		on conflict (order_id, exchange_trade_id) do update set
			price = fills.price,
			quantity = fills.quantity,
			fee = fills.fee,
			exchange_trade_time = coalesce(fills.exchange_trade_time, excluded.exchange_trade_time)
		returning id, order_id, exchange_trade_id, exchange_trade_time, dedup_fallback_fingerprint, price, quantity, fee, created_at
	`, fill.ID, fill.OrderID, fill.ExchangeTradeID, fill.ExchangeTradeTime, nil, fill.Price, fill.Quantity, fill.Fee, fill.CreatedAt)
	var (
		exchangeTradeID     sql.NullString
		exchangeTradeTime   sql.NullTime
		fallbackFingerprint sql.NullString
	)
	err := row.Scan(
		&fill.ID,
		&fill.OrderID,
		&exchangeTradeID,
		&exchangeTradeTime,
		&fallbackFingerprint,
		&fill.Price,
		&fill.Quantity,
		&fill.Fee,
		&fill.CreatedAt,
	)
	fill.ExchangeTradeID = exchangeTradeID.String
	fill.DedupFingerprint = fallbackFingerprint.String
	if exchangeTradeTime.Valid {
		parsed := exchangeTradeTime.Time.UTC()
		fill.ExchangeTradeTime = &parsed
	} else {
		fill.ExchangeTradeTime = nil
	}
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

func (s *Store) QueryPositions(query domain.PositionQuery) ([]domain.Position, error) {
	var builder strings.Builder
	builder.WriteString(`
			select id, account_id, strategy_version_id, symbol, side, quantity, entry_price, mark_price, updated_at
			from positions
			where 1=1
		`)
	var args []any
	appendQueryCondition(&builder, &args, "account_id = %s", strings.TrimSpace(query.AccountID))
	appendQueryCondition(&builder, &args, "upper(symbol) = %s", strings.ToUpper(strings.TrimSpace(query.Symbol)))
	builder.WriteString(` order by updated_at asc `)

	rows, err := s.db.Query(builder.String(), args...)
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
	stateRaw, err := json.Marshal(state)
	if err != nil {
		return domain.PaperSession{}, fmt.Errorf("failed to marshal state: %w", err)
	}
	_, err = s.db.Exec(`update paper_sessions set state = $2 where id = $1`, sessionID, stateRaw)
	if err != nil {
		return domain.PaperSession{}, err
	}
	return s.GetPaperSession(sessionID)
}

func (s *Store) ListLiveSessions() ([]domain.LiveSession, error) {
	rows, err := s.db.Query(`
		select id, alias, account_id, strategy_id, status, state, created_at
		from live_sessions order by created_at asc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.LiveSession{}
	for rows.Next() {
		var item domain.LiveSession
		var stateRaw []byte
		if err := rows.Scan(&item.ID, &item.Alias, &item.AccountID, &item.StrategyID, &item.Status, &stateRaw, &item.CreatedAt); err != nil {
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

func (s *Store) GetLiveSession(sessionID string) (domain.LiveSession, error) {
	var item domain.LiveSession
	var stateRaw []byte
	err := s.db.QueryRow(`
		select id, alias, account_id, strategy_id, status, state, created_at
		from live_sessions
		where id = $1
	`, sessionID).Scan(&item.ID, &item.Alias, &item.AccountID, &item.StrategyID, &item.Status, &stateRaw, &item.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.LiveSession{}, fmt.Errorf("live session not found: %s", sessionID)
		}
		return domain.LiveSession{}, err
	}
	item.State = map[string]any{}
	if len(stateRaw) > 0 {
		_ = json.Unmarshal(stateRaw, &item.State)
	}
	return item, nil
}

func (s *Store) CreateLiveSession(accountID, strategyID string) (domain.LiveSession, error) {
	item := domain.LiveSession{
		ID:         fmt.Sprintf("live-session-%d", time.Now().UTC().UnixNano()),
		Alias:      "",
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
	stateRaw, _ := json.Marshal(item.State)

	_, err := s.db.Exec(`
		insert into live_sessions (id, alias, account_id, strategy_id, status, state, created_at)
		values ($1, $2, $3, $4, $5, $6, $7)
	`, item.ID, item.Alias, item.AccountID, item.StrategyID, item.Status, stateRaw, item.CreatedAt)
	return item, err
}

func (s *Store) UpdateLiveSession(item domain.LiveSession) (domain.LiveSession, error) {
	stateRaw, err := json.Marshal(item.State)
	if err != nil {
		return domain.LiveSession{}, fmt.Errorf("failed to marshal state: %w", err)
	}
	result, err := s.db.Exec(`
		update live_sessions
		set alias = $2, account_id = $3, strategy_id = $4, status = $5, state = $6
		where id = $1
	`, item.ID, item.Alias, item.AccountID, item.StrategyID, item.Status, stateRaw)
	if err != nil {
		return domain.LiveSession{}, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return domain.LiveSession{}, err
	}
	if rows == 0 {
		return domain.LiveSession{}, fmt.Errorf("live session not found: %s", item.ID)
	}
	return s.GetLiveSession(item.ID)
}

func (s *Store) DeleteLiveSession(sessionID string) error {
	result, err := s.db.Exec(`delete from live_sessions where id = $1`, sessionID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("live session not found: %s", sessionID)
	}
	return nil
}

func (s *Store) UpdateLiveSessionStatus(sessionID, status string) (domain.LiveSession, error) {
	_, err := s.db.Exec(`update live_sessions set status = $2 where id = $1`, sessionID, status)
	if err != nil {
		return domain.LiveSession{}, err
	}
	return s.GetLiveSession(sessionID)
}

func (s *Store) UpdateLiveSessionState(sessionID string, state map[string]any) (domain.LiveSession, error) {
	stateRaw, err := json.Marshal(state)
	if err != nil {
		return domain.LiveSession{}, fmt.Errorf("failed to marshal state: %w", err)
	}
	_, err = s.db.Exec(`update live_sessions set state = $2 where id = $1`, sessionID, stateRaw)
	if err != nil {
		return domain.LiveSession{}, err
	}
	return s.GetLiveSession(sessionID)
}

func (s *Store) ListSignalRuntimeSessions() ([]domain.SignalRuntimeSession, error) {
	rows, err := s.db.Query(`
		select id, account_id, strategy_id, status, runtime_adapter, transport, subscription_count, state, created_at, updated_at
		from signal_runtime_sessions
		order by updated_at desc, id asc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.SignalRuntimeSession{}
	for rows.Next() {
		item, err := scanSignalRuntimeSession(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetSignalRuntimeSession(sessionID string) (domain.SignalRuntimeSession, error) {
	row := s.db.QueryRow(`
		select id, account_id, strategy_id, status, runtime_adapter, transport, subscription_count, state, created_at, updated_at
		from signal_runtime_sessions
		where id = $1
	`, sessionID)
	item, err := scanSignalRuntimeSession(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.SignalRuntimeSession{}, fmt.Errorf("%w: %s", storepkg.ErrSignalRuntimeSessionNotFound, sessionID)
		}
		return domain.SignalRuntimeSession{}, err
	}
	return item, nil
}

func (s *Store) CreateSignalRuntimeSession(item domain.SignalRuntimeSession) (domain.SignalRuntimeSession, error) {
	if strings.TrimSpace(item.Status) == "" {
		return domain.SignalRuntimeSession{}, fmt.Errorf("signal runtime session status is required")
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = item.CreatedAt
	}
	stateRaw, err := json.Marshal(item.State)
	if err != nil {
		return domain.SignalRuntimeSession{}, fmt.Errorf("failed to marshal signal runtime session state: %w", err)
	}
	row := s.db.QueryRow(`
		insert into signal_runtime_sessions (
			id, account_id, strategy_id, status, runtime_adapter, transport, subscription_count, state, created_at, updated_at
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		on conflict (account_id, strategy_id) do update set
			status = excluded.status,
			runtime_adapter = excluded.runtime_adapter,
			transport = excluded.transport,
			subscription_count = excluded.subscription_count,
			state = excluded.state,
			updated_at = excluded.updated_at
		returning id, account_id, strategy_id, status, runtime_adapter, transport, subscription_count, state, created_at, updated_at
	`, item.ID, item.AccountID, item.StrategyID, item.Status, item.RuntimeAdapter, item.Transport, item.SubscriptionCnt, stateRaw, item.CreatedAt, item.UpdatedAt)
	return scanSignalRuntimeSession(row)
}

func (s *Store) UpdateSignalRuntimeSession(item domain.SignalRuntimeSession) (domain.SignalRuntimeSession, error) {
	if strings.TrimSpace(item.Status) == "" {
		return domain.SignalRuntimeSession{}, fmt.Errorf("signal runtime session status is required")
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = time.Now().UTC()
	}
	stateRaw, err := json.Marshal(item.State)
	if err != nil {
		return domain.SignalRuntimeSession{}, fmt.Errorf("failed to marshal signal runtime session state: %w", err)
	}
	row := s.db.QueryRow(`
		update signal_runtime_sessions
		set account_id = $2,
			strategy_id = $3,
			status = $4,
			runtime_adapter = $5,
			transport = $6,
			subscription_count = $7,
			state = $8,
			updated_at = $9
		where id = $1
		returning id, account_id, strategy_id, status, runtime_adapter, transport, subscription_count, state, created_at, updated_at
	`, item.ID, item.AccountID, item.StrategyID, item.Status, item.RuntimeAdapter, item.Transport, item.SubscriptionCnt, stateRaw, item.UpdatedAt)
	updated, err := scanSignalRuntimeSession(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.SignalRuntimeSession{}, fmt.Errorf("%w: %s", storepkg.ErrSignalRuntimeSessionNotFound, item.ID)
		}
		return domain.SignalRuntimeSession{}, err
	}
	return updated, nil
}

func (s *Store) DeleteSignalRuntimeSession(sessionID string) error {
	result, err := s.db.Exec(`delete from signal_runtime_sessions where id = $1`, sessionID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("%w: %s", storepkg.ErrSignalRuntimeSessionNotFound, sessionID)
	}
	return nil
}

func (s *Store) ListAccountEquitySnapshots(query domain.AccountEquitySnapshotQuery) ([]domain.AccountEquitySnapshot, error) {
	args := []any{query.AccountID}
	filters := []string{"account_id = $1"}
	if !query.From.IsZero() {
		args = append(args, query.From)
		filters = append(filters, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if !query.To.IsZero() {
		args = append(args, query.To)
		filters = append(filters, fmt.Sprintf("created_at <= $%d", len(args)))
	}
	sqlQuery := `
		select id, account_id, start_equity, realized_pnl, unrealized_pnl, fees, net_equity, exposure_notional, open_position_count, created_at
		from account_equity_snapshots
		where ` + strings.Join(filters, " and ")
	if query.Limit > 0 {
		args = append(args, query.Limit)
		sqlQuery = `select * from (` + sqlQuery + fmt.Sprintf(` order by created_at desc limit $%d`, len(args)) + `) limited_snapshots order by created_at asc`
	} else {
		sqlQuery += ` order by created_at asc`
	}
	rows, err := s.db.Query(sqlQuery, args...)
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

func (s *Store) ListStrategyDecisionEvents(liveSessionID string) ([]domain.StrategyDecisionEvent, error) {
	rows, err := s.db.Query(`
		select
			id, live_session_id, runtime_session_id, account_id, strategy_id, strategy_version_id, symbol,
			trigger_type, action, reason, signal_kind, decision_state, intent_signature,
			source_gate_ready, missing_count, stale_count, event_time, recorded_at,
			trigger_summary, source_gate, source_states, signal_bar_states, position_snapshot,
			decision_metadata, signal_intent, execution_proposal, evaluation_context
		from strategy_decision_events
		where ($1 = '' or live_session_id = $1)
		order by event_time asc, recorded_at asc
	`, liveSessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.StrategyDecisionEvent, 0)
	for rows.Next() {
		var item domain.StrategyDecisionEvent
		var runtimeSessionID sql.NullString
		var strategyVersionID sql.NullString
		var triggerType sql.NullString
		var signalKind sql.NullString
		var decisionState sql.NullString
		var intentSignature sql.NullString
		var triggerSummaryRaw, sourceGateRaw, sourceStatesRaw, signalBarStatesRaw []byte
		var positionSnapshotRaw, decisionMetadataRaw, signalIntentRaw, executionProposalRaw, evaluationContextRaw []byte
		if err := rows.Scan(
			&item.ID, &item.LiveSessionID, &runtimeSessionID, &item.AccountID, &item.StrategyID, &strategyVersionID, &item.Symbol,
			&triggerType, &item.Action, &item.Reason, &signalKind, &decisionState, &intentSignature,
			&item.SourceGateReady, &item.MissingCount, &item.StaleCount, &item.EventTime, &item.RecordedAt,
			&triggerSummaryRaw, &sourceGateRaw, &sourceStatesRaw, &signalBarStatesRaw, &positionSnapshotRaw,
			&decisionMetadataRaw, &signalIntentRaw, &executionProposalRaw, &evaluationContextRaw,
		); err != nil {
			return nil, err
		}
		item.RuntimeSessionID = runtimeSessionID.String
		item.StrategyVersionID = strategyVersionID.String
		item.TriggerType = triggerType.String
		item.SignalKind = signalKind.String
		item.DecisionState = decisionState.String
		item.IntentSignature = intentSignature.String
		item.TriggerSummary = unmarshalJSONMap(triggerSummaryRaw)
		item.SourceGate = unmarshalJSONMap(sourceGateRaw)
		item.SourceStates = unmarshalJSONMap(sourceStatesRaw)
		item.SignalBarStates = unmarshalJSONMap(signalBarStatesRaw)
		item.PositionSnapshot = unmarshalJSONMap(positionSnapshotRaw)
		item.DecisionMetadata = unmarshalJSONMap(decisionMetadataRaw)
		item.SignalIntent = unmarshalJSONMap(signalIntentRaw)
		item.ExecutionProposal = unmarshalJSONMap(executionProposalRaw)
		item.EvaluationContext = unmarshalJSONMap(evaluationContextRaw)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateStrategyDecisionEvent(event domain.StrategyDecisionEvent) (domain.StrategyDecisionEvent, error) {
	if event.ID == "" {
		event.ID = fmt.Sprintf("strategy-decision-event-%d", time.Now().UTC().UnixNano())
	}
	if event.EventTime.IsZero() {
		event.EventTime = time.Now().UTC()
	}
	if event.RecordedAt.IsZero() {
		event.RecordedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(`
		insert into strategy_decision_events (
			id, live_session_id, runtime_session_id, account_id, strategy_id, strategy_version_id, symbol,
			trigger_type, action, reason, signal_kind, decision_state, intent_signature,
			source_gate_ready, missing_count, stale_count, event_time, recorded_at,
			trigger_summary, source_gate, source_states, signal_bar_states, position_snapshot,
			decision_metadata, signal_intent, execution_proposal, evaluation_context
		) values (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18,
			$19, $20, $21, $22, $23,
			$24, $25, $26, $27
		)
	`,
		event.ID, event.LiveSessionID, nullIfEmpty(event.RuntimeSessionID), event.AccountID, event.StrategyID, nullIfEmpty(event.StrategyVersionID), event.Symbol,
		nullIfEmpty(event.TriggerType), event.Action, event.Reason, nullIfEmpty(event.SignalKind), nullIfEmpty(event.DecisionState), nullIfEmpty(event.IntentSignature),
		event.SourceGateReady, event.MissingCount, event.StaleCount, event.EventTime, event.RecordedAt,
		marshalJSONValue(event.TriggerSummary), marshalJSONValue(event.SourceGate), marshalJSONValue(event.SourceStates), marshalJSONValue(event.SignalBarStates), marshalJSONValue(event.PositionSnapshot),
		marshalJSONValue(event.DecisionMetadata), marshalJSONValue(event.SignalIntent), marshalJSONValue(event.ExecutionProposal), marshalJSONValue(event.EvaluationContext),
	)
	return event, err
}

func (s *Store) ListOrderExecutionEvents(orderID string) ([]domain.OrderExecutionEvent, error) {
	rows, err := s.db.Query(`
		select
			id, order_id, exchange_order_id, live_session_id, decision_event_id, runtime_session_id, account_id,
			strategy_version_id, symbol, side, order_type, event_type, status,
			execution_strategy, execution_decision, execution_mode,
			quantity, price, expected_price, price_drift_bps, raw_quantity, normalized_quantity,
			raw_price_reference, normalized_price, spread_bps, book_imbalance,
			submit_latency_ms, sync_latency_ms, fill_latency_ms, event_time, recorded_at,
			fallback, post_only, reduce_only, failed, error,
			runtime_preflight, dispatch_summary, adapter_submission, adapter_sync, normalization, symbol_rules, metadata
		from order_execution_events
		where ($1 = '' or order_id = $1)
		order by event_time asc, recorded_at asc
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.OrderExecutionEvent, 0)
	for rows.Next() {
		var item domain.OrderExecutionEvent
		var exchangeOrderID, liveSessionID, decisionEventID, runtimeSessionID, strategyVersionID sql.NullString
		var executionStrategy, executionDecision, executionMode, errText sql.NullString
		var runtimePreflightRaw, dispatchSummaryRaw, adapterSubmissionRaw, adapterSyncRaw, normalizationRaw, symbolRulesRaw, metadataRaw []byte
		if err := rows.Scan(
			&item.ID, &item.OrderID, &exchangeOrderID, &liveSessionID, &decisionEventID, &runtimeSessionID, &item.AccountID,
			&strategyVersionID, &item.Symbol, &item.Side, &item.OrderType, &item.EventType, &item.Status,
			&executionStrategy, &executionDecision, &executionMode,
			&item.Quantity, &item.Price, &item.ExpectedPrice, &item.PriceDriftBps, &item.RawQuantity, &item.NormalizedQty,
			&item.RawPriceReference, &item.NormalizedPrice, &item.SpreadBps, &item.BookImbalance,
			&item.SubmitLatencyMs, &item.SyncLatencyMs, &item.FillLatencyMs, &item.EventTime, &item.RecordedAt,
			&item.Fallback, &item.PostOnly, &item.ReduceOnly, &item.Failed, &errText,
			&runtimePreflightRaw, &dispatchSummaryRaw, &adapterSubmissionRaw, &adapterSyncRaw, &normalizationRaw, &symbolRulesRaw, &metadataRaw,
		); err != nil {
			return nil, err
		}
		item.ExchangeOrderID = exchangeOrderID.String
		item.LiveSessionID = liveSessionID.String
		item.DecisionEventID = decisionEventID.String
		item.RuntimeSessionID = runtimeSessionID.String
		item.StrategyVersionID = strategyVersionID.String
		item.ExecutionStrategy = executionStrategy.String
		item.ExecutionDecision = executionDecision.String
		item.ExecutionMode = executionMode.String
		item.Error = errText.String
		item.RuntimePreflight = unmarshalJSONMap(runtimePreflightRaw)
		item.DispatchSummary = unmarshalJSONMap(dispatchSummaryRaw)
		item.AdapterSubmission = unmarshalJSONMap(adapterSubmissionRaw)
		item.AdapterSync = unmarshalJSONMap(adapterSyncRaw)
		item.Normalization = unmarshalJSONMap(normalizationRaw)
		item.SymbolRules = unmarshalJSONMap(symbolRulesRaw)
		item.Metadata = unmarshalJSONMap(metadataRaw)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListTelegramTradeEventCandidates(from time.Time, limit int) ([]domain.OrderExecutionEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if from.IsZero() {
		from = time.Now().UTC().Add(-24 * time.Hour)
	}
	rows, err := s.db.Query(`
		select
			e.id, e.order_id, e.exchange_order_id, e.live_session_id, e.decision_event_id, e.runtime_session_id, e.account_id,
			e.strategy_version_id, e.symbol, e.side, e.order_type, e.event_type, e.status,
			e.execution_strategy, e.execution_decision, e.execution_mode,
			e.quantity, e.price, e.expected_price, e.price_drift_bps, e.raw_quantity, e.normalized_quantity,
			e.raw_price_reference, e.normalized_price, e.spread_bps, e.book_imbalance,
			e.submit_latency_ms, e.sync_latency_ms, e.fill_latency_ms, e.event_time, e.recorded_at,
			e.fallback, e.post_only, e.reduce_only, e.failed, e.error,
			e.runtime_preflight, e.dispatch_summary, e.adapter_submission, e.adapter_sync, e.normalization, e.symbol_rules, e.metadata
		from order_execution_events e
		where e.event_type = 'filled'
			and e.failed = false
			and coalesce(e.error, '') = ''
			and e.event_time >= $1
			and not exists (
				select 1
				from notification_deliveries d
				where d.channel = 'telegram'
					and d.metadata->>'kind' = 'trade-event'
					and d.metadata->>'eventId' = e.id
			)
		order by e.event_time asc, e.recorded_at asc, e.id asc
		limit $2
	`, from.UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.OrderExecutionEvent, 0, limit)
	for rows.Next() {
		var item domain.OrderExecutionEvent
		var exchangeOrderID, liveSessionID, decisionEventID, runtimeSessionID, strategyVersionID sql.NullString
		var executionStrategy, executionDecision, executionMode, errText sql.NullString
		var runtimePreflightRaw, dispatchSummaryRaw, adapterSubmissionRaw, adapterSyncRaw, normalizationRaw, symbolRulesRaw, metadataRaw []byte
		if err := rows.Scan(
			&item.ID, &item.OrderID, &exchangeOrderID, &liveSessionID, &decisionEventID, &runtimeSessionID, &item.AccountID,
			&strategyVersionID, &item.Symbol, &item.Side, &item.OrderType, &item.EventType, &item.Status,
			&executionStrategy, &executionDecision, &executionMode,
			&item.Quantity, &item.Price, &item.ExpectedPrice, &item.PriceDriftBps, &item.RawQuantity, &item.NormalizedQty,
			&item.RawPriceReference, &item.NormalizedPrice, &item.SpreadBps, &item.BookImbalance,
			&item.SubmitLatencyMs, &item.SyncLatencyMs, &item.FillLatencyMs, &item.EventTime, &item.RecordedAt,
			&item.Fallback, &item.PostOnly, &item.ReduceOnly, &item.Failed, &errText,
			&runtimePreflightRaw, &dispatchSummaryRaw, &adapterSubmissionRaw, &adapterSyncRaw, &normalizationRaw, &symbolRulesRaw, &metadataRaw,
		); err != nil {
			return nil, err
		}
		item.ExchangeOrderID = exchangeOrderID.String
		item.LiveSessionID = liveSessionID.String
		item.DecisionEventID = decisionEventID.String
		item.RuntimeSessionID = runtimeSessionID.String
		item.StrategyVersionID = strategyVersionID.String
		item.ExecutionStrategy = executionStrategy.String
		item.ExecutionDecision = executionDecision.String
		item.ExecutionMode = executionMode.String
		item.Error = errText.String
		item.RuntimePreflight = unmarshalJSONMap(runtimePreflightRaw)
		item.DispatchSummary = unmarshalJSONMap(dispatchSummaryRaw)
		item.AdapterSubmission = unmarshalJSONMap(adapterSubmissionRaw)
		item.AdapterSync = unmarshalJSONMap(adapterSyncRaw)
		item.Normalization = unmarshalJSONMap(normalizationRaw)
		item.SymbolRules = unmarshalJSONMap(symbolRulesRaw)
		item.Metadata = unmarshalJSONMap(metadataRaw)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateOrderExecutionEvent(event domain.OrderExecutionEvent) (domain.OrderExecutionEvent, error) {
	if event.ID == "" {
		event.ID = fmt.Sprintf("order-execution-event-%d", time.Now().UTC().UnixNano())
	}
	if event.EventTime.IsZero() {
		event.EventTime = time.Now().UTC()
	}
	if event.RecordedAt.IsZero() {
		event.RecordedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(`
		insert into order_execution_events (
			id, order_id, exchange_order_id, live_session_id, decision_event_id, runtime_session_id, account_id,
			strategy_version_id, symbol, side, order_type, event_type, status,
			execution_strategy, execution_decision, execution_mode,
			quantity, price, expected_price, price_drift_bps, raw_quantity, normalized_quantity,
			raw_price_reference, normalized_price, spread_bps, book_imbalance,
			submit_latency_ms, sync_latency_ms, fill_latency_ms, event_time, recorded_at,
			fallback, post_only, reduce_only, failed, error,
			runtime_preflight, dispatch_summary, adapter_submission, adapter_sync, normalization, symbol_rules, metadata
		) values (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13,
			$14, $15, $16,
			$17, $18, $19, $20, $21, $22,
			$23, $24, $25, $26,
			$27, $28, $29, $30, $31,
			$32, $33, $34, $35, $36,
			$37, $38, $39, $40, $41, $42, $43
		)
	`,
		event.ID, event.OrderID, nullIfEmpty(event.ExchangeOrderID), nullIfEmpty(event.LiveSessionID), nullIfEmpty(event.DecisionEventID), nullIfEmpty(event.RuntimeSessionID), event.AccountID,
		nullIfEmpty(event.StrategyVersionID), event.Symbol, event.Side, event.OrderType, event.EventType, event.Status,
		nullIfEmpty(event.ExecutionStrategy), nullIfEmpty(event.ExecutionDecision), nullIfEmpty(event.ExecutionMode),
		event.Quantity, event.Price, event.ExpectedPrice, event.PriceDriftBps, event.RawQuantity, event.NormalizedQty,
		event.RawPriceReference, event.NormalizedPrice, event.SpreadBps, event.BookImbalance,
		event.SubmitLatencyMs, event.SyncLatencyMs, event.FillLatencyMs, event.EventTime, event.RecordedAt,
		event.Fallback, event.PostOnly, event.ReduceOnly, event.Failed, nullIfEmpty(event.Error),
		marshalJSONValue(event.RuntimePreflight), marshalJSONValue(event.DispatchSummary), marshalJSONValue(event.AdapterSubmission), marshalJSONValue(event.AdapterSync), marshalJSONValue(event.Normalization), marshalJSONValue(event.SymbolRules), marshalJSONValue(event.Metadata),
	)
	return event, err
}

func (s *Store) ListPositionAccountSnapshots(accountID string) ([]domain.PositionAccountSnapshot, error) {
	rows, err := s.db.Query(`
		select
			id, live_session_id, decision_event_id, order_id, account_id, strategy_id, symbol, trigger, intent_signature,
			position_found, position_side, position_quantity, entry_price, mark_price,
			net_equity, available_balance, margin_balance, wallet_balance, exposure_notional, open_position_count,
			sync_status, event_time, recorded_at,
			position_snapshot, live_position_state, account_snapshot, account_summary, metadata
		from position_account_snapshots
		where ($1 = '' or account_id = $1)
		order by event_time asc, recorded_at asc
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.PositionAccountSnapshot, 0)
	for rows.Next() {
		var item domain.PositionAccountSnapshot
		var decisionEventID, orderID, intentSignature, positionSide, syncStatus sql.NullString
		var positionSnapshotRaw, livePositionStateRaw, accountSnapshotRaw, accountSummaryRaw, metadataRaw []byte
		if err := rows.Scan(
			&item.ID, &item.LiveSessionID, &decisionEventID, &orderID, &item.AccountID, &item.StrategyID, &item.Symbol, &item.Trigger, &intentSignature,
			&item.PositionFound, &positionSide, &item.PositionQuantity, &item.EntryPrice, &item.MarkPrice,
			&item.NetEquity, &item.AvailableBalance, &item.MarginBalance, &item.WalletBalance, &item.ExposureNotional, &item.OpenPositionCount,
			&syncStatus, &item.EventTime, &item.RecordedAt,
			&positionSnapshotRaw, &livePositionStateRaw, &accountSnapshotRaw, &accountSummaryRaw, &metadataRaw,
		); err != nil {
			return nil, err
		}
		item.DecisionEventID = decisionEventID.String
		item.OrderID = orderID.String
		item.IntentSignature = intentSignature.String
		item.PositionSide = positionSide.String
		item.SyncStatus = syncStatus.String
		item.PositionSnapshot = unmarshalJSONMap(positionSnapshotRaw)
		item.LivePositionState = unmarshalJSONMap(livePositionStateRaw)
		item.AccountSnapshot = unmarshalJSONMap(accountSnapshotRaw)
		item.AccountSummary = unmarshalJSONMap(accountSummaryRaw)
		item.Metadata = unmarshalJSONMap(metadataRaw)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) QueryStrategyDecisionEvents(query domain.StrategyDecisionEventQuery) ([]domain.StrategyDecisionEvent, error) {
	var builder strings.Builder
	args := make([]any, 0, 12)
	builder.WriteString(`
		select
			id, live_session_id, runtime_session_id, account_id, strategy_id, strategy_version_id, symbol,
			trigger_type, action, reason, signal_kind, decision_state, intent_signature,
			source_gate_ready, missing_count, stale_count, event_time, recorded_at,
			trigger_summary, source_gate, source_states, signal_bar_states, position_snapshot,
			decision_metadata, signal_intent, execution_proposal, evaluation_context
		from strategy_decision_events
		where 1 = 1
	`)
	appendQueryCondition(&builder, &args, "live_session_id = %s", strings.TrimSpace(query.LiveSessionID))
	appendQueryCondition(&builder, &args, "account_id = %s", strings.TrimSpace(query.AccountID))
	appendQueryCondition(&builder, &args, "strategy_id = %s", strings.TrimSpace(query.StrategyID))
	appendQueryCondition(&builder, &args, "runtime_session_id = %s", strings.TrimSpace(query.RuntimeSessionID))
	appendQueryCondition(&builder, &args, "id = %s", strings.TrimSpace(query.DecisionEventID))
	if len(query.DecisionEventIDs) > 0 {
		placeholders := make([]string, 0, len(query.DecisionEventIDs))
		for _, id := range query.DecisionEventIDs {
			if strings.TrimSpace(id) == "" {
				continue
			}
			args = append(args, strings.TrimSpace(id))
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
		}
		if len(placeholders) > 0 {
			builder.WriteString(fmt.Sprintf(" and id in (%s)", strings.Join(placeholders, ", ")))
		}
	}
	appendQueryTimeCondition(&builder, &args, "event_time >= %s", query.From)
	appendQueryTimeCondition(&builder, &args, "event_time <= %s", query.To)
	appendEventCursorCondition(&builder, &args, query.Before)
	builder.WriteString(" order by event_time desc, recorded_at desc, id desc")
	appendLimitCondition(&builder, &args, query.Limit)

	rows, err := s.db.Query(builder.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.StrategyDecisionEvent, 0)
	for rows.Next() {
		var item domain.StrategyDecisionEvent
		var runtimeSessionID sql.NullString
		var strategyVersionID sql.NullString
		var triggerType sql.NullString
		var signalKind sql.NullString
		var decisionState sql.NullString
		var intentSignature sql.NullString
		var triggerSummaryRaw, sourceGateRaw, sourceStatesRaw, signalBarStatesRaw []byte
		var positionSnapshotRaw, decisionMetadataRaw, signalIntentRaw, executionProposalRaw, evaluationContextRaw []byte
		if err := rows.Scan(
			&item.ID, &item.LiveSessionID, &runtimeSessionID, &item.AccountID, &item.StrategyID, &strategyVersionID, &item.Symbol,
			&triggerType, &item.Action, &item.Reason, &signalKind, &decisionState, &intentSignature,
			&item.SourceGateReady, &item.MissingCount, &item.StaleCount, &item.EventTime, &item.RecordedAt,
			&triggerSummaryRaw, &sourceGateRaw, &sourceStatesRaw, &signalBarStatesRaw, &positionSnapshotRaw,
			&decisionMetadataRaw, &signalIntentRaw, &executionProposalRaw, &evaluationContextRaw,
		); err != nil {
			return nil, err
		}
		item.RuntimeSessionID = runtimeSessionID.String
		item.StrategyVersionID = strategyVersionID.String
		item.TriggerType = triggerType.String
		item.SignalKind = signalKind.String
		item.DecisionState = decisionState.String
		item.IntentSignature = intentSignature.String
		item.TriggerSummary = unmarshalJSONMap(triggerSummaryRaw)
		item.SourceGate = unmarshalJSONMap(sourceGateRaw)
		item.SourceStates = unmarshalJSONMap(sourceStatesRaw)
		item.SignalBarStates = unmarshalJSONMap(signalBarStatesRaw)
		item.PositionSnapshot = unmarshalJSONMap(positionSnapshotRaw)
		item.DecisionMetadata = unmarshalJSONMap(decisionMetadataRaw)
		item.SignalIntent = unmarshalJSONMap(signalIntentRaw)
		item.ExecutionProposal = unmarshalJSONMap(executionProposalRaw)
		item.EvaluationContext = unmarshalJSONMap(evaluationContextRaw)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) QueryOrderExecutionEvents(query domain.OrderExecutionEventQuery) ([]domain.OrderExecutionEvent, error) {
	var builder strings.Builder
	args := make([]any, 0, 14)
	builder.WriteString(`
		select
			id, order_id, exchange_order_id, live_session_id, decision_event_id, runtime_session_id, account_id,
			strategy_version_id, symbol, side, order_type, event_type, status,
			execution_strategy, execution_decision, execution_mode,
			quantity, price, expected_price, price_drift_bps, raw_quantity, normalized_quantity,
			raw_price_reference, normalized_price, spread_bps, book_imbalance,
			submit_latency_ms, sync_latency_ms, fill_latency_ms, event_time, recorded_at,
			fallback, post_only, reduce_only, failed, error,
			runtime_preflight, dispatch_summary, adapter_submission, adapter_sync, normalization, symbol_rules, metadata
		from order_execution_events
		where 1 = 1
	`)
	appendQueryCondition(&builder, &args, "account_id = %s", strings.TrimSpace(query.AccountID))
	appendQueryCondition(&builder, &args, "live_session_id = %s", strings.TrimSpace(query.LiveSessionID))
	appendQueryCondition(&builder, &args, "runtime_session_id = %s", strings.TrimSpace(query.RuntimeSessionID))
	appendQueryCondition(&builder, &args, "order_id = %s", strings.TrimSpace(query.OrderID))
	appendQueryCondition(&builder, &args, "decision_event_id = %s", strings.TrimSpace(query.DecisionEventID))
	appendQueryTimeCondition(&builder, &args, "event_time >= %s", query.From)
	appendQueryTimeCondition(&builder, &args, "event_time <= %s", query.To)
	appendEventCursorCondition(&builder, &args, query.Before)
	if strings.TrimSpace(query.StrategyID) != "" {
		args = append(args, strings.TrimSpace(query.StrategyID))
		builder.WriteString(fmt.Sprintf(" and coalesce(metadata->>'strategyId', '') = $%d", len(args)))
	}
	builder.WriteString(" order by event_time desc, recorded_at desc, id desc")
	appendLimitCondition(&builder, &args, query.Limit)

	rows, err := s.db.Query(builder.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.OrderExecutionEvent, 0)
	for rows.Next() {
		var item domain.OrderExecutionEvent
		var exchangeOrderID, liveSessionID, decisionEventID, runtimeSessionID, strategyVersionID sql.NullString
		var executionStrategy, executionDecision, executionMode, errText sql.NullString
		var runtimePreflightRaw, dispatchSummaryRaw, adapterSubmissionRaw, adapterSyncRaw, normalizationRaw, symbolRulesRaw, metadataRaw []byte
		if err := rows.Scan(
			&item.ID, &item.OrderID, &exchangeOrderID, &liveSessionID, &decisionEventID, &runtimeSessionID, &item.AccountID,
			&strategyVersionID, &item.Symbol, &item.Side, &item.OrderType, &item.EventType, &item.Status,
			&executionStrategy, &executionDecision, &executionMode,
			&item.Quantity, &item.Price, &item.ExpectedPrice, &item.PriceDriftBps, &item.RawQuantity, &item.NormalizedQty,
			&item.RawPriceReference, &item.NormalizedPrice, &item.SpreadBps, &item.BookImbalance,
			&item.SubmitLatencyMs, &item.SyncLatencyMs, &item.FillLatencyMs, &item.EventTime, &item.RecordedAt,
			&item.Fallback, &item.PostOnly, &item.ReduceOnly, &item.Failed, &errText,
			&runtimePreflightRaw, &dispatchSummaryRaw, &adapterSubmissionRaw, &adapterSyncRaw, &normalizationRaw, &symbolRulesRaw, &metadataRaw,
		); err != nil {
			return nil, err
		}
		item.ExchangeOrderID = exchangeOrderID.String
		item.LiveSessionID = liveSessionID.String
		item.DecisionEventID = decisionEventID.String
		item.RuntimeSessionID = runtimeSessionID.String
		item.StrategyVersionID = strategyVersionID.String
		item.ExecutionStrategy = executionStrategy.String
		item.ExecutionDecision = executionDecision.String
		item.ExecutionMode = executionMode.String
		item.Error = errText.String
		item.RuntimePreflight = unmarshalJSONMap(runtimePreflightRaw)
		item.DispatchSummary = unmarshalJSONMap(dispatchSummaryRaw)
		item.AdapterSubmission = unmarshalJSONMap(adapterSubmissionRaw)
		item.AdapterSync = unmarshalJSONMap(adapterSyncRaw)
		item.Normalization = unmarshalJSONMap(normalizationRaw)
		item.SymbolRules = unmarshalJSONMap(symbolRulesRaw)
		item.Metadata = unmarshalJSONMap(metadataRaw)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) QueryPositionAccountSnapshots(query domain.PositionAccountSnapshotQuery) ([]domain.PositionAccountSnapshot, error) {
	var builder strings.Builder
	args := make([]any, 0, 12)
	builder.WriteString(`
		select
			id, live_session_id, decision_event_id, order_id, account_id, strategy_id, symbol, trigger, intent_signature,
			position_found, position_side, position_quantity, entry_price, mark_price,
			net_equity, available_balance, margin_balance, wallet_balance, exposure_notional, open_position_count,
			sync_status, event_time, recorded_at,
			position_snapshot, live_position_state, account_snapshot, account_summary, metadata
		from position_account_snapshots
		where 1 = 1
	`)
	appendQueryCondition(&builder, &args, "account_id = %s", strings.TrimSpace(query.AccountID))
	appendQueryCondition(&builder, &args, "strategy_id = %s", strings.TrimSpace(query.StrategyID))
	appendQueryCondition(&builder, &args, "live_session_id = %s", strings.TrimSpace(query.LiveSessionID))
	appendQueryCondition(&builder, &args, "order_id = %s", strings.TrimSpace(query.OrderID))
	appendQueryCondition(&builder, &args, "decision_event_id = %s", strings.TrimSpace(query.DecisionEventID))
	appendQueryTimeCondition(&builder, &args, "event_time >= %s", query.From)
	appendQueryTimeCondition(&builder, &args, "event_time <= %s", query.To)
	appendEventCursorCondition(&builder, &args, query.Before)
	builder.WriteString(" order by event_time desc, recorded_at desc, id desc")
	appendLimitCondition(&builder, &args, query.Limit)

	rows, err := s.db.Query(builder.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.PositionAccountSnapshot, 0)
	for rows.Next() {
		var item domain.PositionAccountSnapshot
		var decisionEventID, orderID, intentSignature, positionSide, syncStatus sql.NullString
		var positionSnapshotRaw, livePositionStateRaw, accountSnapshotRaw, accountSummaryRaw, metadataRaw []byte
		if err := rows.Scan(
			&item.ID, &item.LiveSessionID, &decisionEventID, &orderID, &item.AccountID, &item.StrategyID, &item.Symbol, &item.Trigger, &intentSignature,
			&item.PositionFound, &positionSide, &item.PositionQuantity, &item.EntryPrice, &item.MarkPrice,
			&item.NetEquity, &item.AvailableBalance, &item.MarginBalance, &item.WalletBalance, &item.ExposureNotional, &item.OpenPositionCount,
			&syncStatus, &item.EventTime, &item.RecordedAt,
			&positionSnapshotRaw, &livePositionStateRaw, &accountSnapshotRaw, &accountSummaryRaw, &metadataRaw,
		); err != nil {
			return nil, err
		}
		item.DecisionEventID = decisionEventID.String
		item.OrderID = orderID.String
		item.IntentSignature = intentSignature.String
		item.PositionSide = positionSide.String
		item.SyncStatus = syncStatus.String
		item.PositionSnapshot = unmarshalJSONMap(positionSnapshotRaw)
		item.LivePositionState = unmarshalJSONMap(livePositionStateRaw)
		item.AccountSnapshot = unmarshalJSONMap(accountSnapshotRaw)
		item.AccountSummary = unmarshalJSONMap(accountSummaryRaw)
		item.Metadata = unmarshalJSONMap(metadataRaw)
		items = append(items, item)
	}
	return items, rows.Err()
}

func appendQueryCondition(builder *strings.Builder, args *[]any, clause, value string) {
	if value == "" {
		return
	}
	*args = append(*args, value)
	builder.WriteString(fmt.Sprintf(" and "+clause, fmt.Sprintf("$%d", len(*args))))
}

func appendQueryTimeCondition(builder *strings.Builder, args *[]any, clause string, value time.Time) {
	if value.IsZero() {
		return
	}
	*args = append(*args, value.UTC())
	builder.WriteString(fmt.Sprintf(" and "+clause, fmt.Sprintf("$%d", len(*args))))
}

func normalizedUpperList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.ToUpper(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		out = append(out, normalized)
	}
	return out
}

func appendStringListCondition(builder *strings.Builder, args *[]any, expression string, values []string, include bool) {
	if len(values) == 0 {
		return
	}
	placeholders := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		*args = append(*args, value)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(*args)))
	}
	if len(placeholders) == 0 {
		return
	}
	operator := "in"
	if !include {
		operator = "not in"
	}
	builder.WriteString(fmt.Sprintf(" and %s %s (%s)", expression, operator, strings.Join(placeholders, ", ")))
}

func appendMetadataBoolConditions(builder *strings.Builder, args *[]any, values map[string]bool) {
	if len(values) == 0 {
		return
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		key = strings.TrimSpace(key)
		builder.WriteString(fmt.Sprintf(" and metadata->>%s = %s", sqlStringLiteral(key), sqlStringLiteral(strconv.FormatBool(values[key]))))
	}
}

func sqlStringLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func appendEventCursorCondition(builder *strings.Builder, args *[]any, cursor *domain.EventCursor) {
	if cursor == nil {
		return
	}
	*args = append(*args, cursor.EventTime.UTC(), domain.NormalizeEventRecordedAt(cursor.RecordedAt, cursor.EventTime), cursor.ID)
	eventIdx := len(*args) - 2
	recordedIdx := len(*args) - 1
	idIdx := len(*args)
	builder.WriteString(fmt.Sprintf(
		" and (event_time < $%d or (event_time = $%d and recorded_at < $%d) or (event_time = $%d and recorded_at = $%d and id < $%d))",
		eventIdx, eventIdx, recordedIdx, eventIdx, recordedIdx, idIdx,
	))
}

func appendLimitCondition(builder *strings.Builder, args *[]any, limit int) {
	if limit <= 0 {
		return
	}
	*args = append(*args, limit)
	builder.WriteString(fmt.Sprintf(" limit $%d", len(*args)))
}

func appendLimitOffsetCondition(builder *strings.Builder, args *[]any, limit, offset int) {
	appendLimitCondition(builder, args, limit)
	if offset <= 0 {
		return
	}
	*args = append(*args, offset)
	builder.WriteString(fmt.Sprintf(" offset $%d", len(*args)))
}

func (s *Store) CreatePositionAccountSnapshot(snapshot domain.PositionAccountSnapshot) (domain.PositionAccountSnapshot, error) {
	if snapshot.ID == "" {
		snapshot.ID = fmt.Sprintf("position-account-snapshot-%d", time.Now().UTC().UnixNano())
	}
	if snapshot.EventTime.IsZero() {
		snapshot.EventTime = time.Now().UTC()
	}
	if snapshot.RecordedAt.IsZero() {
		snapshot.RecordedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(`
		insert into position_account_snapshots (
			id, live_session_id, decision_event_id, order_id, account_id, strategy_id, symbol, trigger, intent_signature,
			position_found, position_side, position_quantity, entry_price, mark_price,
			net_equity, available_balance, margin_balance, wallet_balance, exposure_notional, open_position_count,
			sync_status, event_time, recorded_at,
			position_snapshot, live_position_state, account_snapshot, account_summary, metadata
		) values (
			$1, $2, $3, $4, $5, $6, $7, $8, $9,
			$10, $11, $12, $13, $14,
			$15, $16, $17, $18, $19, $20,
			$21, $22, $23,
			$24, $25, $26, $27, $28
		)
	`,
		snapshot.ID, snapshot.LiveSessionID, nullIfEmpty(snapshot.DecisionEventID), nullIfEmpty(snapshot.OrderID), snapshot.AccountID, snapshot.StrategyID, snapshot.Symbol, snapshot.Trigger, nullIfEmpty(snapshot.IntentSignature),
		snapshot.PositionFound, nullIfEmpty(snapshot.PositionSide), snapshot.PositionQuantity, snapshot.EntryPrice, snapshot.MarkPrice,
		snapshot.NetEquity, snapshot.AvailableBalance, snapshot.MarginBalance, snapshot.WalletBalance, snapshot.ExposureNotional, snapshot.OpenPositionCount,
		nullIfEmpty(snapshot.SyncStatus), snapshot.EventTime, snapshot.RecordedAt,
		marshalJSONValue(snapshot.PositionSnapshot), marshalJSONValue(snapshot.LivePositionState), marshalJSONValue(snapshot.AccountSnapshot), marshalJSONValue(snapshot.AccountSummary), marshalJSONValue(snapshot.Metadata),
	)
	return snapshot, err
}

func (s *Store) CreateOrderCloseVerification(item domain.OrderCloseVerification) (domain.OrderCloseVerification, error) {
	if item.ID == "" {
		item.ID = fmt.Sprintf("order-close-verification-%d", time.Now().UTC().UnixNano())
	}
	item.Symbol = strings.ToUpper(strings.TrimSpace(item.Symbol))
	if item.EventTime.IsZero() {
		item.EventTime = time.Now().UTC()
	}
	if item.RecordedAt.IsZero() {
		item.RecordedAt = time.Now().UTC()
	}
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	metadataRaw, _ := json.Marshal(item.Metadata)

	_, err := s.db.Exec(`
		insert into order_close_verifications (
			id, live_session_id, order_id, decision_event_id, account_id, strategy_id, symbol,
			verified_closed, remaining_position_qty, verification_source, event_time, recorded_at, metadata
		) values (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13
		)
	`,
		item.ID, item.LiveSessionID, item.OrderID, nullIfEmpty(item.DecisionEventID), item.AccountID, item.StrategyID, item.Symbol,
		item.VerifiedClosed, item.RemainingPositionQty, item.VerificationSource, item.EventTime, item.RecordedAt, metadataRaw,
	)
	return item, err
}

func (s *Store) QueryOrderCloseVerifications(query domain.OrderCloseVerificationQuery) ([]domain.OrderCloseVerification, error) {
	builder := strings.Builder{}
	builder.WriteString(`
		select id, live_session_id, order_id, decision_event_id, account_id, strategy_id, symbol,
			verified_closed, remaining_position_qty, verification_source, event_time, recorded_at, metadata
		from order_close_verifications
		where 1=1
	`)
	var args []any

	if strings.TrimSpace(query.LiveSessionID) != "" {
		args = append(args, strings.TrimSpace(query.LiveSessionID))
		builder.WriteString(fmt.Sprintf(" and live_session_id = $%d", len(args)))
	}
	if strings.TrimSpace(query.OrderID) != "" {
		args = append(args, strings.TrimSpace(query.OrderID))
		builder.WriteString(fmt.Sprintf(" and order_id = $%d", len(args)))
	}
	if strings.TrimSpace(query.AccountID) != "" {
		args = append(args, strings.TrimSpace(query.AccountID))
		builder.WriteString(fmt.Sprintf(" and account_id = $%d", len(args)))
	}
	if strings.TrimSpace(query.StrategyID) != "" {
		args = append(args, strings.TrimSpace(query.StrategyID))
		builder.WriteString(fmt.Sprintf(" and strategy_id = $%d", len(args)))
	}
	if strings.TrimSpace(query.Symbol) != "" {
		args = append(args, strings.ToUpper(strings.TrimSpace(query.Symbol)))
		builder.WriteString(fmt.Sprintf(" and symbol = $%d", len(args)))
	}
	if len(query.OrderIDs) > 0 {
		placeholders := make([]string, 0, len(query.OrderIDs))
		for _, id := range query.OrderIDs {
			if strings.TrimSpace(id) == "" {
				continue
			}
			args = append(args, strings.TrimSpace(id))
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
		}
		if len(placeholders) > 0 {
			builder.WriteString(fmt.Sprintf(" and order_id in (%s)", strings.Join(placeholders, ",")))
		}
	}

	builder.WriteString(" order by event_time desc, recorded_at desc, id desc")

	if query.Limit > 0 {
		args = append(args, query.Limit)
		builder.WriteString(fmt.Sprintf(" limit $%d", len(args)))
	}

	rows, err := s.db.Query(builder.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.OrderCloseVerification
	for rows.Next() {
		var item domain.OrderCloseVerification
		var decisionEventID sql.NullString
		var metadataRaw []byte

		if err := rows.Scan(
			&item.ID, &item.LiveSessionID, &item.OrderID, &decisionEventID, &item.AccountID, &item.StrategyID, &item.Symbol,
			&item.VerifiedClosed, &item.RemainingPositionQty, &item.VerificationSource, &item.EventTime, &item.RecordedAt, &metadataRaw,
		); err != nil {
			return nil, err
		}

		item.DecisionEventID = decisionEventID.String
		item.Metadata = map[string]any{}
		if len(metadataRaw) > 0 {
			_ = json.Unmarshal(metadataRaw, &item.Metadata)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListMarketBars(exchange, symbol, timeframe string, from, to int64, limit int) ([]domain.MarketBar, error) {
	query := `
		select id, exchange, symbol, timeframe, open_time, close_time, open, high, low, close, volume, is_closed, source, updated_at
		from market_bars
		where ($1 = '' or upper(exchange) = upper($1))
		  and ($2 = '' or upper(symbol) = upper($2))
		  and ($3 = '' or lower(timeframe) = lower($3))
		  and ($4 = 0 or open_time >= to_timestamp($4))
		  and ($5 = 0 or open_time <= to_timestamp($5))
		order by open_time asc
	`
	args := []any{exchange, symbol, timeframe, from, to}
	if limit > 0 {
		query += ` limit $6`
		args = append(args, limit)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.MarketBar{}
	for rows.Next() {
		var item domain.MarketBar
		if err := rows.Scan(
			&item.ID,
			&item.Exchange,
			&item.Symbol,
			&item.Timeframe,
			&item.OpenTime,
			&item.CloseTime,
			&item.Open,
			&item.High,
			&item.Low,
			&item.Close,
			&item.Volume,
			&item.IsClosed,
			&item.Source,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UpsertMarketBars(bars []domain.MarketBar) error {
	for _, item := range bars {
		if item.OpenTime.IsZero() {
			continue
		}
		if strings.TrimSpace(item.ID) == "" {
			item.ID = strings.ToUpper(strings.TrimSpace(item.Exchange)) + "|" +
				strings.ToUpper(strings.TrimSpace(item.Symbol)) + "|" +
				strings.ToLower(strings.TrimSpace(item.Timeframe)) + "|" +
				item.OpenTime.UTC().Format(time.RFC3339)
		}
		if item.UpdatedAt.IsZero() {
			item.UpdatedAt = time.Now().UTC()
		}
		if strings.TrimSpace(item.Source) == "" {
			item.Source = "exchange"
		}
		_, err := s.db.Exec(`
			insert into market_bars (
				id, exchange, symbol, timeframe, open_time, close_time, open, high, low, close, volume, is_closed, source, updated_at
			) values (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
			)
			on conflict (exchange, symbol, timeframe, open_time) do update set
				close_time = excluded.close_time,
				open = excluded.open,
				high = excluded.high,
				low = excluded.low,
				close = excluded.close,
				volume = excluded.volume,
				is_closed = excluded.is_closed,
				source = excluded.source,
				updated_at = excluded.updated_at
		`,
			item.ID,
			item.Exchange,
			item.Symbol,
			item.Timeframe,
			item.OpenTime,
			item.CloseTime,
			item.Open,
			item.High,
			item.Low,
			item.Close,
			item.Volume,
			item.IsClosed,
			item.Source,
			item.UpdatedAt,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}

type signalRuntimeScanner interface {
	Scan(dest ...any) error
}

func scanSignalRuntimeSession(scanner signalRuntimeScanner) (domain.SignalRuntimeSession, error) {
	var item domain.SignalRuntimeSession
	var stateRaw []byte
	err := scanner.Scan(
		&item.ID,
		&item.AccountID,
		&item.StrategyID,
		&item.Status,
		&item.RuntimeAdapter,
		&item.Transport,
		&item.SubscriptionCnt,
		&stateRaw,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return domain.SignalRuntimeSession{}, err
	}
	item.State = unmarshalJSONMap(stateRaw)
	return item, nil
}

func marshalJSONValue(value any) []byte {
	if value == nil {
		return []byte(`{}`)
	}
	raw, _ := json.Marshal(value)
	if len(raw) == 0 || string(raw) == "null" {
		return []byte(`{}`)
	}
	return raw
}

func unmarshalJSONMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func accountStatusForMode(mode string) string {
	if mode == "LIVE" {
		return "PENDING_SETUP"
	}
	return "READY"
}
