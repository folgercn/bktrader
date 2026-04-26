create table if not exists signal_runtime_sessions (
    id text primary key,
    account_id text not null references accounts(id),
    strategy_id text not null references strategies(id),
    status text not null,
    runtime_adapter text not null default '',
    transport text not null default '',
    subscription_count integer not null default 0,
    state jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint signal_runtime_sessions_status_nonempty check (length(trim(status)) > 0)
);

create unique index if not exists signal_runtime_sessions_account_strategy_uidx
    on signal_runtime_sessions (account_id, strategy_id);

create index if not exists signal_runtime_sessions_updated_at_idx
    on signal_runtime_sessions (updated_at desc);
