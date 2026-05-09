# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 36 | -7.1188% | -3.4464% | -4.9314% | 0.407635 | 41.67% | -7.7318% | 2495.50s | `{'InitialSL': 21, 'TrailingSL': 12, 'BreakevenSL': 2, 'MaxHoldExit': 1}` | `{'candidate_events': 40, 'entries': 36, 'busy_skipped': 2, 'same_signal_bar_skipped': 2, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-09` | 36 | -7.331049% | 41.6667% |

