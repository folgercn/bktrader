# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 52 | -2.0820% | -1.0580% | -1.4689% | 0.387041 | 53.85% | -2.1042% | 1947.00s | `{'InitialSL': 21, 'BreakevenSL': 15, 'TrailingSL': 12, 'MaxHoldExit': 4}` | `{'candidate_events': 52, 'entries': 52, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 20 | -0.546826% | 50.0000% |
| `2026-02` | 20 | -1.270246% | 55.0000% |
| `2026-03` | 12 | -0.283242% | 58.3333% |

