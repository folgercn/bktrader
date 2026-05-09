# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 54 | 0.9226% | 2.0176% | 1.5785% | 1.480255 | 66.67% | -0.6414% | 3349.50s | `{'TrailingSL': 19, 'InitialSL': 17, 'BreakevenSL': 16, 'MaxHoldExit': 2}` | `{'candidate_events': 56, 'entries': 54, 'busy_skipped': 2, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 56 | -2.0425% | -0.9393% | -1.3818% | 0.560765 | 46.43% | -2.3417% | 2505.50s | `{'InitialSL': 28, 'TrailingSL': 14, 'BreakevenSL': 11, 'MaxHoldExit': 3}` | `{'candidate_events': 56, 'entries': 56, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 20 | 0.815868% | 70.0000% |
| `2026-02` | 15 | 0.465794% | 66.6667% |
| `2026-03` | 19 | -0.359950% | 63.1579% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 18 | -1.033955% | 33.3333% |
| `2026-02` | 21 | -0.800635% | 52.3810% |
| `2026-03` | 17 | -0.222325% | 52.9412% |

