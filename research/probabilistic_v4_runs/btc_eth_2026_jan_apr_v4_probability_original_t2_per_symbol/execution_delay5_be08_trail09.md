# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 71 | 0.8314% | 2.2723% | 1.6940% | 1.286062 | 69.01% | -0.8058% | 1523.00s | `{'TrailingSL': 38, 'InitialSL': 21, 'BreakevenSL': 11, 'MaxHoldExit': 1}` | `{'candidate_events': 110, 'entries': 71, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 39, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 63 | -0.8649% | 0.3924% | -0.1125% | 0.7888 | 58.73% | -1.5755% | 1268.00s | `{'TrailingSL': 33, 'InitialSL': 25, 'BreakevenSL': 4, 'MaxHoldExit': 1}` | `{'candidate_events': 66, 'entries': 63, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 3, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 13 | 0.453735% | 84.6154% |
| `2026-02` | 28 | 0.774142% | 75.0000% |
| `2026-03` | 30 | -0.395544% | 56.6667% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 18 | -0.233007% | 61.1111% |
| `2026-02` | 23 | -0.671722% | 52.1739% |
| `2026-03` | 22 | 0.041947% | 63.6364% |

