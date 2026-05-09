# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 89 | 0.3852% | 2.1877% | 1.4630% | 1.094911 | 56.18% | -1.0795% | 3590.00s | `{'InitialSL': 38, 'TrailingSL': 27, 'BreakevenSL': 20, 'MaxHoldExit': 4}` | `{'candidate_events': 91, 'entries': 89, 'busy_skipped': 2, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 86 | -1.0308% | 0.6866% | -0.0041% | 0.827348 | 53.49% | -1.9935% | 2417.00s | `{'InitialSL': 39, 'TrailingSL': 23, 'BreakevenSL': 22, 'MaxHoldExit': 2}` | `{'candidate_events': 87, 'entries': 86, 'busy_skipped': 1, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 30 | -0.034400% | 43.3333% |
| `2026-02` | 28 | 0.988067% | 67.8571% |
| `2026-03` | 31 | -0.562599% | 58.0645% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 25 | -1.120060% | 36.0000% |
| `2026-02` | 31 | -0.241017% | 58.0645% |
| `2026-03` | 30 | 0.336375% | 63.3333% |

