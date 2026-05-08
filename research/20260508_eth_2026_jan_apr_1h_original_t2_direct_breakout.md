# ETH 2026 Jan-Apr 1h original_t2 直接突破回测

范围：仅限 `research`。该回测使用 `original_t2` 三根 bar 结构：long level 为 `prev_high_2`，short level 为 `prev_low_2`，当前 signal bar 未闭合，由当前 bar 内 `1s high/low` 触发。进场价为触发那根 `1s close`，不使用 `re_p`，不使用 VSL，不使用 reclaim reentry。

成本：滑点 `2bps/side`，手续费 maker entry `2bps` + market exit `4bps`，realistic 包含两者。允许同一根 signal bar 内最多两次真实开仓，仓位 schedule 为 `[0.20, 0.10]`。

## 结果

| Timeframe | Shape | 笔数 | Realistic | Raw | 2bps Slip No Fee | Fees | 胜率 | Max DD | Exit Reasons | Avg Hold | Median Hold | Max Entries/Bar |
|---|---|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|
| `1h` | `original_t2` | 940 | `-9.7055%` | `+5.6316%` | `-0.7917%` | `8.9917%` | `36.70%` | `-3.65%` | `InitialSL:594, TrailingSL:345, FinalMarkToMarket:1` | 1285.23s | 612.00s | 2 |

## Slot 贡献

| Slot | 笔数 | Realistic Contribution | Raw Contribution | 2bps Slip Contribution | Fees | 胜率 | Median Hold | Exit Reasons |
|---:|---:|---:|---:|---:|---:|---:|---:|---|
| 0 | 629 | `-7.3009%` | `+4.9602%` | `-0.1748%` | `7.2102%` | `37.52%` | 705.00s | `InitialSL:392, TrailingSL:236, FinalMarkToMarket:1` |
| 1 | 311 | `-2.4046%` | `+0.6714%` | `-0.6169%` | `1.7815%` | `35.05%` | 486.00s | `InitialSL:202, TrailingSL:109` |

## 诊断

- Direct entries：940
- Long / short：477 / 463
- 有开仓的 signal bar：629
- 同 bar 两次开仓的 bar：311
- Entry 距离 breakout 中位数：`1.2756bps`
- `67.87%` entry 在 breakout level `5bps` 内
- Median MFE：`22.5910bps` / `0.2777 ATR`
- Median MAE：`21.5862bps` / `0.3036 ATR`

结论：`original_t2` 直接突破在 raw 上有正收益，但交易数太高，2bps/side 滑点后已经转负，叠加 6bps 往返手续费后 realistic 为 `-9.7055%`。这说明 `original_t2` 的问题首先是过度交易和成本压力，不是简单的 `prev_high_8` proxy 那类低频结果。

## 文件

- Summary JSON：`research/eth_2026_jan_apr_1h_original_t2_direct_breakout_summary.json`
- Ledger：`research/tmp_eth_2026_jan_apr_1h_original_t2_direct_breakout_1h_original_t2_default_baseline_observed_close_ledger.csv`
