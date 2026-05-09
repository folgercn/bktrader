# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 33 | -1.1130% | 2.4272% | 0.9968% | 0.868305 | 63.64% | -3.7031% | 2817.00s | `{'TrailingSL': 15, 'InitialSL': 12, 'BreakevenSL': 6}` | `{'candidate_events': 35, 'entries': 33, 'busy_skipped': 2, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-03` | 33 | -1.070246% | 63.6364% |

