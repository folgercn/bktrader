# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 18 | -0.5742% | 2.1153% | 1.0308% | 0.870889 | 55.56% | -2.7358% | 1384.50s | `{'TrailingSL': 10, 'InitialSL': 8}` | `{'candidate_events': 19, 'entries': 18, 'busy_skipped': 0, 'same_signal_bar_skipped': 1, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-07` | 18 | -0.554159% | 55.5556% |
