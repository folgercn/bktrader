# ETH original_t2 + micro strength 1s 回测（2026-01-01T00:00:00+00:00 至 2026-04-30T23:59:59+00:00）

范围：仅限 `research`。该回测使用 `original_t2` 三根 bar 结构：long level 为 `prev_high_2`，short level 为 `prev_low_2`，当前 signal bar 未闭合，由 `1s high/low` 触发。开仓前额外计算最近 `1s` micro strength，weak 跳过，base/strong 按 slot0/slot1 仓位入场。

成本：滑点 `2bps/side`，手续费 maker entry `2bps` + market exit `4bps`，realistic 包含两者。

| Timeframe | Variant | 笔数 | Realistic | Raw | 2bps Slip No Fee | Fees | 胜率 | Max DD | Avg Hold | Median Hold | Exit Reasons | Quality | Candidate | Weak Skip | Max Entries/Bar |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|
| `1h` | `strong_only` | 555 | -6.0883% | 3.9848% | -0.1670% | 5.9648% | 38.38% | -3.19% | 1051.30s | 445.00s | `{'InitialSL': 342, 'TrailingSL': 213}` | `{'strong': {'trades': 555, 'win_rate_pct': 38.38, 'avg_pnl_pct': -0.0035, 'median_pnl_pct': -0.1899, 'exit_reasons': {'InitialSL': 342, 'TrailingSL': 213}}}` | 4110 | 3555 | 2 |

## 文件

- Summary JSON：`research/eth_2026_jan_apr_1h_original_t2_micro_strong_only_summary.json`
- `strong_only` ledger：`research/tmp_eth_2026_jan_apr_1h_original_t2_micro_strong_only_1h_strong_only_ledger.csv`
