# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 15 | 0.5526% | 0.8542% | 0.7336% | 2.536762 | 80.00% | -0.3586% | 3638.00s | `{'TrailingSL': 10, 'InitialSL': 3, 'BreakevenSL': 2}` | `{'candidate_events': 15, 'entries': 15, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 17 | 0.2901% | 0.6313% | 0.4948% | 1.309991 | 64.71% | -0.7020% | 772.00s | `{'TrailingSL': 11, 'InitialSL': 5, 'MaxHoldExit': 1}` | `{'candidate_events': 17, 'entries': 17, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 5 | 0.173333% | 100.0000% |
| `2026-02` | 5 | 0.567536% | 100.0000% |
| `2026-03` | 5 | -0.189132% | 40.0000% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 5 | 0.193067% | 80.0000% |
| `2026-02` | 10 | -0.105833% | 50.0000% |
| `2026-03` | 2 | 0.204326% | 100.0000% |

