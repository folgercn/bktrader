# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 13 | -0.0082% | 1.2169% | 0.7257% | 1.004272 | 61.54% | -2.0028% | 3620.00s | `{'TrailingSL': 7, 'InitialSL': 5, 'BreakevenSL': 1}` | `{'candidate_events': 17, 'entries': 13, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 4, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 12 | -1.1362% | -0.0984% | -0.5142% | 0.655278 | 50.00% | -1.7188% | 3504.00s | `{'TrailingSL': 5, 'MaxHoldExit': 3, 'InitialSL': 3, 'BreakevenSL': 1}` | `{'candidate_events': 15, 'entries': 12, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 3, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-03` | 13 | 0.013905% | 61.5385% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-03` | 12 | -1.125641% | 50.0000% |

