# Probabilistic V6 No-Trade Gate Analyzer

范围：仅限 `research`。本报告只消费已有 `symbol_rows.csv`，扫描二级 no-trade gate；它不是新的实盘策略结论。

## Baseline Candidates

| Source | Month | Symbol | TopK | Model | Trades | Realistic | Val Edge | Val TopK Return | Val TopK SL | Val TopK DD | Val Return/DD | Test Edge |
|---|---|---|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `walkforward_delay60_original_t2_feature60_valbest` | `2025-11` | `BTCUSDT` | 15 | `gradient_boosting` | 6 | 0.4103% | 0.089323 | 0.784172% | 0.1818 | -0.563612% | 1.3913 | 0.104448 |
| `walkforward_delay60_original_t2_feature60_valbest` | `2025-12` | `BTCUSDT` | 20 | `logistic` | 11 | -0.7900% | 0.106156 | 1.494166% | 0.2500 | -0.615120% | 2.4291 | -0.005785 |
| `walkforward_delay60_original_t2_feature60_valbest` | `2026-01` | `BTCUSDT` | 20 | `extra_trees` | 16 | 0.3375% | 0.123106 | 1.783297% | 0.2000 | -0.746622% | 2.3885 | 0.126375 |
| `walkforward_delay60_original_t2_feature60_valbest` | `2026-01` | `ETHUSDT` | 20 | `logistic` | 9 | -0.2238% | 0.131220 | 1.780046% | 0.2941 | -2.621126% | 0.6791 | 0.232334 |
| `walkforward_delay60_original_t2_feature60_valbest` | `2026-02` | `BTCUSDT` | 10 | `random_forest` | 10 | 3.0090% | 0.064319 | 1.049322% | 0.2000 | -0.463344% | 2.2647 | 0.146692 |
| `walkforward_delay60_original_t2_feature60_valbest` | `2026-02` | `ETHUSDT` | 10 | `gradient_boosting` | 3 | -0.8980% | 0.042089 | 0.462381% | 0.3333 | -0.598942% | 0.7720 | 0.077025 |
| `walkforward_delay60_original_t2_feature60_valbest` | `2026-03` | `BTCUSDT` | 15 | `random_forest` | 6 | -1.3738% | 0.583358 | 7.040773% | 0.0909 | -0.172769% | 28.1631 | -0.179209 |
| `walkforward_delay60_original_t2_feature60_valbest` | `2026-03` | `ETHUSDT` | 10 | `logistic` | 8 | 3.1271% | 0.592445 | 5.142537% | 0.2222 | -0.576902% | 8.9141 | 0.372527 |

## Top Gate Sweeps

| Rank | Policy | Active | Trades | Total Realistic | Worst Month | Gate |
|---:|---|---:|---:|---:|---:|---|
| 1 | `all_sleeves` | 6 | 57 | 4.7201% | -0.7900% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=1.0 |
| 2 | `all_sleeves` | 6 | 57 | 4.7201% | -0.7900% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=2.0%, ret/DD>=1.0 |
| 3 | `all_sleeves` | 6 | 57 | 4.7201% | -0.7900% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=1.5%, ret/DD>=1.0 |
| 4 | `all_sleeves` | 6 | 57 | 4.7201% | -0.7900% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=1.0%, ret/DD>=1.0 |
| 5 | `all_sleeves` | 6 | 57 | 4.7201% | -0.7900% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=0.75%, ret/DD>=1.0 |
| 6 | `all_sleeves` | 6 | 57 | 4.7201% | -0.7900% | edge>=-999.0, ret>=-999.0%, SL<=0.4, DD<=100.0%, ret/DD>=1.0 |
| 7 | `all_sleeves` | 6 | 57 | 4.7201% | -0.7900% | edge>=-999.0, ret>=-999.0%, SL<=0.4, DD<=2.0%, ret/DD>=1.0 |
| 8 | `all_sleeves` | 6 | 57 | 4.7201% | -0.7900% | edge>=-999.0, ret>=-999.0%, SL<=0.4, DD<=1.5%, ret/DD>=1.0 |
| 9 | `all_sleeves` | 6 | 57 | 4.7201% | -0.7900% | edge>=-999.0, ret>=-999.0%, SL<=0.4, DD<=1.0%, ret/DD>=1.0 |
| 10 | `all_sleeves` | 6 | 57 | 4.7201% | -0.7900% | edge>=-999.0, ret>=-999.0%, SL<=0.4, DD<=0.75%, ret/DD>=1.0 |
| 11 | `all_sleeves` | 6 | 57 | 4.7201% | -0.7900% | edge>=-999.0, ret>=-999.0%, SL<=0.35, DD<=100.0%, ret/DD>=1.0 |
| 12 | `all_sleeves` | 6 | 57 | 4.7201% | -0.7900% | edge>=-999.0, ret>=-999.0%, SL<=0.35, DD<=2.0%, ret/DD>=1.0 |
| 13 | `all_sleeves` | 6 | 57 | 4.7201% | -0.7900% | edge>=-999.0, ret>=-999.0%, SL<=0.35, DD<=1.5%, ret/DD>=1.0 |
| 14 | `all_sleeves` | 6 | 57 | 4.7201% | -0.7900% | edge>=-999.0, ret>=-999.0%, SL<=0.35, DD<=1.0%, ret/DD>=1.0 |
| 15 | `all_sleeves` | 6 | 57 | 4.7201% | -0.7900% | edge>=-999.0, ret>=-999.0%, SL<=0.35, DD<=0.75%, ret/DD>=1.0 |
| 16 | `all_sleeves` | 6 | 57 | 4.7201% | -0.7900% | edge>=-999.0, ret>=-999.0%, SL<=0.3, DD<=100.0%, ret/DD>=1.0 |
| 17 | `all_sleeves` | 6 | 57 | 4.7201% | -0.7900% | edge>=-999.0, ret>=-999.0%, SL<=0.3, DD<=2.0%, ret/DD>=-999.0 |
| 18 | `all_sleeves` | 6 | 57 | 4.7201% | -0.7900% | edge>=-999.0, ret>=-999.0%, SL<=0.3, DD<=2.0%, ret/DD>=0.0 |
| 19 | `all_sleeves` | 6 | 57 | 4.7201% | -0.7900% | edge>=-999.0, ret>=-999.0%, SL<=0.3, DD<=2.0%, ret/DD>=1.0 |
| 20 | `all_sleeves` | 6 | 57 | 4.7201% | -0.7900% | edge>=-999.0, ret>=-999.0%, SL<=0.3, DD<=1.5%, ret/DD>=-999.0 |

## Best Non-Empty Selection

- policy=`all_sleeves`，active_rows=6，trades=57，total_realistic=4.7201%，worst_month=-0.7900%。

| Source | Month | Symbol | TopK | Trades | Realistic | Val Return/DD |
|---|---|---|---:|---:|---:|---:|
| `walkforward_delay60_original_t2_feature60_valbest` | `2025-11` | `BTCUSDT` | 15 | 6 | 0.4103% | 1.3913 |
| `walkforward_delay60_original_t2_feature60_valbest` | `2025-12` | `BTCUSDT` | 20 | 11 | -0.7900% | 2.4291 |
| `walkforward_delay60_original_t2_feature60_valbest` | `2026-01` | `BTCUSDT` | 20 | 16 | 0.3375% | 2.3885 |
| `walkforward_delay60_original_t2_feature60_valbest` | `2026-02` | `BTCUSDT` | 10 | 10 | 3.0090% | 2.2647 |
| `walkforward_delay60_original_t2_feature60_valbest` | `2026-03` | `BTCUSDT` | 15 | 6 | -1.3738% | 28.1631 |
| `walkforward_delay60_original_t2_feature60_valbest` | `2026-03` | `ETHUSDT` | 10 | 8 | 3.1271% | 8.9141 |

## Interpretation

- 若最佳结果来自 `active_rows=0`，说明当前验证月字段只能选择空仓，不能证明概率模型已经具备可交易收益。
- 若非空最佳只保留单个样本，也只能作为下一轮特征/事件来源假设，不能作为实盘候选。
- Gate 只使用验证月字段；`test_edge` 只用于复盘解释，不参与筛选。
