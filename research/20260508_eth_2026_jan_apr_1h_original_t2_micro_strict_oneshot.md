# ETH original_t2 + micro strength 1s 回测（2026-01-01T00:00:00+00:00 至 2026-04-30T23:59:59+00:00）

范围：仅限 `research`。该回测使用 `original_t2` 三根 bar 结构：long level 为 `prev_high_2`，short level 为 `prev_low_2`，当前 signal bar 未闭合，由 `1s high/low` 触发。开仓前额外计算最近 `1s` micro strength，weak 跳过，base/strong 按 slot0/slot1 仓位入场。

成本：滑点 `2bps/side`，手续费 maker entry `2bps` + market exit `4bps`，realistic 包含两者。

| Timeframe | Variant | 笔数 | Realistic | Raw | 2bps Slip No Fee | Fees | 胜率 | Max DD | Avg Hold | Median Hold | Exit Reasons | Quality | Candidate | Weak Skip | Max Entries/Bar |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|
| `1h` | `strict_oneshot` | 809 | -8.8503% | 5.0863% | -0.7255% | 8.2004% | 36.71% | -3.67% | 1269.73s | 577.00s | `{'InitialSL': 511, 'TrailingSL': 297, 'FinalMarkToMarket': 1}` | `{'base': {'trades': 141, 'win_rate_pct': 34.75, 'avg_pnl_pct': -0.0216, 'median_pnl_pct': -0.1693, 'exit_reasons': {'InitialSL': 91, 'TrailingSL': 49, 'FinalMarkToMarket': 1}}, 'strong': {'trades': 668, 'win_rate_pct': 37.13, 'avg_pnl_pct': -0.0037, 'median_pnl_pct': -0.1901, 'exit_reasons': {'InitialSL': 420, 'TrailingSL': 248}}}` | 936 | 127 | 2 |

## 文件

- Summary JSON：`research/eth_2026_jan_apr_1h_original_t2_micro_strict_oneshot_summary.json`
- `strict_oneshot` ledger：`research/tmp_eth_2026_jan_apr_1h_original_t2_micro_strict_oneshot_1h_strict_oneshot_ledger.csv`
