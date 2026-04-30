update strategies
set name = 'BK BTCUSDT 30m T2-only 0.5bps',
    description = 'BTCUSDT 30m live intrabar SMA5 original T2 breakout with 0.5 bps tolerance.'
where id = 'strategy-bk-btc-30m-enhanced';

update strategy_versions
set parameters = (
    parameters
    || '{
        "strategyEngine": "bk-live-intrabar-sma5-t2-only-0p5bps",
        "breakout_shape": "original_t2",
        "breakout_shape_tolerance_bps": 0.5,
        "use_sma5_intraday_structure": true
    }'::jsonb
) - 't3_min_sma_atr_separation'
where id = 'strategy-version-bk-btc-30m-enhanced-v010'
  and strategy_id = 'strategy-bk-btc-30m-enhanced';

update live_sessions
set state = (
    state
    || '{
        "strategyEngine": "bk-live-intrabar-sma5-t2-only-0p5bps",
        "breakout_shape": "original_t2",
        "breakout_shape_tolerance_bps": 0.5,
        "use_sma5_intraday_structure": true
    }'::jsonb
) - 't3_min_sma_atr_separation'
where strategy_id = 'strategy-bk-btc-30m-enhanced'
  and (
    state->>'launchTemplateKey' = 'binance-testnet-btc-30m-enhanced'
    or state->>'strategyEngine' = 'bk-live-intrabar-sma5-t3-sep'
    or state->>'strategyEngine' = 'bk-live-intrabar-sma5-t2-only-0p5bps'
  );
