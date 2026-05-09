# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 66 | 0.0684% | 1.3972% | 0.8640% | 1.024077 | 57.58% | -0.9204% | 3610.50s | `{'InitialSL': 28, 'TrailingSL': 18, 'BreakevenSL': 17, 'MaxHoldExit': 3}` | `{'candidate_events': 68, 'entries': 66, 'busy_skipped': 2, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 74 | -0.7254% | 0.7548% | 0.1601% | 0.839584 | 60.81% | -1.9920% | 2736.50s | `{'InitialSL': 29, 'BreakevenSL': 23, 'TrailingSL': 20, 'MaxHoldExit': 2}` | `{'candidate_events': 75, 'entries': 74, 'busy_skipped': 1, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 24 | 0.086516% | 45.8333% |
| `2026-02` | 19 | 0.395673% | 73.6842% |
| `2026-03` | 23 | -0.409012% | 56.5217% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 25 | -0.866810% | 48.0000% |
| `2026-02` | 28 | -0.232657% | 64.2857% |
| `2026-03` | 21 | 0.379336% | 71.4286% |

