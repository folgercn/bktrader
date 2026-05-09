# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 89 | 0.5616% | 2.3672% | 1.6412% | 1.147608 | 57.30% | -0.9741% | 2681.00s | `{'InitialSL': 37, 'TrailingSL': 28, 'BreakevenSL': 21, 'MaxHoldExit': 3}` | `{'candidate_events': 91, 'entries': 89, 'busy_skipped': 2, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 85 | -0.6189% | 1.0855% | 0.4000% | 0.889464 | 54.12% | -1.9253% | 2006.00s | `{'InitialSL': 39, 'TrailingSL': 25, 'BreakevenSL': 20, 'MaxHoldExit': 1}` | `{'candidate_events': 87, 'entries': 85, 'busy_skipped': 1, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 1, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 30 | -0.125876% | 40.0000% |
| `2026-02` | 28 | 1.199735% | 71.4286% |
| `2026-03` | 31 | -0.507584% | 61.2903% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 24 | -1.035816% | 37.5000% |
| `2026-02` | 31 | -0.250985% | 58.0645% |
| `2026-03` | 30 | 0.676716% | 63.3333% |

