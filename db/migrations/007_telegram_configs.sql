create table if not exists telegram_configs (
  id integer primary key,
  enabled boolean not null,
  bot_token text not null,
  chat_id text not null,
  send_levels jsonb not null default '[]'::jsonb,
  updated_at timestamptz not null
);
