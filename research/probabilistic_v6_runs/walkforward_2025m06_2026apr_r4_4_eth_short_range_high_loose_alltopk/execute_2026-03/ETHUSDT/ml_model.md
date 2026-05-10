# Probabilistic V5 ML Probability Sweep

范围：仅限 `research`。本文件比较多种 ML 概率模型，并输出给执行层消费的 `quality_pass` 与动态仓位。

- Split: `train<= 2026-01-31T23:59:59+00:00, validation<= 2026-02-28T23:59:59+00:00, test> 2026-02-28T23:59:59+00:00`
- Selected model: `logistic`
- Selected rule: `prob_min=0.0`, `ev_atr_min=-999.0`, `sl_prob_max=1.0`
- Markov windows: `[5, 15, 30, 60]`
- Sizing: `mode=hybrid_markov`, `min_share=0.2`, `max_share=1.5`

## Validation

| Events | Success | Net Edge ATR | Sum Net Edge ATR | Avg Prob | Avg Prob EV ATR | Avg InitialSL Prob |
|---:|---:|---:|---:|---:|---:|---:|
| 3 | 100.0000% | 1.110296 | 3.330887 | 0.245642 | -0.364199 | 0.000000 |

## Model Leaderboard

| Rank | Model | Prob Min | EV Min | SL Max | Events | Success | Net Edge ATR | Sum Net Edge ATR | Score |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | `logistic` | 0.0 | -999.0 | 1.0 | 3 | 100.0000% | 1.110296 | 3.330887 | -999.000000 |

## Metrics

| Model | Brier | Log Loss | AUC | SL Brier | SL Log Loss | SL AUC |
|---|---:|---:|---:|---:|---:|---:|
| `logistic` | 0.63889056 | 2.05491217 | 0.00000000 | 0.63889056 | 2.05491217 | 0.00000000 |
| `random_forest` | 0.31673703 | 0.82776385 | 0.00000000 | 0.31673703 | 0.82776385 | 0.00000000 |
| `extra_trees` | 0.36015042 | 0.91706694 | 0.00000000 | 0.36015042 | 0.91706694 | 0.00000000 |
| `gradient_boosting` | 0.54055661 | 1.62551131 | 0.00000000 | 0.50821087 | 1.51711325 | 0.00000000 |
| `svm_rbf` | 0.14887924 | 0.48750879 | 0.00000000 | 0.15723021 | 0.50503098 | 0.00000000 |
