# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 46 | -1.5497% | -0.6392% | -1.0046% | 0.553135 | 47.83% | -1.9692% | 1677.00s | `{'InitialSL': 22, 'TrailingSL': 13, 'BreakevenSL': 9, 'MaxHoldExit': 2}` | `{'candidate_events': 46, 'entries': 46, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 14 | -0.404717% | 42.8571% |
| `2026-02` | 16 | -0.759835% | 50.0000% |
| `2026-03` | 16 | -0.392591% | 50.0000% |

