# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 52 | -1.5415% | -0.5120% | -0.9250% | 0.578982 | 51.92% | -1.9070% | 2364.50s | `{'InitialSL': 24, 'TrailingSL': 14, 'BreakevenSL': 12, 'MaxHoldExit': 2}` | `{'candidate_events': 52, 'entries': 52, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 20 | -0.537982% | 45.0000% |
| `2026-02` | 20 | -1.055815% | 55.0000% |
| `2026-03` | 12 | 0.045118% | 58.3333% |

