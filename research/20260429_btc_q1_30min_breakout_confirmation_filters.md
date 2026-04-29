# BTC Q1 2026 30min Breakout Confirmation Filters

Scope: research-only Python replay. No live or execution path is changed by this report.

## Setup

- Symbol/window: `BTCUSDT`, `2026-01-01T00:00:00+00:00` to `2026-03-31T23:59:59+00:00`
- Execution bars: continuous `1s` bars rebuilt from Binance trade archives
- Signal timeframe: `30min`
- Replay mode: `live_intrabar_sma5`, breakout shape `baseline_plus_t3`, t3 SMA/ATR separation `0.25`
- Sizing: `dir2_zero_initial=true`, `zero_initial_mode=reentry_window`, `reentry_size_schedule=[0.20, 0.10]`, `max_trades_per_bar=2`
- Risk params use the BTC 30m enhanced/live-like research profile: `stop_loss_atr=0.3`, `trailing_stop_atr=0.3`, `delayed_trailing_activation=0.5`
- `confirm_2x_1s` / `confirm_3x_1s` are multi-tick proxies: they require consecutive 1s observations to cross the same breakout level.

## Results

| Variant | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Entry Mix | Filter Rejects |
|---|---:|---:|---:|---:|---:|---:|---|---|
| `baseline_single_observation` | 227,449.41 | 127.45% | -3.03% | 3177 | 76.46% | 13.23 | `SL-Reentry:2137, Zero-Initial-Reentry:1040` | `margin L/S 0/0; confirm L/S 0/0` |
| `margin_0p02atr` | 227,787.82 | 127.79% | -2.89% | 3111 | 76.73% | 13.36 | `SL-Reentry:2092, Zero-Initial-Reentry:1019` | `margin L/S 6648/7380; confirm L/S 0/0` |
| `confirm_2x_1s` | 226,802.02 | 126.80% | -2.85% | 3126 | 76.52% | 13.22 | `SL-Reentry:2101, Zero-Initial-Reentry:1025` | `margin L/S 0/0; confirm L/S 1502/1490` |
| `confirm_3x_1s` | 227,609.94 | 127.61% | -2.86% | 3079 | 76.78% | 13.30 | `SL-Reentry:2071, Zero-Initial-Reentry:1008` | `margin L/S 0/0; confirm L/S 2877/2907` |

## Delta vs Baseline

| Variant | Final Balance Delta | Return Delta | Max DD Delta | Trades Delta | Win Rate Delta | Sharpe Delta |
|---|---:|---:|---:|---:|---:|---:|
| `margin_0p02atr` | 338.41 | 0.34 pp | 0.14 pp | -66 | 0.27 pp | 0.13 |
| `confirm_2x_1s` | -647.39 | -0.65 pp | 0.18 pp | -51 | 0.06 pp | -0.01 |
| `confirm_3x_1s` | 160.53 | 0.16 pp | 0.17 pp | -98 | 0.32 pp | 0.07 |

## Breakout Attribution

| Variant | Shape | Trades | Win Rate | Avg PnL | Net PnL | Worst PnL | Profit Factor |
|---|---|---:|---:|---:|---:|---:|---:|
| `baseline_single_observation` | `original_t2` | 2441 | 89.47% | 0.3705% | 209,920.87 | -0.7003% | 21.2922 |
| `baseline_single_observation` | `t3_swing` | 736 | 90.35% | 0.3802% | 66,030.95 | -0.6977% | 24.3702 |
| `margin_0p02atr` | `original_t2` | 2383 | 89.55% | 0.3743% | 207,343.76 | -0.7003% | 21.7115 |
| `margin_0p02atr` | `t3_swing` | 728 | 90.52% | 0.3832% | 65,741.71 | -0.6977% | 25.1944 |
| `confirm_2x_1s` | `original_t2` | 2399 | 89.41% | 0.3726% | 206,785.14 | -0.7003% | 21.0589 |
| `confirm_2x_1s` | `t3_swing` | 727 | 90.37% | 0.3827% | 65,491.28 | -0.6977% | 24.4563 |
| `confirm_3x_1s` | `original_t2` | 2357 | 89.56% | 0.3753% | 204,943.75 | -0.7003% | 22.0497 |
| `confirm_3x_1s` | `t3_swing` | 722 | 90.44% | 0.3860% | 65,641.31 | -0.6977% | 25.2337 |

## Variants

- `baseline_single_observation`: Current research behavior: one 1s high/low crossing can lock the zero-initial window.
- `margin_0p02atr`: Require breakout probe to clear the level by at least 0.02 ATR.
- `confirm_2x_1s`: Require two consecutive 1s observations crossing the same breakout level.
- `confirm_3x_1s`: Require three consecutive 1s observations crossing the same breakout level.
