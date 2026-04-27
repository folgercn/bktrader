# ETH Q1 2026 t-3 Breakout Shape, 1s Replay

Scope: research-only backtest work. No live or execution path was changed.

## Setup

- Symbol/window: `ETHUSDT`, `2026-01-01 00:00:00+00:00` to `2026-03-31 23:59:59+00:00`
- Execution bars: continuous `1s` bars rebuilt from raw Binance trades
- Baseline sizing: `dir2_zero_initial=true`, `zero_initial_mode=reentry_window`, `reentry_size_schedule=[0.20, 0.10]`, `max_trades_per_bar=2`
- Shared risk params: `stop_mode=atr`, `stop_loss_atr=0.05`, `trailing_stop_atr=0.3`, `delayed_trailing_activation=0.5`

## Breakout Shapes

- Baseline long: `prev_t2.high > prev_t1.high` and current price crosses `prev_t2.high`.
- Added long: `prev_t3.high > prev_t2.high`, `prev_t3.high > prev_t1.high`, `prev_t1.high > prev_t2.high`, and current price crosses `prev_t3.high`.
- The short side uses the symmetric low-side condition.

## Results

Baseline parity check: the regenerated `30min` baseline ledger exactly matches the previous corrected runner ledger (`3,440` rows, same times/types/reasons, max price and balance delta `0.0`).

| Timeframe | Scenario | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Entry Mix | Breakout Locks |
|---|---|---:|---:|---:|---:|---:|---:|---|---|
| `4h` | `baseline_original_breakout` | 151,400.17 | 51.40% | -0.13% | 180 | 94.44% | 21.61 | `SL-Reentry:106, Zero-Initial-Reentry:74` | `long original_t2:29; short original_t2:45` |
| `4h` | `baseline_plus_t3_breakout` | 170,899.32 | 70.90% | -0.13% | 218 | 95.41% | 22.25 | `SL-Reentry:128, Zero-Initial-Reentry:90` | `long original_t2:29/t3_swing:7; short original_t2:45/t3_swing:9` |
| `1h` | `baseline_original_breakout` | 221,038.12 | 121.04% | -0.45% | 802 | 86.78% | 16.06 | `SL-Reentry:503, Zero-Initial-Reentry:299` | `long original_t2:150; short original_t2:152` |
| `1h` | `baseline_plus_t3_breakout` | 307,720.82 | 207.72% | -0.54% | 1111 | 85.96% | 14.72 | `SL-Reentry:695, Zero-Initial-Reentry:416` | `long original_t2:150/t3_swing:62; short original_t2:151/t3_swing:56` |
| `30min` | `baseline_original_breakout` | 263,492.46 | 163.49% | -0.90% | 1720 | 81.98% | 15.47 | `SL-Reentry:1081, Zero-Initial-Reentry:638, PT-Reentry:1` | `long original_t2:303; short original_t2:341` |
| `30min` | `baseline_plus_t3_breakout` | 336,155.55 | 236.16% | -1.13% | 2193 | 81.71% | 15.23 | `SL-Reentry:1371, Zero-Initial-Reentry:821, PT-Reentry:1` | `long original_t2:303/t3_swing:99; short t3_swing:85/original_t2:340` |

## Delta vs Baseline

| Timeframe | Final Balance Delta | Return Delta | Max DD Delta | Trades Delta | Win Rate Delta | Sharpe Delta |
|---|---:|---:|---:|---:|---:|---:|
| `4h` | 19,499.15 | 19.50 pp | 0.00 pp | 38 | 0.97 pp | 0.64 |
| `1h` | 86,682.70 | 86.68 pp | -0.09 pp | 309 | -0.82 pp | -1.34 |
| `30min` | 72,663.09 | 72.67 pp | -0.23 pp | 473 | -0.27 pp | -0.24 |

## Read

The variant keeps the baseline logic and only broadens the initial breakout lock shape. Because `dir2_zero_initial=true`, the lock itself remains a proof gate; real sizing still starts from the reentry window as `20%` then `10%` inside the same signal bar.
