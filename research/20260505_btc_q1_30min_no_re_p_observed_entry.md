# BTCUSDT Q1 2026 30min No-re_p Observed Entry

Scope: research-only Python replay. No live or execution path is changed by this report.

## Setup

- Symbol/window: `BTCUSDT`, `2026-01-01T00:00:00+00:00` to `2026-03-31T23:59:59+00:00`
- Execution bars: continuous `1s` bars rebuilt from Binance trade archives
- Signal timeframe: `30min`
- Breakout shape: `baseline_plus_t3`
- Fixed stop: `stop_mode=atr`, `stop_loss_atr=0.3`
- Trailing stop: `trailing_stop_atr=0.3`, activated after `0.5 ATR` unrealized profit
- Profit protection: `profit_protect_atr=1.0`
- Sizing: `reentry_size_schedule=[0.2, 0.1]`, `max_trades_per_bar=2`
- Optimization gates: removed
- Entry semantics: first breakout creates only a virtual zero-notional Initial state; real entry requires virtual/real SL, cooldown, and a post-cooldown observed breakout-level event
- `re_p` usage: none for fill, none for entry trigger, none for actionability

## Results

| Variant | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Avg Loss | Worst Loss | Entry Mix | Lock->Entry Median | Exit->Entry Median |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|
| `virtual_sl_second_breakout_30s` | 53,576.30 | -46.42% | -46.42% | 1252 | 17.81% | -6.23 | -0.1884% | -0.8516% | `SL-Reentry:761, Zero-Initial-Reentry:491` | 630.0 | 446.5 |
| `virtual_sl_second_breakout_60s` | 54,469.95 | -45.53% | -45.52% | 1229 | 18.71% | -5.71 | -0.1886% | -0.8516% | `SL-Reentry:746, Zero-Initial-Reentry:483` | 682.0 | 479.0 |
| `virtual_sl_second_breakout_120s` | 56,185.98 | -43.81% | -43.81% | 1162 | 19.19% | -5.61 | -0.1897% | -0.8516% | `SL-Reentry:694, Zero-Initial-Reentry:468` | 754.0 | 540.5 |
| `virtual_sl_acceptance_60s` | 29,205.00 | -70.79% | -70.79% | 2658 | 18.96% | -6.00 | -0.1995% | -0.8515% | `SL-Reentry:1863, Zero-Initial-Reentry:795` | 61.0 | 61.0 |

## Delta vs 60s Fresh-cross

| Variant | Final Delta | Return Delta | Max DD Delta | Trades Delta | Win Delta | Sharpe Delta |
|---|---:|---:|---:|---:|---:|---:|
| `virtual_sl_second_breakout_30s` | -893.65 | -0.89 pp | -0.90 pp | 23 | -0.90 pp | -0.52 |
| `virtual_sl_second_breakout_120s` | 1,716.03 | 1.72 pp | 1.71 pp | -67 | 0.48 pp | 0.10 |
| `virtual_sl_acceptance_60s` | -25,264.95 | -25.26 pp | -25.27 pp | 1429 | 0.25 pp | -0.29 |

## Entry Reason Diagnostics

| Variant | Reason | Trades | Win Rate | Net PnL | Avg PnL |
|---|---|---:|---:|---:|---:|
| `virtual_sl_second_breakout_30s` | `SL-Reentry` | 761 | 21.55% | -8,713.58 | -0.0950% |
| `virtual_sl_second_breakout_30s` | `Zero-Initial-Reentry` | 491 | 27.09% | -5,414.69 | -0.0736% |
| `virtual_sl_second_breakout_60s` | `SL-Reentry` | 746 | 23.59% | -8,040.22 | -0.0861% |
| `virtual_sl_second_breakout_60s` | `Zero-Initial-Reentry` | 483 | 27.54% | -5,295.48 | -0.0728% |
| `virtual_sl_second_breakout_120s` | `SL-Reentry` | 694 | 22.91% | -7,866.16 | -0.0885% |
| `virtual_sl_second_breakout_120s` | `Zero-Initial-Reentry` | 468 | 28.85% | -4,851.25 | -0.0672% |
| `virtual_sl_acceptance_60s` | `SL-Reentry` | 1863 | 25.28% | -14,233.92 | -0.0931% |
| `virtual_sl_acceptance_60s` | `Zero-Initial-Reentry` | 795 | 24.91% | -7,334.98 | -0.0772% |

## Diagnostics

- `virtual_sl_second_breakout_30s`: locks={'long': {'original_t2': 314, 't3_swing': 91}, 'short': {'original_t2': 326, 't3_swing': 93}}, virtual_exits={'SL': 824, 'PT': 0}, pending_expired=704, pending_triggered=1252
- `virtual_sl_second_breakout_60s`: locks={'long': {'original_t2': 309, 't3_swing': 91}, 'short': {'original_t2': 312, 't3_swing': 93}}, virtual_exits={'SL': 805, 'PT': 0}, pending_expired=711, pending_triggered=1229
- `virtual_sl_second_breakout_120s`: locks={'long': {'original_t2': 301, 't3_swing': 91}, 'short': {'original_t2': 306, 't3_swing': 92}}, virtual_exits={'SL': 790, 'PT': 0}, pending_expired=720, pending_triggered=1162
- `virtual_sl_acceptance_60s`: locks={'long': {'original_t2': 397, 't3_swing': 128}, 'short': {'original_t2': 472, 't3_swing': 134}}, virtual_exits={'SL': 1131, 'PT': 0}, pending_expired=334, pending_triggered=2658

## Read

These rows intentionally do not test `re_p`. They test whether the old lesson can be kept: first breakout is structure proof only, and real exposure waits for a later observed event.
