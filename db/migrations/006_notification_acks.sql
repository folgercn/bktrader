create table if not exists notification_acks (
  id text primary key,
  acked_at timestamptz not null,
  updated_at timestamptz not null
);
