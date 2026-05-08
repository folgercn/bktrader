# BTCUSDT original_t2 pre-touch 状态入场回测（2026-01-01T00:00:00+00:00 至 2026-04-30T23:59:59+00:00）

范围：仅限 `research`。本回测消费 true `original_t2` pre-touch 样本表，只在高 proxy edge 分箱出现时，于下一根 `1s close` 市价入场，退出沿用 direct-breakout baseline 的 `InitialSL/TrailingSL/PT` 逻辑。

成本：滑点 `2bps/side`，手续费 maker entry `2bps` + market exit `4bps`，realistic 包含两者。

| Variant | States | 笔数 | Realistic | Raw | 2bps Slip No Fee | Fees | 胜率 | Max DD | Avg Hold | Median Hold | Max Entries/Bar | Exit Reasons |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `fast_clean` | `[{'distance_bucket': '0.10-0.15', 'speed300_bucket': '>=0.20', 'pullback_bucket': '0-0.02'}]` | 103 | -0.5323% | 1.5379% | 0.7047% | 1.2323% | 43.69% | -0.35% | 1908.26s | 1147.00s | 1 | `{'InitialSL': 58, 'TrailingSL': 45}` |
| `fast_clean_or_small_pullback` | `[{'distance_bucket': '0.10-0.15', 'speed300_bucket': '>=0.20', 'pullback_bucket': '0-0.02'}, {'distance_bucket': '0.10-0.15', 'speed300_bucket': '>=0.20', 'pullback_bucket': '0.02-0.05'}]` | 148 | -1.2102% | 1.7577% | 0.5600% | 1.7658% | 41.89% | -0.41% | 1955.89s | 1147.50s | 1 | `{'InitialSL': 86, 'TrailingSL': 62}` |
| `edge10_c1f03` | `[{'distance_bucket': '0.10-0.15', 'speed300_bucket': '>=0.20', 'pullback_bucket': '0-0.02'}, {'distance_bucket': '0.10-0.15', 'speed300_bucket': '>=0.20', 'pullback_bucket': '0.02-0.05'}, {'distance_bucket': '0.15-0.20', 'speed300_bucket': '0.03-0.10', 'pullback_bucket': '0.02-0.05'}]` | 176 | -2.7946% | 0.6883% | -0.7195% | 2.0809% | 38.64% | -1.34% | 2143.38s | 1263.50s | 1 | `{'InitialSL': 108, 'TrailingSL': 68}` |
| `edge8_c05f03` | `[{'distance_bucket': '0.10-0.15', 'speed300_bucket': '>=0.20', 'pullback_bucket': '0-0.02'}, {'distance_bucket': '0.10-0.15', 'speed300_bucket': '>=0.20', 'pullback_bucket': '0.02-0.05'}, {'distance_bucket': '0.15-0.20', 'speed300_bucket': '0.10-0.20', 'pullback_bucket': '0.02-0.05'}]` | 179 | -1.9133% | 1.6621% | 0.2165% | 2.1256% | 41.90% | -0.41% | 1966.30s | 1206.00s | 1 | `{'InitialSL': 104, 'TrailingSL': 75}` |

## 结论

- 同一套 ETH best-state 不能直接迁移到 BTC：四个真实 1s 执行 variant 全部 realistic 为负。
- BTC 的概率表本身也更弱：best compact proxy edge 只有约 `6-7bps/notional`，没有达到约 `10bps/notional` 的成本线。
- `fast_clean` raw `+1.5379%`、滑点后 `+0.7047%`，但手续费后 `-0.5323%`；这说明 BTC 不是完全没有方向性，而是收益密度不足以覆盖成本。
- `edge10_c1f03` 扩大分箱后 raw 也被打薄到 `+0.6883%`，滑点后已转负，说明弱分箱会迅速污染信号。
- 月度归因也不支持：`edge10_c1f03` 2026-01 到 2026-04 全部为负；`fast_clean` 只有 2 月小正，其他月份为负。
- 暂不建议把 ETH pre-touch 规则作为跨品种 baseline；下一步应先做 ETH 月度/OOS 稳定性，再考虑按品种分别学习分箱。

## 月度归因

| Variant | Month | Trades | Realistic | Raw | 2bps Slip No Fee | Fees | Win Rate | Exit Reasons |
|---|---|---:|---:|---:|---:|---:|---:|---|
| `fast_clean` | 2026-01 | 29 | -0.6315% | -0.0525% | -0.2841% | 0.3473% | 34.48% | `InitialSL:19, TrailingSL:10` |
| `fast_clean` | 2026-02 | 13 | 0.4285% | 0.6889% | 0.5847% | 0.1562% | 46.15% | `InitialSL:7, TrailingSL:6` |
| `fast_clean` | 2026-03 | 23 | -0.0431% | 0.4190% | 0.2343% | 0.2774% | 52.17% | `TrailingSL:12, InitialSL:11` |
| `fast_clean` | 2026-04 | 38 | -0.2892% | 0.4759% | 0.1699% | 0.4590% | 44.74% | `InitialSL:21, TrailingSL:17` |
| `edge10_c1f03` | 2026-01 | 43 | -0.5819% | 0.2769% | -0.0666% | 0.5153% | 37.21% | `InitialSL:27, TrailingSL:16` |
| `edge10_c1f03` | 2026-02 | 38 | -0.7312% | 0.0273% | -0.2762% | 0.4550% | 36.84% | `InitialSL:24, TrailingSL:14` |
| `edge10_c1f03` | 2026-03 | 44 | -0.9451% | -0.0698% | -0.4199% | 0.5252% | 38.64% | `InitialSL:27, TrailingSL:17` |
| `edge10_c1f03` | 2026-04 | 51 | -0.5642% | 0.4481% | 0.0431% | 0.6073% | 41.18% | `InitialSL:30, TrailingSL:21` |

## 文件

- Summary JSON：`research/btc_2026_jan_apr_1h_original_t2_pretouch_entry_summary.json`
- 月度归因 JSON：`research/eth_btc_2026_jan_apr_1h_original_t2_pretouch_monthly_attribution.json`
- `fast_clean` ledger：`research/tmp_btc_2026_jan_apr_1h_original_t2_pretouch_entry_1h_fast_clean_ledger.csv`
- `fast_clean_or_small_pullback` ledger：`research/tmp_btc_2026_jan_apr_1h_original_t2_pretouch_entry_1h_fast_clean_or_small_pullback_ledger.csv`
- `edge10_c1f03` ledger：`research/tmp_btc_2026_jan_apr_1h_original_t2_pretouch_entry_1h_edge10_c1f03_ledger.csv`
- `edge8_c05f03` ledger：`research/tmp_btc_2026_jan_apr_1h_original_t2_pretouch_entry_1h_edge8_c05f03_ledger.csv`
