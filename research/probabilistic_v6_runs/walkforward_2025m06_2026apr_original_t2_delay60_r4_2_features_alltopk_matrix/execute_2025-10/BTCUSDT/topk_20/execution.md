# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 8 | -0.8476% | 0.0623% | -0.3020% | 0.62198 | 37.50% | -1.3815% | 3184.50s | `{'InitialSL': 4, 'TrailingSL': 3, 'MaxHoldExit': 1}` | `{'candidate_events': 8, 'entries': 8, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-10` | 8 | -0.840122% | 37.5000% |
