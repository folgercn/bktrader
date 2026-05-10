# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 18 | -1.8209% | -0.5121% | -1.0377% | 0.462032 | 44.44% | -2.1553% | 3123.50s | `{'InitialSL': 9, 'TrailingSL': 6, 'MaxHoldExit': 2, 'BreakevenSL': 1}` | `{'candidate_events': 18, 'entries': 18, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-11` | 18 | -1.828663% | 44.4444% |
