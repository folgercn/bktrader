# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 58 | 2.0715% | 3.2612% | 2.7841% | 2.540186 | 74.14% | -0.3668% | 1514.00s | `{'TrailingSL': 37, 'InitialSL': 15, 'BreakevenSL': 6}` | `{'candidate_events': 58, 'entries': 58, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 20 | 0.276460% | 65.0000% |
| `2026-02` | 19 | 1.624326% | 89.4737% |
| `2026-03` | 19 | 0.152320% | 68.4211% |

