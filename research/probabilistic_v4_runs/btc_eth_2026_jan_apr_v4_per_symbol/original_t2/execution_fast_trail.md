# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 68 | 0.4814% | 1.8567% | 1.3046% | 1.194136 | 64.71% | -0.8326% | 2591.00s | `{'InitialSL': 24, 'TrailingSL': 24, 'BreakevenSL': 19, 'MaxHoldExit': 1}` | `{'candidate_events': 68, 'entries': 68, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 73 | -0.6087% | 0.8531% | 0.2658% | 0.844822 | 64.38% | -1.6756% | 1924.00s | `{'InitialSL': 26, 'TrailingSL': 25, 'BreakevenSL': 21, 'MaxHoldExit': 1}` | `{'candidate_events': 75, 'entries': 73, 'busy_skipped': 1, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 1, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 25 | -0.093319% | 48.0000% |
| `2026-02` | 19 | 0.862617% | 84.2105% |
| `2026-03` | 24 | -0.285254% | 62.5000% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 24 | -0.666990% | 54.1667% |
| `2026-02` | 28 | -0.378532% | 64.2857% |
| `2026-03` | 21 | 0.441013% | 76.1905% |

