# ETH Q1 2026 30min SL-Reentry Filters, 1s Replay

Scope: research-only backtest work. No live or execution path was changed.

## Setup

- Symbol/window: `ETHUSDT`, `2026-01-01 00:00:00+00:00` to `2026-03-31 23:59:59+00:00`
- Execution bars: continuous `1s` bars rebuilt from raw Binance trades
- Signal timeframe: `30min`
- Replay mode: `live_intrabar_sma5` with `baseline_plus_t3` breakout shape
- Sizing baseline includes PR #261 semantics: `Zero-Initial-Reentry=20%`, `SL-Reentry=10%` when schedule is `[0.20, 0.10]`.

## Results

| Variant | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | SL-Reentry | <=10s | <=30s | <=60s | SL-Reentry -> SL | Avg SL-Reentry PnL |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `baseline_sl_slot2` | 409,088.73 | 309.09% | -2.93% | 6077 | 65.94% | 10.96 | 5551 | 2943 | 3032 | 3097 | 100.00% | 82.30 |
| `close_confirm` | 27,418.26 | -72.58% | -72.58% | 6475 | 7.38% | -1.93 | 5977 | 3162 | 3193 | 3228 | 100.00% | -4.41 |
| `close_confirm_buffer_0p02atr` | 27,376.57 | -72.62% | -72.62% | 6473 | 7.35% | -1.95 | 5975 | 3159 | 3194 | 3230 | 100.00% | -4.42 |
| `delay_10s` | 414,838.08 | 314.84% | -2.96% | 6066 | 66.27% | 10.96 | 5536 | 2787 | 2957 | 3041 | 100.00% | 83.49 |
| `delay_20s` | 417,185.42 | 317.19% | -2.95% | 6049 | 66.57% | 11.00 | 5521 | 0 | 2889 | 2993 | 100.00% | 84.15 |
| `delay_30s` | 417,396.17 | 317.40% | -2.83% | 6029 | 66.63% | 11.01 | 5501 | 0 | 2788 | 2944 | 100.00% | 84.31 |
| `delay_60s` | 434,869.12 | 334.87% | -2.75% | 5987 | 67.18% | 11.07 | 5450 | 0 | 0 | 2762 | 100.00% | 88.27 |
| `cooldown_1bar` | 215,122.78 | 115.12% | -1.69% | 3191 | 65.50% | 10.87 | 2643 | 4 | 5 | 15 | 100.00% | 48.73 |
| `close_confirm_delay_10s` | 27,672.28 | -72.33% | -72.32% | 6456 | 7.42% | -1.89 | 5957 | 2925 | 3068 | 3138 | 100.00% | -4.45 |
| `close_confirm_delay_30s` | 27,805.53 | -72.19% | -72.19% | 6423 | 7.43% | -1.94 | 5924 | 0 | 2839 | 3012 | 100.00% | -4.50 |

## Delta vs Baseline

| Variant | Final Balance Delta | Return Delta | Max DD Delta | Trades Delta | Win Rate Delta | Sharpe Delta | SL-Reentry Delta |
|---|---:|---:|---:|---:|---:|---:|---:|
| `close_confirm` | -381,670.47 | -381.67 pp | -69.65 pp | 398 | -58.56 pp | -12.89 | 426 |
| `close_confirm_buffer_0p02atr` | -381,712.16 | -381.71 pp | -69.69 pp | 396 | -58.59 pp | -12.91 | 424 |
| `delay_10s` | 5,749.35 | 5.75 pp | -0.03 pp | -11 | 0.33 pp | 0.00 | -15 |
| `delay_20s` | 8,096.69 | 8.10 pp | -0.02 pp | -28 | 0.63 pp | 0.04 | -30 |
| `delay_30s` | 8,307.44 | 8.31 pp | 0.10 pp | -48 | 0.69 pp | 0.05 | -50 |
| `delay_60s` | 25,780.39 | 25.78 pp | 0.18 pp | -90 | 1.24 pp | 0.11 | -101 |
| `cooldown_1bar` | -193,965.95 | -193.97 pp | 1.24 pp | -2886 | -0.44 pp | -0.09 | -2908 |
| `close_confirm_delay_10s` | -381,416.45 | -381.42 pp | -69.39 pp | 379 | -58.52 pp | -12.85 | 406 |
| `close_confirm_delay_30s` | -381,283.20 | -381.28 pp | -69.26 pp | 346 | -58.51 pp | -12.90 | 373 |

## Variants

- `baseline_sl_slot2`: PR #261 sizing semantics only: SL-Reentry starts at schedule slot 2.
- `close_confirm`: SL-Reentry requires reclaim by current 1s close.
- `close_confirm_buffer_0p02atr`: SL-Reentry requires current 1s close reclaim with 0.02 ATR buffer.
- `delay_10s`: SL-Reentry and Zero-Initial-Reentry wait at least 10s after the SL exit.
- `delay_20s`: SL-Reentry and Zero-Initial-Reentry wait at least 20s after the SL exit.
- `delay_30s`: SL-Reentry and Zero-Initial-Reentry wait at least 30s after the SL exit.
- `delay_60s`: SL-Reentry and Zero-Initial-Reentry wait at least 60s after the SL exit.
- `cooldown_1bar`: SL-Reentry waits until the next signal bar after an SL exit.
- `close_confirm_delay_10s`: SL-Reentry requires current 1s close reclaim and at least 10s delay.
- `close_confirm_delay_30s`: SL-Reentry requires current 1s close reclaim and at least 30s delay.
