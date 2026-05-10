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
| 2 | 0.0000% | -0.757582 | -1.515164 | 0.505128 | -0.070948 | 0.000000 |

## Model Leaderboard

| Rank | Model | Prob Min | EV Min | SL Max | Events | Success | Net Edge ATR | Sum Net Edge ATR | Score |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | `logistic` | 0.0 | -999.0 | 1.0 | 2 | 0.0000% | -0.757582 | -1.515164 | -999.000000 |

## Metrics

| Model | Brier | Log Loss | AUC | SL Brier | SL Log Loss | SL AUC |
|---|---:|---:|---:|---:|---:|---:|
| `logistic` | 0.48295380 | 2.03437365 | 0.00000000 | 0.48295380 | 2.03437365 | 0.00000000 |
| `random_forest` | 0.31581315 | 0.83507827 | 0.00000000 | 0.31581315 | 0.83507827 | 0.00000000 |
| `extra_trees` | 0.30251882 | 0.80452879 | 0.00000000 | 0.30251882 | 0.80452879 | 0.00000000 |
| `gradient_boosting` | 0.48489309 | 2.03646544 | 0.00000000 | 0.48489309 | 2.03646544 | 0.00000000 |
| `svm_rbf` | 0.36407470 | 0.92795255 | 0.00000000 | 0.38863934 | 0.97721836 | 0.00000000 |
