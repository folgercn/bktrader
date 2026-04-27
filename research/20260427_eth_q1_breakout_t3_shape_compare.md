# ETH Q1 2026 t-3 Breakout Shape, 1s Replay

Scope: research-only backtest work. No live or execution path was changed.

## Setup

- Symbol/window: `ETHUSDT`, `2026-01-01 00:00:00+00:00` to `2026-03-31 23:59:59+00:00`
- Execution bars: continuous `1s` bars rebuilt from raw Binance trades
- Baseline sizing: `dir2_zero_initial=true`, `zero_initial_mode=reentry_window`, `reentry_size_schedule=[0.20, 0.10]`, `max_trades_per_bar=2`
- Shared risk params: `stop_mode=atr`, `stop_loss_atr=0.05`, `trailing_stop_atr=0.3`, `delayed_trailing_activation=0.5`

## Replay Modes

- `same_bar_parity`: parity mode against the corrected runner. Signal-frame fields are reused inside each replayed signal bar, so this mode is for apples-to-apples research comparison only.
- `live_intrabar_sma5`: live-safe intrabar mode. Each replayed second updates the current signal bar close/high/low from data seen so far and computes `sma5/ma5` from four closed signal bars plus the current realtime close.

## Breakout Shapes

- Baseline long: `prev_t2.high > prev_t1.high` and current price crosses `prev_t2.high`.
- Added long: `prev_t3.high > prev_t2.high`, `prev_t3.high > prev_t1.high`, `prev_t1.high > prev_t2.high`, and current price crosses `prev_t3.high`.
- The short side uses the symmetric low-side condition.

## Results

| Timeframe | Replay Mode | Scenario | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Entry Mix | Breakout Locks |
|---|---|---|---:|---:|---:|---:|---:|---:|---|---|
| `4h` | `same_bar_parity` | `baseline_original_breakout` | 187,430.87 | 87.43% | -0.24% | 244 | 95.90% | 21.23 | `SL-Reentry:146, Zero-Initial-Reentry:98` | `long original_t2:43; short original_t2:56` |
| `4h` | `same_bar_parity` | `baseline_plus_t3_breakout` | 221,906.64 | 121.91% | -0.24% | 304 | 95.72% | 21.75 | `SL-Reentry:181, Zero-Initial-Reentry:123` | `long original_t2:43/t3_swing:13; short original_t2:56/t3_swing:12` |
| `4h` | `live_intrabar_sma5` | `live_intrabar_sma5_baseline_original_breakout` | 198,492.35 | 98.49% | -0.24% | 316 | 91.77% | 17.72 | `SL-Reentry:205, Zero-Initial-Reentry:111` | `long original_t2:47; short original_t2:65` |
| `4h` | `live_intrabar_sma5` | `live_intrabar_sma5_baseline_plus_t3_breakout` | 236,994.21 | 136.99% | -0.24% | 374 | 92.51% | 18.77 | `SL-Reentry:239, Zero-Initial-Reentry:135` | `long original_t2:46/t3_swing:14; short original_t2:64/t3_swing:12` |
| `1h` | `same_bar_parity` | `baseline_original_breakout` | 294,106.86 | 194.11% | -0.50% | 1053 | 88.51% | 16.03 | `SL-Reentry:657, Zero-Initial-Reentry:396` | `long original_t2:216; short original_t2:184` |
| `1h` | `same_bar_parity` | `baseline_plus_t3_breakout` | 453,294.14 | 353.29% | -0.60% | 1443 | 88.91% | 15.23 | `SL-Reentry:894, Zero-Initial-Reentry:549` | `long original_t2:216/t3_swing:84; short t3_swing:69/original_t2:184` |
| `1h` | `live_intrabar_sma5` | `live_intrabar_sma5_baseline_original_breakout` | 315,247.73 | 215.25% | -0.55% | 1247 | 85.49% | 14.99 | `SL-Reentry:804, Zero-Initial-Reentry:443` | `long original_t2:226; short original_t2:220` |
| `1h` | `live_intrabar_sma5` | `live_intrabar_sma5_baseline_plus_t3_breakout` | 481,070.05 | 381.07% | -0.76% | 1634 | 85.19% | 14.39 | `SL-Reentry:1035, Zero-Initial-Reentry:599` | `long original_t2:219/t3_swing:92; short t3_swing:77/original_t2:213` |
| `30min` | `same_bar_parity` | `baseline_original_breakout` | 372,719.52 | 272.72% | -0.99% | 2246 | 84.33% | 14.46 | `SL-Reentry:1421, Zero-Initial-Reentry:824, PT-Reentry:1` | `long original_t2:416; short original_t2:417` |
| `30min` | `same_bar_parity` | `baseline_plus_t3_breakout` | 521,068.99 | 421.07% | -1.24% | 2868 | 83.47% | 14.33 | `SL-Reentry:1810, Zero-Initial-Reentry:1057, PT-Reentry:1` | `long original_t2:416/t3_swing:129; short original_t2:417/t3_swing:106` |
| `30min` | `live_intrabar_sma5` | `live_intrabar_sma5_baseline_original_breakout` | 389,936.51 | 289.94% | -1.71% | 2637 | 80.70% | 13.50 | `SL-Reentry:1705, Zero-Initial-Reentry:931, PT-Reentry:1` | `long original_t2:452; short original_t2:484` |
| `30min` | `live_intrabar_sma5` | `live_intrabar_sma5_baseline_plus_t3_breakout` | 531,886.72 | 431.89% | -2.10% | 3251 | 80.28% | 13.42 | `SL-Reentry:2101, Zero-Initial-Reentry:1150` | `long original_t2:447/t3_swing:127; short original_t2:471/t3_swing:113` |

## Delta vs Baseline

| Timeframe | Replay Mode | Final Balance Delta | Return Delta | Max DD Delta | Trades Delta | Win Rate Delta | Sharpe Delta |
|---|---|---:|---:|---:|---:|---:|---:|
| `4h` | `live_intrabar_sma5` | 38,501.86 | 38.50 pp | 0.00 pp | 58 | 0.74 pp | 1.05 |
| `4h` | `same_bar_parity` | 34,475.77 | 34.48 pp | 0.00 pp | 60 | -0.18 pp | 0.52 |
| `1h` | `live_intrabar_sma5` | 165,822.32 | 165.82 pp | -0.21 pp | 387 | -0.30 pp | -0.60 |
| `1h` | `same_bar_parity` | 159,187.28 | 159.18 pp | -0.10 pp | 390 | 0.40 pp | -0.80 |
| `30min` | `live_intrabar_sma5` | 141,950.21 | 141.95 pp | -0.39 pp | 614 | -0.42 pp | -0.08 |
| `30min` | `same_bar_parity` | 148,349.47 | 148.35 pp | -0.25 pp | 622 | -0.86 pp | -0.13 |

## Read

The variant keeps the baseline logic and only broadens the initial breakout lock shape. Because `dir2_zero_initial=true`, the lock itself remains a proof gate; real sizing still starts from the reentry window as `20%` then `10%` inside the same signal bar.

The key read is the `live_intrabar_sma5` delta, because that mode avoids using final current-bar signal high/low/close/ATR before those values are available in replay time.
