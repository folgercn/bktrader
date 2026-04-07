create table if not exists live_sessions (
    id text primary key,
    account_id text not null references accounts(id),
    strategy_id text not null references strategies(id),
    status text not null,
    state jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now()
);
