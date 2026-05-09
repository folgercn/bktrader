# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 5 | -0.0893% | 0.0106% | -0.0293% | 0.723361 | 40.00% | -0.3215% | 2096.00s | `{'InitialSL': 3, 'TrailingSL': 2}` | `{'candidate_events': 5, 'entries': 5, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 19 | 0.5706% | 0.9533% | 0.8001% | 1.990462 | 68.42% | -0.3636% | 1426.00s | `{'TrailingSL': 10, 'InitialSL': 6, 'BreakevenSL': 3}` | `{'candidate_events': 21, 'entries': 19, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 2, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-12` | 5 | -0.089031% | 40.0000% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-12` | 19 | 0.569983% | 68.4211% |

