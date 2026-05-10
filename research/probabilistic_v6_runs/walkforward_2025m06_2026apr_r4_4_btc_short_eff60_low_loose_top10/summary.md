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
| Active_Silo_Sum | -3.4326% |
| Calendar_Normalized_Return | -0.3121% |
| Active Months | 11 |
| Empty Months | 0 |
| Active Silos | 11 |
| Calendar Symbol-Month Count | 11 |
| Trades | 67 |
| Worst Active Silo | -2.2632% |

## Baseline Comparison

| Baseline | Active_Silo_Sum | Active Months | Trades | Delta Active_Silo_Sum |
|---|---:|---:|---:|---:|
| `delay60 + feature60 + post_selection gate` | 6.0939% | 5 | 51 | -9.5265% |

## Portfolio

| Month | TopK | Active | Equal Weight Realistic | Silo Sum Realistic | Symbols |
|---|---:|---:|---:|---:|---|
| `2025-06` | 10 | 1 | -2.2632% | -2.2632% | `BTCUSDT` |
| `2025-07` | 10 | 1 | -0.1276% | -0.1276% | `BTCUSDT` |
| `2025-08` | 10 | 1 | -0.0791% | -0.0791% | `BTCUSDT` |
| `2025-09` | 10 | 1 | -0.1851% | -0.1851% | `BTCUSDT` |
| `2025-10` | 10 | 1 | 0.1047% | 0.1047% | `BTCUSDT` |
| `2025-11` | 10 | 1 | -0.6677% | -0.6677% | `BTCUSDT` |
| `2025-12` | 10 | 1 | -1.1887% | -1.1887% | `BTCUSDT` |
| `2026-01` | 10 | 1 | 0.0040% | 0.0040% | `BTCUSDT` |
| `2026-02` | 10 | 1 | 2.8985% | 2.8985% | `BTCUSDT` |
| `2026-03` | 10 | 1 | -0.8793% | -0.8793% | `BTCUSDT` |
| `2026-04` | 10 | 1 | -1.0491% | -1.0491% | `BTCUSDT` |

## Symbol Rows

| Month | Symbol | TopK | Gate | Sizing | Final | Fallback | Dynamic Ret | Fixed Ret | Model | Selected | Trades | Realistic | PF | Win | DD | Val Edge | Val TopK Return | Val Ret/DD | Val TopK SL | Val Markov | Test Label Edge |
|---|---|---:|---|---|---|---|---:|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `2025-06` | `BTCUSDT` | 10 | `pass` | `hybrid_markov` | `dynamic` | `` | -2.2632% | 0.0000% | `logistic` | 6 | 6 | -2.2632% | 0.091671 | 16.67% | -2.4878% | 0.331284 | 1.820732% | 7.282928 | 0.0000 | 0.2793 | -0.010169 |
| `2025-07` | `BTCUSDT` | 10 | `pass` | `fixed20` | `fixed_fallback` | `btc_validation_topk_initial_sl_rate=0.6000>0.3000` | -0.4449% | -0.1276% | `svm_rbf` | 4 | 4 | -0.1276% | 0.174267 | 50.00% | -0.1505% | -0.092428 | -1.075319% | -0.737097 | 0.6000 | 0.4000 | 0.004892 |
| `2025-08` | `BTCUSDT` | 10 | `pass` | `fixed20` | `fixed_fallback` | `btc_validation_topk_initial_sl_rate=0.3333>0.3000` | -0.4788% | -0.0791% | `logistic` | 7 | 7 | -0.0791% | 0.653568 | 57.14% | -0.1791% | 0.020842 | -0.098263% | -0.202892 | 0.3333 | 0.2072 | 0.038844 |
| `2025-09` | `BTCUSDT` | 10 | `pass` | `fixed20` | `fixed_fallback` | `btc_validation_topk_initial_sl_rate=0.4000>0.3000` | -1.0633% | -0.1851% | `random_forest` | 9 | 9 | -0.1851% | 0.514587 | 55.56% | -0.2670% | 0.026503 | 0.205248% | 0.464585 | 0.4000 | 0.4132 | -0.024089 |
| `2025-10` | `BTCUSDT` | 10 | `pass` | `fixed20` | `fixed_fallback` | `btc_validation_topk_initial_sl_rate=0.3333>0.3000` | 0.6685% | 0.1047% | `logistic` | 4 | 4 | 0.1047% | 1.752798 | 75.00% | -0.1394% | 0.051869 | 0.350997% | 0.342161 | 0.3333 | 0.5253 | 0.065645 |
| `2025-11` | `BTCUSDT` | 10 | `pass` | `hybrid_markov` | `dynamic` | `` | -0.6677% | 0.0000% | `svm_rbf` | 10 | 10 | -0.6677% | 0.719926 | 50.00% | -1.5051% | 0.144080 | 1.073435% | 1.580910 | 0.1250 | 0.3422 | 0.036832 |
| `2025-12` | `BTCUSDT` | 10 | `pass` | `hybrid_markov` | `dynamic` | `` | -1.1887% | 0.0000% | `logistic` | 2 | 2 | -1.1887% | 0.0 | 0.00% | -1.1887% | 0.043391 | 0.306079% | 0.181581 | 0.2222 | 0.3418 | 0.107509 |
| `2026-01` | `BTCUSDT` | 10 | `pass` | `fixed20` | `fixed_fallback` | `btc_validation_topk_initial_sl_rate=0.3333>0.3000` | 0.0199% | 0.0040% | `random_forest` | 1 | 1 | 0.0040% | inf | 100.00% | 0.0000% | 0.134999 | 0.673313% | 2.503609 | 0.3333 | 0.4655 | 0.063175 |
| `2026-02` | `BTCUSDT` | 10 | `pass` | `hybrid_markov` | `dynamic` | `` | 2.8985% | 0.0000% | `logistic` | 10 | 10 | 2.8985% | 2.533226 | 80.00% | -1.1258% | 0.266440 | 1.065263% | 4.261052 | 0.0000 | 0.4235 | 0.074813 |
| `2026-03` | `BTCUSDT` | 10 | `pass` | `hybrid_markov` | `dynamic` | `` | -0.8793% | 0.0000% | `svm_rbf` | 9 | 9 | -0.8793% | 0.596684 | 44.44% | -1.8174% | 0.529081 | 3.965532% | 7.237006 | 0.1000 | 0.3699 | -0.089662 |
| `2026-04` | `BTCUSDT` | 10 | `pass` | `hybrid_markov` | `dynamic` | `` | -1.0491% | 0.0000% | `extra_trees` | 5 | 5 | -1.0491% | 0.252616 | 60.00% | -1.3850% | 0.185030 | 1.368598% | 3.594611 | 0.2857 | 0.6874 | -0.116223 |
