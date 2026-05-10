# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 17 | 1.0495% | 2.6576% | 2.0120% | 1.360534 | 70.59% | -2.4103% | 1268.00s | `{'TrailingSL': 9, 'InitialSL': 4, 'MaxHoldExit': 2, 'BreakevenSL': 2}` | `{'candidate_events': 17, 'entries': 17, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-11` | 17 | 1.065119% | 70.5882% |
