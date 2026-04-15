create table if not exists strategy_decision_events (
    id text primary key,
    live_session_id text not null references live_sessions(id) on delete cascade,
    runtime_session_id text,
    account_id text not null references accounts(id),
    strategy_id text not null references strategies(id),
    strategy_version_id text references strategy_versions(id),
    symbol text not null,
    trigger_type text,
    action text not null,
    reason text not null,
    signal_kind text,
    decision_state text,
    intent_signature text,
    source_gate_ready boolean not null default false,
    missing_count integer not null default 0,
    stale_count integer not null default 0,
    event_time timestamptz not null,
    recorded_at timestamptz not null default now(),
    trigger_summary jsonb not null default '{}'::jsonb,
    source_gate jsonb not null default '{}'::jsonb,
    source_states jsonb not null default '{}'::jsonb,
    signal_bar_states jsonb not null default '{}'::jsonb,
    position_snapshot jsonb not null default '{}'::jsonb,
    decision_metadata jsonb not null default '{}'::jsonb,
    signal_intent jsonb not null default '{}'::jsonb,
    execution_proposal jsonb not null default '{}'::jsonb,
    evaluation_context jsonb not null default '{}'::jsonb
);

create index if not exists idx_strategy_decision_events_live_session_event_time
    on strategy_decision_events (live_session_id, event_time desc);
create index if not exists idx_strategy_decision_events_account_event_time
    on strategy_decision_events (account_id, event_time desc);

create table if not exists order_execution_events (
    id text primary key,
    order_id text not null references orders(id) on delete cascade,
    exchange_order_id text,
    live_session_id text references live_sessions(id) on delete set null,
    decision_event_id text references strategy_decision_events(id) on delete set null,
    runtime_session_id text,
    account_id text not null references accounts(id),
    strategy_version_id text references strategy_versions(id),
    symbol text not null,
    side text not null,
    order_type text not null,
    event_type text not null,
    status text not null,
    execution_strategy text,
    execution_decision text,
    execution_mode text,
    quantity numeric(24, 8) not null default 0,
    price numeric(24, 8) not null default 0,
    expected_price numeric(24, 8) not null default 0,
    price_drift_bps numeric(18, 8) not null default 0,
    raw_quantity numeric(24, 8) not null default 0,
    normalized_quantity numeric(24, 8) not null default 0,
    raw_price_reference numeric(24, 8) not null default 0,
    normalized_price numeric(24, 8) not null default 0,
    spread_bps numeric(18, 8) not null default 0,
    book_imbalance numeric(18, 8) not null default 0,
    submit_latency_ms integer not null default 0,
    sync_latency_ms integer not null default 0,
    fill_latency_ms integer not null default 0,
    event_time timestamptz not null,
    recorded_at timestamptz not null default now(),
    fallback boolean not null default false,
    post_only boolean not null default false,
    reduce_only boolean not null default false,
    failed boolean not null default false,
    error text,
    runtime_preflight jsonb not null default '{}'::jsonb,
    dispatch_summary jsonb not null default '{}'::jsonb,
    adapter_submission jsonb not null default '{}'::jsonb,
    adapter_sync jsonb not null default '{}'::jsonb,
    normalization jsonb not null default '{}'::jsonb,
    symbol_rules jsonb not null default '{}'::jsonb,
    metadata jsonb not null default '{}'::jsonb
);

create index if not exists idx_order_execution_events_order_event_time
    on order_execution_events (order_id, event_time desc);
create index if not exists idx_order_execution_events_live_session_event_time
    on order_execution_events (live_session_id, event_time desc);

create table if not exists position_account_snapshots (
    id text primary key,
    live_session_id text not null references live_sessions(id) on delete cascade,
    decision_event_id text references strategy_decision_events(id) on delete set null,
    order_id text references orders(id) on delete set null,
    account_id text not null references accounts(id),
    strategy_id text not null references strategies(id),
    symbol text not null,
    trigger text not null,
    intent_signature text,
    position_found boolean not null default false,
    position_side text,
    position_quantity numeric(24, 8) not null default 0,
    entry_price numeric(24, 8) not null default 0,
    mark_price numeric(24, 8) not null default 0,
    net_equity numeric(24, 8) not null default 0,
    available_balance numeric(24, 8) not null default 0,
    margin_balance numeric(24, 8) not null default 0,
    wallet_balance numeric(24, 8) not null default 0,
    exposure_notional numeric(24, 8) not null default 0,
    open_position_count integer not null default 0,
    sync_status text,
    event_time timestamptz not null,
    recorded_at timestamptz not null default now(),
    position_snapshot jsonb not null default '{}'::jsonb,
    live_position_state jsonb not null default '{}'::jsonb,
    account_snapshot jsonb not null default '{}'::jsonb,
    account_summary jsonb not null default '{}'::jsonb,
    metadata jsonb not null default '{}'::jsonb
);

create index if not exists idx_position_account_snapshots_live_session_event_time
    on position_account_snapshots (live_session_id, event_time desc);
create index if not exists idx_position_account_snapshots_account_event_time
    on position_account_snapshots (account_id, event_time desc);
