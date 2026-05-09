# Probabilistic V6 Walk-Forward

范围：仅限 `research`。本报告用 execution-aware labels 做按月 walk-forward，并用真实 1s execution runner 回测 selected events。

## Portfolio

| Month | TopK | Active | Equal Weight Realistic | Silo Sum Realistic | Symbols |
|---|---:|---:|---:|---:|---|
| `2025-09` | 0 | 2 | -3.7475% | -7.4950% | `BTCUSDT,ETHUSDT` |
| `2025-09` | 5 | 2 | -0.6565% | -1.3131% | `BTCUSDT,ETHUSDT` |
| `2025-09` | 10 | 2 | -0.6144% | -1.2288% | `BTCUSDT,ETHUSDT` |
| `2025-09` | 15 | 2 | -1.9817% | -3.9635% | `BTCUSDT,ETHUSDT` |
| `2025-09` | 20 | 2 | -1.9808% | -3.9617% | `BTCUSDT,ETHUSDT` |

## Symbol Rows

| Month | Symbol | TopK | Gate | Model | Selected | Trades | Realistic | PF | Win | DD | Val Edge | Test Label Edge |
|---|---|---:|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `2025-09` | `BTCUSDT` | 0 | `pass` | `extra_trees` | 14 | 14 | -0.3762% | 0.812854 | 57.14% | -0.9556% | -0.038598 | 0.027318 |
| `2025-09` | `BTCUSDT` | 5 | `pass` | `extra_trees` | 5 | 5 | -0.3733% | 0.657214 | 60.00% | -0.6855% | -0.038598 | 0.027318 |
| `2025-09` | `BTCUSDT` | 10 | `pass` | `extra_trees` | 10 | 10 | -0.5536% | 0.677716 | 50.00% | -1.1223% | -0.038598 | 0.027318 |
| `2025-09` | `BTCUSDT` | 15 | `pass` | `extra_trees` | 14 | 14 | -0.3762% | 0.812854 | 57.14% | -0.9556% | -0.038598 | 0.027318 |
| `2025-09` | `BTCUSDT` | 20 | `pass` | `extra_trees` | 14 | 14 | -0.3762% | 0.812854 | 57.14% | -0.9556% | -0.038598 | 0.027318 |
| `2025-09` | `ETHUSDT` | 0 | `pass` | `logistic` | 40 | 36 | -7.1188% | 0.407635 | 41.67% | -7.7318% | 0.128830 | -0.159357 |
| `2025-09` | `ETHUSDT` | 5 | `pass` | `logistic` | 5 | 5 | -0.9398% | 0.499541 | 60.00% | -1.4908% | 0.128830 | -0.159357 |
| `2025-09` | `ETHUSDT` | 10 | `pass` | `logistic` | 10 | 10 | -0.6752% | 0.773329 | 70.00% | -2.2260% | 0.128830 | -0.159357 |
| `2025-09` | `ETHUSDT` | 15 | `pass` | `logistic` | 15 | 15 | -3.5873% | 0.382467 | 46.67% | -4.3179% | 0.128830 | -0.159357 |
| `2025-09` | `ETHUSDT` | 20 | `pass` | `logistic` | 20 | 19 | -3.5855% | 0.416119 | 52.63% | -4.5639% | 0.128830 | -0.159357 |
