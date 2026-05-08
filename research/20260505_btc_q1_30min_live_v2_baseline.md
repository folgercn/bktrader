# BTCUSDT Q1 2026 30min Live-V2 Baseline

Scope: research-only Python backtest. No live or execution path is changed by this report.

## Setup

- Symbol/window: `BTCUSDT`, `2026-01-01T00:00:00+00:00` to `2026-03-31T23:59:59+00:00`
- Execution source: continuous `1s` bars rebuilt from Binance trade archives
- Signal timeframe: `30min`
- Structural logic retained: `baseline_plus_t3` breakout shape, zero-initial reentry window, SL, trailing stop, and profit protection
- New fill semantics: reentry trigger and fill use the observed `1s close` event-price proxy; fills no longer backfill to `prev_low_1 + ATR` / `prev_high_1 - ATR` planned price
- Optimization gates removed from the new baseline and kept for later sweeps.

## Removed Optimization Gates

- `t3_min_sma_atr_separation`
- `reentry_min_stop_bps`
- `reentry_atr_percentile_gte`
- `reentry_close_confirm`
- `reentry_delay_seconds`
- `reentry_actionability_bps`

## Results

| Variant | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Avg Loss | Worst Loss | Entry Mix |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `legacy_planned_fill_t3_sep_0p25` | 227,449.41 | 127.45% | -3.03% | 3177 | 76.46% | 13.23 | -0.1659% | -0.7003% | `SL-Reentry:2137, Zero-Initial-Reentry:1040` |
| `live_v2_no_gates_rolling` | 22,159.23 | -77.84% | -77.84% | 3240 | 18.02% | -5.93 | -0.2030% | -0.8392% | `SL-Reentry:2185, Zero-Initial-Reentry:1055` |
| `live_v2_no_gates_snapshot` | 22,159.23 | -77.84% | -77.84% | 3240 | 18.02% | -5.93 | -0.2030% | -0.8392% | `SL-Reentry:2185, Zero-Initial-Reentry:1055` |

## Delta vs Legacy

| Variant | Final Delta | Return Delta | Max DD Delta | Trades Delta | Win Delta | Sharpe Delta |
|---|---:|---:|---:|---:|---:|---:|
| `live_v2_no_gates_rolling` | -205,290.18 | -205.29 pp | -74.81 pp | 63 | -58.44 pp | -19.16 |
| `live_v2_no_gates_snapshot` | -205,290.18 | -205.29 pp | -74.81 pp | 63 | -58.44 pp | -19.16 |

## Read

The legacy row is a historical reference only. It uses planned-price fills at `re_p`, which is not a realistic live execution model after a breakout has already moved the market.

The live-v2 rows are intended to become the new optimization baseline. Gate sweeps should start from this baseline instead of from the legacy planned-fill result.
