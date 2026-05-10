# Probabilistic V6 No-Trade Gate Analyzer

范围：仅限 `research`。本报告只消费已有 `symbol_rows.csv`，扫描二级 no-trade gate；它不是新的实盘策略结论。

## Objective Diagnostics

- 目标收益：`10.00%` Active_Silo_Sum。
- 组合约束：active_rows>=1，active_months>=6，trades>=40，worst_month>=-2.00%。
- Baseline candidate pool：active_rows=64，trades=589，total=-7.1750%，silo_PF=0.8142。
- Oracle best positive per symbol-month：total=10.4570%，active_rows=8。这是事后上限诊断，不可当作可交易选择器。
- Target hit under validation-only gates：`False`。

## Baseline Candidates

| Source | Month | Symbol | TopK | Model | Trades | Realistic | Val Edge | Val TopK Return | Val TopK SL | Val TopK DD | Val Return/DD | Val Markov | Test Edge |
|---|---|---|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-07` | `BTCUSDT` | 5 | `gradient_boosting` | 5 | -0.4786% | 0.114524 | 0.427410% | 0.2000 | -0.479561% | 0.8913 | 0.4130 | 0.006547 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-07` | `BTCUSDT` | 10 | `gradient_boosting` | 10 | -1.0570% | 0.114524 | 1.211887% | 0.2000 | -0.479561% | 2.5271 | 0.5186 | 0.006547 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-07` | `BTCUSDT` | 15 | `gradient_boosting` | 15 | -1.4013% | 0.114524 | 1.674001% | 0.2000 | -0.479561% | 3.4907 | 0.5686 | 0.006547 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-07` | `BTCUSDT` | 20 | `gradient_boosting` | 18 | -0.7250% | 0.114524 | 1.921236% | 0.1765 | -0.479561% | 4.0062 | 0.5651 | 0.006547 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-07` | `ETHUSDT` | 5 | `svm_rbf` | 5 | -2.1443% | 0.035094 | 1.005023% | 0.4000 | -0.736598% | 1.3644 | 0.4219 | -0.063076 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-07` | `ETHUSDT` | 10 | `svm_rbf` | 9 | -2.7931% | 0.035094 | 1.184167% | 0.5000 | -1.216737% | 0.9732 | 0.3720 | -0.063076 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-07` | `ETHUSDT` | 15 | `svm_rbf` | 9 | -2.7931% | 0.035094 | 1.034360% | 0.5455 | -1.216737% | 0.8501 | 0.3891 | -0.063076 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-07` | `ETHUSDT` | 20 | `svm_rbf` | 9 | -2.7931% | 0.035094 | 1.034360% | 0.5455 | -1.216737% | 0.8501 | 0.3891 | -0.063076 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-08` | `BTCUSDT` | 5 | `gradient_boosting` | 5 | -1.3744% | 0.121069 | 1.368441% | 0.2000 | 0.000000% | 5.4738 | 0.5199 | -0.061392 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-08` | `BTCUSDT` | 10 | `gradient_boosting` | 10 | -0.4280% | 0.121069 | 0.821141% | 0.4000 | -0.763777% | 1.0751 | 0.5420 | -0.061392 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-08` | `BTCUSDT` | 15 | `gradient_boosting` | 15 | -0.6873% | 0.121069 | 1.615328% | 0.3333 | -0.763777% | 2.1149 | 0.4753 | -0.061392 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-08` | `BTCUSDT` | 20 | `gradient_boosting` | 16 | -3.9965% | 0.121069 | 1.928772% | 0.2941 | -0.763777% | 2.5253 | 0.4744 | -0.061392 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-09` | `BTCUSDT` | 5 | `random_forest` | 5 | -0.0487% | 0.044286 | -0.154016% | 0.4000 | -0.567239% | -0.2715 | 0.4973 | -0.006503 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-09` | `BTCUSDT` | 10 | `random_forest` | 7 | -0.0760% | 0.044286 | 0.213509% | 0.3333 | -0.759818% | 0.2810 | 0.4870 | -0.006503 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-09` | `BTCUSDT` | 15 | `random_forest` | 7 | -0.0760% | 0.044286 | 0.213509% | 0.3333 | -0.759818% | 0.2810 | 0.4870 | -0.006503 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-09` | `BTCUSDT` | 20 | `random_forest` | 7 | -0.0760% | 0.044286 | 0.213509% | 0.3333 | -0.759818% | 0.2810 | 0.4870 | -0.006503 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-10` | `BTCUSDT` | 5 | `logistic` | 5 | -0.7972% | 0.053765 | 0.337729% | 0.2000 | -0.604905% | 0.5583 | 0.4913 | 0.049289 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-10` | `BTCUSDT` | 10 | `logistic` | 7 | -1.2245% | 0.053765 | 0.012447% | 0.3000 | -1.067800% | 0.0117 | 0.3978 | 0.049289 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-10` | `BTCUSDT` | 15 | `logistic` | 7 | -1.2245% | 0.053765 | 0.618998% | 0.2667 | -1.067800% | 0.5797 | 0.4113 | 0.049289 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-10` | `BTCUSDT` | 20 | `logistic` | 7 | -1.2245% | 0.053765 | 0.908969% | 0.2353 | -1.067800% | 0.8513 | 0.4086 | 0.049289 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-11` | `BTCUSDT` | 5 | `extra_trees` | 5 | 0.0904% | 0.023508 | -0.081874% | 0.4000 | -0.207108% | -0.3275 | 0.8627 | 0.004816 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-11` | `BTCUSDT` | 10 | `extra_trees` | 10 | -0.2789% | 0.023508 | -0.720135% | 0.5000 | -0.846179% | -0.8510 | 0.7400 | 0.004816 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-11` | `BTCUSDT` | 15 | `extra_trees` | 15 | -0.5391% | 0.023508 | -0.649653% | 0.4667 | -1.161583% | -0.5593 | 0.6557 | 0.004816 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-11` | `BTCUSDT` | 20 | `extra_trees` | 18 | -0.6164% | 0.023508 | -0.230478% | 0.4000 | -0.834839% | -0.2761 | 0.5518 | 0.004816 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-11` | `ETHUSDT` | 5 | `gradient_boosting` | 5 | -1.3212% | 0.032745 | -1.481130% | 0.6000 | -0.730302% | -2.0281 | 0.8083 | 0.073705 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-11` | `ETHUSDT` | 10 | `gradient_boosting` | 10 | 0.4413% | 0.032745 | -0.268866% | 0.3750 | -1.789918% | -0.1502 | 0.7395 | 0.073705 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-11` | `ETHUSDT` | 15 | `gradient_boosting` | 15 | 0.4477% | 0.032745 | -0.268866% | 0.3750 | -1.789918% | -0.1502 | 0.7395 | 0.073705 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-11` | `ETHUSDT` | 20 | `gradient_boosting` | 17 | 1.0495% | 0.032745 | -0.268866% | 0.3750 | -1.789918% | -0.1502 | 0.7395 | 0.073705 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-12` | `BTCUSDT` | 5 | `logistic` | 5 | -0.2443% | 0.050543 | -0.079853% | 0.6000 | -0.481260% | -0.1659 | 0.3932 | 0.065025 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-12` | `BTCUSDT` | 10 | `logistic` | 9 | -0.4182% | 0.050543 | 0.781003% | 0.4000 | -0.567744% | 1.3756 | 0.3936 | 0.065025 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-12` | `BTCUSDT` | 15 | `logistic` | 9 | -0.4182% | 0.050543 | 0.833596% | 0.3636 | -0.567744% | 1.4683 | 0.3578 | 0.065025 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-12` | `BTCUSDT` | 20 | `logistic` | 9 | -0.4182% | 0.050543 | 0.833596% | 0.3636 | -0.567744% | 1.4683 | 0.3578 | 0.065025 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-12` | `ETHUSDT` | 5 | `logistic` | 5 | -1.1696% | 0.126700 | 0.572915% | 0.4000 | -1.083086% | 0.5290 | 0.5782 | 0.101683 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-12` | `ETHUSDT` | 10 | `logistic` | 10 | -0.3509% | 0.126700 | 0.905027% | 0.4000 | -1.391405% | 0.6504 | 0.6454 | 0.101683 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-12` | `ETHUSDT` | 15 | `logistic` | 15 | 0.1590% | 0.126700 | 0.726279% | 0.4667 | -1.994697% | 0.3641 | 0.6765 | 0.101683 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-12` | `ETHUSDT` | 20 | `logistic` | 18 | -0.1849% | 0.126700 | 2.349890% | 0.3684 | -1.994697% | 1.1781 | 0.6208 | 0.101683 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-01` | `BTCUSDT` | 5 | `gradient_boosting` | 5 | 0.1360% | 0.185802 | 0.332763% | 0.2000 | -0.519768% | 0.6402 | 0.2304 | 0.095107 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-01` | `BTCUSDT` | 10 | `gradient_boosting` | 7 | 0.2418% | 0.185802 | 1.604042% | 0.2000 | -0.538990% | 2.9760 | 0.4001 | 0.095107 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-01` | `BTCUSDT` | 15 | `gradient_boosting` | 7 | 0.2418% | 0.185802 | 1.604042% | 0.2000 | -0.538990% | 2.9760 | 0.4001 | 0.095107 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-01` | `BTCUSDT` | 20 | `gradient_boosting` | 7 | 0.2418% | 0.185802 | 1.604042% | 0.2000 | -0.538990% | 2.9760 | 0.4001 | 0.095107 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-02` | `BTCUSDT` | 5 | `logistic` | 5 | 0.1037% | 0.118453 | 0.997720% | 0.4000 | -0.586855% | 1.7001 | 0.5118 | 0.038176 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-02` | `BTCUSDT` | 10 | `logistic` | 10 | 2.3794% | 0.118453 | 1.547843% | 0.2000 | -0.511873% | 3.0239 | 0.4105 | 0.038176 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-02` | `BTCUSDT` | 15 | `logistic` | 12 | 2.4093% | 0.118453 | 1.653005% | 0.2857 | -0.645709% | 2.5600 | 0.3881 | 0.038176 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-02` | `BTCUSDT` | 20 | `logistic` | 12 | 2.4093% | 0.118453 | 1.653005% | 0.2857 | -0.645709% | 2.5600 | 0.3881 | 0.038176 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-02` | `ETHUSDT` | 5 | `logistic` | 5 | 3.3305% | 0.126830 | 1.038121% | 0.2000 | 0.000000% | 4.1525 | 0.4280 | 0.043855 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-02` | `ETHUSDT` | 10 | `logistic` | 10 | 2.2880% | 0.126830 | 1.018582% | 0.2500 | -0.345710% | 2.9463 | 0.3892 | 0.043855 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-02` | `ETHUSDT` | 15 | `logistic` | 11 | 1.5775% | 0.126830 | 1.018582% | 0.2500 | -0.345710% | 2.9463 | 0.3892 | 0.043855 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-02` | `ETHUSDT` | 20 | `logistic` | 11 | 1.5775% | 0.126830 | 1.018582% | 0.2500 | -0.345710% | 2.9463 | 0.3892 | 0.043855 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-03` | `BTCUSDT` | 5 | `extra_trees` | 5 | -0.9530% | 0.574219 | 3.330438% | 0.0000 | 0.000000% | 13.3218 | 0.7918 | -0.037735 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-03` | `BTCUSDT` | 10 | `extra_trees` | 6 | -0.4122% | 0.574219 | 5.392295% | 0.1000 | -0.135635% | 21.5692 | 0.7049 | -0.037735 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-03` | `BTCUSDT` | 15 | `extra_trees` | 6 | -0.4122% | 0.574219 | 5.392295% | 0.1000 | -0.135635% | 21.5692 | 0.7049 | -0.037735 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-03` | `BTCUSDT` | 20 | `extra_trees` | 6 | -0.4122% | 0.574219 | 5.392295% | 0.1000 | -0.135635% | 21.5692 | 0.7049 | -0.037735 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-03` | `ETHUSDT` | 5 | `logistic` | 5 | 2.8842% | 0.374316 | 3.669592% | 0.2000 | -0.680019% | 5.3963 | 0.7158 | 0.153834 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-03` | `ETHUSDT` | 10 | `logistic` | 8 | 3.1296% | 0.374316 | 3.694700% | 0.2500 | -0.939577% | 3.9323 | 0.6166 | 0.153834 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-03` | `ETHUSDT` | 15 | `logistic` | 8 | 3.1296% | 0.374316 | 3.694700% | 0.2500 | -0.939577% | 3.9323 | 0.6166 | 0.153834 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-03` | `ETHUSDT` | 20 | `logistic` | 8 | 3.1296% | 0.374316 | 3.694700% | 0.2500 | -0.939577% | 3.9323 | 0.6166 | 0.153834 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-04` | `BTCUSDT` | 5 | `random_forest` | 5 | 0.0469% | 0.039688 | -0.725981% | 0.4000 | -1.679346% | -0.4323 | 0.7978 | -0.083445 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-04` | `BTCUSDT` | 10 | `random_forest` | 10 | -0.1299% | 0.039688 | -1.527392% | 0.6000 | -2.148507% | -0.7109 | 0.6620 | -0.083445 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-04` | `BTCUSDT` | 15 | `random_forest` | 15 | -0.2382% | 0.039688 | -0.590357% | 0.4000 | -2.148507% | -0.2748 | 0.5296 | -0.083445 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-04` | `BTCUSDT` | 20 | `random_forest` | 20 | -0.3213% | 0.039688 | -0.300786% | 0.3750 | -2.148507% | -0.1400 | 0.5188 | -0.083445 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-04` | `ETHUSDT` | 5 | `logistic` | 5 | -1.2168% | 0.441266 | 2.803271% | 0.0000 | 0.000000% | 11.2131 | 0.4421 | -0.087715 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-04` | `ETHUSDT` | 10 | `logistic` | 9 | -1.0482% | 0.441266 | 3.587521% | 0.1111 | -0.544497% | 6.5887 | 0.4424 | -0.087715 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-04` | `ETHUSDT` | 15 | `logistic` | 9 | -1.0482% | 0.441266 | 3.587521% | 0.1111 | -0.544497% | 6.5887 | 0.4424 | -0.087715 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-04` | `ETHUSDT` | 20 | `logistic` | 9 | -1.0482% | 0.441266 | 3.587521% | 0.1111 | -0.544497% | 6.5887 | 0.4424 | -0.087715 |

## Top Gate Sweeps

| Rank | Policy | Active | Months | Trades | Total Realistic | Silo PF | Worst Month | Target | Gate |
|---:|---|---:|---:|---:|---:|---:|---:|---|---|
| 1 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=100.0%, ret/DD>=1.0, ret/DD<=5.0, ret<=5.0%, markov>=-999.0 |
| 2 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=100.0%, ret/DD>=2.0, ret/DD<=5.0, ret<=7.0%, markov>=-999.0 |
| 3 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=100.0%, ret/DD>=1.0, ret/DD<=5.0, ret<=5.0%, markov>=0.3 |
| 4 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=100.0%, ret/DD>=2.0, ret/DD<=5.0, ret<=7.0%, markov>=0.3 |
| 5 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=100.0%, ret/DD>=2.0, ret/DD<=5.0, ret<=5.0%, markov>=0.3 |
| 6 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=2.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=7.0%, markov>=0.3 |
| 7 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=2.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=5.0%, markov>=0.3 |
| 8 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=2.0%, ret/DD>=2.0, ret/DD<=5.0, ret<=7.0%, markov>=0.3 |
| 9 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=100.0%, ret/DD>=2.0, ret/DD<=5.0, ret<=5.0%, markov>=-999.0 |
| 10 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=2.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=7.0%, markov>=-999.0 |
| 11 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=2.0%, ret/DD>=2.0, ret/DD<=5.0, ret<=5.0%, markov>=-999.0 |
| 12 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=2.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=5.0%, markov>=-999.0 |
| 13 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=2.0%, ret/DD>=2.0, ret/DD<=5.0, ret<=7.0%, markov>=-999.0 |
| 14 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=1.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=7.0%, markov>=-999.0 |
| 15 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=1.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=7.0%, markov>=0.3 |
| 16 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=1.0%, ret/DD>=1.0, ret/DD<=5.0, ret<=7.0%, markov>=0.3 |
| 17 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=1.0%, ret/DD>=1.0, ret/DD<=5.0, ret<=5.0%, markov>=0.3 |
| 18 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=2.0%, ret/DD>=1.0, ret/DD<=5.0, ret<=5.0%, markov>=0.3 |
| 19 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=1.0%, ret/DD>=2.0, ret/DD<=5.0, ret<=5.0%, markov>=-999.0 |
| 20 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=2.0%, ret/DD>=1.0, ret/DD<=5.0, ret<=5.0%, markov>=-999.0 |
| 21 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=2.0%, ret/DD>=1.0, ret/DD<=5.0, ret<=7.0%, markov>=-999.0 |
| 22 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=1.0%, ret/DD>=2.0, ret/DD<=5.0, ret<=7.0%, markov>=-999.0 |
| 23 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=1.0%, ret/DD>=2.0, ret/DD<=5.0, ret<=5.0%, markov>=0.3 |
| 24 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=2.0%, ret/DD>=2.0, ret/DD<=5.0, ret<=5.0%, markov>=0.3 |
| 25 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=2.0%, ret/DD>=1.0, ret/DD<=5.0, ret<=7.0%, markov>=0.3 |
| 26 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=1.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=5.0%, markov>=-999.0 |
| 27 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=1.0%, ret/DD>=1.0, ret/DD<=5.0, ret<=7.0%, markov>=-999.0 |
| 28 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=1.0%, ret/DD>=2.0, ret/DD<=5.0, ret<=7.0%, markov>=0.3 |
| 29 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=1.0%, ret/DD>=1.0, ret/DD<=5.0, ret<=5.0%, markov>=-999.0 |
| 30 | `all_sleeves` | 17 | 5 | 175 | 18.9059% | 3.6332 | -3.9965% | `False` | edge>=0.1, ret>=1.0%, SL<=0.3, DD<=1.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=5.0%, markov>=0.3 |

## Best Non-Empty Selection

- policy=`all_sleeves`，active_rows=17，trades=175，total_realistic=18.9059%，worst_month=-3.9965%。

| Source | Month | Symbol | TopK | Trades | Realistic | Val Return/DD | Val Markov |
|---|---|---|---:|---:|---:|---:|---:|
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-07` | `BTCUSDT` | 10 | 10 | -1.0570% | 2.5271 | 0.5186 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-07` | `BTCUSDT` | 15 | 15 | -1.4013% | 3.4907 | 0.5686 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-07` | `BTCUSDT` | 20 | 18 | -0.7250% | 4.0062 | 0.5651 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-08` | `BTCUSDT` | 20 | 16 | -3.9965% | 2.5253 | 0.4744 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-01` | `BTCUSDT` | 10 | 7 | 0.2418% | 2.9760 | 0.4001 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-01` | `BTCUSDT` | 15 | 7 | 0.2418% | 2.9760 | 0.4001 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-01` | `BTCUSDT` | 20 | 7 | 0.2418% | 2.9760 | 0.4001 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-02` | `BTCUSDT` | 10 | 10 | 2.3794% | 3.0239 | 0.4105 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-02` | `BTCUSDT` | 15 | 12 | 2.4093% | 2.5600 | 0.3881 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-02` | `BTCUSDT` | 20 | 12 | 2.4093% | 2.5600 | 0.3881 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-02` | `ETHUSDT` | 5 | 5 | 3.3305% | 4.1525 | 0.4280 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-02` | `ETHUSDT` | 10 | 10 | 2.2880% | 2.9463 | 0.3892 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-02` | `ETHUSDT` | 15 | 11 | 1.5775% | 2.9463 | 0.3892 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-02` | `ETHUSDT` | 20 | 11 | 1.5775% | 2.9463 | 0.3892 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-03` | `ETHUSDT` | 10 | 8 | 3.1296% | 3.9323 | 0.6166 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-03` | `ETHUSDT` | 15 | 8 | 3.1296% | 3.9323 | 0.6166 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-03` | `ETHUSDT` | 20 | 8 | 3.1296% | 3.9323 | 0.6166 |

## Best Qualified Selection

- policy=`best_validation_per_symbol_month`，active_rows=7，active_months=6，trades=59，total_realistic=5.6883%，silo_PF=2.8072，target_hit=`False`。

| Source | Month | Symbol | TopK | Trades | Realistic | Val Return/DD | Val Markov |
|---|---|---|---:|---:|---:|---:|---:|
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-07` | `BTCUSDT` | 20 | 18 | -0.7250% | 4.0062 | 0.5651 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2025-08` | `BTCUSDT` | 5 | 5 | -1.3744% | 5.4738 | 0.5199 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-01` | `BTCUSDT` | 10 | 7 | 0.2418% | 2.9760 | 0.4001 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-02` | `BTCUSDT` | 10 | 10 | 2.3794% | 3.0239 | 0.4105 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-02` | `ETHUSDT` | 5 | 5 | 3.3305% | 4.1525 | 0.4280 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-03` | `ETHUSDT` | 5 | 5 | 2.8842% | 5.3963 | 0.7158 |
| `walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix` | `2026-04` | `ETHUSDT` | 10 | 9 | -1.0482% | 6.5887 | 0.4424 |

## Interpretation

- 若最佳结果来自 `active_rows=0`，说明当前验证月字段只能选择空仓，不能证明概率模型已经具备可交易收益。
- 若非空最佳只保留单个样本，也只能作为下一轮特征/事件来源假设，不能作为实盘候选。
- Gate 只使用验证月字段；`test_edge` 只用于复盘解释，不参与筛选。
