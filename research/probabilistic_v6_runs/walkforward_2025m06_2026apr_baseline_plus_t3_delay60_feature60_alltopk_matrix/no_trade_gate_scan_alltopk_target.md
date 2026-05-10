# Probabilistic V6 No-Trade Gate Analyzer

范围：仅限 `research`。本报告只消费已有 `symbol_rows.csv`，扫描二级 no-trade gate；它不是新的实盘策略结论。

## Objective Diagnostics

- 目标收益：`10.00%` Active_Silo_Sum。
- 组合约束：active_rows>=4，active_months>=6，trades>=40，worst_month>=-2.00%。
- Baseline candidate pool：active_rows=60，trades=560，total=-50.7477%，silo_PF=0.2962。
- Oracle best positive per symbol-month：total=7.6231%，active_rows=6。这是事后上限诊断，不可当作可交易选择器。
- Target hit under validation-only gates：`False`。

## Baseline Candidates

| Source | Month | Symbol | TopK | Model | Trades | Realistic | Val Edge | Val TopK Return | Val TopK SL | Val TopK DD | Val Return/DD | Val Markov | Test Edge |
|---|---|---|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-07` | `BTCUSDT` | 5 | `gradient_boosting` | 5 | -0.1447% | 0.091476 | -0.286771% | 0.6000 | -0.412899% | -0.6945 | 0.9209 | -0.003921 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-07` | `BTCUSDT` | 10 | `gradient_boosting` | 10 | -0.1095% | 0.091476 | 0.594780% | 0.4000 | -0.452462% | 1.3145 | 0.7263 | -0.003921 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-07` | `BTCUSDT` | 15 | `gradient_boosting` | 15 | -0.0571% | 0.091476 | 0.836400% | 0.3333 | -0.850952% | 0.9829 | 0.5884 | -0.003921 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-07` | `BTCUSDT` | 20 | `gradient_boosting` | 20 | -0.6461% | 0.091476 | 1.164035% | 0.3000 | -0.757492% | 1.5367 | 0.5833 | -0.003921 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-07` | `ETHUSDT` | 5 | `random_forest` | 5 | -2.6500% | 0.301698 | 2.006413% | 0.2000 | -0.405447% | 4.9486 | 0.7010 | -0.110088 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-07` | `ETHUSDT` | 10 | `random_forest` | 5 | -2.6500% | 0.301698 | 2.545887% | 0.2500 | -0.643897% | 3.9539 | 0.6605 | -0.110088 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-07` | `ETHUSDT` | 15 | `random_forest` | 5 | -2.6500% | 0.301698 | 2.545887% | 0.2500 | -0.643897% | 3.9539 | 0.6605 | -0.110088 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-07` | `ETHUSDT` | 20 | `random_forest` | 5 | -2.6500% | 0.301698 | 2.545887% | 0.2500 | -0.643897% | 3.9539 | 0.6605 | -0.110088 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-08` | `BTCUSDT` | 5 | `gradient_boosting` | 5 | -0.6346% | 0.054666 | 1.287392% | 0.2000 | 0.000000% | 5.1496 | 0.7247 | -0.047965 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-08` | `BTCUSDT` | 10 | `gradient_boosting` | 10 | -0.4180% | 0.054666 | 0.196468% | 0.4000 | -0.913074% | 0.2152 | 0.6316 | -0.047965 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-08` | `BTCUSDT` | 15 | `gradient_boosting` | 15 | -0.5690% | 0.054666 | 1.288762% | 0.3333 | -1.148847% | 1.1218 | 0.5130 | -0.047965 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-08` | `BTCUSDT` | 20 | `gradient_boosting` | 16 | -0.6417% | 0.054666 | 1.158207% | 0.4000 | -1.279605% | 0.9051 | 0.4475 | -0.047965 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-09` | `ETHUSDT` | 5 | `logistic` | 5 | -4.4448% | 0.029066 | -1.332994% | 0.6000 | -2.066738% | -0.6450 | 0.5519 | -0.105818 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-09` | `ETHUSDT` | 10 | `logistic` | 10 | -6.0328% | 0.029066 | -2.580607% | 0.6000 | -3.117367% | -0.8278 | 0.5854 | -0.105818 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-09` | `ETHUSDT` | 15 | `logistic` | 15 | -6.1758% | 0.029066 | -3.060662% | 0.6000 | -3.681947% | -0.8313 | 0.5451 | -0.105818 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-09` | `ETHUSDT` | 20 | `logistic` | 20 | -6.7528% | 0.029066 | -1.295408% | 0.4500 | -2.652912% | -0.4883 | 0.5016 | -0.105818 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-11` | `BTCUSDT` | 5 | `gradient_boosting` | 5 | -1.2030% | 0.092051 | 0.787695% | 0.2000 | -0.102467% | 3.1508 | 0.5956 | -0.043889 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-11` | `BTCUSDT` | 10 | `gradient_boosting` | 10 | -2.7966% | 0.092051 | 0.692836% | 0.3000 | -0.769829% | 0.9000 | 0.4674 | -0.043889 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-11` | `BTCUSDT` | 15 | `gradient_boosting` | 15 | -0.8681% | 0.092051 | 0.596513% | 0.4000 | -1.008405% | 0.5915 | 0.5172 | -0.043889 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-11` | `BTCUSDT` | 20 | `gradient_boosting` | 20 | -0.8237% | 0.092051 | 1.004959% | 0.3500 | -1.008405% | 0.9966 | 0.4870 | -0.043889 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-12` | `BTCUSDT` | 5 | `logistic` | 5 | -0.2522% | 0.072177 | -0.152345% | 0.6000 | -1.147754% | -0.1327 | 0.5339 | 0.006658 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-12` | `BTCUSDT` | 10 | `logistic` | 10 | -0.5562% | 0.072177 | 0.705260% | 0.4000 | -1.451961% | 0.4857 | 0.3554 | 0.006658 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-12` | `BTCUSDT` | 15 | `logistic` | 11 | -0.4820% | 0.072177 | 0.705260% | 0.4000 | -1.451961% | 0.4857 | 0.3554 | 0.006658 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-12` | `BTCUSDT` | 20 | `logistic` | 11 | -0.4820% | 0.072177 | 0.705260% | 0.4000 | -1.451961% | 0.4857 | 0.3554 | 0.006658 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-12` | `ETHUSDT` | 5 | `logistic` | 5 | 0.1842% | 0.124987 | 0.591014% | 0.4000 | -0.948362% | 0.6232 | 0.7599 | -0.015316 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-12` | `ETHUSDT` | 10 | `logistic` | 10 | 0.9406% | 0.124987 | 0.226667% | 0.4000 | -1.539514% | 0.1472 | 0.7047 | -0.015316 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-12` | `ETHUSDT` | 15 | `logistic` | 15 | 1.4409% | 0.124987 | 0.770819% | 0.4000 | -1.597173% | 0.4826 | 0.5694 | -0.015316 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-12` | `ETHUSDT` | 20 | `logistic` | 19 | 0.7873% | 0.124987 | 1.674487% | 0.3500 | -1.545128% | 1.0837 | 0.5318 | -0.015316 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-01` | `BTCUSDT` | 5 | `extra_trees` | 5 | 0.0063% | 0.096620 | -0.151772% | 0.6000 | -0.300146% | -0.5057 | 0.8255 | 0.063995 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-01` | `BTCUSDT` | 10 | `extra_trees` | 6 | 0.0148% | 0.096620 | 0.689745% | 0.4000 | -0.758467% | 0.9094 | 0.6683 | 0.063995 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-01` | `BTCUSDT` | 15 | `extra_trees` | 6 | 0.0148% | 0.096620 | 0.774343% | 0.4167 | -0.758467% | 1.0209 | 0.6166 | 0.063995 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-01` | `BTCUSDT` | 20 | `extra_trees` | 6 | 0.0148% | 0.096620 | 0.774343% | 0.4167 | -0.758467% | 1.0209 | 0.6166 | 0.063995 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-01` | `ETHUSDT` | 5 | `logistic` | 3 | -2.0568% | 0.127419 | 1.965645% | 0.2000 | -0.447648% | 4.3911 | 0.4324 | 0.030222 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-01` | `ETHUSDT` | 10 | `logistic` | 3 | -2.0568% | 0.127419 | 2.106942% | 0.4000 | -1.210737% | 1.7402 | 0.5239 | 0.030222 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-01` | `ETHUSDT` | 15 | `logistic` | 3 | -2.0568% | 0.127419 | 2.125149% | 0.3846 | -1.210737% | 1.7553 | 0.4194 | 0.030222 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-01` | `ETHUSDT` | 20 | `logistic` | 3 | -2.0568% | 0.127419 | 2.125149% | 0.3846 | -1.210737% | 1.7553 | 0.4194 | 0.030222 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-02` | `BTCUSDT` | 5 | `logistic` | 5 | 1.8226% | 0.097803 | 1.563057% | 0.0000 | -0.206396% | 6.2522 | 0.3212 | 0.019730 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-02` | `BTCUSDT` | 10 | `logistic` | 10 | 1.0572% | 0.097803 | 1.908369% | 0.1000 | -0.206396% | 7.6335 | 0.3893 | 0.019730 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-02` | `BTCUSDT` | 15 | `logistic` | 13 | 1.1385% | 0.097803 | 1.445834% | 0.2667 | -0.404650% | 3.5730 | 0.4364 | 0.019730 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-02` | `BTCUSDT` | 20 | `logistic` | 13 | 1.1385% | 0.097803 | 1.900966% | 0.2353 | -0.404650% | 4.6978 | 0.4332 | 0.019730 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-02` | `ETHUSDT` | 5 | `gradient_boosting` | 5 | 1.7451% | 0.030675 | 1.373962% | 0.2000 | 0.000000% | 5.4958 | 0.6715 | 0.031729 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-02` | `ETHUSDT` | 10 | `gradient_boosting` | 10 | 1.9126% | 0.030675 | 0.741306% | 0.4000 | -0.867319% | 0.8547 | 0.5792 | 0.031729 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-02` | `ETHUSDT` | 15 | `gradient_boosting` | 11 | 2.8057% | 0.030675 | 0.741306% | 0.4000 | -0.867319% | 0.8547 | 0.5792 | 0.031729 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-02` | `ETHUSDT` | 20 | `gradient_boosting` | 11 | 2.8057% | 0.030675 | 0.741306% | 0.4000 | -0.867319% | 0.8547 | 0.5792 | 0.031729 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-03` | `BTCUSDT` | 5 | `gradient_boosting` | 5 | -1.9120% | 0.555678 | 3.472873% | 0.0000 | 0.000000% | 13.8915 | 0.6749 | -0.034806 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-03` | `BTCUSDT` | 10 | `gradient_boosting` | 10 | -1.8927% | 0.555678 | 6.508307% | 0.0000 | 0.000000% | 26.0332 | 0.6178 | -0.034806 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-03` | `BTCUSDT` | 15 | `gradient_boosting` | 10 | -1.8927% | 0.555678 | 6.683602% | 0.0833 | -0.311612% | 21.4485 | 0.5782 | -0.034806 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-03` | `BTCUSDT` | 20 | `gradient_boosting` | 10 | -1.8927% | 0.555678 | 6.683602% | 0.0833 | -0.311612% | 21.4485 | 0.5782 | -0.034806 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-03` | `ETHUSDT` | 5 | `logistic` | 5 | 1.4466% | 0.455029 | 2.873417% | 0.2000 | -0.730390% | 3.9341 | 0.7397 | -0.025028 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-03` | `ETHUSDT` | 10 | `logistic` | 8 | 0.6619% | 0.455029 | 4.460120% | 0.3000 | -0.958010% | 4.6556 | 0.4965 | -0.025028 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-03` | `ETHUSDT` | 15 | `logistic` | 8 | 0.6619% | 0.455029 | 4.460120% | 0.3000 | -0.958010% | 4.6556 | 0.4965 | -0.025028 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-03` | `ETHUSDT` | 20 | `logistic` | 8 | 0.6619% | 0.455029 | 4.460120% | 0.3000 | -0.958010% | 4.6556 | 0.4965 | -0.025028 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-04` | `BTCUSDT` | 5 | `gradient_boosting` | 5 | 0.0925% | 0.058918 | 0.107776% | 0.4000 | -1.531394% | 0.0704 | 0.5649 | -0.019721 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-04` | `BTCUSDT` | 10 | `gradient_boosting` | 10 | -0.2494% | 0.058918 | 0.859074% | 0.3000 | -1.531394% | 0.5610 | 0.4553 | -0.019721 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-04` | `BTCUSDT` | 15 | `gradient_boosting` | 15 | -0.3092% | 0.058918 | 0.519171% | 0.3636 | -1.871297% | 0.2774 | 0.4889 | -0.019721 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-04` | `BTCUSDT` | 20 | `gradient_boosting` | 16 | -0.2552% | 0.058918 | 0.519171% | 0.3636 | -1.871297% | 0.2774 | 0.4889 | -0.019721 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-04` | `ETHUSDT` | 5 | `gradient_boosting` | 5 | -2.6395% | 0.312602 | 0.752271% | 0.4000 | -0.601842% | 1.2499 | 0.5801 | -0.161792 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-04` | `ETHUSDT` | 10 | `gradient_boosting` | 10 | -2.7136% | 0.312602 | 2.301954% | 0.2000 | -0.847481% | 2.7162 | 0.4275 | -0.161792 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-04` | `ETHUSDT` | 15 | `gradient_boosting` | 10 | -2.7136% | 0.312602 | 2.301954% | 0.2000 | -0.847481% | 2.7162 | 0.4275 | -0.161792 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-04` | `ETHUSDT` | 20 | `gradient_boosting` | 10 | -2.7136% | 0.312602 | 2.301954% | 0.2000 | -0.847481% | 2.7162 | 0.4275 | -0.161792 |

## Top Gate Sweeps

| Rank | Policy | Active | Months | Trades | Total Realistic | Silo PF | Worst Month | Target | Gate |
|---:|---|---:|---:|---:|---:|---:|---:|---|---|
| 1 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.0, ret>=0.0%, SL<=0.6, DD<=3.0%, ret/DD>=0.0, ret/DD<=2.0, ret<=2.0%, markov>=0.5 |
| 2 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.0, ret>=0.0%, SL<=0.6, DD<=3.0%, ret/DD>=0.0, ret/DD<=3.0, ret<=2.0%, markov>=0.5 |
| 3 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.0, ret>=0.0%, SL<=0.6, DD<=5.0%, ret/DD>=0.0, ret/DD<=2.0, ret<=2.0%, markov>=0.5 |
| 4 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.0, ret>=0.0%, SL<=0.6, DD<=5.0%, ret/DD>=0.0, ret/DD<=3.0, ret<=2.0%, markov>=0.5 |
| 5 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.0, ret>=0.0%, SL<=0.6, DD<=100.0%, ret/DD>=0.0, ret/DD<=2.0, ret<=2.0%, markov>=0.5 |
| 6 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.0, ret>=0.0%, SL<=1.0, DD<=5.0%, ret/DD>=0.0, ret/DD<=2.0, ret<=2.0%, markov>=0.5 |
| 7 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.0, ret>=0.0%, SL<=1.0, DD<=3.0%, ret/DD>=0.0, ret/DD<=3.0, ret<=2.0%, markov>=0.5 |
| 8 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.02, ret>=0.0%, SL<=1.0, DD<=2.0%, ret/DD>=0.0, ret/DD<=2.0, ret<=2.0%, markov>=0.5 |
| 9 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.0, ret>=0.0%, SL<=0.6, DD<=100.0%, ret/DD>=0.0, ret/DD<=3.0, ret<=2.0%, markov>=0.5 |
| 10 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.0, ret>=0.0%, SL<=1.0, DD<=5.0%, ret/DD>=0.0, ret/DD<=3.0, ret<=2.0%, markov>=0.5 |
| 11 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.0, ret>=0.0%, SL<=1.0, DD<=2.0%, ret/DD>=0.0, ret/DD<=2.0, ret<=2.0%, markov>=0.5 |
| 12 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.0, ret>=0.0%, SL<=1.0, DD<=100.0%, ret/DD>=0.0, ret/DD<=2.0, ret<=2.0%, markov>=0.5 |
| 13 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.0, ret>=0.0%, SL<=1.0, DD<=100.0%, ret/DD>=0.0, ret/DD<=3.0, ret<=2.0%, markov>=0.5 |
| 14 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.02, ret>=0.0%, SL<=0.6, DD<=2.0%, ret/DD>=0.0, ret/DD<=3.0, ret<=2.0%, markov>=0.5 |
| 15 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.02, ret>=0.0%, SL<=0.6, DD<=2.0%, ret/DD>=0.0, ret/DD<=2.0, ret<=2.0%, markov>=0.5 |
| 16 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.02, ret>=0.0%, SL<=1.0, DD<=100.0%, ret/DD>=0.0, ret/DD<=2.0, ret<=2.0%, markov>=0.5 |
| 17 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.02, ret>=0.0%, SL<=1.0, DD<=3.0%, ret/DD>=0.0, ret/DD<=3.0, ret<=2.0%, markov>=0.5 |
| 18 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.0, ret>=0.0%, SL<=1.0, DD<=2.0%, ret/DD>=0.0, ret/DD<=3.0, ret<=2.0%, markov>=0.5 |
| 19 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.02, ret>=0.0%, SL<=1.0, DD<=5.0%, ret/DD>=0.0, ret/DD<=3.0, ret<=2.0%, markov>=0.5 |
| 20 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.02, ret>=0.0%, SL<=1.0, DD<=3.0%, ret/DD>=0.0, ret/DD<=2.0, ret<=2.0%, markov>=0.5 |
| 21 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.02, ret>=0.0%, SL<=0.6, DD<=5.0%, ret/DD>=0.0, ret/DD<=3.0, ret<=2.0%, markov>=0.5 |
| 22 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.02, ret>=0.0%, SL<=1.0, DD<=5.0%, ret/DD>=0.0, ret/DD<=2.0, ret<=2.0%, markov>=0.5 |
| 23 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.0, ret>=0.0%, SL<=1.0, DD<=3.0%, ret/DD>=0.0, ret/DD<=2.0, ret<=2.0%, markov>=0.5 |
| 24 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.02, ret>=0.0%, SL<=0.6, DD<=100.0%, ret/DD>=0.0, ret/DD<=2.0, ret<=2.0%, markov>=0.5 |
| 25 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.02, ret>=0.0%, SL<=0.6, DD<=100.0%, ret/DD>=0.0, ret/DD<=3.0, ret<=2.0%, markov>=0.5 |
| 26 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.02, ret>=0.0%, SL<=0.6, DD<=3.0%, ret/DD>=0.0, ret/DD<=2.0, ret<=2.0%, markov>=0.5 |
| 27 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.02, ret>=0.0%, SL<=0.6, DD<=5.0%, ret/DD>=0.0, ret/DD<=2.0, ret<=2.0%, markov>=0.5 |
| 28 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.02, ret>=0.0%, SL<=1.0, DD<=2.0%, ret/DD>=0.0, ret/DD<=3.0, ret<=2.0%, markov>=0.5 |
| 29 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.02, ret>=0.0%, SL<=1.0, DD<=100.0%, ret/DD>=0.0, ret/DD<=3.0, ret<=2.0%, markov>=0.5 |
| 30 | `all_sleeves` | 18 | 7 | 194 | 5.7066% | 2.0752 | -2.5470% | `False` | edge>=0.02, ret>=0.0%, SL<=0.6, DD<=3.0%, ret/DD>=0.0, ret/DD<=3.0, ret<=2.0%, markov>=0.5 |

## Best Non-Empty Selection

- policy=`all_sleeves`，active_rows=18，trades=194，total_realistic=5.7066%，worst_month=-2.5470%。

| Source | Month | Symbol | TopK | Trades | Realistic | Val Return/DD | Val Markov |
|---|---|---|---:|---:|---:|---:|---:|
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-07` | `BTCUSDT` | 10 | 10 | -0.1095% | 1.3145 | 0.7263 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-07` | `BTCUSDT` | 15 | 15 | -0.0571% | 0.9829 | 0.5884 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-07` | `BTCUSDT` | 20 | 20 | -0.6461% | 1.5367 | 0.5833 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-08` | `BTCUSDT` | 10 | 10 | -0.4180% | 0.2152 | 0.6316 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-08` | `BTCUSDT` | 15 | 15 | -0.5690% | 1.1218 | 0.5130 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-11` | `BTCUSDT` | 15 | 15 | -0.8681% | 0.5915 | 0.5172 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-12` | `ETHUSDT` | 5 | 5 | 0.1842% | 0.6232 | 0.7599 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-12` | `ETHUSDT` | 10 | 10 | 0.9406% | 0.1472 | 0.7047 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-12` | `ETHUSDT` | 15 | 15 | 1.4409% | 0.4826 | 0.5694 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2025-12` | `ETHUSDT` | 20 | 19 | 0.7873% | 1.0837 | 0.5318 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-01` | `BTCUSDT` | 10 | 6 | 0.0148% | 0.9094 | 0.6683 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-01` | `BTCUSDT` | 15 | 6 | 0.0148% | 1.0209 | 0.6166 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-01` | `BTCUSDT` | 20 | 6 | 0.0148% | 1.0209 | 0.6166 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-02` | `ETHUSDT` | 10 | 10 | 1.9126% | 0.8547 | 0.5792 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-02` | `ETHUSDT` | 15 | 11 | 2.8057% | 0.8547 | 0.5792 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-02` | `ETHUSDT` | 20 | 11 | 2.8057% | 0.8547 | 0.5792 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-04` | `BTCUSDT` | 5 | 5 | 0.0925% | 0.0704 | 0.5649 |
| `walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix` | `2026-04` | `ETHUSDT` | 5 | 5 | -2.6395% | 1.2499 | 0.5801 |

## Interpretation

- 若最佳结果来自 `active_rows=0`，说明当前验证月字段只能选择空仓，不能证明概率模型已经具备可交易收益。
- 若非空最佳只保留单个样本，也只能作为下一轮特征/事件来源假设，不能作为实盘候选。
- Gate 只使用验证月字段；`test_edge` 只用于复盘解释，不参与筛选。
