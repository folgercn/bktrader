# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 56 | 0.4289% | 1.5592% | 1.1060% | 1.17728 | 60.71% | -0.5524% | 3167.00s | `{'InitialSL': 22, 'TrailingSL': 17, 'BreakevenSL': 16, 'MaxHoldExit': 1}` | `{'candidate_events': 58, 'entries': 56, 'busy_skipped': 2, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 56 | -2.8079% | -1.7132% | -2.1523% | 0.319415 | 51.79% | -2.8157% | 2021.50s | `{'InitialSL': 27, 'BreakevenSL': 20, 'TrailingSL': 8, 'MaxHoldExit': 1}` | `{'candidate_events': 56, 'entries': 56, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 19 | 0.345675% | 52.6316% |
| `2026-02` | 19 | 0.600119% | 73.6842% |
| `2026-03` | 18 | -0.513536% | 55.5556% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 14 | -0.779577% | 35.7143% |
| `2026-02` | 19 | -1.093192% | 63.1579% |
| `2026-03` | 23 | -0.970627% | 52.1739% |

