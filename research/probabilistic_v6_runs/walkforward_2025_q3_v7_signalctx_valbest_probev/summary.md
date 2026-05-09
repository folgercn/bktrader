# Probabilistic V6 Walk-Forward

范围：仅限 `research`。本报告用 execution-aware labels 做按月 walk-forward，并用真实 1s execution runner 回测 selected events。

## Portfolio

| Month | TopK | Active | Equal Weight Realistic | Silo Sum Realistic | Symbols |
|---|---:|---:|---:|---:|---|
| `2025-09` | 5 | 0 | 0.0000% | 0.0000% | `` |
| `2025-09` | 10 | 0 | 0.0000% | 0.0000% | `` |
| `2025-09` | 15 | 0 | 0.0000% | 0.0000% | `` |
| `2025-09` | 20 | 2 | 2.4065% | 4.8130% | `BTCUSDT,ETHUSDT` |

## Symbol Rows

| Month | Symbol | TopK | Gate | Model | Selected | Trades | Realistic | PF | Win | DD | Val Edge | Val TopK Return | Val TopK SL | Test Label Edge |
|---|---|---:|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `2025-09` | `BTCUSDT` | 5 | `top_k_not_selected:20` | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.131226 | 1.869518% | 0.0000 | 0.122895 |
| `2025-09` | `BTCUSDT` | 10 | `top_k_not_selected:20` | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.131226 | 1.358298% | 0.2000 | 0.122895 |
| `2025-09` | `BTCUSDT` | 15 | `top_k_not_selected:20` | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.131226 | 2.072279% | 0.1333 | 0.122895 |
| `2025-09` | `BTCUSDT` | 20 | `pass` | `logistic` | 20 | 19 | 1.7000% | 2.014325 | 73.68% | -0.6989% | 0.131226 | 3.834051% | 0.1000 | 0.122895 |
| `2025-09` | `ETHUSDT` | 5 | `top_k_not_selected:20` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.424099 | 2.124165% | 0.2000 | 0.120290 |
| `2025-09` | `ETHUSDT` | 10 | `top_k_not_selected:20` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.424099 | 5.302915% | 0.1000 | 0.120290 |
| `2025-09` | `ETHUSDT` | 15 | `top_k_not_selected:20` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.424099 | 8.482862% | 0.0667 | 0.120290 |
| `2025-09` | `ETHUSDT` | 20 | `pass` | `gradient_boosting` | 20 | 20 | 3.1130% | 3.014885 | 85.00% | -0.5584% | 0.424099 | 10.416448% | 0.0500 | 0.120290 |
