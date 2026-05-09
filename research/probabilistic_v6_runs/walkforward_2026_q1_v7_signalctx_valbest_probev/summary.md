# Probabilistic V6 Walk-Forward

范围：仅限 `research`。本报告用 execution-aware labels 做按月 walk-forward，并用真实 1s execution runner 回测 selected events。

## Portfolio

| Month | TopK | Active | Equal Weight Realistic | Silo Sum Realistic | Symbols |
|---|---:|---:|---:|---:|---|
| `2026-03` | 5 | 0 | 0.0000% | 0.0000% | `` |
| `2026-03` | 10 | 0 | 0.0000% | 0.0000% | `` |
| `2026-03` | 15 | 0 | 0.0000% | 0.0000% | `` |
| `2026-03` | 20 | 2 | 6.3746% | 12.7492% | `BTCUSDT,ETHUSDT` |

## Symbol Rows

| Month | Symbol | TopK | Gate | Model | Selected | Trades | Realistic | PF | Win | DD | Val Edge | Val TopK Return | Val TopK SL | Test Label Edge |
|---|---|---:|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `2026-03` | `BTCUSDT` | 5 | `top_k_not_selected:20` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.426974 | 2.504415% | 0.0000 | 0.266952 |
| `2026-03` | `BTCUSDT` | 10 | `top_k_not_selected:20` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.426974 | 5.268654% | 0.0000 | 0.266952 |
| `2026-03` | `BTCUSDT` | 15 | `top_k_not_selected:20` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.426974 | 7.974217% | 0.0667 | 0.266952 |
| `2026-03` | `BTCUSDT` | 20 | `pass` | `gradient_boosting` | 20 | 20 | 3.8347% | 4.101069 | 90.00% | -0.6916% | 0.426974 | 10.425011% | 0.0500 | 0.266952 |
| `2026-03` | `ETHUSDT` | 5 | `top_k_not_selected:20` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.398431 | 3.584287% | 0.0000 | 0.352770 |
| `2026-03` | `ETHUSDT` | 10 | `top_k_not_selected:20` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.398431 | 6.551337% | 0.0000 | 0.352770 |
| `2026-03` | `ETHUSDT` | 15 | `top_k_not_selected:20` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.398431 | 8.873382% | 0.0667 | 0.352770 |
| `2026-03` | `ETHUSDT` | 20 | `pass` | `gradient_boosting` | 20 | 20 | 8.9145% | 6.398041 | 90.00% | -1.3426% | 0.398431 | 9.500005% | 0.1000 | 0.352770 |
