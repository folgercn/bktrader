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

- `A_sep_0p25`: t3 requires `abs(breakout_level - sma5) >= 0.25 * atr`.
- `B_trend_sep_0p25`: A plus trend direction filter.
- `C_atr_pct30_sep_0p25`: A plus ATR percentile >= `30%`.
- `D_trend_atr_pct30_sep_0p25`: A plus trend direction and ATR percentile >= `30%`.

## Results

| Timeframe | Scenario | Filters | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Entry Mix | Breakout Locks | Quality Rejects |
|---|---|---|---:|---:|---:|---:|---:|---:|---|---|---|
| `30min` | `t3_sma5_baseline` | `none` | 531,886.72 | 431.89% | -2.10% | 3251 | 80.28% | 13.42 | `SL-Reentry:2101, Zero-Initial-Reentry:1150` | `long original_t2:447/t3_swing:127; short original_t2:471/t3_swing:113` | `` |
| `30min` | `A_sep_0p25` | `{"min_sma_atr_separation": 0.25}` | 536,109.15 | 436.11% | -2.04% | 3226 | 80.60% | 13.49 | `SL-Reentry:2086, Zero-Initial-Reentry:1140` | `long original_t2:449/t3_swing:119; short original_t2:470/t3_swing:109` | `long sma_atr_separation:9; short sma_atr_separation:5` |
| `30min` | `B_trend_sep_0p25` | `{"min_sma_atr_separation": 0.25, "trend": true}` | 515,280.51 | 415.28% | -2.04% | 3136 | 80.64% | 13.57 | `SL-Reentry:2030, Zero-Initial-Reentry:1106` | `long original_t2:448/t3_swing:103; short original_t2:473/t3_swing:89` | `long trend_long:34/sma_atr_separation:11; short trend_short:30/sma_atr_separation:5` |
| `30min` | `C_atr_pct30_sep_0p25` | `{"min_atr_percentile": 30.0, "min_sma_atr_separation": 0.25}` | 519,646.71 | 419.65% | -1.79% | 3037 | 81.00% | 13.61 | `SL-Reentry:1969, Zero-Initial-Reentry:1068` | `long original_t2:449/t3_swing:79; short original_t2:476/t3_swing:71` | `long atr_percentile:40/sma_atr_separation:20; short atr_percentile:38/sma_atr_separation:14` |
| `30min` | `D_trend_atr_pct30_sep_0p25` | `{"min_atr_percentile": 30.0, "min_sma_atr_separation": 0.25, "trend": true}` | 500,219.35 | 400.22% | -1.79% | 2985 | 80.90% | 13.65 | `SL-Reentry:1936, Zero-Initial-Reentry:1049` | `long original_t2:448/t3_swing:69; short original_t2:478/t3_swing:61` | `long trend_long:35/atr_percentile:34/sma_atr_separation:21; short atr_percentile:28/trend_short:30/sma_atr_separation:13` |

## Delta vs t3_sma5 Baseline

| Timeframe | Scenario | Final Balance Delta | Return Delta | Max DD Delta | Trades Delta | Win Rate Delta | Sharpe Delta |
|---|---|---:|---:|---:|---:|---:|---:|
| `30min` | `A_sep_0p25` | 4,222.43 | 4.22 pp | 0.06 pp | -25 | 0.32 pp | 0.07 |
| `30min` | `B_trend_sep_0p25` | -16,606.21 | -16.61 pp | 0.06 pp | -115 | 0.36 pp | 0.15 |
| `30min` | `C_atr_pct30_sep_0p25` | -12,240.01 | -12.24 pp | 0.31 pp | -214 | 0.72 pp | 0.19 |
| `30min` | `D_trend_atr_pct30_sep_0p25` | -31,667.37 | -31.67 pp | 0.31 pp | -266 | 0.62 pp | 0.23 |

## Breakout Attribution

| Timeframe | Scenario | Shape | Trades | Win Rate | Avg PnL | Median PnL | PnL Std | Worst PnL | Net PnL | Shape PnL DD | Profit Factor |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `30min` | `t3_sma5_baseline` | `original_t2` | 2576 | 87.97% | 0.5411% | 0.4008% | 0.6352% | -0.1811% | 553,447.23 | -219.54 | 61.5178 |
| `30min` | `t3_sma5_baseline` | `t3_swing` | 675 | 85.33% | 0.5157% | 0.3537% | 0.6280% | -0.1188% | 136,907.94 | -215.84 | 51.2283 |
| `30min` | `A_sep_0p25` | `original_t2` | 2575 | 87.96% | 0.5409% | 0.4009% | 0.6352% | -0.1811% | 555,845.31 | -221.00 | 61.861 |
| `30min` | `A_sep_0p25` | `t3_swing` | 651 | 87.10% | 0.5355% | 0.3717% | 0.6343% | -0.1188% | 137,520.44 | -217.37 | 64.946 |
| `30min` | `B_trend_sep_0p25` | `original_t2` | 2582 | 87.99% | 0.5399% | 0.4008% | 0.6342% | -0.1811% | 543,747.33 | -214.13 | 61.9276 |
| `30min` | `B_trend_sep_0p25` | `t3_swing` | 554 | 86.64% | 0.5476% | 0.3864% | 0.6266% | -0.1365% | 115,305.65 | -212.60 | 62.9664 |
| `30min` | `C_atr_pct30_sep_0p25` | `original_t2` | 2596 | 87.94% | 0.5383% | 0.3965% | 0.6335% | -0.1811% | 549,677.40 | -214.80 | 61.5891 |
| `30min` | `C_atr_pct30_sep_0p25` | `t3_swing` | 441 | 87.76% | 0.6460% | 0.4636% | 0.7090% | -0.1097% | 108,202.07 | -213.67 | 74.0498 |
| `30min` | `D_trend_atr_pct30_sep_0p25` | `original_t2` | 2600 | 87.96% | 0.5376% | 0.3965% | 0.6328% | -0.1811% | 538,455.99 | -208.75 | 61.5903 |
| `30min` | `D_trend_atr_pct30_sep_0p25` | `t3_swing` | 385 | 87.01% | 0.6467% | 0.4682% | 0.6908% | -0.1365% | 90,818.18 | -209.81 | 67.1991 |

## Read

This run keeps the `t3_sma5_baseline` sizing and only filters the added t3 breakout lock. The original_t2 path is left unchanged, so deltas isolate signal-quality filtering rather than position sizing.

## Conclusion

- A (`sep_0p25`) is still the best primary candidate: return improves `+4.22 pp`, MaxDD improves `0.06 pp`, Sharpe improves `+0.07`, and trades drop `25`.
- Adding trend on top of A is defensive but gives back too much return: B improves Sharpe more (`+0.15`) and cuts `115` trades, but return falls `16.61 pp` vs baseline and `20.83 pp` vs A.
- Adding ATR percentile on top of A is the better defensive overlay: C improves MaxDD `0.31 pp`, Sharpe `+0.19`, win rate `+0.72 pp`, and cuts `214` trades, while giving back `12.24 pp` vs baseline and `16.46 pp` vs A.
- Adding both filters on top of A over-constrains the signal: D has the highest Sharpe delta (`+0.23`) and lowest trade count, but loses `31.67 pp` return vs baseline.

Recommended ranking: A for primary 30min candidate; C as the risk-off candidate when drawdown/trade count matters more than raw return.
