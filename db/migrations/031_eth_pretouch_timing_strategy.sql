insert into strategies (id, name, status, description, created_at)
values (
    'strategy-bk-eth-pretouch-timing',
    'BK Live ETH Pretouch Timing',
    'ACTIVE',
    'ETHUSDT 1h live pretouch timing strategy with Go-native timing classifier and RF probability sizing.',
    now()
)
on conflict (id) do update
set name = excluded.name,
    status = excluded.status,
    description = excluded.description;

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
    'strategy-version-bk-eth-pretouch-timing-v010',
    'strategy-bk-eth-pretouch-timing',
    'v0.1.0',
    '1h',
    'tick',
    '{
        "strategyEngine": "bk-live-eth-pretouch-timing",
        "symbol": "ETHUSDT",
        "signalTimeframe": "1h",
        "executionDataSource": "tick",
        "positionSizingMode": "intent_quantity",
        "defaultOrderQuantity": 0.100,
        "pretouchBaseOrderQuantity": 0.100,
        "pretouchBaseShare": 0.80,
        "pretouchCostQ50Threshold": 0.116865,
        "pretouchCostQ50Penalty": 0.50,
        "pretouchSpeedThreshold": 0.228106,
        "pretouchMaxPreTouchSec": 1800.0,
        "pretouchMaxEff300s": 1.0,
        "stop_loss_atr": 0.45,
        "breakeven_at_r": 0.8,
        "trail_start_r": 1.5,
        "trail_buffer_atr": 0.05,
        "max_hold_hours": 2.0,
        "executionStrategy": "book-aware-v1",
        "executionEntryOrderType": "MARKET",
        "executionEntryMaxSpreadBps": 8,
        "executionEntryMaxSlippageBps": 8,
        "executionEntryMaxBookAgeMs": 500,
        "executionEntryMinTopBookCoverage": 0.5,
        "executionEntryMaxSourceDivergenceBps": 8,
        "executionSLExitOrderType": "MARKET",
        "executionSLExitMaxSpreadBps": 8,
        "executionSLMaxSlippageBps": 8,
        "dispatchCooldownSeconds": 30,
        "signalBindings": [
            {
                "sourceKey": "binance-kline",
                "sourceName": "Binance Futures Kline",
                "exchange": "BINANCE",
                "role": "signal",
                "streamType": "signal_bar",
                "symbol": "ETHUSDT",
                "status": "ACTIVE",
                "options": {"timeframe": "1h"},
                "timeframe": "1h"
            },
            {
                "sourceKey": "binance-trade-tick",
                "sourceName": "Binance Futures Trade Tick",
                "exchange": "BINANCE",
                "role": "trigger",
                "streamType": "trade_tick",
                "symbol": "ETHUSDT",
                "status": "ACTIVE",
                "options": {},
                "timeframe": ""
            },
            {
                "sourceKey": "binance-order-book",
                "sourceName": "Binance Futures Order Book",
                "exchange": "BINANCE",
                "role": "feature",
                "streamType": "order_book",
                "symbol": "ETHUSDT",
                "status": "ACTIVE",
                "options": {},
                "timeframe": ""
            }
        ]
    }'::jsonb,
    now()
)
on conflict (id) do update
set signal_timeframe = excluded.signal_timeframe,
    execution_timeframe = excluded.execution_timeframe,
    parameters = excluded.parameters;

update strategy_versions
set parameters = jsonb_set(
    parameters || '{
        "strategyEngine": "bk-live-intrabar-sma5-t2-only-0p5bps",
        "symbol": "BTCUSDT",
        "signalTimeframe": "30m"
    }'::jsonb,
    '{signalBindings}',
    '[
        {
            "sourceKey": "binance-kline",
            "sourceName": "Binance Futures Kline",
            "exchange": "BINANCE",
            "role": "signal",
            "streamType": "signal_bar",
            "symbol": "BTCUSDT",
            "status": "ACTIVE",
            "options": {"timeframe": "30m"},
            "timeframe": "30m"
        },
        {
            "sourceKey": "binance-trade-tick",
            "sourceName": "Binance Futures Trade Tick",
            "exchange": "BINANCE",
            "role": "trigger",
            "streamType": "trade_tick",
            "symbol": "BTCUSDT",
            "status": "ACTIVE",
            "options": {},
            "timeframe": ""
        },
        {
            "sourceKey": "binance-order-book",
            "sourceName": "Binance Futures Order Book",
            "exchange": "BINANCE",
            "role": "feature",
            "streamType": "order_book",
            "symbol": "BTCUSDT",
            "status": "ACTIVE",
            "options": {},
            "timeframe": ""
        }
    ]'::jsonb,
    true
)
where id = 'strategy-version-bk-btc-30m-enhanced-v010'
  and strategy_id = 'strategy-bk-btc-30m-enhanced';

update live_sessions
set strategy_id = 'strategy-bk-eth-pretouch-timing',
    state = state || '{
        "strategyEngine": "bk-live-eth-pretouch-timing",
        "strategyVersionId": "strategy-version-bk-eth-pretouch-timing-v010",
        "symbol": "ETHUSDT",
        "signalTimeframe": "1h",
        "launchTemplateKey": "binance-testnet-eth-pretouch-timing",
        "launchTemplateName": "Binance Testnet ETHUSDT Pretouch Timing",
        "launchTemplateSymbol": "ETHUSDT",
        "launchTemplateTimeframe": "1h"
    }'::jsonb
where state->>'launchTemplateKey' = 'binance-testnet-eth-pretouch-timing'
  and strategy_id = 'strategy-bk-btc-30m-enhanced';

update signal_runtime_sessions as s
set strategy_id = 'strategy-bk-eth-pretouch-timing',
    state = jsonb_set(
        jsonb_set(
            state || '{
                "launchTemplateKey": "binance-testnet-eth-pretouch-timing",
                "launchTemplateName": "Binance Testnet ETHUSDT Pretouch Timing",
                "launchTemplateSymbol": "ETHUSDT",
                "launchTemplateTimeframe": "1h"
            }'::jsonb,
            '{sourceStates}',
            (
                coalesce(s.state->'sourceStates', '{}'::jsonb)
                - '|trigger|BTCUSDT'
                - 'binance-kline|signal|BTCUSDT|30m'
                - 'binance-trade-tick|trigger|BTCUSDT'
                - 'binance-order-book|feature|BTCUSDT'
            ),
            true
        ),
        '{signalBarStates}',
        (
            coalesce(s.state->'signalBarStates', '{}'::jsonb)
            - 'binance-kline|signal|BTCUSDT|30m'
        ),
        true
    )
where s.state->>'launchTemplateKey' = 'binance-testnet-eth-pretouch-timing'
  and s.strategy_id = 'strategy-bk-btc-30m-enhanced'
  and not exists (
      select 1
      from signal_runtime_sessions existing
      where existing.account_id = s.account_id
        and existing.strategy_id = 'strategy-bk-eth-pretouch-timing'
        and existing.id <> s.id
  );
