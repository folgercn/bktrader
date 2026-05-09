# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 8 | 0.0434% | 0.2036% | 0.1395% | 1.135201 | 62.50% | -0.3215% | 3345.00s | `{'TrailingSL': 3, 'InitialSL': 3, 'MaxHoldExit': 2}` | `{'candidate_events': 9, 'entries': 8, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 1, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 28 | 0.8569% | 1.4230% | 1.1963% | 1.895066 | 67.86% | -0.4142% | 1801.50s | `{'TrailingSL': 15, 'InitialSL': 9, 'BreakevenSL': 3, 'MaxHoldExit': 1}` | `{'candidate_events': 33, 'entries': 28, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 5, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-12` | 8 | 0.043811% | 50.0000% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-12` | 28 | 0.854940% | 67.8571% |

