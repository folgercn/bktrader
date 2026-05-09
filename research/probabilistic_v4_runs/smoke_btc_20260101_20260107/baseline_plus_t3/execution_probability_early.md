# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 8 | -0.1531% | 0.0069% | -0.0572% | 0.452784 | 62.50% | -0.2794% | 1188.00s | `{'TrailingSL': 4, 'InitialSL': 3, 'BreakevenSL': 1}` | `{'candidate_events': 12, 'entries': 8, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 3, 'min_stop_skipped': 1, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 8 | -0.153056% | 62.5000% |

