# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 9 | -2.7931% | -1.8799% | -2.2462% | 0.259694 | 44.44% | -3.4032% | 2525.00s | `{'InitialSL': 5, 'TrailingSL': 3, 'BreakevenSL': 1}` | `{'candidate_events': 9, 'entries': 9, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-07` | 9 | -2.815497% | 44.4444% |
