# Probabilistic V4 Execution Runner

范围：仅限 `research`。本文件只验证 scored events 在简单 1s 执行层的资金曲线，不改变 live/internal 逻辑。

| Symbol | Trades | Realistic | Raw | 2bps Slip | PF | Win | DD | Median Hold | Exit Reasons | Diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `BTCUSDT` | 16 | -3.9965% | -2.3324% | -3.0009% | 0.078139 | 25.00% | -4.1357% | 2844.50s | `{'InitialSL': 12, 'TrailingSL': 2, 'BreakevenSL': 1, 'MaxHoldExit': 1}` | `{'candidate_events': 16, 'entries': 16, 'busy_skipped': 0, 'same_signal_bar_skipped': 0, 'dwell_skipped': 0, 'min_stop_skipped': 0, 'missing_entry_second': 0}` |

## Monthly Attribution

### BTCUSDT

| Month | Trades | Weighted Realistic | Win |
|---|---:|---:|---:|
| `2025-08` | 16 | -4.069282% | 25.0000% |
