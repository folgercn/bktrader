# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 9 | -0.7042% | 0.2760% | -0.1178% | 0.674988 | 55.56% | -1.9160% | 1417.00s | `{'InitialSL': 4, 'TrailingSL': 3, 'BreakevenSL': 1, 'MaxHoldExit': 1}` | `{'candidate_events': 10, 'entries': 9, 'busy_skipped': 1, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-12` | 9 | -0.696589% | 55.5556% |

