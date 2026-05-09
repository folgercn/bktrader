# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 68 | 1.5229% | 2.9119% | 2.3545% | 1.783076 | 69.12% | -0.5670% | 2090.50s | `{'TrailingSL': 38, 'InitialSL': 21, 'BreakevenSL': 8, 'MaxHoldExit': 1}` | `{'candidate_events': 68, 'entries': 68, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-01` | 25 | -0.002238% | 52.0000% |
| `2026-02` | 19 | 1.457967% | 89.4737% |
| `2026-03` | 24 | 0.058848% | 66.6667% |

