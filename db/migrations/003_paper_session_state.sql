alter table paper_sessions
add column if not exists state jsonb not null default '{}'::jsonb;
