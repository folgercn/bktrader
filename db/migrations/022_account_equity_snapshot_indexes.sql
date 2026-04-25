create index if not exists idx_account_equity_snapshots_account_created_at
    on account_equity_snapshots (account_id, created_at desc);
