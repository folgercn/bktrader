# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 56 | -0.9312% | 0.1853% | -0.2631% | 0.74587 | 57.14% | -2.3255% | 2480.00s | `{'InitialSL': 24, 'TrailingSL': 16, 'BreakevenSL': 15, 'MaxHoldExit': 1}` | `{'candidate_events': 75, 'entries': 56, 'busy_skipped': 3, 'same_signal_bar_skipped': 0, 'dwell_skipped': 15, 'min_stop_skipped': 1, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 16 | -0.515535% | 50.0000% |
| `2026-02` | 22 | -1.413818% | 54.5455% |
| `2026-03` | 18 | 0.999804% | 66.6667% |

