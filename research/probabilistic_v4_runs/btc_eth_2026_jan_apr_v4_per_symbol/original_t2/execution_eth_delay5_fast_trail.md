# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 73 | -0.3230% | 1.1427% | 0.5540% | 0.915913 | 65.75% | -1.3303% | 1996.00s | `{'TrailingSL': 25, 'InitialSL': 25, 'BreakevenSL': 22, 'MaxHoldExit': 1}` | `{'candidate_events': 75, 'entries': 73, 'busy_skipped': 1, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 1, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 24 | -0.560999% | 58.3333% |
| `2026-02` | 28 | -0.135482% | 64.2857% |
| `2026-03` | 21 | 0.379509% | 76.1905% |

