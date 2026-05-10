# Probabilistic V5 ML Probability Sweep

范围：仅限 `research`。本文件比较多种 ML 概率模型，并输出给执行层消费的 `quality_pass` 与动态仓位。

- Split: `train<= 2025-09-30T23:59:59+00:00, validation<= 2025-10-31T23:59:59+00:00, test> 2025-10-31T23:59:59+00:00`
- Selected model: `logistic`
- Selected rule: `prob_min=0.0`, `ev_atr_min=-999.0`, `sl_prob_max=1.0`
- Markov windows: `[5, 15, 30, 60]`
- Sizing: `mode=hybrid_markov`, `min_share=0.2`, `max_share=1.5`

## Validation

| Events | Success | Net Edge ATR | Sum Net Edge ATR | Avg Prob | Avg Prob EV ATR | Avg InitialSL Prob |
|---:|---:|---:|---:|---:|---:|---:|
| 3 | 66.6667% | 0.181894 | 0.545682 | 0.404867 | -0.184412 | 0.000000 |

## Model Leaderboard

| Rank | Model | Prob Min | EV Min | SL Max | Events | Success | Net Edge ATR | Sum Net Edge ATR | Score |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | `logistic` | 0.0 | -999.0 | 1.0 | 3 | 66.6667% | 0.181894 | 0.545682 | -999.000000 |

## Metrics

| Model | Brier | Log Loss | AUC | SL Brier | SL Log Loss | SL AUC |
|---|---:|---:|---:|---:|---:|---:|
| `logistic` | 0.20149133 | 0.55916415 | 1.00000000 | 0.20149133 | 0.55916415 | 1.00000000 |
| `random_forest` | 0.28296303 | 0.75961625 | 0.00000000 | 0.28296303 | 0.75961625 | 0.00000000 |
| `extra_trees` | 0.25636871 | 0.70555681 | 0.50000000 | 0.25636871 | 0.70555681 | 0.50000000 |
| `gradient_boosting` | 0.47114204 | 1.24864913 | 0.00000000 | 0.47114204 | 1.24864913 | 0.00000000 |
| `svm_rbf` | 0.22727895 | 0.64725913 | 1.00000000 | 0.21720111 | 0.62633239 | 1.00000000 |
