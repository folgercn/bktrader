# ETH original_t2 + micro strength 1s 回测（2026-01-01T00:00:00+00:00 至 2026-04-30T23:59:59+00:00）

范围：仅限 `research`。该回测使用 `original_t2` 三根 bar 结构：long level 为 `prev_high_2`，short level 为 `prev_low_2`，当前 signal bar 未闭合，由 `1s high/low` 触发。开仓前额外计算最近 `1s` micro strength，weak 跳过，base/strong 按 slot0/slot1 仓位入场。

成本：滑点 `2bps/side`，手续费 maker entry `2bps` + market exit `4bps`，realistic 包含两者。

| Timeframe | Variant | 笔数 | Realistic | Raw | 2bps Slip No Fee | Fees | 胜率 | Max DD | Avg Hold | Median Hold | Exit Reasons | Quality | Candidate | Weak Skip | Max Entries/Bar |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|
| `1h` | `skipweak_s10b4` | 934 | -9.7705% | 5.4922% | -0.8987% | 8.9545% | 36.94% | -3.61% | 1282.31s | 597.00s | `{'InitialSL': 588, 'TrailingSL': 345, 'FinalMarkToMarket': 1}` | `{'base': {'trades': 129, 'win_rate_pct': 38.76, 'avg_pnl_pct': -0.0157, 'median_pnl_pct': -0.1712, 'exit_reasons': {'InitialSL': 79, 'TrailingSL': 50}}, 'strong': {'trades': 805, 'win_rate_pct': 36.65, 'avg_pnl_pct': -0.0079, 'median_pnl_pct': -0.1885, 'exit_reasons': {'InitialSL': 509, 'TrailingSL': 295, 'FinalMarkToMarket': 1}}}` | 11448 | 10514 | 2 |
| `1h` | `strict_s10b4` | 933 | -9.6983% | 5.5660% | -0.8254% | 8.9546% | 36.87% | -3.61% | 1280.49s | 599.00s | `{'InitialSL': 588, 'TrailingSL': 344, 'FinalMarkToMarket': 1}` | `{'base': {'trades': 155, 'win_rate_pct': 36.13, 'avg_pnl_pct': -0.0338, 'median_pnl_pct': -0.1787, 'exit_reasons': {'InitialSL': 99, 'TrailingSL': 56}}, 'strong': {'trades': 778, 'win_rate_pct': 37.02, 'avg_pnl_pct': -0.0031, 'median_pnl_pct': -0.1886, 'exit_reasons': {'InitialSL': 489, 'TrailingSL': 288, 'FinalMarkToMarket': 1}}}` | 12762 | 11829 | 2 |

## 文件

- Summary JSON：`research/eth_2026_jan_apr_1h_original_t2_micro_filter_summary.json`
- `skipweak_s10b4` ledger：`research/tmp_eth_2026_jan_apr_1h_original_t2_micro_filter_1h_skipweak_s10b4_ledger.csv`
- `strict_s10b4` ledger：`research/tmp_eth_2026_jan_apr_1h_original_t2_micro_filter_1h_strict_s10b4_ledger.csv`
