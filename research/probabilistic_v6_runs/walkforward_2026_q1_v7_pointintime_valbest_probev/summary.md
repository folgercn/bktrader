# Probabilistic V6 Walk-Forward

范围：仅限 `research`。本报告用 execution-aware labels 做按月 walk-forward，并用真实 1s execution runner 回测 selected events。

## Portfolio

| Month | TopK | Active | Equal Weight Realistic | Silo Sum Realistic | Symbols |
|---|---:|---:|---:|---:|---|
| `2026-03` | 5 | 0 | 0.0000% | 0.0000% | `` |
| `2026-03` | 10 | 0 | 0.0000% | 0.0000% | `` |
| `2026-03` | 15 | 1 | 1.4629% | 1.4629% | `ETHUSDT` |
| `2026-03` | 20 | 1 | -1.4667% | -1.4667% | `BTCUSDT` |

## Symbol Rows

| Month | Symbol | TopK | Gate | Model | Selected | Trades | Realistic | PF | Win | DD | Val Edge | Val TopK Return | Val TopK SL | Test Label Edge |
|---|---|---:|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `2026-03` | `BTCUSDT` | 5 | `top_k_not_selected:20` | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.281615 | 1.246706% | 0.4000 | -0.001094 |
| `2026-03` | `BTCUSDT` | 10 | `top_k_not_selected:20` | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.281615 | 3.254270% | 0.3000 | -0.001094 |
| `2026-03` | `BTCUSDT` | 15 | `top_k_not_selected:20` | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.281615 | 5.559882% | 0.2667 | -0.001094 |
| `2026-03` | `BTCUSDT` | 20 | `pass` | `random_forest` | 20 | 20 | -1.4667% | 0.740176 | 60.00% | -3.2737% | 0.281615 | 6.132250% | 0.2500 | -0.001094 |
| `2026-03` | `ETHUSDT` | 5 | `top_k_not_selected:15` | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.179544 | -0.570178% | 0.4000 | 0.037474 |
| `2026-03` | `ETHUSDT` | 10 | `top_k_not_selected:15` | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.179544 | 2.471552% | 0.2000 | 0.037474 |
| `2026-03` | `ETHUSDT` | 15 | `pass` | `logistic` | 15 | 15 | 1.4629% | 1.381285 | 60.00% | -2.2479% | 0.179544 | 4.946953% | 0.2000 | 0.037474 |
| `2026-03` | `ETHUSDT` | 20 | `top_k_not_selected:15` | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.179544 | 3.857986% | 0.3000 | 0.037474 |
