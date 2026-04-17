alter table fills
add column if not exists dedup_fallback_fingerprint text;

create unique index if not exists idx_fills_order_fallback_fingerprint
on fills (order_id, dedup_fallback_fingerprint);
