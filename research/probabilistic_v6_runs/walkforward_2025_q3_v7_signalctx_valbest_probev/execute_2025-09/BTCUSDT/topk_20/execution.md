# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 19 | 1.7000% | 4.1153% | 3.1430% | 2.014325 | 73.68% | -0.6989% | 1609.00s | `{'TrailingSL': 13, 'InitialSL': 3, 'MaxHoldExit': 2, 'BreakevenSL': 1}` | `{'candidate_events': 20, 'entries': 19, 'busy_skipped': 1, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-09` | 19 | 1.694488% | 73.6842% |

