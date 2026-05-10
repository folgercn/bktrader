# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 15 | -1.6063% | -0.4634% | -0.9221% | 0.472392 | 46.67% | -2.1553% | 2265.00s | `{'InitialSL': 8, 'TrailingSL': 5, 'BreakevenSL': 1, 'MaxHoldExit': 1}` | `{'candidate_events': 15, 'entries': 15, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-11` | 15 | -1.610795% | 46.6667% |
