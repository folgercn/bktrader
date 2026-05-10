# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 15 | 1.4409% | 3.2364% | 2.5149% | 1.378908 | 60.00% | -1.6376% | 532.00s | `{'TrailingSL': 7, 'InitialSL': 6, 'BreakevenSL': 1, 'MaxHoldExit': 1}` | `{'candidate_events': 15, 'entries': 15, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-12` | 15 | 1.463851% | 60.0000% |
