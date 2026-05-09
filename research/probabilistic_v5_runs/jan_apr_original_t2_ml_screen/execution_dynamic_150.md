# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 33 | 3.9386% | 7.9969% | 6.3577% | 1.445897 | 72.73% | -4.6906% | 1380.00s | `{'TrailingSL': 19, 'InitialSL': 9, 'BreakevenSL': 5}` | `{'candidate_events': 41, 'entries': 33, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 8, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 64 | 4.8828% | 13.2180% | 9.8073% | 1.213127 | 64.06% | -4.7986% | 1978.50s | `{'TrailingSL': 36, 'InitialSL': 20, 'BreakevenSL': 5, 'MaxHoldExit': 3}` | `{'candidate_events': 97, 'entries': 64, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 33, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 7 | 1.955154% | 100.0000% |
| `2026-02` | 15 | 5.397088% | 80.0000% |
| `2026-03` | 11 | -3.388134% | 45.4545% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 16 | 1.427736% | 68.7500% |
| `2026-02` | 29 | 4.730311% | 68.9655% |
| `2026-03` | 19 | -1.095727% | 52.6316% |

