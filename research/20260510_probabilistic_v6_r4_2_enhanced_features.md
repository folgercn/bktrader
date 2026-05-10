# 2026-05-10 Probabilistic V6 R4.2 输入增强复盘

## 范围

本轮只覆盖 `research`，不改 `live` / `internal` 执行路径。目标是验证：

> 在不丢弃概率模型的前提下，把更细的小窗口 tick/flow proxy 特征接入 Original_T2 full-window pipeline 后，能否把 validation-only selector 推到 `10% ~ 20%` 实盘候选区间？

产物：

- enhanced dataset: `research/probabilistic_v6_runs/2025m03_2026apr_original_t2_delay60_r4_2_features/events_execution_labeled.csv`
- validation_best run: `research/probabilistic_v6_runs/walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback/summary.md`
- validation_best selector scan: `research/probabilistic_v6_runs/walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_postselect_btc_fallback/no_trade_gate_scan_fullgrid.md`
- all-topK matrix: `research/probabilistic_v6_runs/walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix/summary.md`
- all-topK selector scan: `research/probabilistic_v6_runs/walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix/no_trade_gate_scan_alltopk_best_validation_only.md`

当前 runner 仍是 `Baseline_Derived_Sizing`：event selection + 1s execution + `hybrid_markov` / BTC fixed 20% fallback，不是完整 `dir2_zero_initial=true` + `zero_initial_mode=reentry_window` lifecycle。

## 输入变更

R4.2 没有改变事件定义；Original_T2 full-window 行数仍为 `1303`，只是列数从 `67` 增到 `83`：

| Dataset | Rows | Columns | BTCUSDT | ETHUSDT | Months |
|---|---:|---:|---:|---:|---|
| Original_T2 old | 1303 | 67 | 748 | 555 | 2025-03 ~ 2026-04 |
| Original_T2 R4.2 enhanced | 1303 | 83 | 748 | 555 | 2025-03 ~ 2026-04 |
| baseline_plus_t3 R4.1 | 1679 | 67 | 948 | 731 | 2025-03 ~ 2026-04 |

新增或接入 V5 模型的输入包括：

- `flow_ratio_5s` / `flow_ratio_15s` / `flow_ratio_30s`
- `flow_delta_5_60s` / `flow_delta_15_60s` / `flow_delta_30_120s`
- `volume_ratio_5s` / `volume_ratio_15s` / `volume_ratio_60s`
- `speed_5s_atr`
- `speed_decay_5_60s_atr` / `speed_decay_15_60s_atr` / `speed_decay_60_300s_atr`
- `eff_15s`
- `close_pos_15s`
- `pullback_5s_atr`

## validation_best 结果

同 R3 参数口径：

- `entry_delay_seconds=60`
- `feature_horizon_seconds=60`
- `top_k_policy=validation_best`
- `rank_by=prob_ev_atr`
- `validation_topk_gate_stage=post_selection`
- BTC sizing fallback: `fixed20_on_initial_sl`

| Metric | Old Original_T2 | R4.2 Enhanced |
|---|---:|---:|
| Active_Silo_Sum | +2.3601% | -0.2361% |
| Calendar_Normalized_Return | +0.107277% | -0.010732% |
| Active Months | 7 | 8 |
| Active Silos | 11 | 12 |
| Trades | 93 | 105 |
| Worst Active Silo | -2.1443% | -2.9684% |
| Negative Active Silos | 7 | 8 |

R4.2 validation_best 按固定 topK 聚合：

| TopK | Active_Silo_Sum | Active Months | Trades | Worst Active Silo |
|---:|---:|---:|---:|---:|
| 5 | +1.5001% | 3 | 15 | -1.0587% |
| 10 | -3.3414% | 6 | 49 | -2.9684% |
| 15 | -0.1373% | 1 | 7 | -0.1373% |
| 20 | +1.7425% | 2 | 34 | -0.5625% |

validation_best selector scan 的最佳非空为 `+8.5737%`，但 active months 只有 `3`；满足 active_months/trades/worst-month 约束后为 `+5.7748%`，未达到 `10%`。

## all-topK matrix 结果

R4.2 all-topK 全放行明显恶化：

| Metric | Old all-topK | R4.2 all-topK |
|---|---:|---:|
| Active_Silo_Sum | -7.1750% | -57.7621% |
| Calendar_Normalized_Return | -0.326136% | -2.625550% |
| Active Months | 10 | 11 |
| Active Silos | 64 | 68 |
| Trades | 589 | 659 |
| Worst Active Silo | -3.9965% | -7.4177% |
| Negative Active Silos | 42 | 45 |

固定 topK 全部为负：

| TopK | Active_Silo_Sum | Active Months | Trades | Worst Active Silo |
|---:|---:|---:|---:|---:|
| 5 | -5.8930% | 11 | 85 | -4.3009% |
| 10 | -18.5083% | 11 | 154 | -5.9216% |
| 15 | -16.4853% | 11 | 194 | -6.5317% |
| 20 | -16.8755% | 11 | 226 | -7.4177% |

可交易口径 `best_validation_per_symbol_month`：

| Selector | Active_Silo_Sum | Active Months | Rows | Trades | Worst Month | Target Hit |
|---|---:|---:|---:|---:|---:|---|
| best non-empty | +8.9897% | 4 | 6 | 45 | +0.4160% | False |
| qualified constraints | +5.7496% | 6 | 9 | 64 | -1.1777% | False |
| oracle positive per symbol-month | +9.2568% | 6 | 9 | 67 | +0.0469% | post-hoc only |

诊断口径 `all_sleeves` 曾出现 `+23.0362%` qualified result，但它的 `unique_symbol_month_selection=false`，同一 `(execute_month, symbol)` 会重复选择多个 topK sleeve，不能作为可交易组合。scanner 已加上唯一 symbol-month 约束，避免把这种诊断口径误判为 `target_hit`。

## 归因

1. R4.2 特征对 2026-02 / 2026-03 的正收益簇有帮助，但同时把 2025-06、2025-09、2026-04 这类坏 regime 也放大了。
2. validation 月强收益仍会误导 execute 月。例如 2026-04 ETHUSDT validation edge 很高，但 execute 仍为负。
3. Markov / RF / SVM / ExtraTrees 能在局部排序上找到好袖子，但当前输入还不足以可靠判断“这个 symbol-month 是否应该交易”。
4. 事后 oracle positive per symbol-month 也只有 `+9.2568%`，低于 `10%`，说明当前增强特征没有把可交易上限推开。

## 结论

R4.2 判定为“工程完成，但收益目标失败”：

- validation_best：`-0.2361%`
- all-topK：`-57.7621%`
- 可交易 selector 最佳非空：`+8.9897%`，active months 只有 `4`
- 可交易 selector qualified：`+5.7496%`
- post-hoc oracle positive per symbol-month：`+9.2568%`

概率模型继续保留，但不能再按当前路线只调阈值。下一阶段应进入 R4.3：先做 tradeable regime classifier，再让概率模型决定 topK / sizing；同时要把标签从单事件收益推进到 symbol-month 或事件簇是否可交易，而不是继续把所有月份混在一个 selection gate 里。
