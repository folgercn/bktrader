# Probabilistic V6 No-Trade Gate Analyzer

范围：仅限 `research`。本报告只消费已有 `symbol_rows.csv`，扫描二级 no-trade gate；它不是新的实盘策略结论。

## Objective Diagnostics

- 目标收益：`10.00%` Active_Silo_Sum。
- 组合约束：active_rows>=4，active_months>=6，trades>=40，worst_month>=-2.00%。
- Baseline candidate pool：active_rows=12，trades=105，total=-0.2361%，silo_PF=0.9736。
- Oracle best positive per symbol-month：total=8.7110%，active_rows=4。这是事后上限诊断，不可当作可交易选择器。
- Target hit under validation-only gates：`False`。

## Baseline Candidates

| Source | Month | Symbol | TopK | Model | Trades | Realistic | Val Edge | Val TopK Return | Val TopK SL | Val TopK DD | Val Return/DD | Val Markov | Test Edge |
|---|---|---|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2025-07` | `ETHUSDT` | 5 | `svm_rbf` | 5 | -1.0587% | 0.081132 | 2.232426% | 0.2000 | -0.300162% | 7.4374 | 0.4395 | -0.094544 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2025-08` | `BTCUSDT` | 10 | `gradient_boosting` | 10 | -2.9684% | 0.069479 | 2.254096% | 0.2000 | -0.447845% | 5.0332 | 0.5461 | -0.004287 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2025-10` | `BTCUSDT` | 10 | `logistic` | 8 | -0.8476% | 0.044263 | 0.710537% | 0.2000 | -0.600911% | 1.1824 | 0.4808 | 0.049168 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2025-12` | `BTCUSDT` | 5 | `extra_trees` | 5 | -0.5354% | 0.048671 | 1.555662% | 0.0000 | 0.000000% | 6.2226 | 0.7421 | 0.057927 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2025-12` | `ETHUSDT` | 20 | `logistic` | 18 | -0.5625% | 0.173354 | 3.424843% | 0.3000 | -2.089412% | 1.6391 | 0.5843 | 0.049271 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2026-01` | `BTCUSDT` | 15 | `random_forest` | 7 | -0.1373% | 0.159479 | 1.375587% | 0.1818 | -0.450932% | 3.0505 | 0.5193 | 0.092492 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2026-01` | `ETHUSDT` | 10 | `logistic` | 5 | 0.7800% | 0.096916 | 2.072337% | 0.3000 | -0.950131% | 2.1811 | 0.5794 | 0.134196 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2026-02` | `BTCUSDT` | 20 | `gradient_boosting` | 16 | 2.3050% | 0.112803 | 1.549543% | 0.2632 | -0.813557% | 1.9047 | 0.3912 | 0.024061 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2026-02` | `ETHUSDT` | 10 | `gradient_boosting` | 7 | 2.5318% | 0.107894 | 1.445494% | 0.3333 | -0.174598% | 5.7820 | 0.5470 | 0.131039 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2026-03` | `BTCUSDT` | 10 | `extra_trees` | 10 | -1.6595% | 0.400864 | 5.768322% | 0.1000 | -0.157643% | 23.0733 | 0.7674 | -0.055626 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2026-03` | `ETHUSDT` | 5 | `logistic` | 5 | 3.0942% | 0.371458 | 3.628357% | 0.2000 | -0.679087% | 5.3430 | 0.5827 | 0.093466 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2026-04` | `ETHUSDT` | 10 | `logistic` | 9 | -1.1777% | 0.578788 | 4.194972% | 0.0000 | 0.000000% | 16.7799 | 0.3693 | -0.087715 |

## Top Gate Sweeps

| Rank | Policy | Active | Months | Trades | Total Realistic | Silo PF | Worst Month | Target | Gate |
|---:|---|---:|---:|---:|---:|---:|---:|---|---|
| 1 | `best_validation_per_symbol_month` | 5 | 3 | 40 | 8.5737% | 63.4450 | 0.6427% | `False` | edge>=0.08, ret>=1.0%, SL<=1.0, DD<=2.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=7.0%, markov>=0.2 |
| 2 | `best_validation_per_symbol_month` | 5 | 3 | 40 | 8.5737% | 63.4450 | 0.6427% | `False` | edge>=0.08, ret>=1.0%, SL<=1.0, DD<=2.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=7.0%, markov>=0.3 |
| 3 | `best_validation_per_symbol_month` | 5 | 3 | 40 | 8.5737% | 63.4450 | 0.6427% | `False` | edge>=0.08, ret>=1.0%, SL<=1.0, DD<=2.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=999999.0%, markov>=0.2 |
| 4 | `best_validation_per_symbol_month` | 5 | 3 | 40 | 8.5737% | 63.4450 | 0.6427% | `False` | edge>=0.08, ret>=1.0%, SL<=1.0, DD<=2.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=10.0%, markov>=0.3 |
| 5 | `best_validation_per_symbol_month` | 5 | 3 | 40 | 8.5737% | 63.4450 | 0.6427% | `False` | edge>=0.08, ret>=1.0%, SL<=1.0, DD<=2.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=10.0%, markov>=0.0 |
| 6 | `best_validation_per_symbol_month` | 5 | 3 | 40 | 8.5737% | 63.4450 | 0.6427% | `False` | edge>=0.08, ret>=1.0%, SL<=1.0, DD<=2.0%, ret/DD>=1.5, ret/DD<=7.0, ret<=7.0%, markov>=0.0 |
| 7 | `best_validation_per_symbol_month` | 5 | 3 | 40 | 8.5737% | 63.4450 | 0.6427% | `False` | edge>=0.08, ret>=1.0%, SL<=1.0, DD<=2.0%, ret/DD>=1.5, ret/DD<=7.0, ret<=5.0%, markov>=0.2 |
| 8 | `best_validation_per_symbol_month` | 5 | 3 | 40 | 8.5737% | 63.4450 | 0.6427% | `False` | edge>=0.08, ret>=1.0%, SL<=1.0, DD<=2.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=999999.0%, markov>=0.0 |
| 9 | `best_validation_per_symbol_month` | 5 | 3 | 40 | 8.5737% | 63.4450 | 0.6427% | `False` | edge>=0.08, ret>=1.0%, SL<=1.0, DD<=2.0%, ret/DD>=1.5, ret/DD<=7.0, ret<=5.0%, markov>=0.0 |
| 10 | `best_validation_per_symbol_month` | 5 | 3 | 40 | 8.5737% | 63.4450 | 0.6427% | `False` | edge>=0.08, ret>=1.0%, SL<=1.0, DD<=2.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=999999.0%, markov>=0.3 |
| 11 | `best_validation_per_symbol_month` | 5 | 3 | 40 | 8.5737% | 63.4450 | 0.6427% | `False` | edge>=0.08, ret>=1.0%, SL<=1.0, DD<=2.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=10.0%, markov>=0.2 |
| 12 | `best_validation_per_symbol_month` | 5 | 3 | 40 | 8.5737% | 63.4450 | 0.6427% | `False` | edge>=0.08, ret>=1.0%, SL<=1.0, DD<=2.0%, ret/DD>=1.5, ret/DD<=7.0, ret<=10.0%, markov>=0.3 |
| 13 | `best_validation_per_symbol_month` | 5 | 3 | 40 | 8.5737% | 63.4450 | 0.6427% | `False` | edge>=0.08, ret>=1.0%, SL<=1.0, DD<=2.0%, ret/DD>=1.5, ret/DD<=7.0, ret<=7.0%, markov>=0.3 |
| 14 | `best_validation_per_symbol_month` | 5 | 3 | 40 | 8.5737% | 63.4450 | 0.6427% | `False` | edge>=0.08, ret>=1.0%, SL<=1.0, DD<=2.0%, ret/DD>=1.5, ret/DD<=7.0, ret<=999999.0%, markov>=0.3 |
| 15 | `best_validation_per_symbol_month` | 5 | 3 | 40 | 8.5737% | 63.4450 | 0.6427% | `False` | edge>=0.08, ret>=1.0%, SL<=1.0, DD<=2.0%, ret/DD>=1.5, ret/DD<=7.0, ret<=999999.0%, markov>=0.0 |
| 16 | `best_validation_per_symbol_month` | 5 | 3 | 40 | 8.5737% | 63.4450 | 0.6427% | `False` | edge>=0.08, ret>=1.0%, SL<=1.0, DD<=2.0%, ret/DD>=1.5, ret/DD<=7.0, ret<=5.0%, markov>=0.3 |
| 17 | `best_validation_per_symbol_month` | 5 | 3 | 40 | 8.5737% | 63.4450 | 0.6427% | `False` | edge>=0.08, ret>=1.0%, SL<=1.0, DD<=2.0%, ret/DD>=1.5, ret/DD<=7.0, ret<=7.0%, markov>=0.2 |
| 18 | `best_validation_per_symbol_month` | 5 | 3 | 40 | 8.5737% | 63.4450 | 0.6427% | `False` | edge>=0.08, ret>=1.0%, SL<=1.0, DD<=2.0%, ret/DD>=1.5, ret/DD<=7.0, ret<=999999.0%, markov>=0.2 |
| 19 | `best_validation_per_symbol_month` | 5 | 3 | 40 | 8.5737% | 63.4450 | 0.6427% | `False` | edge>=0.08, ret>=1.0%, SL<=1.0, DD<=2.0%, ret/DD>=1.5, ret/DD<=7.0, ret<=10.0%, markov>=0.2 |
| 20 | `best_validation_per_symbol_month` | 5 | 3 | 40 | 8.5737% | 63.4450 | 0.6427% | `False` | edge>=0.08, ret>=1.0%, SL<=1.0, DD<=2.0%, ret/DD>=1.5, ret/DD<=7.0, ret<=10.0%, markov>=0.0 |

## Best Non-Empty Selection

- policy=`all_sleeves`，active_rows=5，trades=40，total_realistic=8.5737%，worst_month=0.6427%。

| Source | Month | Symbol | TopK | Trades | Realistic | Val Return/DD | Val Markov |
|---|---|---|---:|---:|---:|---:|---:|
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2026-01` | `BTCUSDT` | 15 | 7 | -0.1373% | 3.0505 | 0.5193 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2026-01` | `ETHUSDT` | 10 | 5 | 0.7800% | 2.1811 | 0.5794 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2026-02` | `BTCUSDT` | 20 | 16 | 2.3050% | 1.9047 | 0.3912 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2026-02` | `ETHUSDT` | 10 | 7 | 2.5318% | 5.7820 | 0.5470 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2026-03` | `ETHUSDT` | 5 | 5 | 3.0942% | 5.3430 | 0.5827 |

## Best Qualified Selection

- policy=`all_sleeves`，active_rows=8，active_months=6，trades=72，total_realistic=5.7748%，silo_PF=2.9668，target_hit=`False`。

| Source | Month | Symbol | TopK | Trades | Realistic | Val Return/DD | Val Markov |
|---|---|---|---:|---:|---:|---:|---:|
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2025-07` | `ETHUSDT` | 5 | 5 | -1.0587% | 7.4374 | 0.4395 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2025-12` | `ETHUSDT` | 20 | 18 | -0.5625% | 1.6391 | 0.5843 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2026-01` | `BTCUSDT` | 15 | 7 | -0.1373% | 3.0505 | 0.5193 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2026-01` | `ETHUSDT` | 10 | 5 | 0.7800% | 2.1811 | 0.5794 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2026-02` | `BTCUSDT` | 20 | 16 | 2.3050% | 1.9047 | 0.3912 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2026-02` | `ETHUSDT` | 10 | 7 | 2.5318% | 5.7820 | 0.5470 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2026-03` | `ETHUSDT` | 5 | 5 | 3.0942% | 5.3430 | 0.5827 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback` | `2026-04` | `ETHUSDT` | 10 | 9 | -1.1777% | 16.7799 | 0.3693 |

## Interpretation

- 若最佳结果来自 `active_rows=0`，说明当前验证月字段只能选择空仓，不能证明概率模型已经具备可交易收益。
- 若非空最佳只保留单个样本，也只能作为下一轮特征/事件来源假设，不能作为实盘候选。
- Gate 只使用验证月字段；`test_edge` 只用于复盘解释，不参与筛选。
