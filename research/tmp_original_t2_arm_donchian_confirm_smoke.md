# original_t2 arm + Donchian confirm 回测（2026-01-01T00:00:00+00:00 至 2026-01-03T23:59:59+00:00）

范围：仅限 `research`。`original_t2` 只负责在当前 signal bar 内 armed；真实开仓等同一根 bar 触碰 `prev_high_8/prev_low_8` 后，以下一根 `1s close` 市价成交。`prev_high_8/prev_low_8` 是 Donchian-style confirm level，不是 baseline 结构语义。

成本：滑点 `2bps/side`，手续费 maker entry `2bps` + market exit `4bps`，realistic 包含两者。

| Symbol | Variant | Exit | Trades | Realistic | Raw | 2bps Slip No Fee | Fees | Win | Max DD | Avg Hold | Exits | Entries | Touches | BarGateSkip | WeakSkip | Quality |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|---|
| `ETHUSDT` | `s10b4` | `baseline` | 3 | -0.0919% | -0.0019% | -0.0379% | 0.0540% | 33.33% | -0.10% | 642.00s | `{'InitialSL': 2, 'TrailingSL': 1}` | 3 | 2319 | 2316 | 0 | `{'weak': 0, 'base': 0, 'strong': 3}` |

## 文件

- Summary JSON：`research/tmp_original_t2_arm_donchian_confirm_smoke_summary.json`
- `ETHUSDT s10b4 baseline` ledger：`research/tmp_original_t2_arm_donchian_confirm_smoke_ETHUSDT_1h_s10b4_ledger.csv`
