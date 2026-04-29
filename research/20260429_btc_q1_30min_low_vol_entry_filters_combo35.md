# BTCUSDT Q1 2026 30min Low-Volatility Entry Gates

Scope: research-only Python replay. No live or execution path is changed by this report.

## Setup

- Symbol/window: `BTCUSDT`, `2026-01-01T00:00:00+00:00` to `2026-03-31T23:59:59+00:00`
- Execution bars: continuous `1s` bars rebuilt from Binance trade archives
- Signal timeframe: `30min`
- Replay mode: `live_intrabar_sma5`, breakout shape `baseline_plus_t3`, t3 SMA/ATR separation `0.25`
- Sizing: `dir2_zero_initial=true`, `zero_initial_mode=reentry_window`, `reentry_size_schedule=[0.20, 0.10]`, `max_trades_per_bar=2`
- Gate point: reentry price resolution, so the gate blocks both Zero-Initial-Reentry and SL-Reentry entries. Stop-loss exits are not filtered.

## Results

| Variant | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Avg Loss | Worst Loss | Entry Mix | Gate Reject Calls |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|
| `min_stop_bps_6_atr_pct_gte_35` | 227,154.89 | 127.15% | -1.13% | 2017 | 84.04% | 14.74 | -0.2218% | -0.7003% | `SL-Reentry:1344, Zero-Initial-Reentry:673` | 1146020 |

## Delta vs Baseline

| Variant | Final Delta | Return Delta | Max DD Delta | Trades Delta | Win Delta | Sharpe Delta |
|---|---:|---:|---:|---:|---:|---:|

## Variants

- `min_stop_bps_6_atr_pct_gte_35`: Require both min stop distance of 6 bps and ATR percentile at least 35.
