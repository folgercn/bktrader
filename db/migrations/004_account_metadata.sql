alter table accounts
add column if not exists metadata jsonb not null default '{}'::jsonb;
