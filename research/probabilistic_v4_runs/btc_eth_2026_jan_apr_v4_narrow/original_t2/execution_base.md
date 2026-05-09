# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 56 | 1.0251% | 2.1621% | 1.7061% | 1.479005 | 64.29% | -0.3861% | 2489.50s | `{'InitialSL': 20, 'TrailingSL': 20, 'BreakevenSL': 15, 'MaxHoldExit': 1}` | `{'candidate_events': 58, 'entries': 56, 'busy_skipped': 2, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |
| `ETHUSDT` | 55 | -2.3211% | -1.2407% | -1.6740% | 0.385802 | 54.55% | -2.3289% | 1922.00s | `{'InitialSL': 25, 'BreakevenSL': 19, 'TrailingSL': 10, 'MaxHoldExit': 1}` | `{'candidate_events': 56, 'entries': 55, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 1, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 19 | 0.306660% | 52.6316% |
| `2026-02` | 19 | 0.944438% | 78.9474% |
| `2026-03` | 18 | -0.226967% | 61.1111% |

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 13 | -0.736505% | 38.4615% |
| `2026-02` | 19 | -1.130795% | 63.1579% |
| `2026-03` | 23 | -0.476794% | 56.5217% |

