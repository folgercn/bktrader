alter table telegram_configs
  add column if not exists trade_events_enabled boolean not null default true,
  add column if not exists position_report_enabled boolean not null default true,
  add column if not exists position_report_interval_minutes integer not null default 30;
