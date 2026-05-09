# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 31 | -1.1390% | 1.4224% | 0.3899% | 0.810298 | 54.84% | -3.6276% | 1615.00s | `{'TrailingSL': 13, 'InitialSL': 13, 'MaxHoldExit': 3, 'BreakevenSL': 2}` | `{'candidate_events': 32, 'entries': 31, 'busy_skipped': 1, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-12` | 31 | -1.119836% | 51.6129% |

