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
| Active_Silo_Sum | 6.0939% |
| Calendar_Normalized_Return | 0.5078% |
| Active Months | 5 |
| Empty Months | 1 |
| Active Silos | 5 |
| Calendar Symbol-Month Count | 12 |
| Trades | 51 |
| Worst Active Silo | -0.7900% |

## Baseline Comparison

| Baseline | Active_Silo_Sum | Active Months | Trades | Delta Active_Silo_Sum |
|---|---:|---:|---:|---:|
| `delay60 + feature60 + post_selection gate` | 6.0939% | 5 | 51 | 0.0000% |

## Portfolio

| Month | TopK | Active | Equal Weight Realistic | Silo Sum Realistic | Symbols |
|---|---:|---:|---:|---:|---|
| `2025-10` | 5 | 0 | 0.0000% | 0.0000% | `` |
| `2025-10` | 10 | 0 | 0.0000% | 0.0000% | `` |
| `2025-10` | 15 | 0 | 0.0000% | 0.0000% | `` |
| `2025-10` | 20 | 0 | 0.0000% | 0.0000% | `` |
| `2025-11` | 5 | 0 | 0.0000% | 0.0000% | `` |
| `2025-11` | 10 | 0 | 0.0000% | 0.0000% | `` |
| `2025-11` | 15 | 1 | 0.4103% | 0.4103% | `BTCUSDT` |
| `2025-11` | 20 | 0 | 0.0000% | 0.0000% | `` |
| `2025-12` | 5 | 0 | 0.0000% | 0.0000% | `` |
| `2025-12` | 10 | 0 | 0.0000% | 0.0000% | `` |
| `2025-12` | 15 | 0 | 0.0000% | 0.0000% | `` |
| `2025-12` | 20 | 1 | -0.7900% | -0.7900% | `BTCUSDT` |
| `2026-01` | 5 | 0 | 0.0000% | 0.0000% | `` |
| `2026-01` | 10 | 0 | 0.0000% | 0.0000% | `` |
| `2026-01` | 15 | 0 | 0.0000% | 0.0000% | `` |
| `2026-01` | 20 | 1 | 0.3375% | 0.3375% | `BTCUSDT` |
| `2026-02` | 5 | 0 | 0.0000% | 0.0000% | `` |
| `2026-02` | 10 | 1 | 3.0090% | 3.0090% | `BTCUSDT` |
| `2026-02` | 15 | 0 | 0.0000% | 0.0000% | `` |
| `2026-02` | 20 | 0 | 0.0000% | 0.0000% | `` |
| `2026-03` | 5 | 0 | 0.0000% | 0.0000% | `` |
| `2026-03` | 10 | 1 | 3.1271% | 3.1271% | `ETHUSDT` |
| `2026-03` | 15 | 0 | 0.0000% | 0.0000% | `` |
| `2026-03` | 20 | 0 | 0.0000% | 0.0000% | `` |

## Symbol Rows

| Month | Symbol | TopK | Gate | Sizing | Final | Fallback | Dynamic Ret | Fixed Ret | Model | Selected | Trades | Realistic | PF | Win | DD | Val Edge | Val TopK Return | Val Ret/DD | Val TopK SL | Val Markov | Test Label Edge |
|---|---|---:|---|---|---|---|---:|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `2025-10` | `BTCUSDT` | 5 | `validation_edge<0.02` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.021875 | -0.138593% | -0.554372 | 0.4000 | 0.4903 | 0.109029 |
| `2025-10` | `BTCUSDT` | 10 | `validation_edge<0.02` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.021875 | -0.215640% | -0.262071 | 0.3750 | 0.4666 | 0.109029 |
| `2025-10` | `BTCUSDT` | 15 | `validation_edge<0.02` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.021875 | -0.215640% | -0.262071 | 0.3750 | 0.4666 | 0.109029 |
| `2025-10` | `BTCUSDT` | 20 | `validation_edge<0.02` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.021875 | -0.215640% | -0.262071 | 0.3750 | 0.4666 | 0.109029 |
| `2025-10` | `ETHUSDT` | 5 | `validation_edge<0.02` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `extra_trees` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.013638 | -0.104626% | -0.191303 | 0.4000 | 0.4438 | 0.059649 |
| `2025-10` | `ETHUSDT` | 10 | `validation_edge<0.02` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `extra_trees` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.013638 | -0.163991% | -0.192799 | 0.4000 | 0.3466 | 0.059649 |
| `2025-10` | `ETHUSDT` | 15 | `validation_edge<0.02` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `extra_trees` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.013638 | -0.074179% | -0.097506 | 0.3636 | 0.3151 | 0.059649 |
| `2025-10` | `ETHUSDT` | 20 | `validation_edge<0.02` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `extra_trees` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.013638 | -0.074179% | -0.097506 | 0.3636 | 0.3151 | 0.059649 |
| `2025-11` | `BTCUSDT` | 5 | `top_k_not_selected:15` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.089323 | 0.273647% | 0.633910 | 0.2000 | 0.5670 | 0.104448 |
| `2025-11` | `BTCUSDT` | 10 | `top_k_not_selected:15` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.089323 | 0.578446% | 1.026320 | 0.2000 | 0.4714 | 0.104448 |
| `2025-11` | `BTCUSDT` | 15 | `pass` | `hybrid_markov` | `dynamic` | `` | 0.4103% | 0.0000% | `gradient_boosting` | 6 | 6 | 0.4103% | 1.505627 | 66.67% | -0.5726% | 0.089323 | 0.784172% | 1.391333 | 0.1818 | 0.4618 | 0.104448 |
| `2025-11` | `BTCUSDT` | 20 | `top_k_not_selected:15` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.089323 | 0.784172% | 1.391333 | 0.1818 | 0.4618 | 0.104448 |
| `2025-11` | `ETHUSDT` | 5 | `validation_edge<0.02` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.077047 | -1.705519% | -0.747449 | 0.6000 | 0.5369 | 0.181436 |
| `2025-11` | `ETHUSDT` | 10 | `validation_edge<0.02` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.077047 | -1.103238% | -0.483498 | 0.4444 | 0.5518 | 0.181436 |
| `2025-11` | `ETHUSDT` | 15 | `validation_edge<0.02` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.077047 | -1.103238% | -0.483498 | 0.4444 | 0.5518 | 0.181436 |
| `2025-11` | `ETHUSDT` | 20 | `validation_edge<0.02` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | -0.077047 | -1.103238% | -0.483498 | 0.4444 | 0.5518 | 0.181436 |
| `2025-12` | `BTCUSDT` | 5 | `top_k_not_selected:20` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.106156 | 1.323080% | 2.195047 | 0.2000 | 0.3898 | -0.005785 |
| `2025-12` | `BTCUSDT` | 10 | `top_k_not_selected:20` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.106156 | 1.139648% | 1.852725 | 0.2000 | 0.2631 | -0.005785 |
| `2025-12` | `BTCUSDT` | 15 | `top_k_not_selected:20` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.106156 | 1.255884% | 2.041689 | 0.2667 | 0.2817 | -0.005785 |
| `2025-12` | `BTCUSDT` | 20 | `pass` | `hybrid_markov` | `dynamic` | `` | -0.7900% | 0.0000% | `logistic` | 11 | 11 | -0.7900% | 0.624833 | 45.45% | -2.0575% | 0.106156 | 1.494166% | 2.429064 | 0.2500 | 0.2641 | -0.005785 |
| `2025-12` | `ETHUSDT` | 5 | `validation_topk_no_candidate` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.048571 | -0.346563% | -0.569190 | 0.6000 | 0.6096 | 0.193370 |
| `2025-12` | `ETHUSDT` | 10 | `validation_topk_no_candidate` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.048571 | 0.260375% | 0.196721 | 0.4000 | 0.3759 | 0.193370 |
| `2025-12` | `ETHUSDT` | 15 | `validation_topk_no_candidate` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.048571 | 0.184246% | 0.130242 | 0.4167 | 0.3280 | 0.193370 |
| `2025-12` | `ETHUSDT` | 20 | `validation_topk_no_candidate` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.048571 | 0.184246% | 0.130242 | 0.4167 | 0.3280 | 0.193370 |
| `2026-01` | `BTCUSDT` | 5 | `top_k_not_selected:20` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `extra_trees` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.123106 | 0.115706% | 0.171035 | 0.4000 | 0.7397 | 0.126375 |
| `2026-01` | `BTCUSDT` | 10 | `top_k_not_selected:20` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `extra_trees` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.123106 | 0.919453% | 1.995791 | 0.2000 | 0.6953 | 0.126375 |
| `2026-01` | `BTCUSDT` | 15 | `top_k_not_selected:20` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `extra_trees` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.123106 | 1.077308% | 1.442910 | 0.2000 | 0.6855 | 0.126375 |
| `2026-01` | `BTCUSDT` | 20 | `pass` | `hybrid_markov` | `dynamic` | `` | 0.3375% | 0.0000% | `extra_trees` | 17 | 16 | 0.3375% | 1.289225 | 75.00% | -0.9409% | 0.123106 | 1.783297% | 2.388487 | 0.2000 | 0.6260 | 0.126375 |
| `2026-01` | `ETHUSDT` | 5 | `validation_topk_return_over_dd<1.0` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.131220 | -0.422232% | -0.161088 | 0.4000 | 0.6825 | 0.232334 |
| `2026-01` | `ETHUSDT` | 10 | `validation_topk_return_over_dd<1.0` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.131220 | 1.221041% | 0.465846 | 0.3000 | 0.5153 | 0.232334 |
| `2026-01` | `ETHUSDT` | 15 | `validation_topk_return_over_dd<1.0` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.131220 | 1.629065% | 0.621513 | 0.3333 | 0.4494 | 0.232334 |
| `2026-01` | `ETHUSDT` | 20 | `validation_topk_return_over_dd<1.0` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.131220 | 1.780046% | 0.679115 | 0.2941 | 0.4026 | 0.232334 |
| `2026-02` | `BTCUSDT` | 5 | `top_k_not_selected:10` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.064319 | -0.033100% | -0.071437 | 0.4000 | 0.6708 | 0.146692 |
| `2026-02` | `BTCUSDT` | 10 | `pass` | `hybrid_markov` | `dynamic` | `` | 3.0090% | 0.0000% | `random_forest` | 10 | 10 | 3.0090% | 2.442726 | 70.00% | -1.1790% | 0.064319 | 1.049322% | 2.264672 | 0.2000 | 0.6522 | 0.146692 |
| `2026-02` | `BTCUSDT` | 15 | `top_k_not_selected:10` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.064319 | 1.077164% | 1.253443 | 0.3333 | 0.6200 | 0.146692 |
| `2026-02` | `BTCUSDT` | 20 | `top_k_not_selected:10` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.064319 | 1.146710% | 1.308117 | 0.3000 | 0.5502 | 0.146692 |
| `2026-02` | `ETHUSDT` | 5 | `validation_topk_return_over_dd<1.0` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.042089 | -0.232335% | -0.387909 | 0.4000 | 0.5027 | 0.077025 |
| `2026-02` | `ETHUSDT` | 10 | `validation_topk_return_over_dd<1.0` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.042089 | 0.462381% | 0.771996 | 0.3333 | 0.5571 | 0.077025 |
| `2026-02` | `ETHUSDT` | 15 | `validation_topk_return_over_dd<1.0` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.042089 | 0.462381% | 0.771996 | 0.3333 | 0.5571 | 0.077025 |
| `2026-02` | `ETHUSDT` | 20 | `validation_topk_return_over_dd<1.0` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `gradient_boosting` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.042089 | 0.462381% | 0.771996 | 0.3333 | 0.5571 | 0.077025 |
| `2026-03` | `BTCUSDT` | 5 | `validation_topk_return>7.0` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.583358 | 4.137918% | 16.551672 | 0.0000 | 0.7659 | -0.179209 |
| `2026-03` | `BTCUSDT` | 10 | `validation_topk_return>7.0` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.583358 | 6.587203% | 26.348812 | 0.1000 | 0.6568 | -0.179209 |
| `2026-03` | `BTCUSDT` | 15 | `validation_topk_return>7.0` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.583358 | 7.040773% | 28.163092 | 0.0909 | 0.5973 | -0.179209 |
| `2026-03` | `BTCUSDT` | 20 | `validation_topk_return>7.0` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `random_forest` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.583358 | 7.040773% | 28.163092 | 0.0909 | 0.5973 | -0.179209 |
| `2026-03` | `ETHUSDT` | 5 | `top_k_not_selected:10` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.592445 | 3.465511% | 6.007105 | 0.2000 | 0.5979 | 0.372527 |
| `2026-03` | `ETHUSDT` | 10 | `pass` | `hybrid_markov` | `dynamic` | `` | 3.1271% | 0.0000% | `logistic` | 8 | 8 | 3.1271% | 7.084294 | 87.50% | -0.5085% | 0.592445 | 5.142537% | 8.914056 | 0.2222 | 0.4455 | 0.372527 |
| `2026-03` | `ETHUSDT` | 15 | `top_k_not_selected:10` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.592445 | 5.142537% | 8.914056 | 0.2222 | 0.4455 | 0.372527 |
| `2026-03` | `ETHUSDT` | 20 | `top_k_not_selected:10` | `hybrid_markov` | `` | `` | 0.0000% | 0.0000% | `logistic` | 0 | 0 | 0.0000% | 0.0 | 0.00% | 0.0000% | 0.592445 | 5.142537% | 8.914056 | 0.2222 | 0.4455 | 0.372527 |
