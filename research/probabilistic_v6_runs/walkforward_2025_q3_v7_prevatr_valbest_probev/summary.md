# Probabilistic V6 Walk-Forward

范围：仅限 `research`。本报告用 execution-aware labels 做按月 walk-forward，并用真实 1s execution runner 回测 selected events。

## Portfolio

| Month | TopK | Active | Equal Weight Realistic | Silo Sum Realistic | Symbols |
|---|---:|---:|---:|---:|---|
| `2025-09` | 5 | 0 | 0.0000% | 0.0000% | `` |
| `2025-09` | 10 | 0 | 0.0000% | 0.0000% | `` |
| `2025-09` | 15 | 0 | 0.0000% | 0.0000% | `` |
| `2025-09` | 20 | 1 | -2.6501% | -2.6501% | `ETHUSDT` |

## Symbol Rows

| Month | Symbol | TopK | Gate | Model | Selected | Trades | Realistic | PF | Win | DD | Val Edge | Val TopK Return | Val TopK SL | Test Label Edge |
|---|---|---:|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `2025-09` | `BTCUSDT` | 5 | `validation_edge<0.05` | `extra_trees` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.027737 | -0.238628% | 0.4000 | 0.025833 |
| `2025-09` | `BTCUSDT` | 10 | `validation_edge<0.05` | `extra_trees` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.027737 | 0.050383% | 0.3000 | 0.025833 |
| `2025-09` | `BTCUSDT` | 15 | `validation_edge<0.05` | `extra_trees` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.027737 | -0.285241% | 0.3846 | 0.025833 |
| `2025-09` | `BTCUSDT` | 20 | `validation_edge<0.05` | `extra_trees` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.027737 | -0.285241% | 0.3846 | 0.025833 |
| `2025-09` | `ETHUSDT` | 5 | `top_k_not_selected:20` | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.187505 | 1.132545% | 0.2000 | -0.084234 |
| `2025-09` | `ETHUSDT` | 10 | `top_k_not_selected:20` | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.187505 | 1.495287% | 0.3000 | -0.084234 |
| `2025-09` | `ETHUSDT` | 15 | `top_k_not_selected:20` | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.187505 | 1.887986% | 0.2667 | -0.084234 |
| `2025-09` | `ETHUSDT` | 20 | `pass` | `random_forest` | 20 | 19 | -2.6501% | 0.484045 | 52.63% | -3.6054% | 0.187505 | 3.025216% | 0.2500 | -0.084234 |
