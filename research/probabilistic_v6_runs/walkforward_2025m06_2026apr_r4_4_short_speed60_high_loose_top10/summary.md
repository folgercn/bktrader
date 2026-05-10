# Probabilistic V6 Walk-Forward

范围：仅限 `research`。本报告用 execution-aware labels 做按月 walk-forward，并用真实 1s execution runner 回测 selected events。

## Scheme Semantic Contract

| Field | Value |
|---|---|
| Scheme | `Scheme B: delay60 + feature60 + post_selection gate` |
| Entry Source | `true Original_T2 intrabar touch` |
| Breakout Semantic | `long uses current unclosed signal bar 1s high >= prev_high_2; short uses 1s low <= prev_low_2` |
| Feature / Entry Delay | `60s <= 60s` |
| Execution Model | `research/probabilistic_v4_execution_runner.py 1s execution runner` |
| Sizing Mode | `ETH uses hybrid_markov dynamic event sizing; BTC falls back to fixed20 when validation InitialSL rate is too high` |
| BTC Fallback | `mode=fixed20_on_initial_sl`, `InitialSL_rate<=0.3`, `fixed_share=0.2` |
| Lifecycle Claim | `Baseline_Derived_Sizing` |
| Full Reentry Window Lifecycle | `False` |
| Research Baseline | `dir2_zero_initial=true`, `zero_initial_mode=reentry_window`, `reentry_size_schedule=[0.20, 0.10]`, `max_trades_per_bar=2` (AGENTS §2 Core Memory) |
| Current Runner Gap | Current V6 runner performs event selection plus one-shot 1s execution. It does not implement current+next signal-bar reentry windows or slot0/slot1 lifecycle. |

## Run Metrics

Calendar_Normalized_Return 将空仓 symbol-month silo 按 0% 计入后，对 execute_month × symbol 固定网格取平均。

| Metric | Value |
|---|---:|
| Active_Silo_Sum | 3.4592% |
| Calendar_Normalized_Return | 0.1572% |
| Active Months | 11 |
| Empty Months | 0 |
| Active Silos | 22 |
| Calendar Symbol-Month Count | 22 |
| Trades | 111 |
| Worst Active Silo | -2.3018% |

## Baseline Comparison

| Baseline | Active_Silo_Sum | Active Months | Trades | Delta Active_Silo_Sum |
|---|---:|---:|---:|---:|
| `delay60 + feature60 + post_selection gate` | 6.0939% | 5 | 51 | -2.6347% |

## Portfolio

| Month | TopK | Active | Equal Weight Realistic | Silo Sum Realistic | Symbols |
|---|---:|---:|---:|---:|---|
| `2025-06` | 10 | 2 | 0.0326% | 0.0652% | `BTCUSDT,ETHUSDT` |
| `2025-07` | 10 | 2 | -1.0642% | -2.1285% | `BTCUSDT,ETHUSDT` |
| `2025-08` | 10 | 2 | 0.8596% | 1.7191% | `BTCUSDT,ETHUSDT` |
| `2025-09` | 10 | 2 | 0.0984% | 0.1968% | `BTCUSDT,ETHUSDT` |
| `2025-10` | 10 | 2 | -0.9399% | -1.8798% | `BTCUSDT,ETHUSDT` |
| `2025-11` | 10 | 2 | 1.6244% | 3.2488% | `BTCUSDT,ETHUSDT` |
| `2025-12` | 10 | 2 | 0.1309% | 0.2618% | `BTCUSDT,ETHUSDT` |
| `2026-01` | 10 | 2 | 0.5299% | 1.0598% | `BTCUSDT,ETHUSDT` |
| `2026-02` | 10 | 2 | 0.9343% | 1.8685% | `BTCUSDT,ETHUSDT` |
| `2026-03` | 10 | 2 | 0.7822% | 1.5643% | `BTCUSDT,ETHUSDT` |
| `2026-04` | 10 | 2 | -1.2584% | -2.5168% | `BTCUSDT,ETHUSDT` |

## Symbol Rows

| Month | Symbol | TopK | Gate | Sizing | Final | Fallback | Dynamic Ret | Fixed Ret | Model | Selected | Trades | Realistic | PF | Win | DD | Val Edge | Val TopK Return | Val Ret/DD | Val TopK SL | Val Markov | Test Label Edge |
|---|---|---:|---|---|---|---|---:|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `2025-06` | `BTCUSDT` | 10 | `pass` | `fixed20` | `fixed_fallback` | `btc_validation_topk_initial_sl_rate=0.6000>0.3000` | -0.1197% | -0.0438% | `logistic` | 1 | 1 | -0.0438% | 0.0 | 0.00% | -0.0438% | -0.047197 | -0.222156% | -0.570658 | 0.6000 | 0.3775 | 0.082414 |
| `2025-06` | `ETHUSDT` | 10 | `pass` | `hybrid_markov` | `dynamic` | `` | 0.1090% | 0.0000% | `svm_rbf` | 3 | 3 | 0.1090% | 1.169504 | 66.67% | -0.6651% | 0.105443 | 0.518147% | 0.281265 | 0.3750 | 0.5000 | 0.112238 |
| `2025-07` | `BTCUSDT` | 10 | `pass` | `fixed20` | `fixed_fallback` | `btc_validation_topk_initial_sl_rate=1.0000>0.3000` | -0.3194% | -0.0658% | `logistic` | 6 | 6 | -0.0658% | 0.572831 | 66.67% | -0.0920% | -0.242867 | -0.575766% | -2.303064 | 1.0000 | 0.5545 | 0.056053 |
| `2025-07` | `ETHUSDT` | 10 | `pass` | `hybrid_markov` | `dynamic` | `` | -2.0627% | 0.0000% | `logistic` | 5 | 5 | -2.0627% | 0.210739 | 40.00% | -2.0627% | -0.056912 | -0.620657% | -0.512616 | 0.5000 | 0.4411 | 0.080328 |
| `2025-08` | `BTCUSDT` | 10 | `pass` | `fixed20` | `fixed_fallback` | `btc_validation_topk_initial_sl_rate=0.3333>0.3000` | -0.3075% | -0.0530% | `random_forest` | 5 | 5 | -0.0530% | 0.645737 | 60.00% | -0.1494% | 0.005247 | 0.076595% | 0.162719 | 0.3333 | 0.5232 | 0.061595 |
| `2025-08` | `ETHUSDT` | 10 | `pass` | `hybrid_markov` | `dynamic` | `` | 1.7721% | 0.0000% | `gradient_boosting` | 2 | 2 | 1.7721% | inf | 100.00% | 0.0000% | -0.305066 | -1.993485% | -1.301993 | 0.6000 | 0.3708 | 0.112171 |
| `2025-09` | `BTCUSDT` | 10 | `pass` | `fixed20` | `fixed_fallback` | `btc_validation_topk_initial_sl_rate=0.4000>0.3000` | 0.0549% | -0.0476% | `logistic` | 4 | 4 | -0.0476% | 0.589412 | 75.00% | -0.1157% | 0.007047 | 0.117296% | 0.177257 | 0.4000 | 0.5125 | 0.067050 |
| `2025-09` | `ETHUSDT` | 10 | `pass` | `hybrid_markov` | `dynamic` | `` | 0.2444% | 0.0000% | `logistic` | 5 | 5 | 0.2444% | 1.483886 | 80.00% | -0.5092% | 0.810127 | 2.334046% | 9.336184 | 0.0000 | 0.8793 | 0.089146 |
| `2025-10` | `BTCUSDT` | 10 | `pass` | `hybrid_markov` | `dynamic` | `` | -0.9047% | 0.0000% | `logistic` | 3 | 3 | -0.9047% | 0.26003 | 33.33% | -1.2187% | 0.000610 | 0.288474% | 0.786710 | 0.2500 | 0.3682 | 0.072828 |
| `2025-10` | `ETHUSDT` | 10 | `pass` | `hybrid_markov` | `dynamic` | `` | -0.9751% | 0.0000% | `extra_trees` | 1 | 1 | -0.9751% | 0.0 | 0.00% | -0.9751% | 0.127809 | 0.393512% | 1.574048 | 0.2000 | 0.5039 | 0.043160 |
| `2025-11` | `BTCUSDT` | 10 | `pass` | `fixed20` | `fixed_fallback` | `btc_validation_topk_initial_sl_rate=0.3333>0.3000` | 0.2471% | -0.0322% | `logistic` | 7 | 7 | -0.0322% | 0.894243 | 42.86% | -0.1822% | -0.133845 | -0.638423% | -1.488422 | 0.3333 | 0.8688 | 0.087247 |
| `2025-11` | `ETHUSDT` | 10 | `pass` | `hybrid_markov` | `dynamic` | `` | 3.2810% | 0.0000% | `logistic` | 10 | 10 | 3.2810% | 2.643289 | 70.00% | -0.9245% | -0.757582 | -1.679526% | -1.577073 | 1.0000 | 0.5574 | 0.121758 |
| `2025-12` | `BTCUSDT` | 10 | `pass` | `fixed20` | `fixed_fallback` | `btc_validation_topk_initial_sl_rate=0.4286>0.3000` | -0.6929% | -0.1176% | `svm_rbf` | 9 | 9 | -0.1176% | 0.70726 | 33.33% | -0.2721% | 0.037147 | 0.477827% | 1.093053 | 0.4286 | 0.4536 | 0.096988 |
| `2025-12` | `ETHUSDT` | 10 | `pass` | `hybrid_markov` | `dynamic` | `` | 0.3794% | 0.0000% | `svm_rbf` | 8 | 8 | 0.3794% | 1.218939 | 62.50% | -1.7733% | 0.323615 | 2.845111% | 4.889389 | 0.3000 | 0.4559 | 0.058317 |
| `2026-01` | `BTCUSDT` | 10 | `pass` | `fixed20` | `fixed_fallback` | `btc_validation_topk_initial_sl_rate=0.4000>0.3000` | 0.7848% | 0.1524% | `gradient_boosting` | 2 | 2 | 0.1524% | inf | 100.00% | 0.0000% | 0.011170 | 0.272920% | 0.296813 | 0.4000 | 0.5605 | 0.166527 |
| `2026-01` | `ETHUSDT` | 10 | `pass` | `hybrid_markov` | `dynamic` | `` | 0.9074% | 0.0000% | `logistic` | 2 | 2 | 0.9074% | inf | 100.00% | 0.0000% | 0.167435 | 0.829432% | 0.425864 | 0.3750 | 0.3712 | -0.027157 |
| `2026-02` | `BTCUSDT` | 10 | `pass` | `hybrid_markov` | `dynamic` | `` | 2.5186% | 0.0000% | `svm_rbf` | 5 | 5 | 2.5186% | inf | 100.00% | 0.0000% | 0.233333 | 1.201155% | 4.804620 | 0.0000 | 0.4921 | 0.107801 |
| `2026-02` | `ETHUSDT` | 10 | `pass` | `hybrid_markov` | `dynamic` | `` | -0.6501% | 0.0000% | `svm_rbf` | 10 | 10 | -0.6501% | 0.865449 | 50.00% | -3.3842% | 0.253067 | 1.379674% | 5.518696 | 0.2000 | 0.5714 | -0.025624 |
| `2026-03` | `BTCUSDT` | 10 | `pass` | `hybrid_markov` | `dynamic` | `` | 0.6713% | 0.0000% | `logistic` | 1 | 1 | 0.6713% | inf | 100.00% | 0.0000% | 0.627536 | 3.638048% | 14.552192 | 0.0000 | 0.4065 | 0.067053 |
| `2026-03` | `ETHUSDT` | 10 | `pass` | `hybrid_markov` | `dynamic` | `` | 0.8930% | 0.0000% | `svm_rbf` | 2 | 2 | 0.8930% | inf | 100.00% | 0.0000% | 0.111351 | 0.541187% | 0.246760 | 0.3333 | 0.6702 | -0.052880 |
| `2026-04` | `BTCUSDT` | 10 | `pass` | `fixed20` | `fixed_fallback` | `btc_validation_topk_initial_sl_rate=0.6000>0.3000` | -0.4275% | -0.2150% | `svm_rbf` | 10 | 10 | -0.2150% | 0.528715 | 50.00% | -0.3415% | -0.129132 | -0.163871% | -0.141348 | 0.6000 | 0.6244 | -0.035881 |
| `2026-04` | `ETHUSDT` | 10 | `pass` | `hybrid_markov` | `dynamic` | `` | -2.3018% | 0.0000% | `logistic` | 10 | 10 | -2.3018% | 0.35765 | 30.00% | -2.8531% | 0.570816 | 1.410868% | 5.643472 | 0.0000 | 0.9115 | -0.177619 |
