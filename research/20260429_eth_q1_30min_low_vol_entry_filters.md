# ETHUSDT Q1 2026 30min Low-Volatility Entry Gates

Scope: research-only Python replay. No live or execution path is changed by this report.

## Setup

- Symbol/window: `ETHUSDT`, `2026-01-01T00:00:00+00:00` to `2026-03-31T23:59:59+00:00`
- Execution bars: continuous `1s` bars rebuilt from Binance trade archives
- Signal timeframe: `30min`
- Replay mode: `live_intrabar_sma5`, breakout shape `baseline_plus_t3`, t3 SMA/ATR separation `0.25`
- Sizing: `dir2_zero_initial=true`, `zero_initial_mode=reentry_window`, `reentry_size_schedule=[0.20, 0.10]`, `max_trades_per_bar=2`
- Risk profile: `stop_loss_atr=0.05`, `trailing_stop_atr=0.3`, `delayed_trailing_activation=0.5`
- Gate point: reentry price resolution, so the gate blocks both Zero-Initial-Reentry and SL-Reentry entries. Stop-loss exits are not filtered.

## Stop Distance Distribution

| Count | Min | P05 | P10 | P25 | P50 | P75 | P90 | P95 | Max |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 4307 | 0.6144 | 1.2044 | 1.5776 | 2.3642 | 3.4132 | 4.6035 | 5.9251 | 7.2042 | 15.9955 |

## Results

| Variant | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Avg Loss | Worst Loss | Entry Mix | Gate Reject Calls |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|
| `baseline` | 536,109.15 | 436.11% | -2.04% | 3226 | 80.60% | 13.49 | -0.0798% | -0.1811% | `SL-Reentry:2086, Zero-Initial-Reentry:1140` | 0 |
| `min_stop_bps_2` | 577,665.51 | 477.67% | -0.38% | 2622 | 86.84% | 15.18 | -0.0887% | -0.1811% | `SL-Reentry:1681, Zero-Initial-Reentry:941` | 560979 |
| `min_stop_bps_4` | 388,532.61 | 288.53% | -0.21% | 1185 | 91.39% | 17.71 | -0.1055% | -0.1811% | `SL-Reentry:739, Zero-Initial-Reentry:446` | 1950049 |
| `min_stop_bps_6` | 192,490.09 | 92.49% | -0.17% | 307 | 92.83% | 18.79 | -0.1368% | -0.1811% | `SL-Reentry:190, Zero-Initial-Reentry:117` | 2741898 |
| `min_stop_bps_8` | 141,721.63 | 41.72% | -0.17% | 133 | 90.23% | 18.49 | -0.1490% | -0.1811% | `SL-Reentry:83, Zero-Initial-Reentry:50` | 2901698 |
| `atr_pct_gte_25` | 486,720.09 | 386.72% | -0.34% | 2297 | 85.46% | 14.66 | -0.0884% | -0.1811% | `SL-Reentry:1471, Zero-Initial-Reentry:826` | 855849 |
| `min_stop_bps_4_atr_pct_gte_25` | 364,445.73 | 264.45% | -0.21% | 1101 | 91.37% | 17.55 | -0.1064% | -0.1811% | `SL-Reentry:686, Zero-Initial-Reentry:415` | 2025929 |

## Delta vs Baseline

| Variant | Final Delta | Return Delta | Max DD Delta | Trades Delta | Win Delta | Sharpe Delta |
|---|---:|---:|---:|---:|---:|---:|
| `min_stop_bps_2` | 41,556.36 | 41.56 pp | 1.66 pp | -604 | 6.24 pp | 1.69 |
| `min_stop_bps_4` | -147,576.54 | -147.58 pp | 1.83 pp | -2041 | 10.79 pp | 4.22 |
| `min_stop_bps_6` | -343,619.06 | -343.62 pp | 1.87 pp | -2919 | 12.23 pp | 5.30 |
| `min_stop_bps_8` | -394,387.52 | -394.39 pp | 1.87 pp | -3093 | 9.63 pp | 5.00 |
| `atr_pct_gte_25` | -49,389.06 | -49.39 pp | 1.70 pp | -929 | 4.86 pp | 1.17 |
| `min_stop_bps_4_atr_pct_gte_25` | -171,663.42 | -171.66 pp | 1.83 pp | -2125 | 10.77 pp | 4.06 |

## Variants

- `baseline`: No low-volatility entry gate.
- `min_stop_bps_2`: Require stop_loss_atr * ATR to be at least 2 bps of entry reference price.
- `min_stop_bps_4`: Require stop_loss_atr * ATR to be at least 4 bps of entry reference price.
- `min_stop_bps_6`: Require stop_loss_atr * ATR to be at least 6 bps of entry reference price.
- `min_stop_bps_8`: Require stop_loss_atr * ATR to be at least 8 bps of entry reference price.
- `atr_pct_gte_25`: Require signal ATR percentile over the rolling 240 signal bars to be at least 25.
- `min_stop_bps_4_atr_pct_gte_25`: Require both min stop distance of 4 bps and ATR percentile at least 25.
