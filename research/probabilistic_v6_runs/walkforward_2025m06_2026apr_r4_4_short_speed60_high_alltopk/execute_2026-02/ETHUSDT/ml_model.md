# Probabilistic V5 ML Probability Sweep

范围：仅限 `research`。本文件比较多种 ML 概率模型，并输出给执行层消费的 `quality_pass` 与动态仓位。

- Split: `train<= 2025-12-31T23:59:59+00:00, validation<= 2026-01-31T23:59:59+00:00, test> 2026-01-31T23:59:59+00:00`
- Selected model: `logistic`
- Selected rule: `prob_min=0.0`, `ev_atr_min=-999.0`, `sl_prob_max=1.0`
- Markov windows: `[5, 15, 30, 60]`
- Sizing: `mode=hybrid_markov`, `min_share=0.2`, `max_share=1.5`

## Validation

| Events | Success | Net Edge ATR | Sum Net Edge ATR | Avg Prob | Avg Prob EV ATR | Avg InitialSL Prob |
|---:|---:|---:|---:|---:|---:|---:|
| 5 | 80.0000% | 0.253067 | 1.265337 | 0.496821 | -0.047109 | 0.000000 |

## Model Leaderboard

| Rank | Model | Prob Min | EV Min | SL Max | Events | Success | Net Edge ATR | Sum Net Edge ATR | Score |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | `logistic` | 0.0 | -999.0 | 1.0 | 5 | 80.0000% | 0.253067 | 1.265337 | -999.000000 |

## Metrics

| Model | Brier | Log Loss | AUC | SL Brier | SL Log Loss | SL AUC |
|---|---:|---:|---:|---:|---:|---:|
| `logistic` | 0.24135794 | 1.53353626 | 0.75000000 | 0.24135794 | 1.53353626 | 0.75000000 |
| `random_forest` | 0.23593716 | 0.66599481 | 0.75000000 | 0.23593716 | 0.66599481 | 0.75000000 |
| `extra_trees` | 0.22957193 | 0.65232496 | 0.75000000 | 0.22957193 | 0.65232496 | 0.75000000 |
| `gradient_boosting` | 0.32183520 | 0.88793565 | 0.50000000 | 0.32207218 | 0.88462232 | 0.50000000 |
| `svm_rbf` | 0.19243686 | 0.57593517 | 0.25000000 | 0.19298544 | 0.57717325 | 0.25000000 |
