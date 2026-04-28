alter table fills
add column if not exists fill_source text;

update fills
set fill_source = case
    when exchange_trade_id is not null and btrim(exchange_trade_id) <> '' then 'real'
    when dedup_fallback_fingerprint is not null and dedup_fallback_fingerprint like 'synthetic-remainder|%' then 'remainder'
    when dedup_fallback_fingerprint is not null and btrim(dedup_fallback_fingerprint) <> '' then 'synthetic'
    else 'real'
end
where fill_source is null or btrim(fill_source) = '';

alter table fills
alter column fill_source set default 'real';

alter table fills
alter column fill_source set not null;

do $$
begin
    alter table fills
    add constraint fills_fill_source_check
    check (fill_source in ('real', 'synthetic', 'remainder', 'paper', 'manual'));
exception
    when duplicate_object then null;
end $$;

create index if not exists idx_fills_order_fill_source
on fills (order_id, fill_source);
