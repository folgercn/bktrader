# original_t2 + prev_high_8/prev_low_8 hybrid 阶段结论

范围：仅限 `research`。本轮没有修改 live/internal 逻辑。

## 语义边界

- `original_t2` 仍按真实三根 bar 结构理解：long level = `prev_high_2`，short level = `prev_low_2`，当前 1h signal bar 未闭合，触发/执行用连续 `1s` bar。
- `prev_high_8/prev_low_8` 在本轮只作为 Donchian-style proxy/filter/target 变量，不作为 baseline 语义来源。
- 成本统一使用：滑点 `2bps/side`，maker entry fee `2bps`，market exit fee `4bps`，也就是 roundtrip fee `6bps`。

## 已验证组合

| 数据 | 组合 | 笔数 | Realistic | Raw | 结论 |
|---|---|---:|---:|---:|---|
| ETH 2026 Jan-Apr | `fast_clean` | 85 | +0.4430% | +2.1646% | 原 pre-touch edge 为正但偏薄 |
| ETH 2026 Jan-Apr | `fast_clean_tp1p0` | 85 | +0.6068% | +2.3312% | 小幅增强，仍薄 |
| ETH 2026 Jan-Apr | `headroom_fast_small_pullback_tp1p0` | 101 | -0.0495% | +1.9897% | Donchian headroom 过滤未过成本线 |
| ETH 2026 Jan-Apr | `fast_clean_d8_far` | 65 | -0.2929% | +1.0115% | T2 离 d8 远不是好信号 |
| ETH 2026 Jan-Apr | `edge10_d8_near_trail0p5_act1p0` | 26 | +1.3150% | +1.8432% | 小样本较强 |
| ETH 2026 Jan-Apr | `edge10_d8_near_structure1p0_b4` | 25 | +2.0281% | +2.5391% | 小样本接近旧 proxy 强度 |
| BTC 2026 Jan-Apr | `fast_clean` | 103 | -0.5323% | +1.5379% | 跨币种不成立 |
| BTC 2026 Jan-Apr | `fast_clean_d8_exact` | 21 | +0.0312% | +0.4520% | 近似打平，样本少 |
| BTC 2026 Jan-Apr | `edge10_d8_near_structure1p0_b4` | 36 | -1.8564% | -1.1468% | ETH 小样本增强没有泛化 |
| ETH 2025 全年 | old closed-bar `s10b4_nohold` proxy | 112 | +12.9616% | +16.3758% | 旧 `prev_high_8` 强结果 |
| ETH 2025 全年 | true `original_t2 fast_clean` | 355 | -4.5688% | +2.4542% | 真 intrabar T2 全年不过成本 |
| ETH 2025 全年 | true `fast_clean_d8_exact_structure1p0_b4` | 62 | -1.8306% | -0.6049% | d8 near/exact 没有复现旧强度 |
| ETH 2025 全年 | true `edge10_d8_near_structure1p0_b4` | 96 | -2.9752% | -1.0924% | 2026 小样本明显过拟合 |

## 判断

`prev_high_8/prev_low_8` 不能直接作为 `original_t2` pre-touch 的简单过滤器来放大 edge。

旧 `s10b4_nohold` 强结果主要来自另一套组合：closed-bar 8-bar Donchian breakout candidate、强 bar shape/range/trend gate、按 1s speed/efficiency 分档仓位、达到 1 ATR MFE 后的结构移动止损。把其中一个 `prev_high_8` 变量拿出来接到 true `original_t2` pre-touch 上，ETH 2026 Jan-Apr 会出现漂亮小样本，但 BTC 同期和 ETH 2025 全年都不支持。

这说明当前更合理的方向不是“保留 original_t2 pre-touch，再加 d8 headroom”，而是单独做一版 live-feasible Donchian-confirmed breakout：

- `original_t2` 只作为早期 armed/观察条件，不直接开仓。
- 真正开仓改为触碰或接近 `prev_high_8/prev_low_8` 后，再用当前 1h intrabar shape、1s speed、efficiency、close position 确认。
- 退出只在 Donchian-confirmed 之后测试结构移动止损，不要给所有 T2 pre-touch 交易套结构退出。
- 先在 ETH 2025 全年、ETH 2026 Jan-Apr、BTC 2026 Jan-Apr 三组上同时过成本，再讨论接 live。

## 下一步建议

做一个新的 research runner：`original_t2_arm_donchian_confirm_entry`。

建议第一版规则：

- arm：当前 1h bar 满足 true `original_t2` 方向结构，价格尚未开仓。
- confirm level：long 用 `prev_high_8`，short 用 `prev_low_8`。
- entry：当前 1h bar 内 `1s high/low` 触碰 confirm level 后，以下一根 `1s close` 市价入场。
- gate：沿用 old proxy 的 bar shape 思路，但全部用 intrabar 当前值计算：`range_atr`、`body_ratio`、`close_pos`、`pre_range_6`、EMA20 slope。
- micro：沿用 300s/60s speed、efficiency、close_pos 分档，不满足 base/strong 就跳过。
- exit：先测 old `structure_start_atr=1.0, structure_bars=4, structure_buffer_atr=0.05`，再测 direct baseline 和 `trail0p5_act1p0`。
- 对照：同一成本模型下和 old closed-bar proxy、true original_t2 direct/pre-touch 一起列在同一张表。

只有这版能跨 ETH 2025、ETH 2026 Jan-Apr、BTC 2026 Jan-Apr 站住，才值得继续把参数压缩成 live 可配置项。

## 产物

- ETH 2026 Donchian distance sweep：`research/20260508_eth_2026_jan_apr_1h_original_t2_donchian_distance_pretouch_exit_sweep.md`
- ETH 2026 structure exit sweep：`research/20260508_eth_2026_jan_apr_1h_original_t2_donchian_structure_exit_sweep.md`
- BTC 2026 Donchian distance sweep：`research/20260508_btc_2026_jan_apr_1h_original_t2_donchian_distance_pretouch_exit_sweep.md`
- BTC 2026 structure exit sweep：`research/20260508_btc_2026_jan_apr_1h_original_t2_donchian_structure_exit_sweep.md`
- ETH 2025 structure exit sweep：`research/20260508_eth_2025_1h_original_t2_donchian_structure_exit_sweep.md`
- Runner：`research/eth_original_t2_pretouch_continuation_table.py`
- Runner：`research/eth_original_t2_pretouch_entry_replay.py`
