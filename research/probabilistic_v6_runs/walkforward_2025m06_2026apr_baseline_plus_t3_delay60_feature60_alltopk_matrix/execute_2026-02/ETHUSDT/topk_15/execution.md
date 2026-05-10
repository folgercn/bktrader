# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ETHUSDT` | 11 | 2.8057% | 3.8789% | 3.4496% | 2.100974 | 63.64% | -1.5382% | 1320.00s | `{'TrailingSL': 7, 'InitialSL': 3, 'MaxHoldExit': 1}` | `{'candidate_events': 11, 'entries': 11, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### ETHUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2026-02` | 11 | 2.798667% | 63.6364% |
