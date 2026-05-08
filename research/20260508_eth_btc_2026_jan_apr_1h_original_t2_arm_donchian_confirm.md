# original_t2 arm + Donchian confirm 回测（2026-01-01T00:00:00+00:00 至 2026-04-30T23:59:59+00:00）

范围：仅限 `research`。`original_t2` 只负责在当前 signal bar 内 armed；真实开仓等同一根 bar 触碰 `prev_high_8/prev_low_8` 后，以下一根 `1s close` 市价成交。`prev_high_8/prev_low_8` 是 Donchian-style confirm level，不是 baseline 结构语义。

成本：滑点 `2bps/side`，手续费 maker entry `2bps` + market exit `4bps`，realistic 包含两者。

| Symbol | Variant | Exit | Trades | Realistic | Raw | 2bps Slip No Fee | Fees | Win | Max DD | Avg Hold | Exits | Entries | Touches | BarGateSkip | WeakSkip | Quality |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|---|
| `ETHUSDT` | `s10b4` | `baseline` | 105 | -1.9873% | 1.1182% | -0.1350% | 1.8616% | 35.24% | -1.64% | 405.01s | `{'InitialSL': 68, 'TrailingSL': 37}` | 105 | 151554 | 133087 | 23 | `{'weak': 23, 'base': 3, 'strong': 102}` |
| `ETHUSDT` | `s10b4_trail0p5_act1p0` | `trail0p5_act1p0` | 105 | -0.5089% | 2.6435% | 1.3710% | 1.8720% | 20.95% | -1.41% | 929.71s | `{'InitialSL': 83, 'TrailingSL': 22}` | 105 | 151308 | 132841 | 23 | `{'weak': 23, 'base': 3, 'strong': 102}` |
| `ETHUSDT` | `s10b4_structure1p0_b4` | `structure1p0_b4` | 105 | -3.5931% | -0.5380% | -1.7709% | 1.8427% | 9.52% | -2.53% | 4298.67s | `{'InitialSL': 94, 'StructureSL': 11}` | 105 | 143006 | 124539 | 23 | `{'weak': 23, 'base': 3, 'strong': 102}` |
| `ETHUSDT` | `s10b4_notrend` | `baseline` | 108 | -2.0862% | 1.1071% | -0.1819% | 1.9141% | 35.19% | -1.65% | 395.56s | `{'InitialSL': 70, 'TrailingSL': 38}` | 108 | 150524 | 132123 | 23 | `{'weak': 23, 'base': 3, 'strong': 105}` |
| `ETHUSDT` | `s10b4_notrend_trail0p5_act1p0` | `trail0p5_act1p0` | 108 | -0.7186% | 2.5194% | 1.2121% | 1.9247% | 20.37% | -1.51% | 922.27s | `{'InitialSL': 86, 'TrailingSL': 22}` | 108 | 150278 | 131877 | 23 | `{'weak': 23, 'base': 3, 'strong': 105}` |
| `ETHUSDT` | `s10b4_notrend_structure1p0_b4` | `structure1p0_b4` | 108 | -3.7963% | -0.6582% | -1.9249% | 1.8944% | 9.26% | -2.65% | 4197.64s | `{'InitialSL': 97, 'StructureSL': 11}` | 108 | 141976 | 123575 | 23 | `{'weak': 23, 'base': 3, 'strong': 105}` |
| `ETHUSDT` | `b55_loose` | `baseline` | 151 | -1.7526% | 2.7688% | 0.9363% | 2.6898% | 36.42% | -1.88% | 559.92s | `{'InitialSL': 96, 'TrailingSL': 55}` | 151 | 90416 | 79688 | 27 | `{'weak': 27, 'base': 3, 'strong': 148}` |
| `ETHUSDT` | `b55_loose_trail0p5_act1p0` | `trail0p5_act1p0` | 151 | -3.8789% | 0.5458% | -1.2478% | 2.6512% | 21.85% | -2.97% | 1157.25s | `{'InitialSL': 118, 'TrailingSL': 33}` | 151 | 90416 | 79688 | 27 | `{'weak': 27, 'base': 3, 'strong': 148}` |
| `ETHUSDT` | `b55_loose_structure1p0_b4` | `structure1p0_b4` | 148 | 1.7724% | 6.3595% | 4.5007% | 2.6856% | 12.84% | -2.75% | 6974.52s | `{'InitialSL': 127, 'StructureSL': 21}` | 148 | 73595 | 65368 | 21 | `{'weak': 21, 'base': 3, 'strong': 145}` |
| `BTCUSDT` | `s10b4` | `baseline` | 89 | -2.8822% | -0.2640% | -1.3194% | 1.5755% | 31.46% | -1.53% | 693.66s | `{'InitialSL': 61, 'TrailingSL': 28}` | 89 | 161508 | 139723 | 24 | `{'weak': 24, 'base': 1, 'strong': 88}` |
| `BTCUSDT` | `s10b4_trail0p5_act1p0` | `trail0p5_act1p0` | 89 | -2.8612% | -0.2418% | -1.2981% | 1.5711% | 19.10% | -1.46% | 1413.46s | `{'InitialSL': 72, 'TrailingSL': 17}` | 89 | 160022 | 138237 | 24 | `{'weak': 24, 'base': 1, 'strong': 88}` |
| `BTCUSDT` | `s10b4_structure1p0_b4` | `structure1p0_b4` | 88 | -2.0191% | 0.5937% | -0.4605% | 1.5562% | 9.09% | -1.68% | 5375.44s | `{'InitialSL': 80, 'StructureSL': 8}` | 88 | 150735 | 130179 | 22 | `{'weak': 22, 'base': 1, 'strong': 87}` |
| `BTCUSDT` | `s10b4_notrend` | `baseline` | 92 | -3.0377% | -0.3339% | -1.4241% | 1.6274% | 31.52% | -1.53% | 672.73s | `{'InitialSL': 63, 'TrailingSL': 29}` | 92 | 158236 | 136348 | 24 | `{'weak': 24, 'base': 1, 'strong': 91}` |
| `BTCUSDT` | `s10b4_notrend_trail0p5_act1p0` | `trail0p5_act1p0` | 92 | -2.9529% | -0.2463% | -1.3380% | 1.6240% | 19.57% | -1.55% | 1376.48s | `{'InitialSL': 74, 'TrailingSL': 18}` | 92 | 156750 | 134862 | 24 | `{'weak': 24, 'base': 1, 'strong': 91}` |
| `BTCUSDT` | `s10b4_notrend_structure1p0_b4` | `structure1p0_b4` | 91 | -2.2079% | 0.4903% | -0.5986% | 1.6074% | 8.79% | -1.73% | 5243.91s | `{'InitialSL': 83, 'StructureSL': 8}` | 91 | 147463 | 126804 | 22 | `{'weak': 22, 'base': 1, 'strong': 90}` |
| `BTCUSDT` | `b55_loose` | `baseline` | 145 | -3.0757% | 1.1825% | -0.5422% | 2.5471% | 40.00% | -1.27% | 1012.02s | `{'InitialSL': 87, 'TrailingSL': 58}` | 145 | 87132 | 78617 | 169 | `{'weak': 169, 'base': 5, 'strong': 140}` |
| `BTCUSDT` | `b55_loose_trail0p5_act1p0` | `trail0p5_act1p0` | 144 | -2.9275% | 1.3072% | -0.4080% | 2.5353% | 25.00% | -2.03% | 1879.72s | `{'InitialSL': 108, 'TrailingSL': 36}` | 144 | 85575 | 77061 | 169 | `{'weak': 169, 'base': 5, 'strong': 139}` |
| `BTCUSDT` | `b55_loose_structure1p0_b4` | `structure1p0_b4` | 136 | 0.0985% | 4.2143% | 2.5481% | 2.4264% | 13.97% | -1.75% | 7585.35s | `{'InitialSL': 115, 'StructureSL': 21}` | 136 | 73466 | 64974 | 169 | `{'weak': 169, 'base': 5, 'strong': 131}` |

## 文件

- Summary JSON：`research/eth_btc_2026_jan_apr_1h_original_t2_arm_donchian_confirm_summary.json`
- `ETHUSDT s10b4 baseline` ledger：`research/tmp_eth_btc_2026_jan_apr_1h_original_t2_arm_donchian_confirm_ETHUSDT_1h_s10b4_ledger.csv`
- `ETHUSDT s10b4_trail0p5_act1p0 trail0p5_act1p0` ledger：`research/tmp_eth_btc_2026_jan_apr_1h_original_t2_arm_donchian_confirm_ETHUSDT_1h_s10b4_trail0p5_act1p0_ledger.csv`
- `ETHUSDT s10b4_structure1p0_b4 structure1p0_b4` ledger：`research/tmp_eth_btc_2026_jan_apr_1h_original_t2_arm_donchian_confirm_ETHUSDT_1h_s10b4_structure1p0_b4_ledger.csv`
- `ETHUSDT s10b4_notrend baseline` ledger：`research/tmp_eth_btc_2026_jan_apr_1h_original_t2_arm_donchian_confirm_ETHUSDT_1h_s10b4_notrend_ledger.csv`
- `ETHUSDT s10b4_notrend_trail0p5_act1p0 trail0p5_act1p0` ledger：`research/tmp_eth_btc_2026_jan_apr_1h_original_t2_arm_donchian_confirm_ETHUSDT_1h_s10b4_notrend_trail0p5_act1p0_ledger.csv`
- `ETHUSDT s10b4_notrend_structure1p0_b4 structure1p0_b4` ledger：`research/tmp_eth_btc_2026_jan_apr_1h_original_t2_arm_donchian_confirm_ETHUSDT_1h_s10b4_notrend_structure1p0_b4_ledger.csv`
- `ETHUSDT b55_loose baseline` ledger：`research/tmp_eth_btc_2026_jan_apr_1h_original_t2_arm_donchian_confirm_ETHUSDT_1h_b55_loose_ledger.csv`
- `ETHUSDT b55_loose_trail0p5_act1p0 trail0p5_act1p0` ledger：`research/tmp_eth_btc_2026_jan_apr_1h_original_t2_arm_donchian_confirm_ETHUSDT_1h_b55_loose_trail0p5_act1p0_ledger.csv`
- `ETHUSDT b55_loose_structure1p0_b4 structure1p0_b4` ledger：`research/tmp_eth_btc_2026_jan_apr_1h_original_t2_arm_donchian_confirm_ETHUSDT_1h_b55_loose_structure1p0_b4_ledger.csv`
- `BTCUSDT s10b4 baseline` ledger：`research/tmp_eth_btc_2026_jan_apr_1h_original_t2_arm_donchian_confirm_BTCUSDT_1h_s10b4_ledger.csv`
- `BTCUSDT s10b4_trail0p5_act1p0 trail0p5_act1p0` ledger：`research/tmp_eth_btc_2026_jan_apr_1h_original_t2_arm_donchian_confirm_BTCUSDT_1h_s10b4_trail0p5_act1p0_ledger.csv`
- `BTCUSDT s10b4_structure1p0_b4 structure1p0_b4` ledger：`research/tmp_eth_btc_2026_jan_apr_1h_original_t2_arm_donchian_confirm_BTCUSDT_1h_s10b4_structure1p0_b4_ledger.csv`
- `BTCUSDT s10b4_notrend baseline` ledger：`research/tmp_eth_btc_2026_jan_apr_1h_original_t2_arm_donchian_confirm_BTCUSDT_1h_s10b4_notrend_ledger.csv`
- `BTCUSDT s10b4_notrend_trail0p5_act1p0 trail0p5_act1p0` ledger：`research/tmp_eth_btc_2026_jan_apr_1h_original_t2_arm_donchian_confirm_BTCUSDT_1h_s10b4_notrend_trail0p5_act1p0_ledger.csv`
- `BTCUSDT s10b4_notrend_structure1p0_b4 structure1p0_b4` ledger：`research/tmp_eth_btc_2026_jan_apr_1h_original_t2_arm_donchian_confirm_BTCUSDT_1h_s10b4_notrend_structure1p0_b4_ledger.csv`
- `BTCUSDT b55_loose baseline` ledger：`research/tmp_eth_btc_2026_jan_apr_1h_original_t2_arm_donchian_confirm_BTCUSDT_1h_b55_loose_ledger.csv`
- `BTCUSDT b55_loose_trail0p5_act1p0 trail0p5_act1p0` ledger：`research/tmp_eth_btc_2026_jan_apr_1h_original_t2_arm_donchian_confirm_BTCUSDT_1h_b55_loose_trail0p5_act1p0_ledger.csv`
- `BTCUSDT b55_loose_structure1p0_b4 structure1p0_b4` ledger：`research/tmp_eth_btc_2026_jan_apr_1h_original_t2_arm_donchian_confirm_BTCUSDT_1h_b55_loose_structure1p0_b4_ledger.csv`
