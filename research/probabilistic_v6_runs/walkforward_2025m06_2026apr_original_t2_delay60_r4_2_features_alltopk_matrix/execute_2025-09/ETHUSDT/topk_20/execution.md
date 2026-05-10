# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 20 | -7.4177% | -5.2291% | -6.1112% | 0.195193 | 35.00% | -8.1180% | 3526.50s | `{'InitialSL': 13, 'TrailingSL': 5, 'MaxHoldExit': 1, 'BreakevenSL': 1}` | `{'candidate_events': 20, 'entries': 20, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-09` | 20 | -7.662204% | 35.0000% |
