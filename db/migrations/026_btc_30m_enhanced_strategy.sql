insert into strategies (id, name, status, description, created_at)
values (
    'strategy-bk-btc-30m-enhanced',
    'BK BTCUSDT 30m Enhanced',
    'ACTIVE',
    'BTCUSDT 30m live intrabar SMA5 baseline plus t3 breakout with sep_0p25.',
    now()
)
on conflict (id) do nothing;

insert into strategy_versions (
    id,
    strategy_id,
    version,
    signal_timeframe,
    execution_timeframe,
    parameters,
    created_at
)
values (
    'strategy-version-bk-btc-30m-enhanced-v010',
    'strategy-bk-btc-30m-enhanced',
    'v0.1.0',
    '30m',
    'tick',
    '{
        "strategyEngine": "bk-live-intrabar-sma5-t3-sep",
        "symbol": "BTCUSDT",
        "signalTimeframe": "30m",
        "executionDataSource": "tick",
        "positionSizingMode": "reentry_size_schedule",
        "dir2_zero_initial": true,
        "zero_initial_mode": "reentry_window",
        "max_trades_per_bar": 2,
        "reentry_size_schedule": [0.20, 0.10],
        "breakout_shape": "baseline_plus_t3",
        "t3_min_sma_atr_separation": 0.25,
        "use_sma5_intraday_structure": true,
        "stop_mode": "atr",
        "stop_loss_atr": 0.3,
        "profit_protect_atr": 1.0,
        "trailing_stop_atr": 0.3,
        "delayed_trailing_activation_atr": 0.5,
        "long_reentry_atr": 0.1,
        "short_reentry_atr": 0.0,
        "tradingFeeBps": 10.0,
        "fundingRateBps": 0.0,
        "fundingIntervalHours": 8
    }'::jsonb,
    now()
)
on conflict (id) do nothing;
