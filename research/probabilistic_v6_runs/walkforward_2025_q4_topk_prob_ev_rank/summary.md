# Probabilistic V6 Walk-Forward

范围：仅限 `research`。本报告用 execution-aware labels 做按月 walk-forward，并用真实 1s execution runner 回测 selected events。

## Portfolio

| Month | TopK | Active | Equal Weight Realistic | Silo Sum Realistic | Symbols |
|---|---:|---:|---:|---:|---|
| `2025-12` | 0 | 1 | -1.1390% | -1.1390% | `ETHUSDT` |
| `2025-12` | 5 | 1 | 0.4674% | 0.4674% | `ETHUSDT` |
| `2025-12` | 10 | 1 | -1.3458% | -1.3458% | `ETHUSDT` |
| `2025-12` | 15 | 1 | -0.3902% | -0.3902% | `ETHUSDT` |
| `2025-12` | 20 | 1 | -0.3234% | -0.3234% | `ETHUSDT` |

## Symbol Rows

| Month | Symbol | TopK | Gate | Model | Selected | Trades | Realistic | PF | Win | DD | Val Edge | Test Label Edge |
|---|---|---:|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `2025-12` | `BTCUSDT` | 0 | `validation_edge<0.05` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.003248 | -0.112338 |
| `2025-12` | `BTCUSDT` | 5 | `validation_edge<0.05` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.003248 | -0.112338 |
| `2025-12` | `BTCUSDT` | 10 | `validation_edge<0.05` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.003248 | -0.112338 |
| `2025-12` | `BTCUSDT` | 15 | `validation_edge<0.05` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.003248 | -0.112338 |
| `2025-12` | `BTCUSDT` | 20 | `validation_edge<0.05` | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.003248 | -0.112338 |
| `2025-12` | `ETHUSDT` | 0 | `pass` | `random_forest` | 32 | 31 | -1.1390% | 0.810298 | 54.84% | -3.6276% | 0.174520 | -0.019174 |
| `2025-12` | `ETHUSDT` | 5 | `pass` | `random_forest` | 5 | 5 | 0.4674% | 1.462727 | 60.00% | -1.0230% | 0.174520 | -0.019174 |
| `2025-12` | `ETHUSDT` | 10 | `pass` | `random_forest` | 10 | 10 | -1.3458% | 0.58102 | 50.00% | -2.8093% | 0.174520 | -0.019174 |
| `2025-12` | `ETHUSDT` | 15 | `pass` | `random_forest` | 15 | 14 | -0.3902% | 0.88469 | 57.14% | -2.2882% | 0.174520 | -0.019174 |
| `2025-12` | `ETHUSDT` | 20 | `pass` | `random_forest` | 20 | 19 | -0.3234% | 0.91744 | 57.89% | -2.5644% | 0.174520 | -0.019174 |
