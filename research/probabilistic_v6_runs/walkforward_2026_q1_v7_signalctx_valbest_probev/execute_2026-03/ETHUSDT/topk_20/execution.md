# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 20 | 8.9145% | 11.5596% | 10.4939% | 6.398041 | 90.00% | -1.3426% | 1037.50s | `{'TrailingSL': 16, 'BreakevenSL': 2, 'InitialSL': 1, 'MaxHoldExit': 1}` | `{'candidate_events': 20, 'entries': 20, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-03` | 20 | 8.588747% | 90.0000% |

