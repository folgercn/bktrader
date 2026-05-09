# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 58 | 1.1224% | 2.3014% | 1.8285% | 1.645179 | 70.69% | -0.4205% | 1891.50s | `{'TrailingSL': 23, 'BreakevenSL': 18, 'InitialSL': 17}` | `{'candidate_events': 58, 'entries': 58, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 55 | -1.9733% | -0.8889% | -1.3240% | 0.436674 | 58.18% | -2.1899% | 1545.00s | `{'InitialSL': 22, 'BreakevenSL': 19, 'TrailingSL': 13, 'MaxHoldExit': 1}` | `{'candidate_events': 56, 'entries': 55, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 1, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 20 | 0.184920% | 60.0000% |
| `2026-02` | 19 | 1.075634% | 84.2105% |
| `2026-03` | 19 | -0.141156% | 68.4211% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 13 | -0.624263% | 46.1538% |
| `2026-02` | 19 | -1.174820% | 63.1579% |
| `2026-03` | 23 | -0.189799% | 60.8696% |

