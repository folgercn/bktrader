# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 74 | 0.0813% | 1.5730% | 0.9739% | 1.024956 | 68.92% | -0.7339% | 1360.00s | `{'TrailingSL': 29, 'InitialSL': 23, 'BreakevenSL': 22}` | `{'candidate_events': 75, 'entries': 74, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 1, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 24 | -0.547637% | 58.3333% |
| `2026-02` | 29 | 0.315087% | 68.9655% |
| `2026-03` | 21 | 0.319903% | 80.9524% |

