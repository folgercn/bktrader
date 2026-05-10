# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 3 | -0.9047% | -0.5081% | -0.6670% | 0.26003 | 33.33% | -1.2187% | 3474.00s | `{'InitialSL': 1, 'MaxHoldExit': 1, 'TrailingSL': 1}` | `{'candidate_events': 3, 'entries': 3, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-10` | 3 | -0.904540% | 33.3333% |
