# Probabilistic V6 Regime Classifier

范围：仅限 `research`。本报告只消费已生成的 `symbol_rows.csv`，用于验证 no-trade / tradeable regime classifier 是否值得进入下一轮 runner 集成。

## Summary

- rows_csv: `research/probabilistic_v6_runs/walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix/symbol_rows.csv`
- months: `2025-07, 2025-08, 2025-09, 2025-10, 2025-11, 2025-12, 2026-01, 2026-02, 2026-03, 2026-04`
- target_hit: `False`
- oracle_positive_per_symbol_month: `10.4570%`

| Metric | Value |
|---|---:|
| active_rows | 2 |
| active_months | 2 |
| trades | 14 |
| total_realistic_pct | -2.0012% |
| worst_month_realistic_pct | -1.0482% |
| best_month_realistic_pct | -0.953% |

## Walk-Forward Decisions

| Execute Month | Validation Month | Model | Prob Min | Val Return | Selected | Trades | Execute Return | Worst Month |
|---|---|---|---:|---:|---:|---:|---:|---:|
| `2026-03` | `2026-02` | `gradient_boosting` | 0.45 | 5.7398% | 1 | 5 | -0.9530% | -0.9530% |
| `2026-04` | `2026-03` | `logistic` | 0.45 | 2.7174% | 1 | 9 | -1.0482% | -1.0482% |

## Selected Rows

| Month | Symbol | TopK | Model | Regime Prob | Trades | Realistic | Val Return/DD | Val Markov |
|---|---|---:|---|---:|---:|---:|---:|---:|
| `2026-03` | `BTCUSDT` | 5 | `extra_trees` | 0.7447 | 5 | -0.9530% | 13.3218 | 0.7918 |
| `2026-04` | `ETHUSDT` | 10 | `logistic` | 0.6668 | 9 | -1.0482% | 6.5887 | 0.4424 |

## Interpretation

- 模型和阈值只用历史月份与上一个已完成 execute month 选择；当前 execute month 的收益只用于 OOS 评分。
- 若结果仍低于 `10%`，说明仅靠 symbol-row 级 validation metrics 不足，需要更上游的 regime label 或事件簇标签。
