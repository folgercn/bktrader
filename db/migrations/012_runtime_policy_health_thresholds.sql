alter table runtime_policies
    add column if not exists strategy_evaluation_quiet_seconds integer not null default 15,
    add column if not exists live_account_sync_freshness_seconds integer not null default 60;
