create table if not exists strategies (
    id text primary key,
    name text not null,
    status text not null,
    description text,
    created_at timestamptz not null default now()
);

create table if not exists strategy_versions (
    id text primary key,
    strategy_id text not null references strategies(id),
    version text not null,
    signal_timeframe text not null,
    execution_timeframe text not null,
    parameters jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now()
);

create table if not exists accounts (
    id text primary key,
    name text not null,
    mode text not null,
    exchange text not null,
    status text not null,
    created_at timestamptz not null default now()
);

create table if not exists signals (
    id text primary key,
    strategy_version_id text not null references strategy_versions(id),
    symbol text not null,
    side text not null,
    reason text not null,
    metadata jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now()
);

create table if not exists orders (
    id text primary key,
    account_id text not null references accounts(id),
    strategy_version_id text references strategy_versions(id),
    symbol text not null,
    side text not null,
    type text not null,
    status text not null,
    quantity numeric(24, 8) not null,
    price numeric(24, 8),
    metadata jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now()
);

create table if not exists fills (
    id text primary key,
    order_id text not null references orders(id),
    price numeric(24, 8) not null,
    quantity numeric(24, 8) not null,
    fee numeric(24, 8) not null default 0,
    created_at timestamptz not null default now()
);

create table if not exists positions (
    id text primary key,
    account_id text not null references accounts(id),
    strategy_version_id text references strategy_versions(id),
    symbol text not null,
    side text not null,
    quantity numeric(24, 8) not null,
    entry_price numeric(24, 8) not null,
    mark_price numeric(24, 8),
    updated_at timestamptz not null default now()
);

create table if not exists backtest_runs (
    id text primary key,
    strategy_version_id text not null references strategy_versions(id),
    status text not null,
    parameters jsonb not null default '{}'::jsonb,
    result_summary jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now()
);

create table if not exists paper_sessions (
    id text primary key,
    account_id text not null references accounts(id),
    strategy_id text not null references strategies(id),
    status text not null,
    start_equity numeric(24, 8) not null,
    created_at timestamptz not null default now()
);
