# Probabilistic V6 No-Trade Gate Analyzer

范围：仅限 `research`。本报告只消费已有 `symbol_rows.csv`，扫描二级 no-trade gate；它不是新的实盘策略结论。

## Objective Diagnostics

- 目标收益：`10.00%` Active_Silo_Sum。
- 组合约束：active_rows>=4，active_months>=4，trades>=20，worst_month>=-2.00%。
- Target hit 额外要求每个 `(execute_month, symbol)` 最多选择一个 sleeve；重复 topK sleeve 只作为诊断口径。
- Baseline candidate pool：active_rows=22，trades=111，total=3.4592%，silo_PF=1.4631。
- Oracle best positive per symbol-month：total=10.9286%，active_rows=10。这是事后上限诊断，不可当作可交易选择器。
- Target hit under validation-only gates：`False`。

## Baseline Candidates

| Source | Month | Symbol | TopK | Model | Trades | Realistic | Val Edge | Val TopK Return | Val TopK SL | Val TopK DD | Val Return/DD | Val Markov | Test Edge |
|---|---|---|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-06` | `BTCUSDT` | 10 | `logistic` | 1 | -0.0438% | -0.047197 | -0.222156% | 0.6000 | -0.389298% | -0.5707 | 0.3775 | 0.082414 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-06` | `ETHUSDT` | 10 | `svm_rbf` | 3 | 0.1090% | 0.105443 | 0.518147% | 0.3750 | -1.842202% | 0.2813 | 0.5000 | 0.112238 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-07` | `BTCUSDT` | 10 | `logistic` | 6 | -0.0658% | -0.242867 | -0.575766% | 1.0000 | -0.218170% | -2.3031 | 0.5545 | 0.056053 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-07` | `ETHUSDT` | 10 | `logistic` | 5 | -2.0627% | -0.056912 | -0.620657% | 0.5000 | -1.210763% | -0.5126 | 0.4411 | 0.080328 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-08` | `BTCUSDT` | 10 | `random_forest` | 5 | -0.0530% | 0.005247 | 0.076595% | 0.3333 | -0.470719% | 0.1627 | 0.5232 | 0.061595 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-08` | `ETHUSDT` | 10 | `gradient_boosting` | 2 | 1.7721% | -0.305066 | -1.993485% | 0.6000 | -1.531103% | -1.3020 | 0.3708 | 0.112171 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-09` | `BTCUSDT` | 10 | `logistic` | 4 | -0.0476% | 0.007047 | 0.117296% | 0.4000 | -0.661729% | 0.1773 | 0.5125 | 0.067050 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-09` | `ETHUSDT` | 10 | `logistic` | 5 | 0.2444% | 0.810127 | 2.334046% | 0.0000 | 0.000000% | 9.3362 | 0.8793 | 0.089146 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-10` | `BTCUSDT` | 10 | `logistic` | 3 | -0.9047% | 0.000610 | 0.288474% | 0.2500 | -0.366684% | 0.7867 | 0.3682 | 0.072828 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-10` | `ETHUSDT` | 10 | `extra_trees` | 1 | -0.9751% | 0.127809 | 0.393512% | 0.2000 | 0.000000% | 1.5740 | 0.5039 | 0.043160 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-11` | `BTCUSDT` | 10 | `logistic` | 7 | -0.0322% | -0.133845 | -0.638423% | 0.3333 | -0.428926% | -1.4884 | 0.8688 | 0.087247 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-11` | `ETHUSDT` | 10 | `logistic` | 10 | 3.2810% | -0.757582 | -1.679526% | 1.0000 | -1.064964% | -1.5771 | 0.5574 | 0.121758 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-12` | `BTCUSDT` | 10 | `svm_rbf` | 9 | -0.1176% | 0.037147 | 0.477827% | 0.4286 | -0.437149% | 1.0931 | 0.4536 | 0.096988 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-12` | `ETHUSDT` | 10 | `svm_rbf` | 8 | 0.3794% | 0.323615 | 2.845111% | 0.3000 | -0.581895% | 4.8894 | 0.4559 | 0.058317 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-01` | `BTCUSDT` | 10 | `gradient_boosting` | 2 | 0.1524% | 0.011170 | 0.272920% | 0.4000 | -0.919503% | 0.2968 | 0.5605 | 0.166527 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-01` | `ETHUSDT` | 10 | `logistic` | 2 | 0.9074% | 0.167435 | 0.829432% | 0.3750 | -1.947644% | 0.4259 | 0.3712 | -0.027157 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-02` | `BTCUSDT` | 10 | `svm_rbf` | 5 | 2.5186% | 0.233333 | 1.201155% | 0.0000 | 0.000000% | 4.8046 | 0.4921 | 0.107801 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-02` | `ETHUSDT` | 10 | `svm_rbf` | 10 | -0.6501% | 0.253067 | 1.379674% | 0.2000 | 0.000000% | 5.5187 | 0.5714 | -0.025624 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-03` | `BTCUSDT` | 10 | `logistic` | 1 | 0.6713% | 0.627536 | 3.638048% | 0.0000 | 0.000000% | 14.5522 | 0.4065 | 0.067053 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-03` | `ETHUSDT` | 10 | `svm_rbf` | 2 | 0.8930% | 0.111351 | 0.541187% | 0.3333 | -2.193172% | 0.2468 | 0.6702 | -0.052880 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-04` | `BTCUSDT` | 10 | `svm_rbf` | 10 | -0.2150% | -0.129132 | -0.163871% | 0.6000 | -1.159344% | -0.1413 | 0.6244 | -0.035881 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-04` | `ETHUSDT` | 10 | `logistic` | 10 | -2.3018% | 0.570816 | 1.410868% | 0.0000 | 0.000000% | 5.6435 | 0.9115 | -0.177619 |

## Top Gate Sweeps

| Rank | Policy | Active | Months | Trades | Total Realistic | Silo PF | Worst Month | Unique | Target | Gate |
|---:|---|---:|---:|---:|---:|---:|---:|---|---|---|
| 1 | `all_sleeves` | 18 | 11 | 85 | 5.4954% | 2.2165 | -2.1285% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=999.0%, markov>=-999.0 |
| 2 | `all_sleeves` | 18 | 11 | 85 | 5.4954% | 2.2165 | -2.1285% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=4.0%, markov>=-999.0 |
| 3 | `best_validation_per_symbol_month` | 18 | 11 | 85 | 5.4954% | 2.2165 | -2.1285% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=999.0%, markov>=-999.0 |
| 4 | `best_validation_per_symbol_month` | 18 | 11 | 85 | 5.4954% | 2.2165 | -2.1285% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=4.0%, markov>=-999.0 |
| 5 | `all_sleeves` | 17 | 11 | 77 | 5.1160% | 2.1325 | -2.1285% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=2.0%, markov>=-999.0 |
| 6 | `best_validation_per_symbol_month` | 17 | 11 | 77 | 5.1160% | 2.1325 | -2.1285% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=2.0%, markov>=-999.0 |
| 7 | `all_sleeves` | 17 | 10 | 83 | 4.6024% | 2.0188 | -2.1285% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=2.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=999.0%, markov>=-999.0 |
| 8 | `best_validation_per_symbol_month` | 17 | 10 | 83 | 4.6024% | 2.0188 | -2.1285% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=2.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=4.0%, markov>=-999.0 |
| 9 | `best_validation_per_symbol_month` | 17 | 10 | 83 | 4.6024% | 2.0188 | -2.1285% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=2.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=999.0%, markov>=-999.0 |
| 10 | `all_sleeves` | 17 | 10 | 83 | 4.6024% | 2.0188 | -2.1285% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=2.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=4.0%, markov>=-999.0 |
| 11 | `best_validation_per_symbol_month` | 16 | 10 | 75 | 4.2230% | 1.9348 | -2.1285% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=2.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=2.0%, markov>=-999.0 |
| 12 | `all_sleeves` | 16 | 10 | 75 | 4.2230% | 1.9348 | -2.1285% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=2.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=2.0%, markov>=-999.0 |
| 13 | `best_validation_per_symbol_month` | 6 | 6 | 21 | 3.8323% | 4.9302 | -0.9751% | `True` | `False` | edge>=0.1, ret>=-999.0%, SL<=0.6, DD<=100.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=999.0%, markov>=-999.0 |
| 14 | `best_validation_per_symbol_month` | 6 | 6 | 21 | 3.8323% | 4.9302 | -0.9751% | `True` | `False` | edge>=0.1, ret>=-999.0%, SL<=0.4, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=999.0%, markov>=-999.0 |
| 15 | `best_validation_per_symbol_month` | 6 | 6 | 21 | 3.8323% | 4.9302 | -0.9751% | `True` | `False` | edge>=0.1, ret>=-999.0%, SL<=0.6, DD<=100.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=4.0%, markov>=-999.0 |
| 16 | `best_validation_per_symbol_month` | 6 | 6 | 21 | 3.8323% | 4.9302 | -0.9751% | `True` | `False` | edge>=0.1, ret>=-999.0%, SL<=0.4, DD<=100.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=999.0%, markov>=-999.0 |
| 17 | `best_validation_per_symbol_month` | 6 | 6 | 21 | 3.8323% | 4.9302 | -0.9751% | `True` | `False` | edge>=0.1, ret>=0.0%, SL<=1.0, DD<=100.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=4.0%, markov>=-999.0 |
| 18 | `best_validation_per_symbol_month` | 6 | 6 | 21 | 3.8323% | 4.9302 | -0.9751% | `True` | `False` | edge>=0.1, ret>=0.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=4.0%, markov>=-999.0 |
| 19 | `best_validation_per_symbol_month` | 6 | 6 | 21 | 3.8323% | 4.9302 | -0.9751% | `True` | `False` | edge>=0.1, ret>=-999.0%, SL<=0.4, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=4.0%, markov>=-999.0 |
| 20 | `best_validation_per_symbol_month` | 6 | 6 | 21 | 3.8323% | 4.9302 | -0.9751% | `True` | `False` | edge>=0.1, ret>=0.0%, SL<=1.0, DD<=100.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=999.0%, markov>=-999.0 |
| 21 | `best_validation_per_symbol_month` | 6 | 6 | 21 | 3.8323% | 4.9302 | -0.9751% | `True` | `False` | edge>=0.1, ret>=0.0%, SL<=0.6, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=999.0%, markov>=-999.0 |
| 22 | `best_validation_per_symbol_month` | 6 | 6 | 21 | 3.8323% | 4.9302 | -0.9751% | `True` | `False` | edge>=0.1, ret>=0.0%, SL<=0.6, DD<=100.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=999.0%, markov>=-999.0 |
| 23 | `best_validation_per_symbol_month` | 6 | 6 | 21 | 3.8323% | 4.9302 | -0.9751% | `True` | `False` | edge>=0.1, ret>=0.0%, SL<=0.6, DD<=100.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=4.0%, markov>=-999.0 |
| 24 | `best_validation_per_symbol_month` | 6 | 6 | 21 | 3.8323% | 4.9302 | -0.9751% | `True` | `False` | edge>=0.1, ret>=0.0%, SL<=0.4, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=999.0%, markov>=-999.0 |
| 25 | `best_validation_per_symbol_month` | 6 | 6 | 21 | 3.8323% | 4.9302 | -0.9751% | `True` | `False` | edge>=0.1, ret>=-999.0%, SL<=0.4, DD<=100.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=4.0%, markov>=-999.0 |
| 26 | `best_validation_per_symbol_month` | 6 | 6 | 21 | 3.8323% | 4.9302 | -0.9751% | `True` | `False` | edge>=0.1, ret>=0.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=999.0%, markov>=-999.0 |
| 27 | `best_validation_per_symbol_month` | 6 | 6 | 21 | 3.8323% | 4.9302 | -0.9751% | `True` | `False` | edge>=0.1, ret>=0.0%, SL<=0.6, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=4.0%, markov>=-999.0 |
| 28 | `best_validation_per_symbol_month` | 6 | 6 | 21 | 3.8323% | 4.9302 | -0.9751% | `True` | `False` | edge>=0.1, ret>=0.0%, SL<=0.4, DD<=100.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=999.0%, markov>=-999.0 |
| 29 | `best_validation_per_symbol_month` | 6 | 6 | 21 | 3.8323% | 4.9302 | -0.9751% | `True` | `False` | edge>=0.1, ret>=0.0%, SL<=0.4, DD<=100.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=4.0%, markov>=-999.0 |
| 30 | `best_validation_per_symbol_month` | 6 | 6 | 21 | 3.8323% | 4.9302 | -0.9751% | `True` | `False` | edge>=0.1, ret>=0.0%, SL<=0.4, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=4.0%, markov>=-999.0 |

## Best Non-Empty Selection

- policy=`all_sleeves`，active_rows=18，trades=85，total_realistic=5.4954%，worst_month=-2.1285%，unique_symbol_month=`True`。

| Source | Month | Symbol | TopK | Trades | Realistic | Val Return/DD | Val Markov |
|---|---|---|---:|---:|---:|---:|---:|
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-06` | `BTCUSDT` | 10 | 1 | -0.0438% | -0.5707 | 0.3775 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-06` | `ETHUSDT` | 10 | 3 | 0.1090% | 0.2813 | 0.5000 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-07` | `BTCUSDT` | 10 | 6 | -0.0658% | -2.3031 | 0.5545 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-07` | `ETHUSDT` | 10 | 5 | -2.0627% | -0.5126 | 0.4411 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-08` | `BTCUSDT` | 10 | 5 | -0.0530% | 0.1627 | 0.5232 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-08` | `ETHUSDT` | 10 | 2 | 1.7721% | -1.3020 | 0.3708 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-09` | `BTCUSDT` | 10 | 4 | -0.0476% | 0.1773 | 0.5125 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-10` | `BTCUSDT` | 10 | 3 | -0.9047% | 0.7867 | 0.3682 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-10` | `ETHUSDT` | 10 | 1 | -0.9751% | 1.5740 | 0.5039 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-11` | `BTCUSDT` | 10 | 7 | -0.0322% | -1.4884 | 0.8688 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-11` | `ETHUSDT` | 10 | 10 | 3.2810% | -1.5771 | 0.5574 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-12` | `BTCUSDT` | 10 | 9 | -0.1176% | 1.0931 | 0.4536 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-12` | `ETHUSDT` | 10 | 8 | 0.3794% | 4.8894 | 0.4559 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-01` | `BTCUSDT` | 10 | 2 | 0.1524% | 0.2968 | 0.5605 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-01` | `ETHUSDT` | 10 | 2 | 0.9074% | 0.4259 | 0.3712 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-02` | `BTCUSDT` | 10 | 5 | 2.5186% | 4.8046 | 0.4921 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-03` | `ETHUSDT` | 10 | 2 | 0.8930% | 0.2468 | 0.6702 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-04` | `BTCUSDT` | 10 | 10 | -0.2150% | -0.1413 | 0.6244 |

## Best Qualified Selection

- policy=`all_sleeves`，active_rows=6，active_months=6，trades=21，total_realistic=3.8323%，silo_PF=4.9302，unique_symbol_month=`True`，target_hit=`False`。

| Source | Month | Symbol | TopK | Trades | Realistic | Val Return/DD | Val Markov |
|---|---|---|---:|---:|---:|---:|---:|
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-06` | `ETHUSDT` | 10 | 3 | 0.1090% | 0.2813 | 0.5000 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-10` | `ETHUSDT` | 10 | 1 | -0.9751% | 1.5740 | 0.5039 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-12` | `ETHUSDT` | 10 | 8 | 0.3794% | 4.8894 | 0.4559 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-01` | `ETHUSDT` | 10 | 2 | 0.9074% | 0.4259 | 0.3712 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-02` | `BTCUSDT` | 10 | 5 | 2.5186% | 4.8046 | 0.4921 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-03` | `ETHUSDT` | 10 | 2 | 0.8930% | 0.2468 | 0.6702 |

## Interpretation

- 若最佳结果来自 `active_rows=0`，说明当前验证月字段只能选择空仓，不能证明概率模型已经具备可交易收益。
- 若非空最佳只保留单个样本，也只能作为下一轮特征/事件来源假设，不能作为实盘候选。
- Gate 只使用验证月字段；`test_edge` 只用于复盘解释，不参与筛选。
