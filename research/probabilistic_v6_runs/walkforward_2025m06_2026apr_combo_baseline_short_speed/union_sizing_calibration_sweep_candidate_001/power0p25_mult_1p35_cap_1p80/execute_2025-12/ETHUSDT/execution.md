# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 24 | 0.9901% | 4.3052% | 2.9678% | 1.176003 | 62.50% | -3.5264% | 1268.00s | `{'TrailingSL': 11, 'InitialSL': 9, 'BreakevenSL': 3, 'MaxHoldExit': 1}` | `{'candidate_events': 25, 'entries': 24, 'busy_skipped': 1, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-12` | 24 | 1.032855% | 62.5000% |
