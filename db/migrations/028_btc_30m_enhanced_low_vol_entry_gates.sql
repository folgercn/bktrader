update strategy_versions
set parameters = jsonb_set(
    jsonb_set(parameters, '{reentry_min_stop_bps}', '6.0'::jsonb, true),
    '{reentry_atr_percentile_gte}',
    '25.0'::jsonb,
    true
)
where id = 'strategy-version-bk-btc-30m-enhanced-v010'
  and strategy_id = 'strategy-bk-btc-30m-enhanced';

update live_sessions
set state = jsonb_set(
    jsonb_set(state, '{reentry_min_stop_bps}', '6.0'::jsonb, true),
    '{reentry_atr_percentile_gte}',
    '25.0'::jsonb,
    true
)
where strategy_id = 'strategy-bk-btc-30m-enhanced'
  and (
    state->>'launchTemplateKey' = 'binance-testnet-btc-30m-enhanced'
    or state->>'strategyEngine' = 'bk-live-intrabar-sma5-t3-sep'
  );
