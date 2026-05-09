# Probabilistic V6 Walk-Forward

范围：仅限 `research`。本报告用 execution-aware labels 做按月 walk-forward，并用真实 1s execution runner 回测 selected events。

## Portfolio

| Month | TopK | Active | Equal Weight Realistic | Silo Sum Realistic | Symbols |
|---|---:|---:|---:|---:|---|
| `2025-12` | 5 | 0 | 0.0000% | 0.0000% | `` |
| `2025-12` | 10 | 0 | 0.0000% | 0.0000% | `` |
| `2025-12` | 15 | 0 | 0.0000% | 0.0000% | `` |
| `2025-12` | 20 | 2 | 6.8971% | 13.7941% | `BTCUSDT,ETHUSDT` |

## Symbol Rows

| Month | Symbol | TopK | Gate | Model | Selected | Trades | Realistic | PF | Win | DD | Val Edge | Val TopK Return | Val TopK SL | Test Label Edge |
|---|---|---:|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `2025-12` | `BTCUSDT` | 5 | `top_k_not_selected:20` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.142455 | 1.381426% | 0.2000 | 0.088304 |
| `2025-12` | `BTCUSDT` | 10 | `top_k_not_selected:20` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.142455 | 3.651795% | 0.2000 | 0.088304 |
| `2025-12` | `BTCUSDT` | 15 | `top_k_not_selected:20` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.142455 | 2.595165% | 0.2667 | 0.088304 |
| `2025-12` | `BTCUSDT` | 20 | `pass` | `gradient_boosting` | 20 | 20 | 3.7495% | 3.366121 | 80.00% | -0.6577% | 0.142455 | 4.023273% | 0.2000 | 0.088304 |
| `2025-12` | `ETHUSDT` | 5 | `top_k_not_selected:20` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.348952 | 3.435611% | 0.0000 | 0.204301 |
| `2025-12` | `ETHUSDT` | 10 | `top_k_not_selected:20` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.348952 | 7.421642% | 0.0000 | 0.204301 |
| `2025-12` | `ETHUSDT` | 15 | `top_k_not_selected:20` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.348952 | 10.139506% | 0.0667 | 0.204301 |
| `2025-12` | `ETHUSDT` | 20 | `pass` | `gradient_boosting` | 20 | 20 | 10.0446% | 1530.870236 | 100.00% | -0.0063% | 0.348952 | 12.616163% | 0.1500 | 0.204301 |
