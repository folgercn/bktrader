# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 18 | -1.2193% | 0.6057% | -0.1283% | 0.639244 | 44.44% | -1.9710% | 1243.00s | `{'InitialSL': 10, 'TrailingSL': 7, 'BreakevenSL': 1}` | `{'candidate_events': 18, 'entries': 18, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-07` | 18 | -1.216082% | 44.4444% |
