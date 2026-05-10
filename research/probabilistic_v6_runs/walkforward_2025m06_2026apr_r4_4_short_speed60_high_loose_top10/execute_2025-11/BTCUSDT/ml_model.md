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
| 3 | 33.3333% | -0.133845 | -0.401534 | 0.549440 | -0.034431 | 0.000000 |

## Model Leaderboard

| Rank | Model | Prob Min | EV Min | SL Max | Events | Success | Net Edge ATR | Sum Net Edge ATR | Score |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | `logistic` | 0.0 | -999.0 | 1.0 | 3 | 33.3333% | -0.133845 | -0.401534 | -999.000000 |

## Metrics

| Model | Brier | Log Loss | AUC | SL Brier | SL Log Loss | SL AUC |
|---|---:|---:|---:|---:|---:|---:|
| `logistic` | 0.64464500 | 2.13167273 | 0.00000000 | 0.79546744 | 2.92969550 | 0.00000000 |
| `random_forest` | 0.25682688 | 0.70402376 | 1.00000000 | 0.17563323 | 0.53920727 | 1.00000000 |
| `extra_trees` | 0.24599267 | 0.68139149 | 1.00000000 | 0.20195714 | 0.59442537 | 1.00000000 |
| `gradient_boosting` | 0.45386991 | 1.74960614 | 0.50000000 | 0.01290368 | 0.09102658 | 1.00000000 |
| `svm_rbf` | 0.25899239 | 0.71121784 | 0.50000000 | 0.27180817 | 0.73685477 | 0.00000000 |
