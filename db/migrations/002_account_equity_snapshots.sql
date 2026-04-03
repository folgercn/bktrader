create table if not exists account_equity_snapshots (
    id text primary key,
    account_id text not null references accounts(id),
    start_equity numeric(24, 8) not null,
    realized_pnl numeric(24, 8) not null default 0,
    unrealized_pnl numeric(24, 8) not null default 0,
    fees numeric(24, 8) not null default 0,
    net_equity numeric(24, 8) not null,
    exposure_notional numeric(24, 8) not null default 0,
    open_position_count integer not null default 0,
    created_at timestamptz not null default now()
);
