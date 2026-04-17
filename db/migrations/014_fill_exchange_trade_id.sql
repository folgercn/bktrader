alter table fills
add column if not exists exchange_trade_id text;

create unique index if not exists idx_fills_order_trade_id
on fills (order_id, exchange_trade_id);
