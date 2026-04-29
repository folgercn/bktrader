# BTCUSDT Q1 2026 30min Low-Volatility Entry Gates

Scope: research-only Python replay. No live or execution path is changed by this report.

## Setup

- Symbol/window: `BTCUSDT`, `2026-01-01T00:00:00+00:00` to `2026-03-31T23:59:59+00:00`
- Execution bars: continuous `1s` bars rebuilt from Binance trade archives
- Signal timeframe: `30min`
- Replay mode: `live_intrabar_sma5`, breakout shape `baseline_plus_t3`, t3 SMA/ATR separation `0.25`
- Sizing: `dir2_zero_initial=true`, `zero_initial_mode=reentry_window`, `reentry_size_schedule=[0.20, 0.10]`, `max_trades_per_bar=2`
- Gate point: reentry price resolution, so the gate blocks both Zero-Initial-Reentry and SL-Reentry entries. Stop-loss exits are not filtered.

## Results

| Variant | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Avg Loss | Worst Loss | Entry Mix | Gate Reject Calls |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|
| `baseline` | 227,449.41 | 127.45% | -3.03% | 3177 | 76.46% | 13.23 | -0.1659% | -0.7003% | `SL-Reentry:2137, Zero-Initial-Reentry:1040` | 0 |
| `min_stop_bps_6` | 240,319.34 | 140.32% | -1.27% | 2967 | 80.49% | 13.91 | -0.1977% | -0.7003% | `SL-Reentry:1996, Zero-Initial-Reentry:971` | 205782 |
| `min_stop_bps_8` | 251,377.98 | 151.38% | -0.66% | 2748 | 83.08% | 14.56 | -0.2130% | -0.7003% | `SL-Reentry:1847, Zero-Initial-Reentry:901` | 443571 |
| `atr_pct_gte_25` | 237,251.08 | 137.25% | -1.27% | 2334 | 82.73% | 14.47 | -0.2117% | -0.7003% | `SL-Reentry:1558, Zero-Initial-Reentry:776` | 826574 |
| `atr_pct_gte_35` | 227,154.89 | 127.15% | -1.13% | 2017 | 84.04% | 14.74 | -0.2218% | -0.7003% | `SL-Reentry:1344, Zero-Initial-Reentry:673` | 1146020 |
| `min_stop_bps_6_atr_pct_gte_25` | 238,079.98 | 138.08% | -1.27% | 2318 | 83.09% | 14.54 | -0.2146% | -0.7003% | `SL-Reentry:1549, Zero-Initial-Reentry:769` | 843709 |

## Delta vs Baseline

| Variant | Final Delta | Return Delta | Max DD Delta | Trades Delta | Win Delta | Sharpe Delta |
|---|---:|---:|---:|---:|---:|---:|
| `min_stop_bps_6` | 12,869.93 | 12.87 pp | 1.76 pp | -210 | 4.03 pp | 0.68 |
| `min_stop_bps_8` | 23,928.57 | 23.93 pp | 2.37 pp | -429 | 6.62 pp | 1.33 |
| `atr_pct_gte_25` | 9,801.67 | 9.80 pp | 1.76 pp | -843 | 6.27 pp | 1.24 |
| `atr_pct_gte_35` | -294.52 | -0.30 pp | 1.90 pp | -1160 | 7.58 pp | 1.51 |
| `min_stop_bps_6_atr_pct_gte_25` | 10,630.57 | 10.63 pp | 1.76 pp | -859 | 6.63 pp | 1.31 |

## Variants

- `baseline`: No low-volatility entry gate.
- `min_stop_bps_6`: Require stop_loss_atr * ATR to be at least 6 bps of entry reference price.
- `min_stop_bps_8`: Require stop_loss_atr * ATR to be at least 8 bps of entry reference price.
- `atr_pct_gte_25`: Require signal ATR percentile over the rolling 240 signal bars to be at least 25.
- `atr_pct_gte_35`: Require signal ATR percentile over the rolling 240 signal bars to be at least 35.
- `min_stop_bps_6_atr_pct_gte_25`: Require both min stop distance of 6 bps and ATR percentile at least 25.
