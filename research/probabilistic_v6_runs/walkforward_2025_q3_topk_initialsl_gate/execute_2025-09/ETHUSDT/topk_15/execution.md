# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 15 | -5.3681% | -3.3812% | -4.1799% | 0.239885 | 40.00% | -6.3627% | 3321.00s | `{'InitialSL': 8, 'TrailingSL': 5, 'BreakevenSL': 1, 'MaxHoldExit': 1}` | `{'candidate_events': 15, 'entries': 15, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-09` | 15 | -5.482709% | 40.0000% |

