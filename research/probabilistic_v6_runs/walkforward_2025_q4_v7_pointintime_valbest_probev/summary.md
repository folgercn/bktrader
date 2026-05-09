# Probabilistic V6 Walk-Forward

范围：仅限 `research`。本报告用 execution-aware labels 做按月 walk-forward，并用真实 1s execution runner 回测 selected events。

## Portfolio

| Month | TopK | Active | Equal Weight Realistic | Silo Sum Realistic | Symbols |
|---|---:|---:|---:|---:|---|
| `2025-12` | 5 | 0 | 0.0000% | 0.0000% | `` |
| `2025-12` | 10 | 0 | 0.0000% | 0.0000% | `` |
| `2025-12` | 15 | 1 | -3.7057% | -3.7057% | `BTCUSDT` |
| `2025-12` | 20 | 0 | 0.0000% | 0.0000% | `` |

## Symbol Rows

| Month | Symbol | TopK | Gate | Model | Selected | Trades | Realistic | PF | Win | DD | Val Edge | Val TopK Return | Val TopK SL | Test Label Edge |
|---|---|---:|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `2025-12` | `BTCUSDT` | 5 | `top_k_not_selected:15` | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.073672 | -0.122820% | 0.6000 | -0.219148 |
| `2025-12` | `BTCUSDT` | 10 | `top_k_not_selected:15` | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.073672 | -0.070960% | 0.4000 | -0.219148 |
| `2025-12` | `BTCUSDT` | 15 | `pass` | `random_forest` | 12 | 11 | -3.7057% | 0.080992 | 27.27% | -3.7057% | 0.073672 | 1.008339% | 0.2857 | -0.219148 |
| `2025-12` | `BTCUSDT` | 20 | `top_k_not_selected:15` | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.073672 | 1.008339% | 0.2857 | -0.219148 |
| `2025-12` | `ETHUSDT` | 5 | `validation_topk_no_candidate` | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.129876 | 1.066263% | 0.4000 | -0.046285 |
| `2025-12` | `ETHUSDT` | 10 | `validation_topk_no_candidate` | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.129876 | 0.616118% | 0.5000 | -0.046285 |
| `2025-12` | `ETHUSDT` | 15 | `validation_topk_no_candidate` | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.129876 | 2.248259% | 0.4000 | -0.046285 |
| `2025-12` | `ETHUSDT` | 20 | `validation_topk_no_candidate` | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.129876 | 1.298173% | 0.4500 | -0.046285 |
