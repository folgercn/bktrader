# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 7 | -0.0322% | 0.1078% | 0.0518% | 0.894243 | 42.86% | -0.1822% | 4009.00s | `{'InitialSL': 3, 'TrailingSL': 3, 'MaxHoldExit': 1}` | `{'candidate_events': 7, 'entries': 7, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-11` | 7 | -0.031994% | 42.8571% |
