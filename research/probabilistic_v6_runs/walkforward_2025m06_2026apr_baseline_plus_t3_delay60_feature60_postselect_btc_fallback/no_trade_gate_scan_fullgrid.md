# Probabilistic V6 No-Trade Gate Analyzer

范围：仅限 `research`。本报告只消费已有 `symbol_rows.csv`，扫描二级 no-trade gate；它不是新的实盘策略结论。

## Objective Diagnostics

- 目标收益：`10.00%` Active_Silo_Sum。
- 组合约束：active_rows>=4，active_months>=6，trades>=40，worst_month>=-2.00%。
- Baseline candidate pool：active_rows=11，trades=100，total=-7.5453%，silo_PF=0.3604。
- Oracle best positive per symbol-month：total=4.2515%，active_rows=4。这是事后上限诊断，不可当作可交易选择器。
- Target hit under validation-only gates：`False`。

## Baseline Candidates

| Source | Month | Symbol | TopK | Model | Trades | Realistic | Val Edge | Val TopK Return | Val TopK SL | Val TopK DD | Val Return/DD | Val Markov | Test Edge |
|---|---|---|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_postselect_btc_fallback` | `2025-07` | `BTCUSDT` | 20 | `gradient_boosting` | 20 | -0.6461% | 0.091476 | 1.164035% | 0.3000 | -0.757492% | 1.5367 | 0.5833 | -0.003921 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_postselect_btc_fallback` | `2025-07` | `ETHUSDT` | 5 | `random_forest` | 5 | -2.6500% | 0.301698 | 2.006413% | 0.2000 | -0.405447% | 4.9486 | 0.7010 | -0.110088 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_postselect_btc_fallback` | `2025-08` | `BTCUSDT` | 5 | `gradient_boosting` | 5 | -0.6346% | 0.054666 | 1.287392% | 0.2000 | 0.000000% | 5.1496 | 0.7247 | -0.047965 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_postselect_btc_fallback` | `2025-11` | `BTCUSDT` | 5 | `gradient_boosting` | 5 | -1.2030% | 0.092051 | 0.787695% | 0.2000 | -0.102467% | 3.1508 | 0.5956 | -0.043889 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_postselect_btc_fallback` | `2025-12` | `ETHUSDT` | 20 | `logistic` | 19 | 0.7873% | 0.124987 | 1.674487% | 0.3500 | -1.545128% | 1.0837 | 0.5318 | -0.015316 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_postselect_btc_fallback` | `2026-01` | `ETHUSDT` | 5 | `logistic` | 3 | -2.0568% | 0.127419 | 1.965645% | 0.2000 | -0.447648% | 4.3911 | 0.4324 | 0.030222 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_postselect_btc_fallback` | `2026-02` | `BTCUSDT` | 10 | `logistic` | 10 | 1.0572% | 0.097803 | 1.908369% | 0.1000 | -0.206396% | 7.6335 | 0.3893 | 0.019730 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_postselect_btc_fallback` | `2026-02` | `ETHUSDT` | 5 | `gradient_boosting` | 5 | 1.7451% | 0.030675 | 1.373962% | 0.2000 | 0.000000% | 5.4958 | 0.6715 | 0.031729 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_postselect_btc_fallback` | `2026-03` | `BTCUSDT` | 10 | `gradient_boosting` | 10 | -1.8927% | 0.555678 | 6.508307% | 0.0000 | 0.000000% | 26.0332 | 0.6178 | -0.034806 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_postselect_btc_fallback` | `2026-03` | `ETHUSDT` | 10 | `logistic` | 8 | 0.6619% | 0.455029 | 4.460120% | 0.3000 | -0.958010% | 4.6556 | 0.4965 | -0.025028 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_postselect_btc_fallback` | `2026-04` | `ETHUSDT` | 10 | `gradient_boosting` | 10 | -2.7136% | 0.312602 | 2.301954% | 0.2000 | -0.847481% | 2.7162 | 0.4275 | -0.161792 |

## Top Gate Sweeps

| Rank | Policy | Active | Months | Trades | Total Realistic | Silo PF | Worst Month | Target | Gate |
|---:|---|---:|---:|---:|---:|---:|---:|---|---|
| 1 | `best_validation_per_symbol_month` | 4 | 4 | 49 | 1.2517% | 1.9774 | -0.6461% | `False` | edge>=0.02, ret>=1.0%, SL<=1.0, DD<=5.0%, ret/DD>=0.5, ret/DD<=7.0, ret<=2.0%, markov>=0.5 |
| 2 | `best_validation_per_symbol_month` | 4 | 4 | 49 | 1.2517% | 1.9774 | -0.6461% | `False` | edge>=0.02, ret>=1.0%, SL<=1.0, DD<=5.0%, ret/DD>=0.5, ret/DD<=999999.0, ret<=2.0%, markov>=0.5 |
| 3 | `best_validation_per_symbol_month` | 4 | 4 | 49 | 1.2517% | 1.9774 | -0.6461% | `False` | edge>=0.02, ret>=1.0%, SL<=1.0, DD<=5.0%, ret/DD>=0.5, ret/DD<=10.0, ret<=2.0%, markov>=0.5 |
| 4 | `best_validation_per_symbol_month` | 4 | 4 | 49 | 1.2517% | 1.9774 | -0.6461% | `False` | edge>=0.02, ret>=1.0%, SL<=1.0, DD<=5.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=2.0%, markov>=0.5 |
| 5 | `best_validation_per_symbol_month` | 4 | 4 | 49 | 1.2517% | 1.9774 | -0.6461% | `False` | edge>=0.02, ret>=1.0%, SL<=1.0, DD<=100.0%, ret/DD>=0.5, ret/DD<=7.0, ret<=2.0%, markov>=0.5 |
| 6 | `best_validation_per_symbol_month` | 4 | 4 | 49 | 1.2517% | 1.9774 | -0.6461% | `False` | edge>=0.02, ret>=1.0%, SL<=1.0, DD<=5.0%, ret/DD>=0.5, ret/DD<=20.0, ret<=2.0%, markov>=0.5 |
| 7 | `best_validation_per_symbol_month` | 4 | 4 | 49 | 1.2517% | 1.9774 | -0.6461% | `False` | edge>=0.02, ret>=1.0%, SL<=1.0, DD<=100.0%, ret/DD>=0.0, ret/DD<=7.0, ret<=2.0%, markov>=0.5 |
| 8 | `best_validation_per_symbol_month` | 4 | 4 | 49 | 1.2517% | 1.9774 | -0.6461% | `False` | edge>=0.02, ret>=1.0%, SL<=1.0, DD<=5.0%, ret/DD>=1.0, ret/DD<=10.0, ret<=2.0%, markov>=0.5 |
| 9 | `best_validation_per_symbol_month` | 4 | 4 | 49 | 1.2517% | 1.9774 | -0.6461% | `False` | edge>=0.02, ret>=1.0%, SL<=1.0, DD<=5.0%, ret/DD>=1.0, ret/DD<=20.0, ret<=2.0%, markov>=0.5 |
| 10 | `best_validation_per_symbol_month` | 4 | 4 | 49 | 1.2517% | 1.9774 | -0.6461% | `False` | edge>=0.02, ret>=1.0%, SL<=1.0, DD<=100.0%, ret/DD>=0.5, ret/DD<=10.0, ret<=2.0%, markov>=0.5 |
| 11 | `best_validation_per_symbol_month` | 4 | 4 | 49 | 1.2517% | 1.9774 | -0.6461% | `False` | edge>=0.02, ret>=1.0%, SL<=1.0, DD<=100.0%, ret/DD>=1.0, ret/DD<=999999.0, ret<=2.0%, markov>=0.5 |
| 12 | `best_validation_per_symbol_month` | 4 | 4 | 49 | 1.2517% | 1.9774 | -0.6461% | `False` | edge>=0.02, ret>=1.0%, SL<=1.0, DD<=100.0%, ret/DD>=0.0, ret/DD<=10.0, ret<=2.0%, markov>=0.5 |
| 13 | `best_validation_per_symbol_month` | 4 | 4 | 49 | 1.2517% | 1.9774 | -0.6461% | `False` | edge>=0.02, ret>=1.0%, SL<=1.0, DD<=5.0%, ret/DD>=1.0, ret/DD<=999999.0, ret<=2.0%, markov>=0.5 |
| 14 | `best_validation_per_symbol_month` | 4 | 4 | 49 | 1.2517% | 1.9774 | -0.6461% | `False` | edge>=0.02, ret>=1.0%, SL<=1.0, DD<=100.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=2.0%, markov>=0.5 |
| 15 | `best_validation_per_symbol_month` | 4 | 4 | 49 | 1.2517% | 1.9774 | -0.6461% | `False` | edge>=0.02, ret>=1.0%, SL<=1.0, DD<=100.0%, ret/DD>=1.0, ret/DD<=10.0, ret<=2.0%, markov>=0.5 |
| 16 | `best_validation_per_symbol_month` | 4 | 4 | 49 | 1.2517% | 1.9774 | -0.6461% | `False` | edge>=0.02, ret>=1.0%, SL<=1.0, DD<=100.0%, ret/DD>=1.0, ret/DD<=20.0, ret<=2.0%, markov>=0.5 |
| 17 | `best_validation_per_symbol_month` | 4 | 4 | 49 | 1.2517% | 1.9774 | -0.6461% | `False` | edge>=0.02, ret>=1.0%, SL<=1.0, DD<=100.0%, ret/DD>=0.0, ret/DD<=999999.0, ret<=2.0%, markov>=0.5 |
| 18 | `best_validation_per_symbol_month` | 4 | 4 | 49 | 1.2517% | 1.9774 | -0.6461% | `False` | edge>=0.02, ret>=1.0%, SL<=1.0, DD<=100.0%, ret/DD>=0.5, ret/DD<=999999.0, ret<=2.0%, markov>=0.5 |
| 19 | `best_validation_per_symbol_month` | 4 | 4 | 49 | 1.2517% | 1.9774 | -0.6461% | `False` | edge>=0.02, ret>=1.0%, SL<=1.0, DD<=100.0%, ret/DD>=0.0, ret/DD<=20.0, ret<=2.0%, markov>=0.5 |
| 20 | `best_validation_per_symbol_month` | 4 | 4 | 49 | 1.2517% | 1.9774 | -0.6461% | `False` | edge>=0.02, ret>=1.0%, SL<=1.0, DD<=100.0%, ret/DD>=0.5, ret/DD<=20.0, ret<=2.0%, markov>=0.5 |

## Best Non-Empty Selection

- policy=`all_sleeves`，active_rows=4，trades=49，total_realistic=1.2517%，worst_month=-0.6461%。

| Source | Month | Symbol | TopK | Trades | Realistic | Val Return/DD | Val Markov |
|---|---|---|---:|---:|---:|---:|---:|
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_postselect_btc_fallback` | `2025-07` | `BTCUSDT` | 20 | 20 | -0.6461% | 1.5367 | 0.5833 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_postselect_btc_fallback` | `2025-08` | `BTCUSDT` | 5 | 5 | -0.6346% | 5.1496 | 0.7247 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_postselect_btc_fallback` | `2025-12` | `ETHUSDT` | 20 | 19 | 0.7873% | 1.0837 | 0.5318 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_postselect_btc_fallback` | `2026-02` | `ETHUSDT` | 5 | 5 | 1.7451% | 5.4958 | 0.6715 |

## Interpretation

- 若最佳结果来自 `active_rows=0`，说明当前验证月字段只能选择空仓，不能证明概率模型已经具备可交易收益。
- 若非空最佳只保留单个样本，也只能作为下一轮特征/事件来源假设，不能作为实盘候选。
- Gate 只使用验证月字段；`test_edge` 只用于复盘解释，不参与筛选。
