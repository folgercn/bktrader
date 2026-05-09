# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 56 | 1.9190% | 3.0659% | 2.6059% | 2.299405 | 78.57% | -0.6456% | 1308.00s | `{'TrailingSL': 36, 'InitialSL': 12, 'BreakevenSL': 8}` | `{'candidate_events': 65, 'entries': 56, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 9, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 49 | 0.8553% | 1.8484% | 1.4500% | 1.344764 | 69.39% | -0.7943% | 558.00s | `{'TrailingSL': 29, 'InitialSL': 14, 'BreakevenSL': 5, 'MaxHoldExit': 1}` | `{'candidate_events': 60, 'entries': 49, 'busy_skipped': 1, 'same_signal_bar_skipped': 0, 'dwell_skipped': 10, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 14 | 0.443775% | 85.7143% |
| `2026-02` | 20 | 1.560191% | 90.0000% |
| `2026-03` | 22 | -0.100152% | 63.6364% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 10 | -0.225571% | 60.0000% |
| `2026-02` | 18 | 0.242167% | 66.6667% |
| `2026-03` | 21 | 0.839878% | 76.1905% |

