# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 16 | -3.8313% | -1.9049% | -2.6795% | 0.138638 | 37.50% | -4.3292% | 4325.00s | `{'InitialSL': 10, 'TrailingSL': 3, 'BreakevenSL': 2, 'MaxHoldExit': 1}` | `{'candidate_events': 16, 'entries': 16, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-08` | 16 | -3.894958% | 37.5000% |
