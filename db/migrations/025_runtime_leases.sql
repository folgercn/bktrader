create table if not exists runtime_leases (
  resource_type text not null,
  resource_id text not null,
  owner_id text not null,
  expires_at timestamptz not null,
  acquired_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  primary key (resource_type, resource_id)
);

create index if not exists idx_runtime_leases_owner_updated
  on runtime_leases(owner_id, updated_at desc);

create index if not exists idx_runtime_leases_expires_at
  on runtime_leases(expires_at);
