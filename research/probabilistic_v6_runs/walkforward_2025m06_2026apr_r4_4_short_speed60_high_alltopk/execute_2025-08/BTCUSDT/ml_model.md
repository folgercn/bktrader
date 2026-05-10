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
| 6 | 66.6667% | 0.005247 | 0.031482 | 0.422508 | -0.092191 | 0.000000 |

## Model Leaderboard

| Rank | Model | Prob Min | EV Min | SL Max | Events | Success | Net Edge ATR | Sum Net Edge ATR | Score |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | `logistic` | 0.0 | -999.0 | 1.0 | 6 | 66.6667% | 0.005247 | 0.031482 | -999.000000 |

## Metrics

| Model | Brier | Log Loss | AUC | SL Brier | SL Log Loss | SL AUC |
|---|---:|---:|---:|---:|---:|---:|
| `logistic` | 0.29736315 | 1.05357611 | 0.62500000 | 0.28016561 | 1.03829817 | 0.62500000 |
| `random_forest` | 0.24793103 | 0.68900873 | 0.62500000 | 0.24875828 | 0.69066372 | 0.50000000 |
| `extra_trees` | 0.22685713 | 0.64631409 | 0.75000000 | 0.23348226 | 0.65968411 | 0.50000000 |
| `gradient_boosting` | 0.28212928 | 1.09289557 | 0.75000000 | 0.27709129 | 1.15464724 | 0.75000000 |
| `svm_rbf` | 0.30355225 | 0.81700387 | 0.37500000 | 0.26088216 | 0.71711398 | 0.50000000 |
