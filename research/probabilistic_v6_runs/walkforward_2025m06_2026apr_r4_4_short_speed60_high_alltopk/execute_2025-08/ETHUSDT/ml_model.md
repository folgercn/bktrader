# Probabilistic V5 ML Probability Sweep

范围：仅限 `research`。本文件比较多种 ML 概率模型，并输出给执行层消费的 `quality_pass` 与动态仓位。

- Split: `train<= 2025-06-30T23:59:59+00:00, validation<= 2025-07-31T23:59:59+00:00, test> 2025-07-31T23:59:59+00:00`
- Selected model: `logistic`
- Selected rule: `prob_min=0.0`, `ev_atr_min=-999.0`, `sl_prob_max=1.0`
- Markov windows: `[5, 15, 30, 60]`
- Sizing: `mode=hybrid_markov`, `min_share=0.2`, `max_share=1.5`

## Validation

| Events | Success | Net Edge ATR | Sum Net Edge ATR | Avg Prob | Avg Prob EV ATR | Avg InitialSL Prob |
|---:|---:|---:|---:|---:|---:|---:|
| 5 | 40.0000% | -0.305066 | -1.525330 | 0.612419 | 0.092287 | 0.000000 |

## Model Leaderboard

| Rank | Model | Prob Min | EV Min | SL Max | Events | Success | Net Edge ATR | Sum Net Edge ATR | Score |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | `logistic` | 0.0 | -999.0 | 1.0 | 5 | 40.0000% | -0.305066 | -1.525330 | -999.000000 |

## Metrics

| Model | Brier | Log Loss | AUC | SL Brier | SL Log Loss | SL AUC |
|---|---:|---:|---:|---:|---:|---:|
| `logistic` | 0.43781448 | 1.51528743 | 0.33333333 | 0.43781448 | 1.51528743 | 0.33333333 |
| `random_forest` | 0.28193398 | 0.75720759 | 0.00000000 | 0.28193398 | 0.75720759 | 0.00000000 |
| `extra_trees` | 0.28385994 | 0.76099431 | 0.00000000 | 0.28385994 | 0.76099431 | 0.00000000 |
| `gradient_boosting` | 0.54572590 | 1.72567750 | 0.00000000 | 0.54139472 | 1.71508384 | 0.00000000 |
| `svm_rbf` | 0.28114358 | 0.75648160 | 0.16666667 | 0.27104549 | 0.73457432 | 0.83333333 |
