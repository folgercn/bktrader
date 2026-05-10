# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 9 | -1.1777% | -0.4034% | -0.7141% | 0.421283 | 55.56% | -1.4322% | 676.00s | `{'TrailingSL': 3, 'InitialSL': 3, 'MaxHoldExit': 2, 'BreakevenSL': 1}` | `{'candidate_events': 9, 'entries': 9, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-04` | 9 | -1.178024% | 44.4444% |
