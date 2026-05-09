# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 10 | -0.1691% | 0.0308% | -0.0492% | 0.517344 | 50.00% | -0.2195% | 1453.50s | `{'InitialSL': 5, 'TrailingSL': 3, 'MaxHoldExit': 1, 'BreakevenSL': 1}` | `{'candidate_events': 11, 'entries': 10, 'busy_skipped': 1, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 10 | -0.169042% | 50.0000% |

