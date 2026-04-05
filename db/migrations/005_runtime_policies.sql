create table if not exists runtime_policies (
    id integer primary key,
    trade_tick_freshness_seconds integer not null,
    order_book_freshness_seconds integer not null,
    signal_bar_freshness_seconds integer not null,
    runtime_quiet_seconds integer not null,
    paper_start_readiness_timeout_seconds integer not null,
    updated_at timestamptz not null default now()
);
