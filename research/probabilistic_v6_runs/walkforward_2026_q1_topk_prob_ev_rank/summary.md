# Probabilistic V6 Walk-Forward

范围：仅限 `research`。本报告用 execution-aware labels 做按月 walk-forward，并用真实 1s execution runner 回测 selected events。

## Portfolio

| Month | TopK | Active | Equal Weight Realistic | Silo Sum Realistic | Symbols |
|---|---:|---:|---:|---:|---|
| `2026-03` | 0 | 2 | 0.2160% | 0.4320% | `BTCUSDT,ETHUSDT` |
| `2026-03` | 5 | 2 | 0.0300% | 0.0599% | `BTCUSDT,ETHUSDT` |
| `2026-03` | 10 | 2 | 0.0228% | 0.0457% | `BTCUSDT,ETHUSDT` |
| `2026-03` | 15 | 2 | 0.5618% | 1.1237% | `BTCUSDT,ETHUSDT` |
| `2026-03` | 20 | 2 | 0.3365% | 0.6730% | `BTCUSDT,ETHUSDT` |

## Symbol Rows

| Month | Symbol | TopK | Gate | Model | Selected | Trades | Realistic | PF | Win | DD | Val Edge | Test Label Edge |
|---|---|---:|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `2026-03` | `BTCUSDT` | 0 | `pass` | `gradient_boosting` | 35 | 33 | -1.1130% | 0.868305 | 63.64% | -3.7031% | 0.295562 | 0.052542 |
| `2026-03` | `BTCUSDT` | 5 | `pass` | `gradient_boosting` | 5 | 5 | -0.6264% | 0.545567 | 60.00% | -1.3634% | 0.295562 | 0.052542 |
| `2026-03` | `BTCUSDT` | 10 | `pass` | `gradient_boosting` | 10 | 10 | -2.6524% | 0.347248 | 50.00% | -2.6524% | 0.295562 | 0.052542 |
| `2026-03` | `BTCUSDT` | 15 | `pass` | `gradient_boosting` | 15 | 15 | -0.7844% | 0.833616 | 60.00% | -2.6578% | 0.295562 | 0.052542 |
| `2026-03` | `BTCUSDT` | 20 | `pass` | `gradient_boosting` | 20 | 20 | -0.8720% | 0.856309 | 60.00% | -2.6578% | 0.295562 | 0.052542 |
| `2026-03` | `ETHUSDT` | 0 | `pass` | `logistic` | 18 | 17 | 1.5450% | 1.384505 | 58.82% | -2.6693% | 0.271388 | 0.078997 |
| `2026-03` | `ETHUSDT` | 5 | `pass` | `logistic` | 5 | 5 | 0.6863% | 1.523119 | 60.00% | -1.3313% | 0.271388 | 0.078997 |
| `2026-03` | `ETHUSDT` | 10 | `pass` | `logistic` | 10 | 10 | 2.6981% | 2.486176 | 70.00% | -0.9836% | 0.271388 | 0.078997 |
| `2026-03` | `ETHUSDT` | 15 | `pass` | `logistic` | 15 | 14 | 1.9081% | 1.628289 | 57.14% | -1.6578% | 0.271388 | 0.078997 |
| `2026-03` | `ETHUSDT` | 20 | `pass` | `logistic` | 18 | 17 | 1.5450% | 1.384505 | 58.82% | -2.6693% | 0.271388 | 0.078997 |
