# Probabilistic V6 Walk-Forward

范围：仅限 `research`。本报告用 execution-aware labels 做按月 walk-forward，并用真实 1s execution runner 回测 selected events。

## Portfolio

| Month | TopK | Active | Equal Weight Realistic | Silo Sum Realistic | Symbols |
|---|---:|---:|---:|---:|---|
| `2025-09` | 5 | 0 | 0.0000% | 0.0000% | `` |
| `2025-09` | 10 | 0 | 0.0000% | 0.0000% | `` |
| `2025-09` | 15 | 1 | -3.9517% | -3.9517% | `ETHUSDT` |
| `2025-09` | 20 | 0 | 0.0000% | 0.0000% | `` |

## Symbol Rows

| Month | Symbol | TopK | Gate | Model | Selected | Trades | Realistic | PF | Win | DD | Val Edge | Val TopK Return | Val TopK SL | Test Label Edge |
|---|---|---:|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `2025-09` | `BTCUSDT` | 5 | `validation_edge<0.05` | `extra_trees` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.112709 | -0.988939% | 0.6000 | -0.013802 |
| `2025-09` | `BTCUSDT` | 10 | `validation_edge<0.05` | `extra_trees` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.112709 | -1.085042% | 0.5000 | -0.013802 |
| `2025-09` | `BTCUSDT` | 15 | `validation_edge<0.05` | `extra_trees` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.112709 | -1.494449% | 0.5000 | -0.013802 |
| `2025-09` | `BTCUSDT` | 20 | `validation_edge<0.05` | `extra_trees` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.112709 | -1.494449% | 0.5000 | -0.013802 |
| `2025-09` | `ETHUSDT` | 5 | `top_k_not_selected:15` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.291893 | 0.921527% | 0.2000 | -0.027780 |
| `2025-09` | `ETHUSDT` | 10 | `top_k_not_selected:15` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.291893 | 2.384669% | 0.2000 | -0.027780 |
| `2025-09` | `ETHUSDT` | 15 | `pass` | `gradient_boosting` | 15 | 15 | -3.9517% | 0.328805 | 46.67% | -4.7858% | 0.291893 | 2.950184% | 0.1818 | -0.027780 |
| `2025-09` | `ETHUSDT` | 20 | `top_k_not_selected:15` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.291893 | 2.950184% | 0.1818 | -0.027780 |
