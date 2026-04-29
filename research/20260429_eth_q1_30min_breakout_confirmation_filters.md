# ETHUSDT Q1 2026 30min Breakout Confirmation Filters

Scope: research-only Python replay. No live or execution path is changed by this report.

## Setup

- Symbol/window: `ETHUSDT`, `2026-01-01T00:00:00+00:00` to `2026-03-31T23:59:59+00:00`
- Execution bars: continuous `1s` bars rebuilt from Binance trade archives
- Signal timeframe: `30min`
- Replay mode: `live_intrabar_sma5`, breakout shape `baseline_plus_t3`, t3 SMA/ATR separation `0.25`
- Sizing: `dir2_zero_initial=true`, `zero_initial_mode=reentry_window`, `reentry_size_schedule=[0.20, 0.10]`, `max_trades_per_bar=2`
- Replay profile: `eth_research`
- Risk params: `stop_loss_atr=0.05`, `trailing_stop_atr=0.3`, `delayed_trailing_activation=0.5`
- `confirm_2x_1s` / `confirm_3x_1s` are multi-tick proxies: they require consecutive 1s observations to cross the same breakout level.

## Results

| Variant | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Entry Mix | Filter Rejects |
|---|---:|---:|---:|---:|---:|---:|---|---|
| `baseline_single_observation` | 536,109.15 | 436.11% | -2.04% | 3226 | 80.60% | 13.49 | `SL-Reentry:2086, Zero-Initial-Reentry:1140` | `margin L/S 0/0; confirm L/S 0/0` |
| `margin_0p02atr` | 517,226.25 | 417.23% | -1.97% | 3162 | 80.80% | 13.68 | `SL-Reentry:2047, Zero-Initial-Reentry:1115` | `margin L/S 7450/7523; confirm L/S 0/0` |
| `confirm_2x_1s` | 524,591.79 | 424.59% | -2.04% | 3158 | 80.65% | 13.53 | `SL-Reentry:2042, Zero-Initial-Reentry:1116` | `margin L/S 0/0; confirm L/S 1660/1723` |
| `confirm_3x_1s` | 523,434.32 | 423.43% | -2.04% | 3109 | 80.60% | 12.97 | `SL-Reentry:2011, Zero-Initial-Reentry:1098` | `margin L/S 0/0; confirm L/S 3190/3307` |

## Delta vs Baseline

| Variant | Final Balance Delta | Return Delta | Max DD Delta | Trades Delta | Win Rate Delta | Sharpe Delta |
|---|---:|---:|---:|---:|---:|---:|
| `margin_0p02atr` | -18,882.90 | -18.88 pp | 0.07 pp | -64 | 0.20 pp | 0.19 |
| `confirm_2x_1s` | -11,517.36 | -11.52 pp | 0.00 pp | -68 | 0.05 pp | 0.04 |
| `confirm_3x_1s` | -12,674.83 | -12.68 pp | 0.00 pp | -117 | 0.00 pp | -0.52 |

## Breakout Attribution

| Variant | Shape | Trades | Win Rate | Avg PnL | Net PnL | Worst PnL | Profit Factor |
|---|---|---:|---:|---:|---:|---:|---:|
| `baseline_single_observation` | `original_t2` | 2575 | 87.96% | 0.5409% | 555,845.31 | -0.1811% | 61.861 |
| `baseline_single_observation` | `t3_swing` | 651 | 87.10% | 0.5355% | 137,520.44 | -0.1188% | 64.946 |
| `margin_0p02atr` | `original_t2` | 2519 | 88.29% | 0.5395% | 529,207.28 | -0.1811% | 63.2888 |
| `margin_0p02atr` | `t3_swing` | 643 | 86.94% | 0.5408% | 133,188.86 | -0.1365% | 61.3525 |
| `confirm_2x_1s` | `original_t2` | 2514 | 87.87% | 0.5435% | 537,196.84 | -0.1811% | 61.4108 |
| `confirm_2x_1s` | `t3_swing` | 644 | 86.80% | 0.5388% | 134,426.18 | -0.1188% | 63.4185 |
| `confirm_3x_1s` | `original_t2` | 2471 | 88.06% | 0.5493% | 535,262.98 | -0.1811% | 63.557 |
| `confirm_3x_1s` | `t3_swing` | 638 | 86.21% | 0.5438% | 132,784.43 | -0.1365% | 59.399 |

## Variants

- `baseline_single_observation`: Current research behavior: one 1s high/low crossing can lock the zero-initial window.
- `margin_0p02atr`: Require breakout probe to clear the level by at least 0.02 ATR.
- `confirm_2x_1s`: Require two consecutive 1s observations crossing the same breakout level.
- `confirm_3x_1s`: Require three consecutive 1s observations crossing the same breakout level.
