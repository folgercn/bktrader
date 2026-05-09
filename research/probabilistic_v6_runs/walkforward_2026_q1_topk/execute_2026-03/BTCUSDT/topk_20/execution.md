# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 19 | -1.6055% | 0.7161% | -0.2184% | 0.738372 | 57.89% | -2.6368% | 2821.00s | `{'TrailingSL': 9, 'InitialSL': 8, 'BreakevenSL': 2}` | `{'candidate_events': 20, 'entries': 19, 'busy_skipped': 1, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-03` | 19 | -1.581163% | 57.8947% |

