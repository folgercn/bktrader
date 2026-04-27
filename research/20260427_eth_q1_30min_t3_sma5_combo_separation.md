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

- `t3_sma5_trend_and_atr_pct30`: combines the trend filter with ATR percentile >= `30%`.
- `t3_sma5_sma_atr_sep_0p25`: t3 requires `abs(breakout_level - sma5) >= 0.25 * atr`.
- `t3_sma5_sma_atr_sep_0p50`: t3 requires `abs(breakout_level - sma5) >= 0.50 * atr`.

## Results

| Timeframe | Scenario | Filters | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Entry Mix | Breakout Locks | Quality Rejects |
|---|---|---|---:|---:|---:|---:|---:|---:|---|---|---|
| `30min` | `t3_sma5_baseline` | `none` | 531,886.72 | 431.89% | -2.10% | 3251 | 80.28% | 13.42 | `SL-Reentry:2101, Zero-Initial-Reentry:1150` | `long original_t2:447/t3_swing:127; short original_t2:471/t3_swing:113` | `` |
| `30min` | `t3_sma5_trend_and_atr_pct30` | `{"min_atr_percentile": 30.0, "trend": true}` | 505,297.73 | 405.30% | -1.85% | 3012 | 80.71% | 13.64 | `SL-Reentry:1954, Zero-Initial-Reentry:1058` | `long original_t2:446/t3_swing:76; short original_t2:479/t3_swing:64` | `long trend_long:35/atr_percentile:37; short atr_percentile:29/trend_short:30` |
| `30min` | `t3_sma5_sma_atr_sep_0p25` | `{"min_sma_atr_separation": 0.25}` | 536,109.15 | 436.11% | -2.04% | 3226 | 80.60% | 13.49 | `SL-Reentry:2086, Zero-Initial-Reentry:1140` | `long original_t2:449/t3_swing:119; short original_t2:470/t3_swing:109` | `long sma_atr_separation:9; short sma_atr_separation:5` |
| `30min` | `t3_sma5_sma_atr_sep_0p50` | `{"min_sma_atr_separation": 0.5}` | 476,215.28 | 376.22% | -1.87% | 2863 | 81.28% | 13.61 | `SL-Reentry:1845, Zero-Initial-Reentry:1017, PT-Reentry:1` | `long original_t2:450/t3_swing:43; short original_t2:482/t3_swing:48` | `long sma_atr_separation:91; short sma_atr_separation:68` |

## Delta vs t3_sma5 Baseline

| Timeframe | Scenario | Final Balance Delta | Return Delta | Max DD Delta | Trades Delta | Win Rate Delta | Sharpe Delta |
|---|---|---:|---:|---:|---:|---:|---:|
| `30min` | `t3_sma5_trend_and_atr_pct30` | -26,588.99 | -26.59 pp | 0.25 pp | -239 | 0.43 pp | 0.22 |
| `30min` | `t3_sma5_sma_atr_sep_0p25` | 4,222.43 | 4.22 pp | 0.06 pp | -25 | 0.32 pp | 0.07 |
| `30min` | `t3_sma5_sma_atr_sep_0p50` | -55,671.44 | -55.67 pp | 0.23 pp | -388 | 1.00 pp | 0.19 |

## Breakout Attribution

| Timeframe | Scenario | Shape | Trades | Win Rate | Avg PnL | Median PnL | PnL Std | Worst PnL | Net PnL | Shape PnL DD | Profit Factor |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `30min` | `t3_sma5_baseline` | `original_t2` | 2576 | 87.97% | 0.5411% | 0.4008% | 0.6352% | -0.1811% | 553,447.23 | -219.54 | 61.5178 |
| `30min` | `t3_sma5_baseline` | `t3_swing` | 675 | 85.33% | 0.5157% | 0.3537% | 0.6280% | -0.1188% | 136,907.94 | -215.84 | 51.2283 |
| `30min` | `t3_sma5_trend_and_atr_pct30` | `original_t2` | 2601 | 87.97% | 0.5378% | 0.3962% | 0.6328% | -0.1811% | 540,269.41 | -210.63 | 61.2308 |
| `30min` | `t3_sma5_trend_and_atr_pct30` | `t3_swing` | 411 | 85.16% | 0.6351% | 0.4678% | 0.6870% | -0.1365% | 97,719.15 | -209.58 | 55.7738 |
| `30min` | `t3_sma5_sma_atr_sep_0p25` | `original_t2` | 2575 | 87.96% | 0.5409% | 0.4009% | 0.6352% | -0.1811% | 555,845.31 | -221.00 | 61.861 |
| `30min` | `t3_sma5_sma_atr_sep_0p25` | `t3_swing` | 651 | 87.10% | 0.5355% | 0.3717% | 0.6343% | -0.1188% | 137,520.44 | -217.37 | 64.946 |
| `30min` | `t3_sma5_sma_atr_sep_0p50` | `original_t2` | 2621 | 87.87% | 0.5369% | 0.3969% | 0.6316% | -0.1811% | 527,981.19 | -201.66 | 61.0195 |
| `30min` | `t3_sma5_sma_atr_sep_0p50` | `t3_swing` | 242 | 94.21% | 0.7532% | 0.5201% | 0.7700% | -0.1024% | 62,952.98 | -28.06 | 397.5286 |

## Read

This run keeps the `t3_sma5_baseline` sizing and only filters the added t3 breakout lock. The original_t2 path is left unchanged, so deltas isolate signal-quality filtering rather than position sizing.

## Conclusion

- `t3_sma5_sma_atr_sep_0p25` is the best candidate in this run. It improves return by `+4.22 pp`, MaxDD by `0.06 pp`, Sharpe by `+0.07`, win rate by `+0.32 pp`, and reduces trades by `25`.
- `t3_sma5_trend_and_atr_pct30` is a defensive filter. It improves MaxDD by `0.25 pp`, Sharpe by `+0.22`, and reduces trades by `239`, but gives back `26.59 pp` return.
- `t3_sma5_sma_atr_sep_0p50` is too strict for a primary 30min setting. It improves MaxDD by `0.23 pp`, Sharpe by `+0.19`, and win rate by `+1.00 pp`, but gives back `55.67 pp` return.

Recommended next candidate for live-aligned research is `t3_sma5_sma_atr_sep_0p25`, because it is the only tested filter that improves return and risk metrics together against `t3_sma5_baseline`.
