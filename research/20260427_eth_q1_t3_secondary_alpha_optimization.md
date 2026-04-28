# ETH Q1 2026 t-3 Breakout Optimization, 1s Replay

Scope: research-only backtest work. No live or execution path was changed.

## Setup

- Symbol/window: `ETHUSDT`, `2026-01-01 00:00:00+00:00` to `2026-03-31 23:59:59+00:00`
- Execution bars: continuous `1s` bars rebuilt from raw Binance trades
- Main comparison baseline: `live_intrabar_sma5_baseline_plus_t3_breakout` with t3 full-size schedule `[0.20, 0.10]`
- Original sizing baseline: `dir2_zero_initial=true`, `zero_initial_mode=reentry_window`, `reentry_size_schedule=[0.20, 0.10]`, `max_trades_per_bar=2`
- Shared risk params: `stop_mode=atr`, `stop_loss_atr=0.05`, `trailing_stop_atr=0.3`, `delayed_trailing_activation=0.5`

## Replay Mode

- `live_intrabar_sma5`: live-safe intrabar mode. Each replayed second updates the current signal bar close/high/low from data seen so far and computes `sma5/ma5` from four closed signal bars plus the current realtime close.

## Breakout Shapes

- Baseline long: `prev_t2.high > prev_t1.high` and current price crosses `prev_t2.high`.
- Added long: `prev_t3.high > prev_t2.high`, `prev_t3.high > prev_t1.high`, `prev_t1.high > prev_t2.high`, and current price crosses `prev_t3.high`.
- The short side uses the symmetric low-side condition.

## Optimization Variants

- `live_intrabar_sma5_t3_half_size`: t3_swing real reentry sizing `[0.10, 0.05]`; original_t2 stays `[0.20, 0.10]`.
- `live_intrabar_sma5_t3_half_size_cooldown1/2`: 30min-only half-size t3 plus a t3-only cooldown of 1 or 2 signal bars after a t3 lock.

## Results

| Timeframe | Scenario | T3 Schedule | T3 Cooldown | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Entry Mix | Breakout Locks |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---|---|
| `4h` | `live_intrabar_sma5_baseline_plus_t3_breakout` | `[0.2, 0.1]` | 0 | 236,994.21 | 136.99% | -0.24% | 374 | 92.51% | 18.77 | `SL-Reentry:239, Zero-Initial-Reentry:135` | `long original_t2:46/t3_swing:14; short original_t2:64/t3_swing:12` |
| `4h` | `live_intrabar_sma5_t3_half_size` | `[0.1, 0.05]` | 0 | 215,980.36 | 115.98% | -0.24% | 374 | 92.51% | 18.77 | `SL-Reentry:239, Zero-Initial-Reentry:135` | `long original_t2:46/t3_swing:14; short original_t2:64/t3_swing:12` |
| `1h` | `live_intrabar_sma5_baseline_plus_t3_breakout` | `[0.2, 0.1]` | 0 | 481,070.05 | 381.07% | -0.76% | 1634 | 85.19% | 14.39 | `SL-Reentry:1035, Zero-Initial-Reentry:599` | `long original_t2:219/t3_swing:92; short t3_swing:77/original_t2:213` |
| `1h` | `live_intrabar_sma5_t3_half_size` | `[0.1, 0.05]` | 0 | 385,321.81 | 285.32% | -0.65% | 1634 | 85.19% | 14.39 | `SL-Reentry:1035, Zero-Initial-Reentry:599` | `long original_t2:219/t3_swing:92; short t3_swing:77/original_t2:213` |
| `30min` | `live_intrabar_sma5_baseline_plus_t3_breakout` | `[0.2, 0.1]` | 0 | 531,886.72 | 431.89% | -2.10% | 3251 | 80.28% | 13.42 | `SL-Reentry:2101, Zero-Initial-Reentry:1150` | `long original_t2:447/t3_swing:127; short original_t2:471/t3_swing:113` |
| `30min` | `live_intrabar_sma5_t3_half_size` | `[0.1, 0.05]` | 0 | 452,621.79 | 352.62% | -1.91% | 3251 | 80.28% | 13.42 | `SL-Reentry:2101, Zero-Initial-Reentry:1150` | `long original_t2:447/t3_swing:127; short original_t2:471/t3_swing:113` |
| `30min` | `live_intrabar_sma5_t3_half_size_cooldown1` | `[0.1, 0.05]` | 1 | 453,582.21 | 353.58% | -1.86% | 3247 | 80.32% | 13.43 | `SL-Reentry:2099, Zero-Initial-Reentry:1148` | `long original_t2:447/t3_swing:124; short original_t2:472/t3_swing:113` |
| `30min` | `live_intrabar_sma5_t3_half_size_cooldown2` | `[0.1, 0.05]` | 2 | 451,192.47 | 351.19% | -1.86% | 3218 | 80.27% | 13.42 | `SL-Reentry:2082, Zero-Initial-Reentry:1136` | `long original_t2:448/t3_swing:117; short original_t2:472/t3_swing:107` |

## Delta vs Full-Size t3 Baseline

| Timeframe | Scenario | Final Balance Delta | Return Delta | Max DD Delta | Trades Delta | Win Rate Delta | Sharpe Delta |
|---|---|---:|---:|---:|---:|---:|---:|
| `4h` | `live_intrabar_sma5_t3_half_size` | -21,013.85 | -21.01 pp | 0.00 pp | 0 | 0.00 pp | 0.00 |
| `1h` | `live_intrabar_sma5_t3_half_size` | -95,748.24 | -95.75 pp | 0.11 pp | 0 | 0.00 pp | 0.00 |
| `30min` | `live_intrabar_sma5_t3_half_size` | -79,264.93 | -79.27 pp | 0.19 pp | 0 | 0.00 pp | 0.00 |
| `30min` | `live_intrabar_sma5_t3_half_size_cooldown1` | -78,304.51 | -78.31 pp | 0.24 pp | -4 | 0.04 pp | 0.01 |
| `30min` | `live_intrabar_sma5_t3_half_size_cooldown2` | -80,694.25 | -80.70 pp | 0.24 pp | -33 | -0.01 pp | 0.00 |

## Breakout Attribution

| Timeframe | Scenario | Shape | Trades | Win Rate | Avg PnL | Median PnL | PnL Std | Worst PnL | Net PnL | Shape PnL DD | Profit Factor |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `4h` | `live_intrabar_sma5_baseline_plus_t3_breakout` | `original_t2` | 312 | 91.67% | 1.6052% | 1.2912% | 1.4432% | -0.2138% | 125,328.66 | -209.76 | 136.7618 |
| `4h` | `live_intrabar_sma5_baseline_plus_t3_breakout` | `t3_swing` | 62 | 96.77% | 2.2348% | 2.3361% | 1.3286% | -0.1613% | 29,715.41 | -53.99 | 551.3826 |
| `4h` | `live_intrabar_sma5_t3_half_size` | `original_t2` | 312 | 91.67% | 1.6052% | 1.2912% | 1.4432% | -0.2138% | 117,584.02 | -191.65 | 137.2045 |
| `4h` | `live_intrabar_sma5_t3_half_size` | `t3_swing` | 62 | 96.77% | 2.2348% | 2.3361% | 1.3286% | -0.1613% | 14,135.14 | -26.75 | 529.395 |
| `1h` | `live_intrabar_sma5_baseline_plus_t3_breakout` | `original_t2` | 1179 | 88.89% | 0.8306% | 0.6353% | 0.8688% | -0.1805% | 354,537.63 | -237.38 | 75.9616 |
| `1h` | `live_intrabar_sma5_baseline_plus_t3_breakout` | `t3_swing` | 455 | 86.15% | 0.8399% | 0.6193% | 1.0377% | -0.2268% | 146,691.41 | -155.89 | 75.4769 |
| `1h` | `live_intrabar_sma5_t3_half_size` | `original_t2` | 1179 | 88.89% | 0.8306% | 0.6353% | 0.8688% | -0.1805% | 311,945.23 | -215.26 | 75.8124 |
| `1h` | `live_intrabar_sma5_t3_half_size` | `t3_swing` | 455 | 86.15% | 0.8399% | 0.6193% | 1.0377% | -0.2268% | 64,095.79 | -68.12 | 74.1792 |
| `30min` | `live_intrabar_sma5_baseline_plus_t3_breakout` | `original_t2` | 2576 | 87.97% | 0.5411% | 0.4008% | 0.6352% | -0.1811% | 553,447.23 | -219.54 | 61.5178 |
| `30min` | `live_intrabar_sma5_baseline_plus_t3_breakout` | `t3_swing` | 675 | 85.33% | 0.5157% | 0.3537% | 0.6280% | -0.1188% | 136,907.94 | -215.84 | 51.2283 |
| `30min` | `live_intrabar_sma5_t3_half_size` | `original_t2` | 2576 | 87.97% | 0.5411% | 0.4008% | 0.6352% | -0.1811% | 501,451.59 | -192.06 | 61.4243 |
| `30min` | `live_intrabar_sma5_t3_half_size` | `t3_swing` | 675 | 85.33% | 0.5157% | 0.3537% | 0.6280% | -0.1188% | 62,002.25 | -96.90 | 50.4756 |
| `30min` | `live_intrabar_sma5_t3_half_size_cooldown1` | `original_t2` | 2578 | 87.98% | 0.5410% | 0.4014% | 0.6350% | -0.1811% | 502,456.69 | -192.46 | 61.4481 |
| `30min` | `live_intrabar_sma5_t3_half_size_cooldown1` | `t3_swing` | 669 | 85.35% | 0.5219% | 0.3578% | 0.6325% | -0.1188% | 62,200.39 | -97.10 | 50.9412 |
| `30min` | `live_intrabar_sma5_t3_half_size_cooldown2` | `original_t2` | 2581 | 87.99% | 0.5410% | 0.4026% | 0.6346% | -0.1811% | 500,955.93 | -191.66 | 61.4796 |
| `30min` | `live_intrabar_sma5_t3_half_size_cooldown2` | `t3_swing` | 637 | 85.09% | 0.5232% | 0.3569% | 0.6396% | -0.1188% | 59,706.15 | -96.70 | 51.6143 |

## Read

The optimization keeps the `live_intrabar_sma5_baseline_plus_t3_breakout` signal logic as the comparison baseline and only changes t3_swing sizing/cooldown. Because `dir2_zero_initial=true`, the lock itself remains a proof gate; real sizing still starts from the reentry window.

The key read is whether the t3-only risk constraints improve Sharpe or drawdown relative to full-size t3 without giving back too much return.

## Conclusion

- `4h`: do not reduce t3_swing size. The full-size t3 baseline remains the best version here: half-size gives up `21.01 pp` return with no Sharpe or MaxDD improvement.
- `1h`: half-size is a partial risk constraint, not an optimization. MaxDD improves by `0.11 pp`, but return drops `95.75 pp` and Sharpe is unchanged.
- `30min`: half-size plus cooldown1 is the best risk-constrained variant among the tested options, but it is still not better than full-size t3 overall. It improves MaxDD by `0.24 pp`, reduces trades by `4`, and adds only `0.01` Sharpe, while giving up `78.31 pp` return.
- `30min cooldown2`: reduces more trades (`-33`) but does not improve Sharpe beyond baseline and gives up the most return among 30min variants.

The attribution supports the PR comment's interpretation: t3_swing is real positive alpha, especially on `4h` where it has higher win rate, average PnL, median PnL, lower worst trade, and stronger profit factor than original_t2. The Sharpe pressure on shorter frames is not fixed by simply halving t3 size; the next useful constraint should be signal-quality filtering rather than just smaller sizing.
