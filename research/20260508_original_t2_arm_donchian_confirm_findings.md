# original_t2 arm + Donchian confirm 阶段结论

范围：仅限 `research`。本轮没有修改 live/internal 逻辑。

## 测试语义

- `original_t2` 只负责 armed：当前 1h signal bar 内先触碰 `prev_high_2/prev_low_2`。
- 真正开仓层级改为 Donchian confirm：同一根 1h bar 内触碰 `prev_high_8/prev_low_8` 后，以下一根 `1s close` 市价成交。
- `prev_high_8/prev_low_8` 是 confirm level，不是 baseline 的结构突破语义。
- bar gate 使用 intrabar 当前值：`range_atr`、`body_ratio`、`close_pos`、`pre_range_6`、EMA20 slope。
- micro gate 使用 300s/60s speed、efficiency、close position，分为 weak/base/strong；weak 默认跳过。
- 成本统一使用：滑点 `2bps/side`，maker entry fee `2bps`，market exit fee `4bps`。

## 2026 Jan-Apr 结果

| Symbol | Variant | Exit | Trades | Realistic | Raw | 结论 |
|---|---|---|---:|---:|---:|---|
| ETHUSDT | `s10b4` | baseline | 105 | -1.9873% | +1.1182% | strict gate 不过成本 |
| ETHUSDT | `s10b4_trail0p5_act1p0` | delayed trailing | 105 | -0.5089% | +2.6435% | 有改善但仍不过成本 |
| ETHUSDT | `s10b4_structure1p0_b4` | structure | 105 | -3.5931% | -0.5380% | 结构退出明显恶化 |
| ETHUSDT | `b55_loose` | baseline | 151 | -1.7526% | +2.7688% | 宽松 gate raw 较好，但成本后亏 |
| ETHUSDT | `b55_loose_structure1p0_b4` | structure | 148 | +1.7724% | +6.3595% | ETH 2026 局部成立 |
| BTCUSDT | `s10b4` | baseline | 89 | -2.8822% | -0.2640% | 不成立 |
| BTCUSDT | `s10b4_structure1p0_b4` | structure | 88 | -2.0191% | +0.5937% | 少亏但不过成本 |
| BTCUSDT | `b55_loose` | baseline | 145 | -3.0757% | +1.1825% | 不成立 |
| BTCUSDT | `b55_loose_structure1p0_b4` | structure | 136 | +0.0985% | +4.2143% | 勉强打平，不够做 baseline |

2026 里唯一值得看的组合是 `b55_loose_structure1p0_b4`：ETH 有 `+1.7724%`，BTC 只有 `+0.0985%`。这说明 Donchian confirm + 宽松 bar gate + 结构退出可以过滤掉一部分差交易，但 edge 仍然很薄，且跨币种不稳定。

## ETH 2025 全年验证

| Variant | Exit | Trades | Realistic | Raw | 结论 |
|---|---|---:|---:|---:|---|
| `b55_loose` | baseline | 505 | -13.5367% | +0.5486% | 成本后大幅亏损 |
| `b55_loose_trail0p5_act1p0` | delayed trailing | 500 | -14.0575% | -0.2063% | 更差 |
| `b55_loose_structure1p0_b4` | structure | 489 | -15.8718% | -2.6337% | 结构退出没有复现旧 proxy 强度 |

这组验证基本否定了把 `original_t2 arm + Donchian confirm` 作为新 baseline 的想法。它在 ETH 2026 Jan-Apr 出现正收益，但在 ETH 2025 全年完全崩掉，和旧 closed-bar `s10b4_nohold` proxy 的 ETH 2025 `+12.9616%` 不是同一类 edge。

## 判断

`prev_high_8/prev_low_8` 的强度不是来自“当前 bar 内先 T2 再 d8 confirm”这个过程。旧 proxy 强在 closed-bar 级别的 8-bar breakout shape：它等一根完整强 bar 收盘确认后才进入 1s 执行段，天然过滤掉了大量 intrabar 假突破。把它改成同一根 bar 内 intrabar confirm，会重新暴露在假突破和成本损耗里。

因此不建议把本轮 `original_t2 arm + Donchian confirm` 接成新 research baseline，也不建议接 live。

## 下一步方向

更可行的路线不是继续压 intrabar confirm，而是转成“强 bar 收盘确认后的 pullback/continuation 入场”：

- 保留 8-bar Donchian closed-bar breakout 作为候选来源。
- 不在 breakout 收盘瞬间追市价，而是在下一根 bar 内等待小回撤或微结构延续确认。
- 用 1s/1m 的速度衰减、回撤深度、VWAP/EMA 位置决定是否入场和仓位。
- 退出继续测结构移动止损，但只对 closed-bar confirmed trend setup 生效。
- 对照必须同时跑 ETH 2025 全年、ETH 2026 Jan-Apr、BTC 2026 Jan-Apr。

## 产物

- Runner：`research/original_t2_arm_donchian_confirm_entry.py`
- ETH/BTC 2026 Jan-Apr：`research/20260508_eth_btc_2026_jan_apr_1h_original_t2_arm_donchian_confirm.md`
- ETH 2025 全年：`research/20260508_eth_2025_1h_original_t2_arm_donchian_confirm_b55.md`
- Summary JSON：`research/eth_btc_2026_jan_apr_1h_original_t2_arm_donchian_confirm_summary.json`
- Summary JSON：`research/eth_2025_1h_original_t2_arm_donchian_confirm_b55_summary.json`
