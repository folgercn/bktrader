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
| 5 | 80.0000% | 0.127809 | 0.639046 | 0.498364 | -0.062749 | 0.000000 |

## Model Leaderboard

| Rank | Model | Prob Min | EV Min | SL Max | Events | Success | Net Edge ATR | Sum Net Edge ATR | Score |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | `logistic` | 0.0 | -999.0 | 1.0 | 5 | 80.0000% | 0.127809 | 0.639046 | -999.000000 |

## Metrics

| Model | Brier | Log Loss | AUC | SL Brier | SL Log Loss | SL AUC |
|---|---:|---:|---:|---:|---:|---:|
| `logistic` | 0.56797082 | 1.74798743 | 0.25000000 | 0.56797082 | 1.74798743 | 0.25000000 |
| `random_forest` | 0.21386861 | 0.61827478 | 0.50000000 | 0.21386861 | 0.61827478 | 0.50000000 |
| `extra_trees` | 0.22585035 | 0.64481333 | 0.00000000 | 0.22585035 | 0.64481333 | 0.00000000 |
| `gradient_boosting` | 0.20780568 | 0.66643776 | 0.50000000 | 0.20780568 | 0.66643759 | 0.50000000 |
| `svm_rbf` | 0.20240322 | 0.59690567 | 0.75000000 | 0.20189143 | 0.59578314 | 0.75000000 |
