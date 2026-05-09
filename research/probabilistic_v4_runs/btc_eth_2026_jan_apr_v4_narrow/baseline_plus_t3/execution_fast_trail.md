# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 55 | 0.8827% | 1.9976% | 1.5506% | 1.590819 | 69.09% | -0.3894% | 2584.00s | `{'TrailingSL': 20, 'BreakevenSL': 18, 'InitialSL': 16, 'MaxHoldExit': 1}` | `{'candidate_events': 56, 'entries': 55, 'busy_skipped': 1, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 56 | -1.8235% | -0.7174% | -1.1613% | 0.541291 | 51.79% | -2.2790% | 1953.00s | `{'InitialSL': 24, 'TrailingSL': 15, 'BreakevenSL': 14, 'MaxHoldExit': 3}` | `{'candidate_events': 56, 'entries': 56, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 21 | 0.259201% | 61.9048% |
| `2026-02` | 15 | 0.465376% | 73.3333% |
| `2026-03` | 19 | 0.156613% | 73.6842% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 18 | -0.825214% | 44.4444% |
| `2026-02` | 21 | -1.038800% | 52.3810% |
| `2026-03` | 17 | 0.029069% | 58.8235% |

