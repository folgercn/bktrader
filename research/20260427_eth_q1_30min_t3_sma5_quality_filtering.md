# ETH Q1 2026 30min t3_sma5 Signal Quality Filtering

Scope: research-only backtest work. No live or execution path was changed.

## Setup

- Symbol/window: `ETHUSDT`, `2026-01-01 00:00:00+00:00` to `2026-03-31 23:59:59+00:00`
- Execution bars: continuous `1s` bars rebuilt from raw Binance trades
- Main comparison baseline: `t3_sma5_baseline` with full-size schedule `[0.20, 0.10]`
- Sizing baseline: `dir2_zero_initial=true`, `zero_initial_mode=reentry_window`, `reentry_size_schedule=[0.20, 0.10]`, `max_trades_per_bar=2`
- Shared risk params: `stop_mode=atr`, `stop_loss_atr=0.05`, `trailing_stop_atr=0.3`, `delayed_trailing_activation=0.5`

## Replay Mode

- `live_intrabar_sma5`: live-safe intrabar mode. Each replayed second updates the current signal bar close/high/low from data seen so far and computes `sma5/ma5` from four closed signal bars plus the current realtime close.

## Breakout Shapes

- Baseline long: `prev_t2.high > prev_t1.high` and current price crosses `prev_t2.high`.
- Added long: `prev_t3.high > prev_t2.high`, `prev_t3.high > prev_t1.high`, `prev_t1.high > prev_t2.high`, and current price crosses `prev_t3.high`.
- The short side uses the symmetric low-side condition.

## Optimization Variants

- `t3_sma5_trend_filter`: t3 long requires `close_now > sma5` and `sma5_slope > 0`; t3 short requires `close_now < sma5` and `sma5_slope < 0`.
- `t3_sma5_sma_atr_sep_0p1`: t3 requires `abs(breakout_level - sma5) >= 0.1 * atr`.
- `t3_sma5_atr_percentile_gte_30`: t3 requires the signal bar ATR percentile to be at least `30%` over the rolling ATR sample.

## Results

| Timeframe | Scenario | Filters | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Entry Mix | Breakout Locks | Quality Rejects |
|---|---|---|---:|---:|---:|---:|---:|---:|---|---|---|
| `30min` | `t3_sma5_baseline` | `none` | 531,886.72 | 431.89% | -2.10% | 3251 | 80.28% | 13.42 | `SL-Reentry:2101, Zero-Initial-Reentry:1150` | `long original_t2:447/t3_swing:127; short original_t2:471/t3_swing:113` | `` |
| `30min` | `t3_sma5_trend_filter` | `{"trend": true}` | 518,859.46 | 418.86% | -2.10% | 3167 | 80.45% | 13.56 | `SL-Reentry:2049, Zero-Initial-Reentry:1118` | `long original_t2:446/t3_swing:113; short original_t2:474/t3_swing:93` | `long trend_long:34; short trend_short:30` |
| `30min` | `t3_sma5_sma_atr_sep_0p1` | `{"min_sma_atr_separation": 0.1}` | 531,886.72 | 431.89% | -2.10% | 3251 | 80.28% | 13.42 | `SL-Reentry:2101, Zero-Initial-Reentry:1150` | `long original_t2:447/t3_swing:127; short original_t2:471/t3_swing:113` | `` |
| `30min` | `t3_sma5_atr_percentile_gte_30` | `{"min_atr_percentile": 30.0}` | 517,661.93 | 417.66% | -1.85% | 3060 | 80.69% | 13.54 | `SL-Reentry:1984, Zero-Initial-Reentry:1076` | `long original_t2:447/t3_swing:85; short original_t2:477/t3_swing:74` | `long atr_percentile:42; short atr_percentile:39` |

## Delta vs t3_sma5 Baseline

| Timeframe | Scenario | Final Balance Delta | Return Delta | Max DD Delta | Trades Delta | Win Rate Delta | Sharpe Delta |
|---|---|---:|---:|---:|---:|---:|---:|
| `30min` | `t3_sma5_trend_filter` | -13,027.26 | -13.03 pp | 0.00 pp | -84 | 0.17 pp | 0.14 |
| `30min` | `t3_sma5_sma_atr_sep_0p1` | 0.00 | 0.00 pp | 0.00 pp | 0 | 0.00 pp | 0.00 |
| `30min` | `t3_sma5_atr_percentile_gte_30` | -14,224.79 | -14.23 pp | 0.25 pp | -191 | 0.41 pp | 0.12 |

## Breakout Attribution

| Timeframe | Scenario | Shape | Trades | Win Rate | Avg PnL | Median PnL | PnL Std | Worst PnL | Net PnL | Shape PnL DD | Profit Factor |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `30min` | `t3_sma5_baseline` | `original_t2` | 2576 | 87.97% | 0.5411% | 0.4008% | 0.6352% | -0.1811% | 553,447.23 | -219.54 | 61.5178 |
| `30min` | `t3_sma5_baseline` | `t3_swing` | 675 | 85.33% | 0.5157% | 0.3537% | 0.6280% | -0.1188% | 136,907.94 | -215.84 | 51.2283 |
| `30min` | `t3_sma5_trend_filter` | `original_t2` | 2583 | 88.00% | 0.5400% | 0.4008% | 0.6342% | -0.1811% | 544,069.92 | -215.19 | 61.5424 |
| `30min` | `t3_sma5_trend_filter` | `t3_swing` | 584 | 85.27% | 0.5377% | 0.3796% | 0.6214% | -0.1365% | 121,978.46 | -211.41 | 54.3655 |
| `30min` | `t3_sma5_sma_atr_sep_0p1` | `original_t2` | 2576 | 87.97% | 0.5411% | 0.4008% | 0.6352% | -0.1811% | 553,447.23 | -219.54 | 61.5178 |
| `30min` | `t3_sma5_sma_atr_sep_0p1` | `t3_swing` | 675 | 85.33% | 0.5157% | 0.3537% | 0.6280% | -0.1188% | 136,907.94 | -215.84 | 51.2283 |
| `30min` | `t3_sma5_atr_percentile_gte_30` | `original_t2` | 2597 | 87.95% | 0.5385% | 0.3962% | 0.6335% | -0.1811% | 549,011.70 | -214.26 | 61.2655 |
| `30min` | `t3_sma5_atr_percentile_gte_30` | `t3_swing` | 463 | 85.31% | 0.6192% | 0.4428% | 0.7049% | -0.1143% | 108,775.50 | -213.12 | 54.2307 |

## Read

This run keeps the `t3_sma5_baseline` sizing and only filters the added t3 breakout lock. The original_t2 path is left unchanged, so deltas isolate signal-quality filtering rather than position sizing.

## Conclusion

- `t3_sma5_trend_filter` is the cleanest Sharpe improvement in this batch: Sharpe improves `+0.14`, trades drop `84`, and win rate improves `+0.17 pp`, while return gives back `13.03 pp`. MaxDD is unchanged.
- `t3_sma5_atr_percentile_gte_30` is the best risk/trade-count filter: trades drop `191`, MaxDD improves `0.25 pp`, win rate improves `+0.41 pp`, and Sharpe improves `+0.12`, while return gives back `14.23 pp`.
- `t3_sma5_sma_atr_sep_0p1` has no effect on this Q1 30min replay. The `0.1 * atr` threshold is too loose for this signal path.

Next useful experiment: combine `trend_filter` and `atr_percentile_gte_30`, then separately try stricter SMA separation thresholds such as `0.25 * atr` and `0.50 * atr`.
