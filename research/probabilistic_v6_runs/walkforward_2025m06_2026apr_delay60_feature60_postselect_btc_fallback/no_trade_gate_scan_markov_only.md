# Probabilistic V6 No-Trade Gate Analyzer

范围：仅限 `research`。本报告只消费已有 `symbol_rows.csv`，扫描二级 no-trade gate；它不是新的实盘策略结论。

## Baseline Candidates

| Source | Month | Symbol | TopK | Model | Trades | Realistic | Val Edge | Val TopK Return | Val TopK SL | Val TopK DD | Val Return/DD | Val Markov | Test Edge |
|---|---|---|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2025-07` | `BTCUSDT` | 20 | `gradient_boosting` | 18 | -0.7250% | 0.114524 | 1.921236% | 0.1765 | -0.479561% | 4.0062 | 0.5651 | 0.006547 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2025-07` | `ETHUSDT` | 5 | `svm_rbf` | 5 | -2.1443% | 0.035094 | 1.005023% | 0.4000 | -0.736598% | 1.3644 | 0.4219 | -0.063076 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2025-08` | `BTCUSDT` | 5 | `gradient_boosting` | 5 | -1.3744% | 0.121069 | 1.368441% | 0.2000 | 0.000000% | 5.4738 | 0.5199 | -0.061392 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2025-12` | `BTCUSDT` | 15 | `logistic` | 9 | -0.4182% | 0.050543 | 0.833596% | 0.3636 | -0.567744% | 1.4683 | 0.3578 | 0.065025 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2025-12` | `ETHUSDT` | 20 | `logistic` | 18 | -0.1849% | 0.126700 | 2.349890% | 0.3684 | -1.994697% | 1.1781 | 0.6208 | 0.101683 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2026-01` | `BTCUSDT` | 10 | `gradient_boosting` | 7 | 0.2418% | 0.185802 | 1.604042% | 0.2000 | -0.538990% | 2.9760 | 0.4001 | 0.095107 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2026-02` | `BTCUSDT` | 10 | `logistic` | 10 | 2.3794% | 0.118453 | 1.547843% | 0.2000 | -0.511873% | 3.0239 | 0.4105 | 0.038176 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2026-02` | `ETHUSDT` | 5 | `logistic` | 5 | 3.3305% | 0.126830 | 1.038121% | 0.2000 | 0.000000% | 4.1525 | 0.4280 | 0.043855 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2026-03` | `BTCUSDT` | 10 | `extra_trees` | 6 | -0.4122% | 0.574219 | 5.392295% | 0.1000 | -0.135635% | 21.5692 | 0.7049 | -0.037735 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2026-03` | `ETHUSDT` | 5 | `logistic` | 5 | 2.8842% | 0.374316 | 3.669592% | 0.2000 | -0.680019% | 5.3963 | 0.7158 | 0.153834 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2026-04` | `ETHUSDT` | 5 | `logistic` | 5 | -1.2168% | 0.441266 | 2.803271% | 0.0000 | 0.000000% | 11.2131 | 0.4421 | -0.087715 |

## Top Gate Sweeps

| Rank | Policy | Active | Trades | Total Realistic | Worst Month | Gate |
|---:|---|---:|---:|---:|---:|---|
| 1 | `all_sleeves` | 10 | 84 | 2.7783% | -2.8693% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=999.0, ret<=999.0%, markov>=0.4 |
| 2 | `all_sleeves` | 2 | 11 | 2.4720% | 2.4720% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=999.0, ret<=999.0%, markov>=0.7 |
| 3 | `all_sleeves` | 11 | 93 | 2.3601% | -2.8693% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=999.0, ret<=999.0%, markov>=-999.0 |
| 4 | `all_sleeves` | 11 | 93 | 2.3601% | -2.8693% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=999.0, ret<=999.0%, markov>=0.2 |
| 5 | `all_sleeves` | 11 | 93 | 2.3601% | -2.8693% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=999.0, ret<=999.0%, markov>=0.3 |
| 6 | `all_sleeves` | 3 | 29 | 2.2871% | -0.1849% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=999.0, ret<=999.0%, markov>=0.6 |
| 7 | `all_sleeves` | 5 | 52 | 0.1877% | -1.3744% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=999.0, ret<=999.0%, markov>=0.5 |

## Best Non-Empty Selection

- policy=`all_sleeves`，active_rows=10，trades=84，total_realistic=2.7783%，worst_month=-2.8693%。

| Source | Month | Symbol | TopK | Trades | Realistic | Val Return/DD | Val Markov |
|---|---|---|---:|---:|---:|---:|---:|
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2025-07` | `BTCUSDT` | 20 | 18 | -0.7250% | 4.0062 | 0.5651 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2025-07` | `ETHUSDT` | 5 | 5 | -2.1443% | 1.3644 | 0.4219 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2025-08` | `BTCUSDT` | 5 | 5 | -1.3744% | 5.4738 | 0.5199 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2025-12` | `ETHUSDT` | 20 | 18 | -0.1849% | 1.1781 | 0.6208 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2026-01` | `BTCUSDT` | 10 | 7 | 0.2418% | 2.9760 | 0.4001 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2026-02` | `BTCUSDT` | 10 | 10 | 2.3794% | 3.0239 | 0.4105 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2026-02` | `ETHUSDT` | 5 | 5 | 3.3305% | 4.1525 | 0.4280 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2026-03` | `BTCUSDT` | 10 | 6 | -0.4122% | 21.5692 | 0.7049 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2026-03` | `ETHUSDT` | 5 | 5 | 2.8842% | 5.3963 | 0.7158 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2026-04` | `ETHUSDT` | 5 | 5 | -1.2168% | 11.2131 | 0.4421 |

## Interpretation

- 若最佳结果来自 `active_rows=0`，说明当前验证月字段只能选择空仓，不能证明概率模型已经具备可交易收益。
- 若非空最佳只保留单个样本，也只能作为下一轮特征/事件来源假设，不能作为实盘候选。
- Gate 只使用验证月字段；`test_edge` 只用于复盘解释，不参与筛选。
