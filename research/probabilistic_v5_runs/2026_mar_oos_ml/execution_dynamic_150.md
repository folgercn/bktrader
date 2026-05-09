# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 4 | -0.5480% | -0.0413% | -0.2449% | 0.74822 | 50.00% | -2.0943% | 3277.50s | `{'TrailingSL': 2, 'InitialSL': 2}` | `{'candidate_events': 6, 'entries': 4, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 2, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 9 | -0.2590% | 0.9147% | 0.4444% | 0.935379 | 55.56% | -3.1864% | 2869.00s | `{'TrailingSL': 4, 'InitialSL': 2, 'MaxHoldExit': 2, 'BreakevenSL': 1}` | `{'candidate_events': 15, 'entries': 9, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 6, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-03` | 4 | -0.530084% | 50.0000% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-03` | 9 | -0.226541% | 55.5556% |

