# Probabilistic V6 Walk-Forward

范围：仅限 `research`。本报告用 execution-aware labels 做按月 walk-forward，并用真实 1s execution runner 回测 selected events。

## Portfolio

| Month | TopK | Active | Equal Weight Realistic | Silo Sum Realistic | Symbols |
|---|---:|---:|---:|---:|---|
| `2026-03` | 0 | 2 | 0.2160% | 0.4320% | `BTCUSDT,ETHUSDT` |
| `2026-03` | 5 | 2 | 0.7388% | 1.4775% | `BTCUSDT,ETHUSDT` |
| `2026-03` | 10 | 2 | 1.3406% | 2.6812% | `BTCUSDT,ETHUSDT` |
| `2026-03` | 15 | 2 | 0.3483% | 0.6965% | `BTCUSDT,ETHUSDT` |
| `2026-03` | 20 | 2 | -0.0302% | -0.0605% | `BTCUSDT,ETHUSDT` |

## Symbol Rows

| Month | Symbol | TopK | Gate | Model | Selected | Trades | Realistic | PF | Win | DD | Val Edge | Test Label Edge |
|---|---|---:|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `2026-03` | `BTCUSDT` | 0 | `pass` | `gradient_boosting` | 35 | 33 | -1.1130% | 0.868305 | 63.64% | -3.7031% | 0.295562 | 0.052542 |
| `2026-03` | `BTCUSDT` | 5 | `pass` | `gradient_boosting` | 5 | 5 | 0.4599% | 1.685183 | 80.00% | -0.6766% | 0.295562 | 0.052542 |
| `2026-03` | `BTCUSDT` | 10 | `pass` | `gradient_boosting` | 10 | 10 | -1.0991% | 0.664948 | 60.00% | -2.3132% | 0.295562 | 0.052542 |
| `2026-03` | `BTCUSDT` | 15 | `pass` | `gradient_boosting` | 15 | 15 | -0.9162% | 0.819088 | 60.00% | -2.1992% | 0.295562 | 0.052542 |
| `2026-03` | `BTCUSDT` | 20 | `pass` | `gradient_boosting` | 20 | 19 | -1.6055% | 0.738372 | 57.89% | -2.6368% | 0.295562 | 0.052542 |
| `2026-03` | `ETHUSDT` | 0 | `pass` | `logistic` | 18 | 17 | 1.5450% | 1.384505 | 58.82% | -2.6693% | 0.271388 | 0.078997 |
| `2026-03` | `ETHUSDT` | 5 | `pass` | `logistic` | 5 | 5 | 1.0176% | 1.769485 | 60.00% | -0.9836% | 0.271388 | 0.078997 |
| `2026-03` | `ETHUSDT` | 10 | `pass` | `logistic` | 10 | 9 | 3.7803% | 4.823399 | 77.78% | -0.7719% | 0.271388 | 0.078997 |
| `2026-03` | `ETHUSDT` | 15 | `pass` | `logistic` | 15 | 14 | 1.6127% | 1.463425 | 57.14% | -2.1235% | 0.271388 | 0.078997 |
| `2026-03` | `ETHUSDT` | 20 | `pass` | `logistic` | 18 | 17 | 1.5450% | 1.384505 | 58.82% | -2.6693% | 0.271388 | 0.078997 |
