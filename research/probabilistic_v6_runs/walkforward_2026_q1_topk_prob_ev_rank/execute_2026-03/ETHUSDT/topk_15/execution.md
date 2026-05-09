# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 14 | 1.9081% | 3.5868% | 2.9125% | 1.628289 | 57.14% | -1.6578% | 3435.00s | `{'TrailingSL': 7, 'InitialSL': 5, 'BreakevenSL': 1, 'MaxHoldExit': 1}` | `{'candidate_events': 15, 'entries': 14, 'busy_skipped': 1, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-03` | 14 | 1.919783% | 57.1429% |

