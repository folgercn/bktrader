# Probabilistic V6 Walk-Forward

范围：仅限 `research`。本报告用 execution-aware labels 做按月 walk-forward，并用真实 1s execution runner 回测 selected events。

## Portfolio

| Month | TopK | Active | Equal Weight Realistic | Silo Sum Realistic | Symbols |
|---|---:|---:|---:|---:|---|
| `2025-09` | 0 | 1 | -7.5896% | -7.5896% | `ETHUSDT` |
| `2025-09` | 5 | 1 | -0.8116% | -0.8116% | `ETHUSDT` |
| `2025-09` | 10 | 1 | -2.6810% | -2.6810% | `ETHUSDT` |
| `2025-09` | 15 | 1 | -5.3681% | -5.3681% | `ETHUSDT` |
| `2025-09` | 20 | 1 | -6.2031% | -6.2031% | `ETHUSDT` |

## Symbol Rows

| Month | Symbol | TopK | Gate | Model | Selected | Trades | Realistic | PF | Win | DD | Val Edge | Test Label Edge |
|---|---|---:|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `2025-09` | `BTCUSDT` | 0 | `validation_edge<0.05` | `extra_trees` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.038598 | 0.027318 |
| `2025-09` | `BTCUSDT` | 5 | `validation_edge<0.05` | `extra_trees` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.038598 | 0.027318 |
| `2025-09` | `BTCUSDT` | 10 | `validation_edge<0.05` | `extra_trees` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.038598 | 0.027318 |
| `2025-09` | `BTCUSDT` | 15 | `validation_edge<0.05` | `extra_trees` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.038598 | 0.027318 |
| `2025-09` | `BTCUSDT` | 20 | `validation_edge<0.05` | `extra_trees` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.038598 | 0.027318 |
| `2025-09` | `ETHUSDT` | 0 | `pass` | `logistic` | 40 | 36 | -7.5896% | 0.40324 | 41.67% | -8.2072% | 0.128830 | -0.159357 |
| `2025-09` | `ETHUSDT` | 5 | `pass` | `logistic` | 5 | 5 | -0.8116% | 0.569179 | 60.00% | -1.7073% | 0.128830 | -0.159357 |
| `2025-09` | `ETHUSDT` | 10 | `pass` | `logistic` | 10 | 10 | -2.6810% | 0.352107 | 50.00% | -3.4483% | 0.128830 | -0.159357 |
| `2025-09` | `ETHUSDT` | 15 | `pass` | `logistic` | 15 | 15 | -5.3681% | 0.239885 | 40.00% | -6.3627% | 0.128830 | -0.159357 |
| `2025-09` | `ETHUSDT` | 20 | `pass` | `logistic` | 20 | 20 | -6.2031% | 0.277455 | 40.00% | -7.1635% | 0.128830 | -0.159357 |
