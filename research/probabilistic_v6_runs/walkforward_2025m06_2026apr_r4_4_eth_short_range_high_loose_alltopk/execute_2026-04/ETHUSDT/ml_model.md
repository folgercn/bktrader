# Probabilistic V5 ML Probability Sweep

范围：仅限 `research`。本文件比较多种 ML 概率模型，并输出给执行层消费的 `quality_pass` 与动态仓位。

- Split: `train<= 2026-02-28T23:59:59+00:00, validation<= 2026-03-31T23:59:59+00:00, test> 2026-03-31T23:59:59+00:00`
- Selected model: `logistic`
- Selected rule: `prob_min=0.0`, `ev_atr_min=-999.0`, `sl_prob_max=1.0`
- Markov windows: `[5, 15, 30, 60]`
- Sizing: `mode=hybrid_markov`, `min_share=0.2`, `max_share=1.5`

## Validation

| Events | Success | Net Edge ATR | Sum Net Edge ATR | Avg Prob | Avg Prob EV ATR | Avg InitialSL Prob |
|---:|---:|---:|---:|---:|---:|---:|
| 2 | 50.0000% | 0.184304 | 0.368608 | 0.521639 | 0.005114 | 0.000000 |

## Model Leaderboard

| Rank | Model | Prob Min | EV Min | SL Max | Events | Success | Net Edge ATR | Sum Net Edge ATR | Score |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | `logistic` | 0.0 | -999.0 | 1.0 | 2 | 50.0000% | 0.184304 | 0.368608 | -999.000000 |

## Metrics

| Model | Brier | Log Loss | AUC | SL Brier | SL Log Loss | SL AUC |
|---|---:|---:|---:|---:|---:|---:|
| `logistic` | 0.00122087 | 0.02806520 | 1.00000000 | 0.45214795 | 1.51013021 | 0.00000000 |
| `random_forest` | 0.17652639 | 0.53557590 | 1.00000000 | 0.15276806 | 0.48802343 | 0.00000000 |
| `extra_trees` | 0.18897570 | 0.56259791 | 1.00000000 | 0.15071486 | 0.48592634 | 0.00000000 |
| `gradient_boosting` | 0.37778141 | 1.02312703 | 1.00000000 | 0.00864074 | 0.07643598 | 0.00000000 |
| `svm_rbf` | 0.24308588 | 0.67274201 | 1.00000000 | 0.14148886 | 0.47162363 | 0.00000000 |
