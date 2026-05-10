# Probabilistic V6 Regime Classifier

范围：仅限 `research`。本报告只消费已生成的 `symbol_rows.csv`，用于验证 no-trade / tradeable regime classifier 是否值得进入下一轮 runner 集成。

## Summary

- rows_csv: `research/probabilistic_v6_runs/walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix/symbol_rows.csv`
- months: `2025-06, 2025-07, 2025-08, 2025-09, 2025-10, 2025-11, 2025-12, 2026-01, 2026-02, 2026-03, 2026-04`
- target_hit: `False`
- oracle_positive_per_symbol_month: `9.2568%`

| Metric | Value |
|---|---:|
| active_rows | 5 |
| active_months | 4 |
| trades | 40 |
| total_realistic_pct | 2.5444% |
| worst_month_realistic_pct | -1.1777% |
| best_month_realistic_pct | 2.5318% |

## Walk-Forward Decisions

| Execute Month | Validation Month | Model | Prob Min | Val Return | Selected | Trades | Execute Return | Worst Month |
|---|---|---|---:|---:|---:|---:|---:|---:|
| `2025-11` | `2025-10` | `logistic` | 0.45 | -0.1194% | 1 | 5 | -0.2630% | -0.2630% |
| `2025-12` | `2025-11` | `logistic` | 0.45 | -0.2630% | 0 | 0 | 0.0000% | 0.0000% |
| `2026-02` | `2026-01` | `logistic` | 0.45 | 0.7800% | 1 | 7 | 2.5318% | 2.5318% |
| `2026-03` | `2026-02` | `logistic` | 0.45 | 2.5318% | 2 | 19 | 1.4533% | 1.4533% |
| `2026-04` | `2026-03` | `svm_rbf` | 0.45 | 3.1128% | 1 | 9 | -1.1777% | -1.1777% |

## Selected Rows

| Month | Symbol | TopK | Model | Regime Prob | Trades | Realistic | Val Return/DD | Val Markov |
|---|---|---:|---|---:|---:|---:|---:|---:|
| `2025-11` | `BTCUSDT` | 5 | `gradient_boosting` | 0.7963 | 5 | -0.2630% | -0.5838 | 0.7559 |
| `2026-02` | `ETHUSDT` | 10 | `gradient_boosting` | 0.7200 | 7 | 2.5318% | 5.7820 | 0.5470 |
| `2026-03` | `BTCUSDT` | 10 | `extra_trees` | 0.9884 | 10 | -1.6595% | 23.0733 | 0.7674 |
| `2026-03` | `ETHUSDT` | 10 | `logistic` | 0.9508 | 9 | 3.1128% | 3.7095 | 0.6030 |
| `2026-04` | `ETHUSDT` | 15 | `logistic` | 0.5476 | 9 | -1.1777% | 16.7799 | 0.3693 |

## Interpretation

- 模型和阈值只用历史月份与上一个已完成 execute month 选择；当前 execute month 的收益只用于 OOS 评分。
- 若结果仍低于 `10%`，说明仅靠 symbol-row 级 validation metrics 不足，需要更上游的 regime label 或事件簇标签。
