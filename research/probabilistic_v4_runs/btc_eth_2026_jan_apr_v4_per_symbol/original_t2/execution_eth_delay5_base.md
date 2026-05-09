# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 72 | -0.1925% | 1.2550% | 0.6736% | 0.9548 | 61.11% | -1.9854% | 2655.00s | `{'InitialSL': 28, 'TrailingSL': 24, 'BreakevenSL': 20}` | `{'candidate_events': 75, 'entries': 72, 'busy_skipped': 2, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 1, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 24 | -0.890437% | 50.0000% |
| `2026-02` | 28 | -0.223839% | 64.2857% |
| `2026-03` | 20 | 0.929001% | 70.0000% |

