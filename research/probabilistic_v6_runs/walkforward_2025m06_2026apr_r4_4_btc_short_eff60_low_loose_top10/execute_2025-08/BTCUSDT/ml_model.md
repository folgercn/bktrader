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
| 6 | 66.6667% | 0.020842 | 0.125050 | 0.169800 | -0.318534 | 0.000000 |

## Model Leaderboard

| Rank | Model | Prob Min | EV Min | SL Max | Events | Success | Net Edge ATR | Sum Net Edge ATR | Score |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | `logistic` | 0.0 | -999.0 | 1.0 | 6 | 66.6667% | 0.020842 | 0.125050 | -999.000000 |

## Metrics

| Model | Brier | Log Loss | AUC | SL Brier | SL Log Loss | SL AUC |
|---|---:|---:|---:|---:|---:|---:|
| `logistic` | 0.66389408 | 2.45453204 | 0.12500000 | 0.60664975 | 2.36638020 | 0.12500000 |
| `random_forest` | 0.40135996 | 1.01499977 | 0.00000000 | 0.38072251 | 0.96602541 | 0.00000000 |
| `extra_trees` | 0.38610207 | 0.98042094 | 0.00000000 | 0.36897476 | 0.94000271 | 0.00000000 |
| `gradient_boosting` | 0.64298307 | 1.90325227 | 0.00000000 | 0.56693391 | 1.52363201 | 0.00000000 |
| `svm_rbf` | 0.26566435 | 0.72467288 | 0.25000000 | 0.22304941 | 0.63837714 | 0.75000000 |
