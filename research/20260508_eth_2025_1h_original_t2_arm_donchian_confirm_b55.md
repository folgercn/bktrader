# original_t2 arm + Donchian confirm 回测（2025-01-01T00:00:00+00:00 至 2025-12-31T23:59:59+00:00）

范围：仅限 `research`。`original_t2` 只负责在当前 signal bar 内 armed；真实开仓等同一根 bar 触碰 `prev_high_8/prev_low_8` 后，以下一根 `1s close` 市价成交。`prev_high_8/prev_low_8` 是 Donchian-style confirm level，不是 baseline 结构语义。

成本：滑点 `2bps/side`，手续费 maker entry `2bps` + market exit `4bps`，realistic 包含两者。

| Symbol | Variant | Exit | Trades | Realistic | Raw | 2bps Slip No Fee | Fees | Win | Max DD | Avg Hold | Exits | Entries | Touches | BarGateSkip | WeakSkip | Quality |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|---|
| `ETHUSDT` | `b55_loose` | `baseline` | 505 | -13.5367% | 0.5486% | -5.3412% | 8.4150% | 34.85% | -7.15% | 877.13s | `{'InitialSL': 329, 'TrailingSL': 176}` | 505 | 236876 | 219661 | 295 | `{'weak': 295, 'base': 6, 'strong': 499}` |
| `ETHUSDT` | `b55_loose_trail0p5_act1p0` | `trail0p5_act1p0` | 500 | -14.0575% | -0.2063% | -5.9959% | 8.3231% | 22.40% | -7.13% | 1732.43s | `{'InitialSL': 388, 'TrailingSL': 112}` | 500 | 236745 | 219535 | 295 | `{'weak': 295, 'base': 6, 'strong': 494}` |
| `ETHUSDT` | `b55_loose_structure1p0_b4` | `structure1p0_b4` | 489 | -15.8718% | -2.6337% | -8.1621% | 8.1957% | 9.41% | -13.81% | 6069.19s | `{'InitialSL': 432, 'StructureSL': 57}` | 489 | 218680 | 201556 | 295 | `{'weak': 295, 'base': 6, 'strong': 483}` |

## 文件

- Summary JSON：`research/eth_2025_1h_original_t2_arm_donchian_confirm_b55_summary.json`
- `ETHUSDT b55_loose baseline` ledger：`research/tmp_eth_2025_1h_original_t2_arm_donchian_confirm_b55_ETHUSDT_1h_b55_loose_ledger.csv`
- `ETHUSDT b55_loose_trail0p5_act1p0 trail0p5_act1p0` ledger：`research/tmp_eth_2025_1h_original_t2_arm_donchian_confirm_b55_ETHUSDT_1h_b55_loose_trail0p5_act1p0_ledger.csv`
- `ETHUSDT b55_loose_structure1p0_b4 structure1p0_b4` ledger：`research/tmp_eth_2025_1h_original_t2_arm_donchian_confirm_b55_ETHUSDT_1h_b55_loose_structure1p0_b4_ledger.csv`
