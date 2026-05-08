# ETH original_t2 pre-touch 状态入场回测（2026-01-01T00:00:00+00:00 至 2026-01-03T23:59:59+00:00）

范围：仅限 `research`。本回测消费 true `original_t2` pre-touch 样本表，只在高 proxy edge 分箱出现时，于下一根 `1s close` 市价入场，退出沿用 direct-breakout baseline 的 `InitialSL/TrailingSL/PT` 逻辑。

成本：滑点 `2bps/side`，手续费 maker entry `2bps` + market exit `4bps`，realistic 包含两者。

| Variant | States | 笔数 | Realistic | Raw | 2bps Slip No Fee | Fees | 胜率 | Max DD | Avg Hold | Median Hold | Max Entries/Bar | Exit Reasons |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `fast_clean` | `[{'distance_bucket': '0.10-0.15', 'speed300_bucket': '>=0.20', 'pullback_bucket': '0-0.02'}]` | 2 | 0.0192% | 0.0592% | 0.0432% | 0.0240% | 0.00% | -0.03% | 964.50s | 964.50s | 1 | `{'TrailingSL': 1, 'InitialSL': 1}` |

## 文件

- Summary JSON：`research/tmp_eth_original_t2_pretouch_entry_smoke_summary.json`
- `fast_clean` ledger：`research/tmp_eth_original_t2_pretouch_entry_smoke_1h_fast_clean_ledger.csv`
