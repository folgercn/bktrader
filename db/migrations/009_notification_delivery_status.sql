alter table notification_deliveries
  add column if not exists status text not null default 'sent',
  add column if not exists last_error text not null default '',
  add column if not exists attempted_at timestamptz not null default now();
