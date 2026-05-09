# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 22 | -0.1552% | 0.2849% | 0.1088% | 0.840335 | 50.00% | -0.4781% | 2155.00s | `{'InitialSL': 9, 'TrailingSL': 8, 'MaxHoldExit': 4, 'BreakevenSL': 1}` | `{'candidate_events': 27, 'entries': 22, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 5, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 12 | 0.7069% | 0.9488% | 0.8519% | 3.757643 | 83.33% | -0.2556% | 2296.00s | `{'TrailingSL': 8, 'BreakevenSL': 2, 'InitialSL': 2}` | `{'candidate_events': 15, 'entries': 12, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 3, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-12` | 22 | -0.154382% | 45.4545% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-12` | 12 | 0.705166% | 83.3333% |

