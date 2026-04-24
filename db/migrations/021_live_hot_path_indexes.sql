create index if not exists idx_orders_account_status_created_at
    on orders (account_id, status, created_at desc);

create index if not exists idx_orders_live_session_created_at
    on orders ((metadata->>'liveSessionId'), created_at asc);

create index if not exists idx_orders_account_settlement_required
    on orders (account_id, symbol, created_at desc)
    where metadata->>'immediateFillSyncRequired' = 'true';

create index if not exists idx_positions_account_symbol_updated_at
    on positions (account_id, symbol, updated_at desc);

create index if not exists idx_fills_order_created_at
    on fills (order_id, created_at asc);
