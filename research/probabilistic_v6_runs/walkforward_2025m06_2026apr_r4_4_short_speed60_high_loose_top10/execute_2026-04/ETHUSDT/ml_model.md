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
| 2 | 100.0000% | 0.570816 | 1.141632 | 0.413085 | -0.152648 | 0.000000 |

## Model Leaderboard

| Rank | Model | Prob Min | EV Min | SL Max | Events | Success | Net Edge ATR | Sum Net Edge ATR | Score |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | `logistic` | 0.0 | -999.0 | 1.0 | 2 | 100.0000% | 0.570816 | 1.141632 | -999.000000 |

## Metrics

| Model | Brier | Log Loss | AUC | SL Brier | SL Log Loss | SL AUC |
|---|---:|---:|---:|---:|---:|---:|
| `logistic` | 0.50411253 | 2.25512377 | 0.00000000 | 0.42234626 | 1.27659297 | 0.00000000 |
| `random_forest` | 0.13132092 | 0.44999818 | 0.00000000 | 0.12744143 | 0.44158938 | 0.00000000 |
| `extra_trees` | 0.16544971 | 0.52201934 | 0.00000000 | 0.15811495 | 0.50670618 | 0.00000000 |
| `gradient_boosting` | 0.06214606 | 0.25144037 | 0.00000000 | 0.04824740 | 0.23512129 | 0.00000000 |
| `svm_rbf` | 0.11453007 | 0.41266606 | 0.00000000 | 0.10911118 | 0.40051655 | 0.00000000 |
