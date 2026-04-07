create table if not exists notification_deliveries (
  notification_id text not null,
  channel text not null,
  sent_at timestamptz not null,
  updated_at timestamptz not null,
  primary key (notification_id, channel)
);
