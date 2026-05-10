# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 20 | -0.4958% | -0.0969% | -0.2567% | 0.390605 | 50.00% | -0.4998% | 1437.50s | `{'InitialSL': 10, 'BreakevenSL': 5, 'TrailingSL': 5}` | `{'candidate_events': 20, 'entries': 20, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-04` | 20 | -0.496589% | 50.0000% |
