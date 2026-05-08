# ETH 1h original_t2 touch 后延续确认回测（2026-01-01T00:00:00+00:00 至 2026-01-03T23:59:59+00:00）

范围：仅限 `research`。本回测使用真正 `original_t2`：long level 为 `prev_high_2`，short level 为 `prev_low_2`，当前 1h signal bar 未闭合，由 `1s high/low` 触发 touch。

成交语义：touch 后不立即成交；只有在同一根 signal bar 内，`1s close` 先到达突破方向 `confirm_atr`，且没有先触达反向 `fail_atr`，才按确认那根 `1s close` 市价成交。若同一根 1s bar 同时满足 fail 和 confirm，按保守顺序记为 fail。

成本：滑点 `2bps/side`，手续费 maker entry `2bps` + market exit `4bps`，realistic 包含两者。

| Variant | 笔数 | Realistic | Raw | 2bps Slip No Fee | Fees | 胜率 | Max DD | Avg Hold | Median Hold | Touch | Fail | Timeout | Entry Ext | PostTouch(s) | Exit Reasons |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---|
| `smoke` | 3 | -0.0244% | 0.0356% | 0.0116% | 0.0360% | 33.33% | -0.05% | 329.00s | 384.00s | 13 | 9 | 0 | `{'avg': 0.1573, 'median': 0.1741, 'p75': 0.1828, 'p90': 0.1879}` | `{'avg': 19.0, 'median': 22.0, 'p75': 28.0, 'p90': 31.6}` | `{'InitialSL': 2, 'TrailingSL': 1}` |

## 参数

- `smoke`: `{'confirm_atr': 0.1, 'fail_atr': 0.05, 'confirm_seconds': 300, 'persist_seconds': 0, 'max_entry_extension_atr': 0.35, 'one_setup_per_bar': 1.0, 'slot0_share': 0.2, 'slot1_share': 0.1}`

## 文件

- Summary JSON：`research/tmp_eth_original_t2_posttouch_quality_smoke_summary.json`
- `smoke` ledger：`research/tmp_eth_original_t2_posttouch_quality_smoke_1h_smoke_ledger.csv`
