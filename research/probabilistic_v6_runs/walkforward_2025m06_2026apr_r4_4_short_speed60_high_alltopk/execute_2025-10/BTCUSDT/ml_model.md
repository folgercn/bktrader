# Probabilistic V5 ML Probability Sweep

范围：仅限 `research`。本文件比较多种 ML 概率模型，并输出给执行层消费的 `quality_pass` 与动态仓位。

- Split: `train<= 2025-08-31T23:59:59+00:00, validation<= 2025-09-30T23:59:59+00:00, test> 2025-09-30T23:59:59+00:00`
- Selected model: `logistic`
- Selected rule: `prob_min=0.0`, `ev_atr_min=-999.0`, `sl_prob_max=1.0`
- Markov windows: `[5, 15, 30, 60]`
- Sizing: `mode=hybrid_markov`, `min_share=0.2`, `max_share=1.5`

## Validation

| Events | Success | Net Edge ATR | Sum Net Edge ATR | Avg Prob | Avg Prob EV ATR | Avg InitialSL Prob |
|---:|---:|---:|---:|---:|---:|---:|
| 4 | 75.0000% | 0.000610 | 0.002438 | 0.348377 | -0.147397 | 0.000000 |

## Model Leaderboard

| Rank | Model | Prob Min | EV Min | SL Max | Events | Success | Net Edge ATR | Sum Net Edge ATR | Score |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | `logistic` | 0.0 | -999.0 | 1.0 | 4 | 75.0000% | 0.000610 | 0.002438 | -999.000000 |

## Metrics

| Model | Brier | Log Loss | AUC | SL Brier | SL Log Loss | SL AUC |
|---|---:|---:|---:|---:|---:|---:|
| `logistic` | 0.34698329 | 1.75766651 | 0.66666667 | 0.31936690 | 1.96419255 | 0.66666667 |
| `random_forest` | 0.27506277 | 0.75011188 | 0.66666667 | 0.26274190 | 0.72022205 | 0.66666667 |
| `extra_trees` | 0.27616156 | 0.75408683 | 0.66666667 | 0.25768234 | 0.71040580 | 0.66666667 |
| `gradient_boosting` | 0.34118648 | 1.28628065 | 0.66666667 | 0.23990247 | 0.76082949 | 1.00000000 |
| `svm_rbf` | 0.25581363 | 0.70461788 | 0.33333333 | 0.23263656 | 0.65782034 | 0.66666667 |
