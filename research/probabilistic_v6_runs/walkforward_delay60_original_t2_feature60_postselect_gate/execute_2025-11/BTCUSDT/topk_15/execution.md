# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 6 | 0.4103% | 0.9372% | 0.7264% | 1.505627 | 66.67% | -0.5726% | 1491.50s | `{'TrailingSL': 4, 'InitialSL': 2}` | `{'candidate_events': 6, 'entries': 6, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-11` | 6 | 0.414075% | 66.6667% |

