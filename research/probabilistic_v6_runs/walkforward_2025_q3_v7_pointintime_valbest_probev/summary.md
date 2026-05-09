# Probabilistic V6 Walk-Forward

范围：仅限 `research`。本报告用 execution-aware labels 做按月 walk-forward，并用真实 1s execution runner 回测 selected events。

## Portfolio

| Month | TopK | Active | Equal Weight Realistic | Silo Sum Realistic | Symbols |
|---|---:|---:|---:|---:|---|
| `2025-09` | 5 | 0 | 0.0000% | 0.0000% | `` |
| `2025-09` | 10 | 0 | 0.0000% | 0.0000% | `` |
| `2025-09` | 15 | 1 | -1.9905% | -1.9905% | `ETHUSDT` |
| `2025-09` | 20 | 0 | 0.0000% | 0.0000% | `` |

## Symbol Rows

| Month | Symbol | TopK | Gate | Model | Selected | Trades | Realistic | PF | Win | DD | Val Edge | Val TopK Return | Val TopK SL | Test Label Edge |
|---|---|---:|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `2025-09` | `BTCUSDT` | 5 | `validation_edge<0.05` | `svm_rbf` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.019920 | -0.436134% | 0.4000 | 0.084327 |
| `2025-09` | `BTCUSDT` | 10 | `validation_edge<0.05` | `svm_rbf` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.019920 | 0.285703% | 0.3000 | 0.084327 |
| `2025-09` | `BTCUSDT` | 15 | `validation_edge<0.05` | `svm_rbf` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.019920 | -0.105407% | 0.3846 | 0.084327 |
| `2025-09` | `BTCUSDT` | 20 | `validation_edge<0.05` | `svm_rbf` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.019920 | -0.105407% | 0.3846 | 0.084327 |
| `2025-09` | `ETHUSDT` | 5 | `top_k_not_selected:15` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.342215 | 1.732899% | 0.2000 | -0.138751 |
| `2025-09` | `ETHUSDT` | 10 | `top_k_not_selected:15` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.342215 | 4.253760% | 0.1000 | -0.138751 |
| `2025-09` | `ETHUSDT` | 15 | `pass` | `gradient_boosting` | 15 | 15 | -1.9905% | 0.565745 | 46.67% | -3.7717% | 0.342215 | 4.837315% | 0.1538 | -0.138751 |
| `2025-09` | `ETHUSDT` | 20 | `top_k_not_selected:15` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.342215 | 4.837315% | 0.1538 | -0.138751 |
