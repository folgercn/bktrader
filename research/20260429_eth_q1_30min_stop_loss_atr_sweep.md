# ETHUSDT Q1 2026 30min Stop-Loss ATR Sweep

Scope: research-only Python replay. No live or execution path is changed by this report.

## Setup

- Symbol/window: `ETHUSDT`, `2026-01-01T00:00:00+00:00` to `2026-03-31T23:59:59+00:00`
- Execution bars: continuous `1s` bars rebuilt from Binance trade archives
- Signal timeframe: `30min`
- Replay mode: `live_intrabar_sma5`, breakout shape `baseline_plus_t3`, t3 SMA/ATR separation `0.25`
- Sizing: `dir2_zero_initial=true`, `zero_initial_mode=reentry_window`, `reentry_size_schedule=[0.20, 0.10]`, `max_trades_per_bar=2`
- Sweep changes only `stop_loss_atr`; `trailing_stop_atr=0.3` and `delayed_trailing_activation=0.5` stay fixed.

## Results

| stop_loss_atr | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Avg Loss | Worst Loss | Profit Factor | Median Hold |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 0.05 | 536,109.15 | 436.11% | -2.04% | 3226 | 80.60% | 13.49 | -0.0798% | -0.1811% | 62.449 | 258s |
| 0.10 | 534,500.90 | 434.50% | -2.12% | 3259 | 80.95% | 13.40 | -0.1103% | -0.3123% | 46.5882 | 271s |
| 0.20 | 537,386.59 | 437.39% | -2.01% | 3229 | 82.41% | 13.40 | -0.1701% | -0.5745% | 33.4078 | 296s |
| 0.30 | 551,198.83 | 451.20% | -1.86% | 3190 | 84.23% | 13.61 | -0.2248% | -0.8368% | 31.3064 | 307s |
| 0.40 | 541,234.86 | 441.23% | -1.63% | 3136 | 85.36% | 13.72 | -0.2729% | -1.0990% | 29.286 | 308s |

## Delta vs 0.05 ATR

| stop_loss_atr | Final Delta | Return Delta | Max DD Delta | Trades Delta | Win Delta | Sharpe Delta |
|---:|---:|---:|---:|---:|---:|---:|
| 0.10 | -1,608.25 | -1.61 pp | -0.08 pp | 33 | 0.35 pp | -0.09 |
| 0.20 | 1,277.44 | 1.28 pp | 0.03 pp | 3 | 1.81 pp | -0.09 |
| 0.30 | 15,089.68 | 15.09 pp | 0.18 pp | -36 | 3.63 pp | 0.12 |
| 0.40 | 5,125.71 | 5.12 pp | 0.41 pp | -90 | 4.76 pp | 0.23 |
