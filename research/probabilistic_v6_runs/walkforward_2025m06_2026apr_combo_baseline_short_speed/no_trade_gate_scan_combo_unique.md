# Probabilistic V6 No-Trade Gate Analyzer

范围：仅限 `research`。本报告只消费已有 `symbol_rows.csv`，扫描二级 no-trade gate；它不是新的实盘策略结论。

## Objective Diagnostics

- 目标收益：`10.00%` Active_Silo_Sum。
- 组合约束：active_rows>=4，active_months>=4，trades>=20，worst_month>=-2.50%。
- Target hit 额外要求每个 `(execute_month, symbol)` 最多选择一个 sleeve；重复 topK sleeve 只作为诊断口径。
- Baseline candidate pool：active_rows=38，trades=255，total=11.9132%，silo_PF=1.8085。
- Oracle best positive per symbol-month：total=17.5790%，active_rows=12。这是事后上限诊断，不可当作可交易选择器。
- Target hit under validation-only gates：`False`。

## Baseline Candidates

| Source | Month | Symbol | TopK | Model | Trades | Realistic | Val Edge | Val TopK Return | Val TopK SL | Val TopK DD | Val Return/DD | Val Markov | Test Edge |
|---|---|---|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `walkforward_delay60_original_t2_feature60_postselect_gate_btc_fallback` | `2025-11` | `BTCUSDT` | 15 | `gradient_boosting` | 6 | 0.4103% | 0.089323 | 0.784172% | 0.1818 | -0.563612% | 1.3913 | 0.4618 | 0.104448 |
| `walkforward_delay60_original_t2_feature60_postselect_gate_btc_fallback` | `2025-12` | `BTCUSDT` | 20 | `logistic` | 11 | -0.7900% | 0.106156 | 1.494166% | 0.2500 | -0.615120% | 2.4291 | 0.2641 | -0.005785 |
| `walkforward_delay60_original_t2_feature60_postselect_gate_btc_fallback` | `2026-01` | `BTCUSDT` | 20 | `extra_trees` | 16 | 0.3375% | 0.123106 | 1.783297% | 0.2000 | -0.746622% | 2.3885 | 0.6260 | 0.126375 |
| `walkforward_delay60_original_t2_feature60_postselect_gate_btc_fallback` | `2026-02` | `BTCUSDT` | 10 | `random_forest` | 10 | 3.0090% | 0.064319 | 1.049322% | 0.2000 | -0.463344% | 2.2647 | 0.6522 | 0.146692 |
| `walkforward_delay60_original_t2_feature60_postselect_gate_btc_fallback` | `2026-03` | `ETHUSDT` | 10 | `logistic` | 8 | 3.1271% | 0.592445 | 5.142537% | 0.2222 | -0.576902% | 8.9141 | 0.4455 | 0.372527 |
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
| 1 | `best_validation_per_symbol_month` | 19 | 11 | 108 | 7.9446% | 2.3469 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=6.0%, markov>=-999.0 |
| 2 | `best_validation_per_symbol_month` | 19 | 11 | 108 | 7.9446% | 2.3469 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=3.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=999.0%, markov>=-999.0 |
| 3 | `best_validation_per_symbol_month` | 19 | 11 | 108 | 7.9446% | 2.3469 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=4.0%, markov>=-999.0 |
| 4 | `best_validation_per_symbol_month` | 19 | 11 | 108 | 7.9446% | 2.3469 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=3.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=6.0%, markov>=-999.0 |
| 5 | `best_validation_per_symbol_month` | 19 | 11 | 108 | 7.9446% | 2.3469 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=3.0%, markov>=-999.0 |
| 6 | `best_validation_per_symbol_month` | 19 | 11 | 108 | 7.9446% | 2.3469 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=3.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=4.0%, markov>=-999.0 |
| 7 | `best_validation_per_symbol_month` | 19 | 11 | 108 | 7.9446% | 2.3469 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=999.0%, markov>=-999.0 |
| 8 | `best_validation_per_symbol_month` | 19 | 11 | 108 | 7.9446% | 2.3469 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=3.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=3.0%, markov>=-999.0 |
| 9 | `best_validation_per_symbol_month` | 18 | 11 | 100 | 7.5652% | 2.2826 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=3.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=2.0%, markov>=-999.0 |
| 10 | `best_validation_per_symbol_month` | 18 | 11 | 100 | 7.5652% | 2.2826 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=2.0%, markov>=-999.0 |
| 11 | `best_validation_per_symbol_month` | 18 | 10 | 106 | 7.0516% | 2.1955 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=2.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=6.0%, markov>=-999.0 |
| 12 | `best_validation_per_symbol_month` | 18 | 10 | 106 | 7.0516% | 2.1955 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=2.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=999.0%, markov>=-999.0 |
| 13 | `best_validation_per_symbol_month` | 18 | 10 | 106 | 7.0516% | 2.1955 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=2.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=4.0%, markov>=-999.0 |
| 14 | `best_validation_per_symbol_month` | 18 | 10 | 106 | 7.0516% | 2.1955 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=2.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=3.0%, markov>=-999.0 |
| 15 | `best_validation_per_symbol_month` | 15 | 11 | 98 | 6.8860% | 2.6098 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=3.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=3.0%, markov>=0.4 |
| 16 | `best_validation_per_symbol_month` | 15 | 11 | 98 | 6.8860% | 2.6098 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=999.0%, markov>=0.4 |
| 17 | `best_validation_per_symbol_month` | 15 | 11 | 98 | 6.8860% | 2.6098 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=3.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=4.0%, markov>=0.4 |
| 18 | `best_validation_per_symbol_month` | 15 | 11 | 98 | 6.8860% | 2.6098 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=4.0%, markov>=0.4 |
| 19 | `best_validation_per_symbol_month` | 15 | 11 | 98 | 6.8860% | 2.6098 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=3.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=6.0%, markov>=0.4 |
| 20 | `best_validation_per_symbol_month` | 15 | 11 | 98 | 6.8860% | 2.6098 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=3.0%, markov>=0.4 |
| 21 | `best_validation_per_symbol_month` | 15 | 11 | 98 | 6.8860% | 2.6098 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=3.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=999.0%, markov>=0.4 |
| 22 | `best_validation_per_symbol_month` | 15 | 11 | 98 | 6.8860% | 2.6098 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=6.0%, markov>=0.4 |
| 23 | `best_validation_per_symbol_month` | 17 | 10 | 98 | 6.6722% | 2.1312 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=2.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=2.0%, markov>=-999.0 |
| 24 | `best_validation_per_symbol_month` | 14 | 11 | 90 | 6.5066% | 2.5211 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=3.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=2.0%, markov>=0.4 |
| 25 | `best_validation_per_symbol_month` | 14 | 11 | 90 | 6.5066% | 2.5211 | -2.8693% | `True` | `False` | edge>=-999.0, ret>=-999.0%, SL<=1.0, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=2.0%, markov>=0.4 |
| 26 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.6, DD<=100.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=3.0%, markov>=-999.0 |
| 27 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.6, DD<=3.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=999.0%, markov>=-999.0 |
| 28 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.6, DD<=3.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=4.0%, markov>=-999.0 |
| 29 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.6, DD<=3.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=4.0%, markov>=-999.0 |
| 30 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.6, DD<=3.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=6.0%, markov>=-999.0 |
| 31 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.4, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=999.0%, markov>=-999.0 |
| 32 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.4, DD<=3.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=999.0%, markov>=-999.0 |
| 33 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.4, DD<=100.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=6.0%, markov>=-999.0 |
| 34 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.4, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=3.0%, markov>=-999.0 |
| 35 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.6, DD<=3.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=3.0%, markov>=-999.0 |
| 36 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.6, DD<=3.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=999.0%, markov>=-999.0 |
| 37 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.4, DD<=3.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=3.0%, markov>=-999.0 |
| 38 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.4, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=6.0%, markov>=-999.0 |
| 39 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.4, DD<=100.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=999.0%, markov>=-999.0 |
| 40 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.4, DD<=100.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=4.0%, markov>=-999.0 |
| 41 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.6, DD<=3.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=3.0%, markov>=-999.0 |
| 42 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.6, DD<=3.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=6.0%, markov>=-999.0 |
| 43 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.4, DD<=3.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=3.0%, markov>=-999.0 |
| 44 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.4, DD<=3.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=4.0%, markov>=-999.0 |
| 45 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.4, DD<=100.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=3.0%, markov>=-999.0 |
| 46 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.4, DD<=100.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=4.0%, markov>=-999.0 |
| 47 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.4, DD<=3.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=4.0%, markov>=-999.0 |
| 48 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.4, DD<=3.0%, ret/DD>=0.0, ret/DD<=5.0, ret<=6.0%, markov>=-999.0 |
| 49 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.4, DD<=3.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=6.0%, markov>=-999.0 |
| 50 | `best_validation_per_symbol_month` | 11 | 8 | 68 | 6.2999% | 3.5300 | -0.9751% | `True` | `False` | edge>=0.05, ret>=0.3%, SL<=0.4, DD<=3.0%, ret/DD>=-999.0, ret/DD<=5.0, ret<=999.0%, markov>=-999.0 |

## Best Non-Empty Selection

- policy=`best_validation_per_symbol_month`，active_rows=19，trades=108，total_realistic=7.9446%，worst_month=-2.8693%，unique_symbol_month=`True`。

| Source | Month | Symbol | TopK | Trades | Realistic | Val Return/DD | Val Markov |
|---|---|---|---:|---:|---:|---:|---:|
| `walkforward_delay60_original_t2_feature60_postselect_gate_btc_fallback` | `2025-11` | `BTCUSDT` | 15 | 6 | 0.4103% | 1.3913 | 0.4618 |
| `walkforward_delay60_original_t2_feature60_postselect_gate_btc_fallback` | `2025-12` | `BTCUSDT` | 20 | 11 | -0.7900% | 2.4291 | 0.2641 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2026-01` | `BTCUSDT` | 10 | 7 | 0.2418% | 2.9760 | 0.4001 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-02` | `BTCUSDT` | 10 | 5 | 2.5186% | 4.8046 | 0.4921 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2025-07` | `BTCUSDT` | 20 | 18 | -0.7250% | 4.0062 | 0.5651 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2025-07` | `ETHUSDT` | 5 | 5 | -2.1443% | 1.3644 | 0.4219 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-12` | `ETHUSDT` | 10 | 8 | 0.3794% | 4.8894 | 0.4559 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2026-02` | `ETHUSDT` | 5 | 5 | 3.3305% | 4.1525 | 0.4280 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-06` | `BTCUSDT` | 10 | 1 | -0.0438% | -0.5707 | 0.3775 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-06` | `ETHUSDT` | 10 | 3 | 0.1090% | 0.2813 | 0.5000 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-08` | `BTCUSDT` | 10 | 5 | -0.0530% | 0.1627 | 0.5232 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-08` | `ETHUSDT` | 10 | 2 | 1.7721% | -1.3020 | 0.3708 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-09` | `BTCUSDT` | 10 | 4 | -0.0476% | 0.1773 | 0.5125 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-10` | `BTCUSDT` | 10 | 3 | -0.9047% | 0.7867 | 0.3682 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-10` | `ETHUSDT` | 10 | 1 | -0.9751% | 1.5740 | 0.5039 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-11` | `ETHUSDT` | 10 | 10 | 3.2810% | -1.5771 | 0.5574 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-01` | `ETHUSDT` | 10 | 2 | 0.9074% | 0.4259 | 0.3712 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-03` | `ETHUSDT` | 10 | 2 | 0.8930% | 0.2468 | 0.6702 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-04` | `BTCUSDT` | 10 | 10 | -0.2150% | -0.1413 | 0.6244 |

## Best Qualified Selection

- policy=`best_validation_per_symbol_month`，active_rows=11，active_months=8，trades=68，total_realistic=6.2999%，silo_PF=3.5300，unique_symbol_month=`True`，target_hit=`False`。

| Source | Month | Symbol | TopK | Trades | Realistic | Val Return/DD | Val Markov |
|---|---|---|---:|---:|---:|---:|---:|
| `walkforward_delay60_original_t2_feature60_postselect_gate_btc_fallback` | `2025-11` | `BTCUSDT` | 15 | 6 | 0.4103% | 1.3913 | 0.4618 |
| `walkforward_delay60_original_t2_feature60_postselect_gate_btc_fallback` | `2025-12` | `BTCUSDT` | 20 | 11 | -0.7900% | 2.4291 | 0.2641 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2026-01` | `BTCUSDT` | 10 | 7 | 0.2418% | 2.9760 | 0.4001 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-02` | `BTCUSDT` | 10 | 5 | 2.5186% | 4.8046 | 0.4921 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2025-07` | `BTCUSDT` | 20 | 18 | -0.7250% | 4.0062 | 0.5651 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-12` | `ETHUSDT` | 10 | 8 | 0.3794% | 4.8894 | 0.4559 |
| `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback` | `2026-02` | `ETHUSDT` | 5 | 5 | 3.3305% | 4.1525 | 0.4280 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-06` | `ETHUSDT` | 10 | 3 | 0.1090% | 0.2813 | 0.5000 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2025-10` | `ETHUSDT` | 10 | 1 | -0.9751% | 1.5740 | 0.5039 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-01` | `ETHUSDT` | 10 | 2 | 0.9074% | 0.4259 | 0.3712 |
| `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10` | `2026-03` | `ETHUSDT` | 10 | 2 | 0.8930% | 0.2468 | 0.6702 |

## Interpretation

- 若最佳结果来自 `active_rows=0`，说明当前验证月字段只能选择空仓，不能证明概率模型已经具备可交易收益。
- 若非空最佳只保留单个样本，也只能作为下一轮特征/事件来源假设，不能作为实盘候选。
- Gate 只使用验证月字段；`test_edge` 只用于复盘解释，不参与筛选。
