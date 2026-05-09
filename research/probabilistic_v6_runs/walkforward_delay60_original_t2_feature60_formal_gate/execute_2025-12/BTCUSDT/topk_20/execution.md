# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 11 | -0.7900% | 0.3599% | -0.1012% | 0.624833 | 45.45% | -2.0575% | 2070.00s | `{'InitialSL': 5, 'TrailingSL': 3, 'MaxHoldExit': 2, 'BreakevenSL': 1}` | `{'candidate_events': 11, 'entries': 11, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-12` | 11 | -0.785592% | 45.4545% |

