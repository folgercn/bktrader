# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 57 | -0.2918% | 0.8521% | 0.3925% | 0.908447 | 61.40% | -1.4100% | 1907.00s | `{'InitialSL': 22, 'TrailingSL': 21, 'BreakevenSL': 13, 'MaxHoldExit': 1}` | `{'candidate_events': 75, 'entries': 57, 'busy_skipped': 2, 'same_signal_bar_skipped': 0, 'dwell_skipped': 15, 'min_stop_skipped': 1, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 17 | -0.426110% | 47.0588% |
| `2026-02` | 22 | -0.501518% | 59.0909% |
| `2026-03` | 18 | 0.640298% | 77.7778% |

