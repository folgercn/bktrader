# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 7 | -3.6076% | -2.8330% | -3.1442% | 0.135712 | 28.57% | -4.1574% | 2525.00s | `{'InitialSL': 5, 'BreakevenSL': 1, 'TrailingSL': 1}` | `{'candidate_events': 7, 'entries': 7, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-07` | 7 | -3.652625% | 28.5714% |
