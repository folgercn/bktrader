# BTCUSDT Q1 2026 30min Zero-Initial Reentry Anchor Comparison

Scope: research-only Python backtest. No live or execution path is changed by this report.

## Setup

- Symbol/window: `BTCUSDT`, `2026-01-01T00:00:00+00:00` to `2026-03-31T23:59:59+00:00`
- Execution source: continuous `1s` bars rebuilt from Binance trade archives
- Signal timeframe: `30min`
- Strategy shape: `baseline_plus_t3`, t3 SMA/ATR separation `0.25`
- Baseline sizing: `dir2_zero_initial=true`, `zero_initial_mode=reentry_window`, `reentry_size_schedule=[0.20, 0.10]`, `max_trades_per_bar=2`
- Compared dimensions: zero-initial reentry anchor mode and reentry trigger observation mode.
- `bar_extrema` is the historical research envelope (`1s high/low`). `close_proxy` uses each 1s close as a conservative live event-price proxy.
- `actionable_8bps` additionally applies the live-style planned-price actionability gate: BUY cannot be more than 8bps above planned price, SELL cannot be more than 8bps below planned price.
- The snapshot anchor only affects zero-initial windows. SL/PT reentry windows still use their existing rolling anchor calculation.

## Results

| Anchor Mode | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Avg Loss | Worst Loss | Entry Mix |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `rolling_bar_extrema` | 227,449.41 | 127.45% | -3.03% | 3177 | 76.46% | 13.23 | -0.1659% | -0.7003% | `SL-Reentry:2137, Zero-Initial-Reentry:1040` |
| `snapshot_bar_extrema` | 227,449.41 | 127.45% | -3.03% | 3177 | 76.46% | 13.23 | -0.1659% | -0.7003% | `SL-Reentry:2137, Zero-Initial-Reentry:1040` |
| `rolling_close_proxy` | 227,449.41 | 127.45% | -3.03% | 3177 | 76.46% | 13.23 | -0.1659% | -0.7003% | `SL-Reentry:2137, Zero-Initial-Reentry:1040` |
| `snapshot_close_proxy` | 227,449.41 | 127.45% | -3.03% | 3177 | 76.46% | 13.23 | -0.1659% | -0.7003% | `SL-Reentry:2137, Zero-Initial-Reentry:1040` |
| `rolling_close_proxy_actionable_8bps` | 75,117.90 | -24.88% | -24.87% | 707 | 19.24% | -2.45 | -0.1598% | -0.5832% | `Zero-Initial-Reentry:355, SL-Reentry:352` |
| `snapshot_close_proxy_actionable_8bps` | 89,042.25 | -10.96% | -10.94% | 265 | 11.70% | -4.66 | -0.1315% | -0.4886% | `Zero-Initial-Reentry:133, SL-Reentry:132` |

## Delta vs Rolling Bar Extrema

| Variant | Final Delta | Return Delta | Max DD Delta | Trades Delta | Win Delta | Sharpe Delta |
|---|---:|---:|---:|---:|---:|---:|
| `snapshot_bar_extrema` | 0.00 | 0.00 pp | 0.00 pp | 0 | 0.00 pp | 0.00 |
| `rolling_close_proxy` | 0.00 | 0.00 pp | 0.00 pp | 0 | 0.00 pp | 0.00 |
| `snapshot_close_proxy` | 0.00 | 0.00 pp | 0.00 pp | 0 | 0.00 pp | 0.00 |
| `rolling_close_proxy_actionable_8bps` | -152,331.51 | -152.33 pp | -21.84 pp | -2470 | -57.22 pp | -15.68 |
| `snapshot_close_proxy_actionable_8bps` | -138,407.16 | -138.41 pp | -7.91 pp | -2912 | -64.76 pp | -17.89 |

## Read

`bar_extrema` rolling and snapshot are identical in this window; the historical research envelope hides the anchor difference.
Under the close-proxy live event-price approximation, snapshot vs rolling changes return by 0.00 pp and trades by 0.
With the 8bps actionability gate included, snapshot vs rolling changes return by 13.92 pp and trades by -442.
