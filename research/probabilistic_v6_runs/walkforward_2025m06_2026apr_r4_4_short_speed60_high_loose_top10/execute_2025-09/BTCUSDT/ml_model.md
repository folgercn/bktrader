# Probabilistic V5 ML Probability Sweep

范围：仅限 `research`。本文件比较多种 ML 概率模型，并输出给执行层消费的 `quality_pass` 与动态仓位。

- Split: `train<= 2025-07-31T23:59:59+00:00, validation<= 2025-08-31T23:59:59+00:00, test> 2025-08-31T23:59:59+00:00`
- Selected model: `logistic`
- Selected rule: `prob_min=0.0`, `ev_atr_min=-999.0`, `sl_prob_max=1.0`
- Markov windows: `[5, 15, 30, 60]`
- Sizing: `mode=hybrid_markov`, `min_share=0.2`, `max_share=1.5`

## Validation

| Events | Success | Net Edge ATR | Sum Net Edge ATR | Avg Prob | Avg Prob EV ATR | Avg InitialSL Prob |
|---:|---:|---:|---:|---:|---:|---:|
| 5 | 60.0000% | 0.007047 | 0.035233 | 0.561810 | -0.014949 | 0.000000 |

## Model Leaderboard

| Rank | Model | Prob Min | EV Min | SL Max | Events | Success | Net Edge ATR | Sum Net Edge ATR | Score |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | `logistic` | 0.0 | -999.0 | 1.0 | 5 | 60.0000% | 0.007047 | 0.035233 | -999.000000 |

## Metrics

| Model | Brier | Log Loss | AUC | SL Brier | SL Log Loss | SL AUC |
|---|---:|---:|---:|---:|---:|---:|
| `logistic` | 0.26106046 | 0.72394866 | 0.66666667 | 0.26656020 | 0.79109029 | 0.50000000 |
| `random_forest` | 0.24475611 | 0.68272694 | 0.50000000 | 0.23813572 | 0.66928230 | 0.66666667 |
| `extra_trees` | 0.25637053 | 0.70565381 | 0.50000000 | 0.25462254 | 0.70230377 | 0.66666667 |
| `gradient_boosting` | 0.38474112 | 1.05731044 | 0.66666667 | 0.25022386 | 0.66762812 | 0.66666667 |
| `svm_rbf` | 0.23579078 | 0.66310912 | 0.66666667 | 0.27483673 | 0.75058363 | 0.33333333 |
