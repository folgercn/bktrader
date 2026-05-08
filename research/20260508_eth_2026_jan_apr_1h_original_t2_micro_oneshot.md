# ETH original_t2 + micro strength 1s 回测（2026-01-01T00:00:00+00:00 至 2026-04-30T23:59:59+00:00）

范围：仅限 `research`。该回测使用 `original_t2` 三根 bar 结构：long level 为 `prev_high_2`，short level 为 `prev_low_2`，当前 signal bar 未闭合，由 `1s high/low` 触发。开仓前额外计算最近 `1s` micro strength，weak 跳过，base/strong 按 slot0/slot1 仓位入场。

成本：滑点 `2bps/side`，手续费 maker entry `2bps` + market exit `4bps`，realistic 包含两者。

| Timeframe | Variant | 笔数 | Realistic | Raw | 2bps Slip No Fee | Fees | 胜率 | Max DD | Avg Hold | Median Hold | Exit Reasons | Quality | Candidate | Weak Skip | Max Entries/Bar |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|
| `1h` | `oneshot_s10b4` | 845 | -9.0969% | 5.3169% | -0.7026% | 8.4765% | 36.80% | -3.77% | 1292.88s | 586.00s | `{'InitialSL': 533, 'TrailingSL': 311, 'FinalMarkToMarket': 1}` | `{'base': {'trades': 52, 'win_rate_pct': 36.54, 'avg_pnl_pct': -0.0474, 'median_pnl_pct': -0.1961, 'exit_reasons': {'InitialSL': 33, 'TrailingSL': 19}}, 'strong': {'trades': 793, 'win_rate_pct': 36.82, 'avg_pnl_pct': -0.0056, 'median_pnl_pct': -0.1876, 'exit_reasons': {'InitialSL': 500, 'TrailingSL': 292, 'FinalMarkToMarket': 1}}}` | 940 | 95 | 2 |

## 文件

- Summary JSON：`research/eth_2026_jan_apr_1h_original_t2_micro_oneshot_summary.json`
- `oneshot_s10b4` ledger：`research/tmp_eth_2026_jan_apr_1h_original_t2_micro_oneshot_1h_oneshot_s10b4_ledger.csv`
