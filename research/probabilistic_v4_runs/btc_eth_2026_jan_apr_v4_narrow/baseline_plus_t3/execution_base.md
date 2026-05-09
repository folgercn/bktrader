# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 54 | 0.9933% | 2.0890% | 1.6497% | 1.540488 | 64.81% | -0.5481% | 3022.00s | `{'TrailingSL': 21, 'InitialSL': 18, 'BreakevenSL': 13, 'MaxHoldExit': 2}` | `{'candidate_events': 56, 'entries': 54, 'busy_skipped': 2, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 56 | -1.6265% | -0.5186% | -0.9631% | 0.607449 | 50.00% | -1.9916% | 2103.50s | `{'InitialSL': 27, 'TrailingSL': 14, 'BreakevenSL': 13, 'MaxHoldExit': 2}` | `{'candidate_events': 56, 'entries': 56, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 20 | 0.582972% | 60.0000% |
| `2026-02` | 15 | 0.460922% | 66.6667% |
| `2026-03` | 19 | -0.052342% | 68.4211% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 18 | -0.809441% | 38.8889% |
| `2026-02` | 21 | -0.886691% | 52.3810% |
| `2026-03` | 17 | 0.062243% | 58.8235% |

