# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 25 | -1.6269% | 0.6249% | -0.2818% | 0.731826 | 52.00% | -3.2380% | 2506.00s | `{'TrailingSL': 12, 'InitialSL': 12, 'BreakevenSL': 1}` | `{'candidate_events': 25, 'entries': 25, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 16 | 0.6604% | 1.9452% | 1.4301% | 1.225083 | 62.50% | -2.6265% | 1989.00s | `{'TrailingSL': 8, 'InitialSL': 5, 'BreakevenSL': 2, 'MaxHoldExit': 1}` | `{'candidate_events': 18, 'entries': 16, 'busy_skipped': 2, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-03` | 25 | -1.613556% | 52.0000% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-03` | 16 | 0.678093% | 62.5000% |

