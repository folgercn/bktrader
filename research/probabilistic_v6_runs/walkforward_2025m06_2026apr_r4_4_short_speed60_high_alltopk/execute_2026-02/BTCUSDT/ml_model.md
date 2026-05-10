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
| 5 | 100.0000% | 0.233333 | 1.166666 | 0.600416 | 0.047824 | 0.000000 |

## Model Leaderboard

| Rank | Model | Prob Min | EV Min | SL Max | Events | Success | Net Edge ATR | Sum Net Edge ATR | Score |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | `logistic` | 0.0 | -999.0 | 1.0 | 5 | 100.0000% | 0.233333 | 1.166666 | -999.000000 |

## Metrics

| Model | Brier | Log Loss | AUC | SL Brier | SL Log Loss | SL AUC |
|---|---:|---:|---:|---:|---:|---:|
| `logistic` | 0.34333099 | 1.26042216 | 0.00000000 | 0.26038592 | 0.73749296 | 0.00000000 |
| `random_forest` | 0.17636603 | 0.53606027 | 0.00000000 | 0.16194423 | 0.51109583 | 0.00000000 |
| `extra_trees` | 0.19283343 | 0.57535133 | 0.00000000 | 0.17670455 | 0.54401473 | 0.00000000 |
| `gradient_boosting` | 0.28367688 | 0.77695420 | 0.00000000 | 0.02147120 | 0.14943657 | 0.00000000 |
| `svm_rbf` | 0.24356006 | 0.68025018 | 0.00000000 | 0.18615801 | 0.56462905 | 0.00000000 |
