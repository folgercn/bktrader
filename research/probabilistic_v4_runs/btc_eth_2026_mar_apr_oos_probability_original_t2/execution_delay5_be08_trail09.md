# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 22 | -0.0449% | 0.3959% | 0.2193% | 0.958659 | 63.64% | -0.6048% | 1351.50s | `{'TrailingSL': 11, 'InitialSL': 8, 'BreakevenSL': 3}` | `{'candidate_events': 22, 'entries': 22, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 17 | 0.5985% | 0.9407% | 0.8039% | 1.954516 | 70.59% | -0.3175% | 1602.00s | `{'TrailingSL': 10, 'InitialSL': 4, 'BreakevenSL': 2, 'MaxHoldExit': 1}` | `{'candidate_events': 19, 'entries': 17, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 2, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-03` | 22 | -0.043649% | 63.6364% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-03` | 17 | 0.597980% | 70.5882% |

