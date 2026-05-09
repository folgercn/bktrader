# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 91 | 0.3538% | 2.1967% | 1.4555% | 1.104932 | 61.54% | -0.8698% | 2446.00s | `{'InitialSL': 34, 'TrailingSL': 31, 'BreakevenSL': 24, 'MaxHoldExit': 2}` | `{'candidate_events': 91, 'entries': 91, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 85 | -0.9525% | 0.7464% | 0.0630% | 0.813242 | 57.65% | -2.0445% | 1793.00s | `{'InitialSL': 35, 'TrailingSL': 26, 'BreakevenSL': 22, 'MaxHoldExit': 2}` | `{'candidate_events': 87, 'entries': 85, 'busy_skipped': 1, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 1, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 31 | -0.267675% | 41.9355% |
| `2026-02` | 28 | 0.894960% | 75.0000% |
| `2026-03` | 32 | -0.269380% | 65.6250% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 24 | -1.037073% | 41.6667% |
| `2026-02` | 31 | -0.444235% | 58.0645% |
| `2026-03` | 30 | 0.532692% | 70.0000% |

