# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 14 | -0.3762% | 0.9666% | 0.4274% | 0.812854 | 57.14% | -0.9556% | 3177.00s | `{'TrailingSL': 8, 'InitialSL': 5, 'MaxHoldExit': 1}` | `{'candidate_events': 14, 'entries': 14, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-09` | 14 | -0.370799% | 57.1429% |

