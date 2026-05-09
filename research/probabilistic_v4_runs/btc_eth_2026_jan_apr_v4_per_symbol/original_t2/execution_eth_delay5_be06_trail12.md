# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 74 | -0.9775% | 0.4986% | -0.0942% | 0.720938 | 68.92% | -1.3859% | 1812.50s | `{'BreakevenSL': 34, 'InitialSL': 23, 'TrailingSL': 17}` | `{'candidate_events': 75, 'entries': 74, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 1, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 24 | -0.624643% | 58.3333% |
| `2026-02` | 29 | -0.442199% | 68.9655% |
| `2026-03` | 21 | 0.090052% | 80.9524% |

