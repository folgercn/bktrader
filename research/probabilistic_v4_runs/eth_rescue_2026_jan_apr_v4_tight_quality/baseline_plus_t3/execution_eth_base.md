# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 46 | -0.8151% | 0.1019% | -0.2661% | 0.757972 | 50.00% | -1.4950% | 1896.50s | `{'InitialSL': 22, 'TrailingSL': 13, 'BreakevenSL': 9, 'MaxHoldExit': 2}` | `{'candidate_events': 46, 'entries': 46, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 14 | -0.150366% | 50.0000% |
| `2026-02` | 16 | -0.650952% | 50.0000% |
| `2026-03` | 16 | -0.011724% | 50.0000% |

