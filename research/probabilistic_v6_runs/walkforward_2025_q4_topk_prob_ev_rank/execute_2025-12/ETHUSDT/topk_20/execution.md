# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 19 | -0.3234% | 1.4586% | 0.7419% | 0.91744 | 57.89% | -2.5644% | 1443.00s | `{'TrailingSL': 8, 'InitialSL': 7, 'BreakevenSL': 2, 'MaxHoldExit': 2}` | `{'candidate_events': 20, 'entries': 19, 'busy_skipped': 1, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-12` | 19 | -0.305657% | 57.8947% |

