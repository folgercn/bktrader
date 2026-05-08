# ETH original_t2 pre-touch 状态入场回测（2026-01-01T00:00:00+00:00 至 2026-04-30T23:59:59+00:00）

范围：仅限 `research`。本回测消费 true `original_t2` pre-touch 样本表，只在高 proxy edge 分箱出现时，于下一根 `1s close` 市价入场，退出沿用 direct-breakout baseline 的 `InitialSL/TrailingSL/PT` 逻辑。

成本：滑点 `2bps/side`，手续费 maker entry `2bps` + market exit `4bps`，realistic 包含两者。

| Variant | States | 笔数 | Realistic | Raw | 2bps Slip No Fee | Fees | 胜率 | Max DD | Avg Hold | Median Hold | Max Entries/Bar | Exit Reasons |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `fast_clean` | `[{'distance_bucket': '0.10-0.15', 'speed300_bucket': '>=0.20', 'pullback_bucket': '0-0.02'}]` | 85 | 0.4430% | 2.1646% | 1.4727% | 1.0228% | 47.06% | -0.50% | 1467.60s | 1012.00s | 1 | `{'InitialSL': 45, 'TrailingSL': 40}` |
| `fast_clean_or_small_pullback` | `[{'distance_bucket': '0.10-0.15', 'speed300_bucket': '>=0.20', 'pullback_bucket': '0-0.02'}, {'distance_bucket': '0.10-0.15', 'speed300_bucket': '>=0.20', 'pullback_bucket': '0.02-0.05'}]` | 143 | 0.3929% | 3.3051% | 2.1304% | 1.7248% | 46.15% | -0.73% | 1768.15s | 907.00s | 1 | `{'InitialSL': 77, 'TrailingSL': 66}` |
| `edge10_c1f03` | `[{'distance_bucket': '0.10-0.15', 'speed300_bucket': '>=0.20', 'pullback_bucket': '0-0.02'}, {'distance_bucket': '0.10-0.15', 'speed300_bucket': '>=0.20', 'pullback_bucket': '0.02-0.05'}, {'distance_bucket': '0.15-0.20', 'speed300_bucket': '0.03-0.10', 'pullback_bucket': '0.02-0.05'}]` | 169 | 0.5107% | 3.9652% | 2.5697% | 2.0444% | 44.38% | -0.69% | 1850.17s | 1018.00s | 1 | `{'InitialSL': 94, 'TrailingSL': 75}` |
| `edge8_c05f03` | `[{'distance_bucket': '0.10-0.15', 'speed300_bucket': '>=0.20', 'pullback_bucket': '0-0.02'}, {'distance_bucket': '0.10-0.15', 'speed300_bucket': '>=0.20', 'pullback_bucket': '0.02-0.05'}, {'distance_bucket': '0.15-0.20', 'speed300_bucket': '0.10-0.20', 'pullback_bucket': '0.02-0.05'}]` | 181 | -0.1453% | 3.5351% | 2.0471% | 2.1799% | 45.30% | -0.84% | 1963.62s | 1051.00s | 1 | `{'InitialSL': 99, 'TrailingSL': 82}` |

## 结论

- 这是目前 Jan-Apr ETH 1h true `original_t2` 系列里第一批成本后为正的候选：`fast_clean` realistic `+0.4430%`，`edge10_c1f03` realistic `+0.5107%`。
- 入场明显早于 breakout：`fast_clean` median `entry_vs_breakout_bps=-9.3352bps`，`edge10_c1f03` median `-11.0298bps`。这解释了为什么它比 post-touch confirmation 更好：不是 touch 后追价，而是在高速度、低回撤状态下提前拿到约 9-11bps 的价格缓冲。
- 但收益仍薄：`edge10_c1f03` raw `+3.9652%`，滑点后 `+2.5697%`，手续费后只剩 `+0.5107%`。这说明方向成立，但还不足以直接作为强 baseline，需要跨品种/月份验证和进一步减少不好的状态。
- `edge8_c05f03` 放入另一个 proxy 正状态后 realistic 转为 `-0.1453%`，说明分箱选择很敏感，不能简单扩大样本。
- 月度归因显示收益高度集中在 2 月：`edge10_c1f03` 在 2026-02 realistic `+2.3109%`，但 1 月 `-0.4862%`、3 月 `-0.2976%`、4 月 `-1.0225%`。因此它是一个有希望的 research lead，还不是稳定 baseline。
- 本轮显式采用 `max_trades_per_bar=1`、slot0 `20%`，不是长期 baseline 的 `20%/10%` 双 slot；这是为了验证单个高质量 pre-touch setup 的可交易性。

## 月度归因

| Variant | Month | Trades | Realistic | Raw | 2bps Slip No Fee | Fees | Win Rate | Exit Reasons |
|---|---|---:|---:|---:|---:|---:|---:|---|
| `fast_clean` | 2026-01 | 31 | -0.2107% | 0.4085% | 0.1608% | 0.3715% | 41.94% | `InitialSL:18, TrailingSL:13` |
| `fast_clean` | 2026-02 | 10 | 1.2423% | 1.4440% | 1.3636% | 0.1212% | 70.00% | `TrailingSL:7, InitialSL:3` |
| `fast_clean` | 2026-03 | 16 | -0.3896% | -0.0652% | -0.1950% | 0.1946% | 31.25% | `InitialSL:11, TrailingSL:5` |
| `fast_clean` | 2026-04 | 28 | -0.1974% | 0.3704% | 0.1433% | 0.3407% | 53.57% | `TrailingSL:15, InitialSL:13` |
| `edge10_c1f03` | 2026-01 | 50 | -0.4862% | 0.5130% | 0.1133% | 0.5995% | 42.00% | `InitialSL:29, TrailingSL:21` |
| `edge10_c1f03` | 2026-02 | 36 | 2.3109% | 3.0454% | 2.7520% | 0.4411% | 55.56% | `TrailingSL:20, InitialSL:16` |
| `edge10_c1f03` | 2026-03 | 34 | -0.2976% | 0.4030% | 0.1227% | 0.4203% | 44.12% | `InitialSL:19, TrailingSL:15` |
| `edge10_c1f03` | 2026-04 | 49 | -1.0225% | -0.0153% | -0.4182% | 0.6043% | 38.78% | `InitialSL:30, TrailingSL:19` |

## 下一步

- 用同一套规则跑 BTC 2026 Jan-Apr 1h，验证是否只是在 ETH 这段样本上偶然有效。
- 对 ETH 做月度切片和 out-of-sample，例如 2025 数据或 2026-04 单月留出，检查 `fast_clean/edge10_c1f03` 是否稳定。
- 在 `edge10_c1f03` 内继续拆 side 和 exit：当前 169 笔里 `InitialSL=94`、`TrailingSL=75`，应重点找出哪些 pre-touch 状态会变成 `InitialSL`。

## 文件

- Summary JSON：`research/eth_2026_jan_apr_1h_original_t2_pretouch_entry_summary.json`
- 月度归因 JSON：`research/eth_btc_2026_jan_apr_1h_original_t2_pretouch_monthly_attribution.json`
- `fast_clean` ledger：`research/tmp_eth_2026_jan_apr_1h_original_t2_pretouch_entry_1h_fast_clean_ledger.csv`
- `fast_clean_or_small_pullback` ledger：`research/tmp_eth_2026_jan_apr_1h_original_t2_pretouch_entry_1h_fast_clean_or_small_pullback_ledger.csv`
- `edge10_c1f03` ledger：`research/tmp_eth_2026_jan_apr_1h_original_t2_pretouch_entry_1h_edge10_c1f03_ledger.csv`
- `edge8_c05f03` ledger：`research/tmp_eth_2026_jan_apr_1h_original_t2_pretouch_entry_1h_edge8_c05f03_ledger.csv`
