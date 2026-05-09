# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 66 | 0.4296% | 1.7632% | 1.2280% | 1.15672 | 60.61% | -0.7716% | 3400.00s | `{'InitialSL': 26, 'TrailingSL': 20, 'BreakevenSL': 18, 'MaxHoldExit': 2}` | `{'candidate_events': 68, 'entries': 66, 'busy_skipped': 2, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 73 | -0.4083% | 1.0563% | 0.4680% | 0.902528 | 61.64% | -1.8486% | 2201.00s | `{'InitialSL': 28, 'TrailingSL': 23, 'BreakevenSL': 21, 'MaxHoldExit': 1}` | `{'candidate_events': 75, 'entries': 73, 'busy_skipped': 1, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 1, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 24 | 0.047502% | 45.8333% |
| `2026-02` | 19 | 0.714095% | 78.9474% |
| `2026-03` | 23 | -0.328274% | 60.8696% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 24 | -0.732874% | 50.0000% |
| `2026-02` | 28 | -0.271698% | 64.2857% |
| `2026-03` | 21 | 0.602414% | 71.4286% |

