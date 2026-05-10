# Probabilistic V5 ML Probability Sweep

范围：仅限 `research`。本文件比较多种 ML 概率模型，并输出给执行层消费的 `quality_pass` 与动态仓位。

- Split: `train<= 2025-05-31T23:59:59+00:00, validation<= 2025-06-30T23:59:59+00:00, test> 2025-06-30T23:59:59+00:00`
- Selected model: `logistic`
- Selected rule: `prob_min=0.0`, `ev_atr_min=-999.0`, `sl_prob_max=1.0`
- Markov windows: `[5, 15, 30, 60]`
- Sizing: `mode=hybrid_markov`, `min_share=0.2`, `max_share=1.5`

## Validation

| Events | Success | Net Edge ATR | Sum Net Edge ATR | Avg Prob | Avg Prob EV ATR | Avg InitialSL Prob |
|---:|---:|---:|---:|---:|---:|---:|
| 4 | 50.0000% | -0.056912 | -0.227647 | 0.519750 | -0.035266 | 0.000000 |

## Model Leaderboard

| Rank | Model | Prob Min | EV Min | SL Max | Events | Success | Net Edge ATR | Sum Net Edge ATR | Score |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | `logistic` | 0.0 | -999.0 | 1.0 | 4 | 50.0000% | -0.056912 | -0.227647 | -999.000000 |

## Metrics

| Model | Brier | Log Loss | AUC | SL Brier | SL Log Loss | SL AUC |
|---|---:|---:|---:|---:|---:|---:|
| `logistic` | 0.73445305 | 3.11893305 | 0.00000000 | 0.73445305 | 3.11893305 | 0.00000000 |
| `random_forest` | 0.25137541 | 0.69589799 | 0.50000000 | 0.25137541 | 0.69589799 | 0.50000000 |
| `extra_trees` | 0.27459061 | 0.74284336 | 0.50000000 | 0.27459061 | 0.74284336 | 0.50000000 |
| `gradient_boosting` | 0.59362947 | 1.87285444 | 0.00000000 | 0.59362233 | 1.87272918 | 0.00000000 |
| `svm_rbf` | 0.24340069 | 0.67766400 | 1.00000000 | 0.24232809 | 0.67512217 | 1.00000000 |
