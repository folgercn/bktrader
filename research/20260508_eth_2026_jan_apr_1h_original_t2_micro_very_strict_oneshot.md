# ETH original_t2 + micro strength 1s 回测（2026-01-01T00:00:00+00:00 至 2026-04-30T23:59:59+00:00）

范围：仅限 `research`。该回测使用 `original_t2` 三根 bar 结构：long level 为 `prev_high_2`，short level 为 `prev_low_2`，当前 signal bar 未闭合，由 `1s high/low` 触发。开仓前额外计算最近 `1s` micro strength，weak 跳过，base/strong 按 slot0/slot1 仓位入场。

成本：滑点 `2bps/side`，手续费 maker entry `2bps` + market exit `4bps`，realistic 包含两者。

| Timeframe | Variant | 笔数 | Realistic | Raw | 2bps Slip No Fee | Fees | 胜率 | Max DD | Avg Hold | Median Hold | Exit Reasons | Quality | Candidate | Weak Skip | Max Entries/Bar |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|
| `1h` | `very_strict_oneshot` | 727 | -7.0333% | 5.7862% | 0.4605% | 7.5246% | 37.55% | -3.16% | 1232.01s | 542.00s | `{'InitialSL': 453, 'TrailingSL': 273, 'FinalMarkToMarket': 1}` | `{'base': {'trades': 292, 'win_rate_pct': 37.67, 'avg_pnl_pct': 0.0254, 'median_pnl_pct': -0.1791, 'exit_reasons': {'InitialSL': 181, 'TrailingSL': 110, 'FinalMarkToMarket': 1}}, 'strong': {'trades': 435, 'win_rate_pct': 37.47, 'avg_pnl_pct': -0.0108, 'median_pnl_pct': -0.188, 'exit_reasons': {'InitialSL': 272, 'TrailingSL': 163}}}` | 915 | 188 | 2 |

## 文件

- Summary JSON：`research/eth_2026_jan_apr_1h_original_t2_micro_very_strict_oneshot_summary.json`
- `very_strict_oneshot` ledger：`research/tmp_eth_2026_jan_apr_1h_original_t2_micro_very_strict_oneshot_1h_very_strict_oneshot_ledger.csv`
