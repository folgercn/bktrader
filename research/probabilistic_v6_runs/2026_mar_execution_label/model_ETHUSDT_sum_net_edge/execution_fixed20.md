# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 17 | 0.1438% | 0.4848% | 0.3483% | 1.186388 | 58.82% | -0.5575% | 2220.00s | `{'TrailingSL': 8, 'InitialSL': 6, 'BreakevenSL': 2, 'MaxHoldExit': 1}` | `{'candidate_events': 18, 'entries': 17, 'busy_skipped': 1, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-03` | 17 | 0.144823% | 58.8235% |

