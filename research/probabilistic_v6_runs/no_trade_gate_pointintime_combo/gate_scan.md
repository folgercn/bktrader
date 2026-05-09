# Probabilistic V6 No-Trade Gate Analyzer

范围：仅限 `research`。本报告只消费已有 `symbol_rows.csv`，扫描二级 no-trade gate；它不是新的实盘策略结论。

## Baseline Candidates

| Source | Month | Symbol | TopK | Model | Trades | Realistic | Val Edge | Val TopK Return | Val TopK SL | Val TopK DD | Val Return/DD | Test Edge |
|---|---|---|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `walkforward_2025_q3_v7_pointintime_valbest_probev` | `2025-09` | `ETHUSDT` | 15 | `gradient_boosting` | 15 | -1.9905% | 0.342215 | 4.837315% | 0.1538 | -0.996246% | 4.8555 | -0.138751 |
| `walkforward_2025_q4_v7_pointintime_valbest_probev` | `2025-12` | `BTCUSDT` | 15 | `random_forest` | 11 | -3.7057% | 0.073672 | 1.008339% | 0.2857 | -0.542292% | 1.8594 | -0.219148 |
| `walkforward_2026_q1_v7_pointintime_valbest_probev` | `2026-03` | `BTCUSDT` | 20 | `random_forest` | 20 | -1.4667% | 0.281615 | 6.132250% | 0.2500 | -1.465936% | 4.1832 | -0.001094 |
| `walkforward_2026_q1_v7_pointintime_valbest_probev` | `2026-03` | `ETHUSDT` | 15 | `logistic` | 15 | 1.4629% | 0.179544 | 4.946953% | 0.2000 | -1.573130% | 3.1447 | 0.037474 |
| `walkforward_2025_q3_baseline_plus_t3_v7_prevatr_valbest_probev` | `2025-09` | `ETHUSDT` | 15 | `gradient_boosting` | 15 | -3.9517% | 0.291893 | 2.950184% | 0.1818 | -1.365932% | 2.1598 | -0.027780 |

## Top Gate Sweeps

| Rank | Policy | Active | Trades | Total Realistic | Worst Month | Gate |
|---:|---|---:|---:|---:|---:|---|
| 1 | `all_sleeves` | 0 | 0 | 0.0000% | 0.0000% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=5.0 |
| 2 | `all_sleeves` | 0 | 0 | 0.0000% | 0.0000% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=2.0%, ret/DD>=5.0 |
| 3 | `all_sleeves` | 0 | 0 | 0.0000% | 0.0000% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=1.5%, ret/DD>=5.0 |
| 4 | `all_sleeves` | 0 | 0 | 0.0000% | 0.0000% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=1.0%, ret/DD>=5.0 |
| 5 | `all_sleeves` | 0 | 0 | 0.0000% | 0.0000% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=0.75%, ret/DD>=2.0 |
| 6 | `all_sleeves` | 0 | 0 | 0.0000% | 0.0000% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=0.75%, ret/DD>=3.0 |
| 7 | `all_sleeves` | 0 | 0 | 0.0000% | 0.0000% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=0.75%, ret/DD>=4.0 |
| 8 | `all_sleeves` | 0 | 0 | 0.0000% | 0.0000% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=0.75%, ret/DD>=5.0 |
| 9 | `all_sleeves` | 0 | 0 | 0.0000% | 0.0000% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=0.5%, ret/DD>=-999.0 |
| 10 | `all_sleeves` | 0 | 0 | 0.0000% | 0.0000% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=0.5%, ret/DD>=0.0 |
| 11 | `all_sleeves` | 0 | 0 | 0.0000% | 0.0000% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=0.5%, ret/DD>=1.0 |
| 12 | `all_sleeves` | 0 | 0 | 0.0000% | 0.0000% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=0.5%, ret/DD>=2.0 |
| 13 | `all_sleeves` | 0 | 0 | 0.0000% | 0.0000% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=0.5%, ret/DD>=3.0 |
| 14 | `all_sleeves` | 0 | 0 | 0.0000% | 0.0000% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=0.5%, ret/DD>=4.0 |
| 15 | `all_sleeves` | 0 | 0 | 0.0000% | 0.0000% | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=0.5%, ret/DD>=5.0 |
| 16 | `all_sleeves` | 0 | 0 | 0.0000% | 0.0000% | edge>=-999.0, ret>=-999.0%, SL<=0.35, DD<=100.0%, ret/DD>=5.0 |
| 17 | `all_sleeves` | 0 | 0 | 0.0000% | 0.0000% | edge>=-999.0, ret>=-999.0%, SL<=0.35, DD<=2.0%, ret/DD>=5.0 |
| 18 | `all_sleeves` | 0 | 0 | 0.0000% | 0.0000% | edge>=-999.0, ret>=-999.0%, SL<=0.35, DD<=1.5%, ret/DD>=5.0 |
| 19 | `all_sleeves` | 0 | 0 | 0.0000% | 0.0000% | edge>=-999.0, ret>=-999.0%, SL<=0.35, DD<=1.0%, ret/DD>=5.0 |
| 20 | `all_sleeves` | 0 | 0 | 0.0000% | 0.0000% | edge>=-999.0, ret>=-999.0%, SL<=0.35, DD<=0.75%, ret/DD>=2.0 |

## Best Non-Empty Selection

- policy=`all_sleeves`，active_rows=2，trades=30，total_realistic=-0.5276%，worst_month=-1.9905%。

| Source | Month | Symbol | TopK | Trades | Realistic | Val Return/DD |
|---|---|---|---:|---:|---:|---:|
| `walkforward_2025_q3_v7_pointintime_valbest_probev` | `2025-09` | `ETHUSDT` | 15 | 15 | -1.9905% | 4.8555 |
| `walkforward_2026_q1_v7_pointintime_valbest_probev` | `2026-03` | `ETHUSDT` | 15 | 15 | 1.4629% | 3.1447 |

## Interpretation

- 若最佳结果来自 `active_rows=0`，说明当前验证月字段只能选择空仓，不能证明概率模型已经具备可交易收益。
- 若非空最佳只保留单个样本，也只能作为下一轮特征/事件来源假设，不能作为实盘候选。
- Gate 只使用验证月字段；`test_edge` 只用于复盘解释，不参与筛选。
