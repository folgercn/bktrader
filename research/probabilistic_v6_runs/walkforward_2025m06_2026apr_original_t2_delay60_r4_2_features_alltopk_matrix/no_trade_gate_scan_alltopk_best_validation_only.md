# Probabilistic V6 No-Trade Gate Analyzer

范围：仅限 `research`。本报告只消费已有 `symbol_rows.csv`，扫描二级 no-trade gate；它不是新的实盘策略结论。

## Objective Diagnostics

- 目标收益：`10.00%` Active_Silo_Sum。
- 组合约束：active_rows>=4，active_months>=6，trades>=40，worst_month>=-2.00%。
- Target hit 额外要求每个 `(execute_month, symbol)` 最多选择一个 sleeve；重复 topK sleeve 只作为诊断口径。
- Baseline candidate pool：active_rows=68，trades=659，total=-57.7621%，silo_PF=0.3427。
- Oracle best positive per symbol-month：total=9.2568%，active_rows=9。这是事后上限诊断，不可当作可交易选择器。
- Target hit under validation-only gates：`False`。

## Baseline Candidates

| Source | Month | Symbol | TopK | Model | Trades | Realistic | Val Edge | Val TopK Return | Val TopK SL | Val TopK DD | Val Return/DD | Val Markov | Test Edge |
|---|---|---|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-06` | `ETHUSDT` | 5 | `logistic` | 5 | -3.3432% | 0.159464 | -3.530384% | 0.8000 | -3.529132% | -1.0004 | 0.5750 | -0.098890 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-06` | `ETHUSDT` | 10 | `logistic` | 10 | -5.7716% | 0.159464 | -1.447811% | 0.5000 | -3.804224% | -0.3806 | 0.6235 | -0.098890 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-06` | `ETHUSDT` | 15 | `logistic` | 15 | -5.7754% | 0.159464 | -0.203488% | 0.4000 | -3.804224% | -0.0535 | 0.4666 | -0.098890 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-06` | `ETHUSDT` | 20 | `logistic` | 20 | -4.7616% | 0.159464 | 1.873423% | 0.3333 | -3.529132% | 0.5308 | 0.4304 | -0.098890 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-07` | `BTCUSDT` | 5 | `gradient_boosting` | 5 | 0.0486% | 0.068543 | -0.322529% | 0.4000 | -0.838748% | -0.3845 | 0.5697 | 0.006526 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-07` | `BTCUSDT` | 10 | `gradient_boosting` | 10 | -0.2202% | 0.068543 | 0.152460% | 0.3000 | -0.838748% | 0.1818 | 0.5522 | 0.006526 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-07` | `BTCUSDT` | 15 | `gradient_boosting` | 15 | -1.0472% | 0.068543 | 0.632213% | 0.2308 | -0.838748% | 0.7538 | 0.5772 | 0.006526 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-07` | `BTCUSDT` | 20 | `gradient_boosting` | 18 | -1.2193% | 0.068543 | 0.632213% | 0.2308 | -0.838748% | 0.7538 | 0.5772 | 0.006526 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-07` | `ETHUSDT` | 5 | `svm_rbf` | 5 | -1.0587% | 0.081132 | 2.232426% | 0.2000 | -0.300162% | 7.4374 | 0.4395 | -0.094544 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-07` | `ETHUSDT` | 10 | `svm_rbf` | 9 | -2.6352% | 0.081132 | 1.182431% | 0.5556 | -1.137624% | 1.0394 | 0.4289 | -0.094544 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-07` | `ETHUSDT` | 15 | `svm_rbf` | 9 | -2.6352% | 0.081132 | 1.182431% | 0.5556 | -1.137624% | 1.0394 | 0.4289 | -0.094544 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-07` | `ETHUSDT` | 20 | `svm_rbf` | 9 | -2.6352% | 0.081132 | 1.182431% | 0.5556 | -1.137624% | 1.0394 | 0.4289 | -0.094544 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-08` | `BTCUSDT` | 5 | `gradient_boosting` | 5 | -0.2176% | 0.069479 | 0.781545% | 0.4000 | -0.447845% | 1.7451 | 0.5280 | -0.004287 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-08` | `BTCUSDT` | 10 | `gradient_boosting` | 10 | -2.9684% | 0.069479 | 2.254096% | 0.2000 | -0.447845% | 5.0332 | 0.5461 | -0.004287 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-08` | `BTCUSDT` | 15 | `gradient_boosting` | 15 | -4.3157% | 0.069479 | 2.385968% | 0.2667 | -0.615592% | 3.8759 | 0.4857 | -0.004287 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-08` | `BTCUSDT` | 20 | `gradient_boosting` | 20 | -0.7779% | 0.069479 | 1.593236% | 0.4000 | -0.801040% | 1.9890 | 0.5858 | -0.004287 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-09` | `ETHUSDT` | 5 | `logistic` | 5 | -4.3009% | 0.022097 | 0.255285% | 0.4000 | -0.837746% | 0.3047 | 0.4475 | -0.137158 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-09` | `ETHUSDT` | 10 | `logistic` | 10 | -5.9216% | 0.022097 | -2.053056% | 0.6000 | -3.079623% | -0.6667 | 0.4669 | -0.137158 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-09` | `ETHUSDT` | 15 | `logistic` | 15 | -6.5317% | 0.022097 | -1.172828% | 0.4667 | -3.474475% | -0.3376 | 0.5212 | -0.137158 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-09` | `ETHUSDT` | 20 | `logistic` | 20 | -7.4177% | 0.022097 | -0.232119% | 0.4375 | -3.230845% | -0.0718 | 0.5091 | -0.137158 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-10` | `BTCUSDT` | 5 | `logistic` | 5 | -0.1194% | 0.044263 | -0.224574% | 0.4000 | -0.600911% | -0.3737 | 0.5944 | 0.049168 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-10` | `BTCUSDT` | 10 | `logistic` | 8 | -0.8476% | 0.044263 | 0.710537% | 0.2000 | -0.600911% | 1.1824 | 0.4808 | 0.049168 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-10` | `BTCUSDT` | 15 | `logistic` | 8 | -0.8476% | 0.044263 | 0.496732% | 0.2667 | -1.031898% | 0.4814 | 0.4480 | 0.049168 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-10` | `BTCUSDT` | 20 | `logistic` | 8 | -0.8476% | 0.044263 | 0.730017% | 0.2500 | -1.031898% | 0.7075 | 0.4200 | 0.049168 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-11` | `BTCUSDT` | 5 | `gradient_boosting` | 5 | -0.2630% | 0.050029 | -0.399014% | 0.4000 | -0.683435% | -0.5838 | 0.7559 | -0.012288 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-11` | `BTCUSDT` | 10 | `gradient_boosting` | 10 | -1.9905% | 0.050029 | -0.485424% | 0.3000 | -0.971972% | -0.4994 | 0.5336 | -0.012288 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-11` | `BTCUSDT` | 15 | `gradient_boosting` | 14 | -0.5476% | 0.050029 | -0.176205% | 0.3333 | -0.971972% | -0.1813 | 0.5176 | -0.012288 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-11` | `BTCUSDT` | 20 | `gradient_boosting` | 19 | -4.0284% | 0.050029 | 0.136404% | 0.3000 | -1.162424% | 0.1173 | 0.5007 | -0.012288 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-12` | `BTCUSDT` | 5 | `extra_trees` | 5 | -0.5354% | 0.048671 | 1.555662% | 0.0000 | 0.000000% | 6.2226 | 0.7421 | 0.057927 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-12` | `BTCUSDT` | 10 | `extra_trees` | 10 | 0.0141% | 0.048671 | 0.629313% | 0.3750 | -0.926349% | 0.6793 | 0.6506 | 0.057927 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-12` | `BTCUSDT` | 15 | `extra_trees` | 10 | 0.0141% | 0.048671 | 0.629313% | 0.3750 | -0.926349% | 0.6793 | 0.6506 | 0.057927 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-12` | `BTCUSDT` | 20 | `extra_trees` | 10 | 0.0141% | 0.048671 | 0.629313% | 0.3750 | -0.926349% | 0.6793 | 0.6506 | 0.057927 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-12` | `ETHUSDT` | 5 | `logistic` | 5 | 0.4160% | 0.173354 | 0.802234% | 0.4000 | -0.704459% | 1.1388 | 0.7997 | 0.049271 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-12` | `ETHUSDT` | 10 | `logistic` | 10 | -1.5565% | 0.173354 | 1.535009% | 0.4000 | -1.735896% | 0.8843 | 0.7026 | 0.049271 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-12` | `ETHUSDT` | 15 | `logistic` | 15 | 0.0714% | 0.173354 | 2.244855% | 0.4000 | -2.089412% | 1.0744 | 0.6149 | 0.049271 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-12` | `ETHUSDT` | 20 | `logistic` | 18 | -0.5625% | 0.173354 | 3.424843% | 0.3000 | -2.089412% | 1.6391 | 0.5843 | 0.049271 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-01` | `BTCUSDT` | 5 | `random_forest` | 5 | 0.0016% | 0.159479 | 0.093482% | 0.2000 | -0.433521% | 0.2156 | 0.3799 | 0.092492 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-01` | `BTCUSDT` | 10 | `random_forest` | 7 | -0.1373% | 0.159479 | 0.992332% | 0.2000 | -0.450932% | 2.2006 | 0.4788 | 0.092492 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-01` | `BTCUSDT` | 15 | `random_forest` | 7 | -0.1373% | 0.159479 | 1.375587% | 0.1818 | -0.450932% | 3.0505 | 0.5193 | 0.092492 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-01` | `BTCUSDT` | 20 | `random_forest` | 7 | -0.1373% | 0.159479 | 1.375587% | 0.1818 | -0.450932% | 3.0505 | 0.5193 | 0.092492 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-01` | `ETHUSDT` | 5 | `logistic` | 5 | 0.7800% | 0.096916 | 0.784646% | 0.4000 | -0.749706% | 1.0466 | 0.5521 | 0.134196 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-01` | `ETHUSDT` | 10 | `logistic` | 5 | 0.7800% | 0.096916 | 2.072337% | 0.3000 | -0.950131% | 2.1811 | 0.5794 | 0.134196 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-01` | `ETHUSDT` | 15 | `logistic` | 5 | 0.7800% | 0.096916 | 0.695924% | 0.4000 | -1.516089% | 0.4590 | 0.5822 | 0.134196 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-01` | `ETHUSDT` | 20 | `logistic` | 5 | 0.7800% | 0.096916 | 2.147738% | 0.3000 | -1.061324% | 2.0236 | 0.6154 | 0.134196 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-02` | `BTCUSDT` | 5 | `gradient_boosting` | 5 | -0.1366% | 0.112803 | 0.374547% | 0.4000 | -0.597020% | 0.6274 | 0.5129 | 0.024061 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-02` | `BTCUSDT` | 10 | `gradient_boosting` | 10 | 0.0690% | 0.112803 | 0.872778% | 0.3000 | -0.597020% | 1.4619 | 0.4717 | 0.024061 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-02` | `BTCUSDT` | 15 | `gradient_boosting` | 15 | 1.9593% | 0.112803 | 1.380958% | 0.2667 | -0.855466% | 1.6143 | 0.4076 | 0.024061 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-02` | `BTCUSDT` | 20 | `gradient_boosting` | 16 | 2.3050% | 0.112803 | 1.549543% | 0.2632 | -0.813557% | 1.9047 | 0.3912 | 0.024061 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-02` | `ETHUSDT` | 5 | `gradient_boosting` | 5 | 2.0056% | 0.107894 | 1.323379% | 0.2000 | 0.000000% | 5.2935 | 0.6934 | 0.131039 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-02` | `ETHUSDT` | 10 | `gradient_boosting` | 7 | 2.5318% | 0.107894 | 1.445494% | 0.3333 | -0.174598% | 5.7820 | 0.5470 | 0.131039 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-02` | `ETHUSDT` | 15 | `gradient_boosting` | 7 | 2.5318% | 0.107894 | 1.445494% | 0.3333 | -0.174598% | 5.7820 | 0.5470 | 0.131039 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-02` | `ETHUSDT` | 20 | `gradient_boosting` | 7 | 2.5318% | 0.107894 | 1.445494% | 0.3333 | -0.174598% | 5.7820 | 0.5470 | 0.131039 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-03` | `BTCUSDT` | 5 | `extra_trees` | 5 | -1.0037% | 0.400864 | 3.399306% | 0.0000 | 0.000000% | 13.5972 | 0.8101 | -0.055626 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-03` | `BTCUSDT` | 10 | `extra_trees` | 10 | -1.6595% | 0.400864 | 5.768322% | 0.1000 | -0.157643% | 23.0733 | 0.7674 | -0.055626 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-03` | `BTCUSDT` | 15 | `extra_trees` | 11 | -1.5582% | 0.400864 | 5.692427% | 0.2143 | -0.618548% | 9.2029 | 0.6367 | -0.055626 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-03` | `BTCUSDT` | 20 | `extra_trees` | 11 | -1.5582% | 0.400864 | 5.692427% | 0.2143 | -0.618548% | 9.2029 | 0.6367 | -0.055626 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-03` | `ETHUSDT` | 5 | `logistic` | 5 | 3.0942% | 0.371458 | 3.628357% | 0.2000 | -0.679087% | 5.3430 | 0.5827 | 0.093466 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-03` | `ETHUSDT` | 10 | `logistic` | 9 | 3.1128% | 0.371458 | 3.735039% | 0.2222 | -1.006872% | 3.7095 | 0.6030 | 0.093466 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-03` | `ETHUSDT` | 15 | `logistic` | 9 | 3.1128% | 0.371458 | 3.735039% | 0.2222 | -1.006872% | 3.7095 | 0.6030 | 0.093466 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-03` | `ETHUSDT` | 20 | `logistic` | 9 | 3.1128% | 0.371458 | 3.735039% | 0.2222 | -1.006872% | 3.7095 | 0.6030 | 0.093466 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-04` | `BTCUSDT` | 5 | `extra_trees` | 5 | 0.0469% | 0.060713 | -0.741541% | 0.4000 | -1.695867% | -0.4373 | 0.8077 | -0.063912 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-04` | `BTCUSDT` | 10 | `extra_trees` | 10 | -0.1299% | 0.060713 | -1.048754% | 0.5000 | -2.160304% | -0.4855 | 0.7157 | -0.063912 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-04` | `BTCUSDT` | 15 | `extra_trees` | 15 | -0.3811% | 0.060713 | -0.095308% | 0.4000 | -2.297602% | -0.0415 | 0.5767 | -0.063912 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-04` | `BTCUSDT` | 20 | `extra_trees` | 20 | -0.4958% | 0.060713 | -0.095308% | 0.4000 | -2.297602% | -0.0415 | 0.5767 | -0.063912 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-04` | `ETHUSDT` | 5 | `logistic` | 5 | -1.3074% | 0.578788 | 2.900902% | 0.0000 | 0.000000% | 11.6036 | 0.4307 | -0.087715 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-04` | `ETHUSDT` | 10 | `logistic` | 9 | -1.1777% | 0.578788 | 4.194972% | 0.0000 | 0.000000% | 16.7799 | 0.3693 | -0.087715 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-04` | `ETHUSDT` | 15 | `logistic` | 9 | -1.1777% | 0.578788 | 4.194972% | 0.0000 | 0.000000% | 16.7799 | 0.3693 | -0.087715 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-04` | `ETHUSDT` | 20 | `logistic` | 9 | -1.1777% | 0.578788 | 4.194972% | 0.0000 | 0.000000% | 16.7799 | 0.3693 | -0.087715 |

## Top Gate Sweeps

| Rank | Policy | Active | Months | Trades | Total Realistic | Silo PF | Worst Month | Unique | Target | Gate |
|---:|---|---:|---:|---:|---:|---:|---:|---|---|---|
| 1 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=0.6, DD<=1.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=7.0%, markov>=0.0 |
| 2 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=0.6, DD<=1.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=7.0%, markov>=0.3 |
| 3 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=0.6, DD<=1.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=10.0%, markov>=0.0 |
| 4 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=0.6, DD<=1.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=999999.0%, markov>=0.0 |
| 5 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=0.0, ret/DD<=7.0, ret<=999999.0%, markov>=0.3 |
| 6 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=0.6, DD<=1.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=10.0%, markov>=0.3 |
| 7 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=0.0, ret/DD<=7.0, ret<=5.0%, markov>=0.3 |
| 8 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=0.0, ret/DD<=7.0, ret<=10.0%, markov>=0.0 |
| 9 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=0.0, ret/DD<=7.0, ret<=5.0%, markov>=0.0 |
| 10 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=7.0%, markov>=0.3 |
| 11 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=0.5, ret/DD<=7.0, ret<=10.0%, markov>=0.3 |
| 12 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=0.6, DD<=1.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=999999.0%, markov>=0.3 |
| 13 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=0.0, ret/DD<=7.0, ret<=7.0%, markov>=0.3 |
| 14 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=0.5, ret/DD<=7.0, ret<=10.0%, markov>=0.0 |
| 15 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=5.0%, markov>=0.0 |
| 16 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=0.5, ret/DD<=7.0, ret<=5.0%, markov>=0.3 |
| 17 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=0.5, ret/DD<=7.0, ret<=5.0%, markov>=0.0 |
| 18 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=0.5, ret/DD<=7.0, ret<=999999.0%, markov>=0.0 |
| 19 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=0.5, ret/DD<=7.0, ret<=7.0%, markov>=0.0 |
| 20 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=10.0%, markov>=0.0 |
| 21 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=999999.0%, markov>=0.3 |
| 22 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=5.0%, markov>=0.3 |
| 23 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=10.0%, markov>=0.3 |
| 24 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=0.0, ret/DD<=7.0, ret<=999999.0%, markov>=0.0 |
| 25 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=0.0, ret/DD<=7.0, ret<=7.0%, markov>=0.0 |
| 26 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=0.0, ret/DD<=7.0, ret<=10.0%, markov>=0.3 |
| 27 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=0.5, ret/DD<=7.0, ret<=7.0%, markov>=0.3 |
| 28 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=999999.0%, markov>=0.0 |
| 29 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=0.5, ret/DD<=7.0, ret<=999999.0%, markov>=0.3 |
| 30 | `best_validation_per_symbol_month` | 6 | 4 | 45 | 8.9897% | 66.4749 | 0.4160% | `True` | `False` | edge>=0.08, ret>=0.8%, SL<=1.0, DD<=1.0%, ret/DD>=1.0, ret/DD<=7.0, ret<=7.0%, markov>=0.0 |

## Best Non-Empty Selection

- policy=`best_validation_per_symbol_month`，active_rows=6，trades=45，total_realistic=8.9897%，worst_month=0.4160%，unique_symbol_month=`True`。

| Source | Month | Symbol | TopK | Trades | Realistic | Val Return/DD | Val Markov |
|---|---|---|---:|---:|---:|---:|---:|
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-12` | `ETHUSDT` | 5 | 5 | 0.4160% | 1.1388 | 0.7997 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-01` | `BTCUSDT` | 15 | 7 | -0.1373% | 3.0505 | 0.5193 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-01` | `ETHUSDT` | 10 | 5 | 0.7800% | 2.1811 | 0.5794 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-02` | `BTCUSDT` | 20 | 16 | 2.3050% | 1.9047 | 0.3912 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-02` | `ETHUSDT` | 10 | 7 | 2.5318% | 5.7820 | 0.5470 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-03` | `ETHUSDT` | 5 | 5 | 3.0942% | 5.3430 | 0.5827 |

## Best Qualified Selection

- policy=`best_validation_per_symbol_month`，active_rows=9，active_months=6，trades=64，total_realistic=5.7496%，silo_PF=2.7024，unique_symbol_month=`True`，target_hit=`False`。

| Source | Month | Symbol | TopK | Trades | Realistic | Val Return/DD | Val Markov |
|---|---|---|---:|---:|---:|---:|---:|
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-07` | `ETHUSDT` | 5 | 5 | -1.0587% | 7.4374 | 0.4395 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2025-12` | `ETHUSDT` | 5 | 5 | 0.4160% | 1.1388 | 0.7997 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-01` | `BTCUSDT` | 15 | 7 | -0.1373% | 3.0505 | 0.5193 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-01` | `ETHUSDT` | 10 | 5 | 0.7800% | 2.1811 | 0.5794 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-02` | `BTCUSDT` | 20 | 16 | 2.3050% | 1.9047 | 0.3912 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-02` | `ETHUSDT` | 10 | 7 | 2.5318% | 5.7820 | 0.5470 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-03` | `BTCUSDT` | 5 | 5 | -1.0037% | 13.5972 | 0.8101 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-03` | `ETHUSDT` | 5 | 5 | 3.0942% | 5.3430 | 0.5827 |
| `walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix` | `2026-04` | `ETHUSDT` | 10 | 9 | -1.1777% | 16.7799 | 0.3693 |

## Interpretation

- 若最佳结果来自 `active_rows=0`，说明当前验证月字段只能选择空仓，不能证明概率模型已经具备可交易收益。
- 若非空最佳只保留单个样本，也只能作为下一轮特征/事件来源假设，不能作为实盘候选。
- Gate 只使用验证月字段；`test_edge` 只用于复盘解释，不参与筛选。
