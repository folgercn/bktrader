# ETH 1h original_t2 touch 后延续确认回测（2026-01-01T00:00:00+00:00 至 2026-04-30T23:59:59+00:00）

范围：仅限 `research`。本回测使用真正 `original_t2`：long level 为 `prev_high_2`，short level 为 `prev_low_2`，当前 1h signal bar 未闭合，由 `1s high/low` 触发 touch。

成交语义：touch 后不立即成交；只有在同一根 signal bar 内，`1s close` 先到达突破方向 `confirm_atr`，且没有先触达反向 `fail_atr`，才按确认那根 `1s close` 市价成交。若同一根 1s bar 同时满足 fail 和 confirm，按保守顺序记为 fail。

成本：滑点 `2bps/side`，手续费 maker entry `2bps` + market exit `4bps`，realistic 包含两者。

| Variant | 笔数 | Realistic | Raw | 2bps Slip No Fee | Fees | 胜率 | Max DD | Avg Hold | Median Hold | Touch | Fail | Timeout | Entry Ext | PostTouch(s) | Exit Reasons |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---|
| `c05_f03_one` | 166 | -1.7185% | 1.5979% | 0.2590% | 1.9776% | 36.75% | -1.08% | 1563.56s | 862.50s | 629 | 458 | 1 | `{'avg': 0.0762, 'median': 0.0643, 'p75': 0.0809, 'p90': 0.1219}` | `{'avg': 11.6325, 'median': 2.0, 'p75': 11.75, 'p90': 34.5}` | `{'InitialSL': 105, 'TrailingSL': 61}` |
| `c10_f05_one` | 162 | -2.3369% | 0.8787% | -0.4196% | 1.9204% | 33.33% | -0.91% | 1529.44s | 696.00s | 629 | 448 | 11 | `{'avg': 0.135, 'median': 0.117, 'p75': 0.1467, 'p90': 0.191}` | `{'avg': 34.4691, 'median': 14.0, 'p75': 41.75, 'p90': 91.2}` | `{'InitialSL': 108, 'TrailingSL': 54}` |
| `c15_f05_one` | 127 | -2.5227% | -0.0151% | -1.0256% | 1.5053% | 30.71% | -1.50% | 1168.14s | 566.00s | 629 | 473 | 25 | `{'avg': 0.1858, 'median': 0.1647, 'p75': 0.1949, 'p90': 0.2383}` | `{'avg': 43.8819, 'median': 21.0, 'p75': 56.5, 'p90': 137.2}` | `{'InitialSL': 88, 'TrailingSL': 39}` |
| `c20_f10_one` | 180 | -2.9785% | 0.5782% | -0.8598% | 2.1266% | 37.78% | -1.71% | 760.27s | 335.00s | 629 | 348 | 97 | `{'avg': 0.2449, 'median': 0.2209, 'p75': 0.2462, 'p90': 0.3294}` | `{'avg': 69.4889, 'median': 38.0, 'p75': 104.5, 'p90': 191.0}` | `{'InitialSL': 112, 'TrailingSL': 68}` |
| `c50_f20_one` | 141 | -2.0164% | 0.7850% | -0.3443% | 1.6808% | 34.75% | -1.65% | 541.77s | 161.00s | 629 | 271 | 215 | `{'avg': 0.5496, 'median': 0.5273, 'p75': 0.5591, 'p90': 0.6347}` | `{'avg': 277.6879, 'median': 200.0, 'p75': 422.0, 'p90': 694.0}` | `{'InitialSL': 92, 'TrailingSL': 49}` |
| `p10_c10_f05_one` | 152 | -2.4848% | 0.5246% | -0.6897% | 1.8015% | 32.89% | -1.09% | 1487.67s | 676.50s | 629 | 456 | 12 | `{'avg': 0.1422, 'median': 0.1208, 'p75': 0.1535, 'p90': 0.2199}` | `{'avg': 42.7105, 'median': 24.0, 'p75': 54.5, 'p90': 100.8}` | `{'InitialSL': 102, 'TrailingSL': 50}` |

## 结论

- touch 后确认确实能降频：`candidate_touches=629`，最宽松 `c05_f03_one` 只成交 `166` 笔；但 best realistic 仍为 `-1.7185%`。
- best raw 是 `c05_f03_one` 的 `+1.5979%`，折算每单位 notional 的 raw edge 约 `4.85bps`。当前成本是滑点约 `4bps` + 手续费 `6bps`，合计约 `10bps`，所以即使 raw 为正也不够覆盖实盘成本。
- `confirm_atr` 越大并没有带来质量拐点：`0.10/0.15/0.20/0.50 ATR` 的确认都被追价吞掉，滑点后已经转负。
- 连续站上/站下 level `10s` 的 `p10_c10_f05_one` 也没有改善，说明单纯“确认更多秒”不是这个问题的主解。
- 当前问题不是只有“假突破太多”，而是 `original_t2` touch 后的可交易延续收益密度太低；确认后追价会牺牲 entry，剩下的趋势段不足以支付成本。

## 与上一轮 original_t2 对照

| Variant | 笔数 | Raw | 2bps Slip No Fee | Realistic | 每 notional raw edge |
|---|---:|---:|---:|---:|---:|
| direct | 940 | `+5.6316%` | `-0.7917%` | `-9.7055%` | `3.76bps` |
| very_strict_oneshot | 727 | `+5.7862%` | `+0.4605%` | `-7.0333%` | `4.61bps` |
| strong_only | 555 | `+3.9848%` | `-0.1670%` | `-6.0883%` | `4.01bps` |
| posttouch best `c05_f03_one` | 166 | `+1.5979%` | `+0.2590%` | `-1.7185%` | `4.85bps` |

这个对照说明：post-touch confirmation 把单笔质量从 `3.76bps` 提到约 `4.85bps`，方向是对的，但离 `10bps` 成本阈值仍很远。

## 下一步建议

不要继续只调 touch 后确认阈值。下一轮更值得测的是“更早、更有条件地进场”，即在 original_t2 level 之前，用 1m/1s 状态预测是否会触发并延续，而不是 touch 后再追。

建议的验证目标：

- 仍以 `original_t2 prev_high_2/prev_low_2` 为目标 level，不再使用 `prev_high_8/prev_low_8` proxy。
- 在距离 level `0.05~0.20 ATR` 的 pre-touch 区间建样本，特征至少包含 distance、最近 60s/300s speed、1m close 位置、回撤深度、近几根 1m range/efficiency。
- 标签不要只看“是否碰到 level”，而要看 touch 后是否先达到 `+0.5 ATR` 或 `+1.0 ATR`，以及是否先发生 `-0.2/-0.3 ATR` adverse move。
- 只有当模型/分箱显示期望收益超过约 `10bps` per notional，才值得接入真实入场回测；否则只是把亏损从 touch 后转移到 touch 前。

## 参数

- `c05_f03_one`: `{'confirm_atr': 0.05, 'fail_atr': 0.03, 'confirm_seconds': 300, 'persist_seconds': 0, 'max_entry_extension_atr': 0.25, 'one_setup_per_bar': 1.0, 'slot0_share': 0.2, 'slot1_share': 0.1}`
- `c10_f05_one`: `{'confirm_atr': 0.1, 'fail_atr': 0.05, 'confirm_seconds': 300, 'persist_seconds': 0, 'max_entry_extension_atr': 0.35, 'one_setup_per_bar': 1.0, 'slot0_share': 0.2, 'slot1_share': 0.1}`
- `c15_f05_one`: `{'confirm_atr': 0.15, 'fail_atr': 0.05, 'confirm_seconds': 300, 'persist_seconds': 0, 'max_entry_extension_atr': 0.45, 'one_setup_per_bar': 1.0, 'slot0_share': 0.2, 'slot1_share': 0.1}`
- `c20_f10_one`: `{'confirm_atr': 0.2, 'fail_atr': 0.1, 'confirm_seconds': 300, 'persist_seconds': 0, 'max_entry_extension_atr': 0.55, 'one_setup_per_bar': 1.0, 'slot0_share': 0.2, 'slot1_share': 0.1}`
- `c50_f20_one`: `{'confirm_atr': 0.5, 'fail_atr': 0.2, 'confirm_seconds': 900, 'persist_seconds': 0, 'max_entry_extension_atr': 0.8, 'one_setup_per_bar': 1.0, 'slot0_share': 0.2, 'slot1_share': 0.1}`
- `p10_c10_f05_one`: `{'confirm_atr': 0.1, 'fail_atr': 0.05, 'confirm_seconds': 300, 'persist_seconds': 10, 'max_entry_extension_atr': 0.35, 'one_setup_per_bar': 1.0, 'slot0_share': 0.2, 'slot1_share': 0.1}`

## 文件

- Summary JSON：`research/eth_2026_jan_apr_1h_original_t2_posttouch_quality_summary.json`
- `c05_f03_one` ledger：`research/tmp_eth_2026_jan_apr_1h_original_t2_posttouch_quality_1h_c05_f03_one_ledger.csv`
- `c10_f05_one` ledger：`research/tmp_eth_2026_jan_apr_1h_original_t2_posttouch_quality_1h_c10_f05_one_ledger.csv`
- `c15_f05_one` ledger：`research/tmp_eth_2026_jan_apr_1h_original_t2_posttouch_quality_1h_c15_f05_one_ledger.csv`
- `c20_f10_one` ledger：`research/tmp_eth_2026_jan_apr_1h_original_t2_posttouch_quality_1h_c20_f10_one_ledger.csv`
- `c50_f20_one` ledger：`research/tmp_eth_2026_jan_apr_1h_original_t2_posttouch_quality_1h_c50_f20_one_ledger.csv`
- `p10_c10_f05_one` ledger：`research/tmp_eth_2026_jan_apr_1h_original_t2_posttouch_quality_1h_p10_c10_f05_one_ledger.csv`
