update strategy_versions
set parameters = jsonb_set(parameters, '{stop_loss_atr}', '0.3'::jsonb, true)
where id = 'strategy-version-bk-btc-30m-enhanced-v010'
  and strategy_id = 'strategy-bk-btc-30m-enhanced';

update live_sessions
set state = jsonb_set(state, '{stop_loss_atr}', '0.3'::jsonb, true)
where strategy_id = 'strategy-bk-btc-30m-enhanced'
  and (
    state->>'launchTemplateKey' = 'binance-testnet-btc-30m-enhanced'
    or state->>'strategyEngine' = 'bk-live-intrabar-sma5-t3-sep'
  );
