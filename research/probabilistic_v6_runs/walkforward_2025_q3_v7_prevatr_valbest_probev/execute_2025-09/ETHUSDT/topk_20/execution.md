# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 19 | -2.6501% | -0.6435% | -1.4504% | 0.484045 | 52.63% | -3.6054% | 1947.00s | `{'TrailingSL': 9, 'InitialSL': 9, 'BreakevenSL': 1}` | `{'candidate_events': 20, 'entries': 19, 'busy_skipped': 1, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-09` | 19 | -2.665365% | 52.6316% |

