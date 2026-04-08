create table if not exists market_bars (
    id text primary key,
    exchange text not null,
    symbol text not null,
    timeframe text not null,
    open_time timestamptz not null,
    close_time timestamptz not null,
    open numeric(24, 8) not null,
    high numeric(24, 8) not null,
    low numeric(24, 8) not null,
    close numeric(24, 8) not null,
    volume numeric(24, 8) not null default 0,
    is_closed boolean not null default false,
    source text not null default 'exchange',
    updated_at timestamptz not null default now()
);

create unique index if not exists market_bars_exchange_symbol_timeframe_open_time_idx
    on market_bars (exchange, symbol, timeframe, open_time);
