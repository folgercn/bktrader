# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 18 | -0.1849% | 1.7367% | 0.9644% | 0.961129 | 61.11% | -2.3141% | 1284.50s | `{'InitialSL': 7, 'TrailingSL': 7, 'BreakevenSL': 3, 'MaxHoldExit': 1}` | `{'candidate_events': 19, 'entries': 18, 'busy_skipped': 1, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-12` | 18 | -0.155563% | 61.1111% |
