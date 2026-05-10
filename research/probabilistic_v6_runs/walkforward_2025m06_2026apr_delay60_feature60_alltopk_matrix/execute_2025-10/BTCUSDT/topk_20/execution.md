# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 7 | -1.2245% | -0.3825% | -0.7196% | 0.495441 | 28.57% | -1.7334% | 2966.00s | `{'InitialSL': 4, 'TrailingSL': 2, 'MaxHoldExit': 1}` | `{'candidate_events': 8, 'entries': 7, 'busy_skipped': 1, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-10` | 7 | -1.220087% | 28.5714% |
