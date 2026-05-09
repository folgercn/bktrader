# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 31 | -3.2538% | -0.4760% | -1.5964% | 0.60303 | 58.06% | -6.3230% | 2817.00s | `{'TrailingSL': 15, 'InitialSL': 13, 'BreakevenSL': 3}` | `{'candidate_events': 65, 'entries': 31, 'busy_skipped': 2, 'same_signal_bar_skipped': 0, 'dwell_skipped': 32, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 31 | -2.8606% | -0.1167% | -1.2226% | 0.671564 | 51.61% | -4.3065% | 2634.00s | `{'TrailingSL': 13, 'InitialSL': 12, 'BreakevenSL': 3, 'MaxHoldExit': 3}` | `{'candidate_events': 80, 'entries': 31, 'busy_skipped': 2, 'same_signal_bar_skipped': 0, 'dwell_skipped': 47, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-03` | 31 | -3.265829% | 58.0645% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-03` | 31 | -2.852560% | 51.6129% |

