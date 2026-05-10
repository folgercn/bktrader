# 2026-05-10 Probabilistic V6 Scheme B Markov Gate 复盘

## 范围

本轮只覆盖 `research`。目标是在不改 live / internal 执行路径的前提下，把概率模型继续用于 Scheme B：

- `entry_delay_seconds=60`
- `feature_horizon_seconds=60`
- `top_k_policy=validation_best`
- `top_k_selection_metric=return_over_drawdown`
- `validation_topk_gate_stage=post_selection`
- `min_validation_topk_return_over_dd=1.0`
- `max_validation_topk_return_pct=7.0`

当前 runner 仍是 Baseline_Derived_Sizing：它使用 event selection + 1s execution + `hybrid_markov` / fixed 20% event sizing，不是完整 `dir2_zero_initial=true` + `zero_initial_mode=reentry_window` + `reentry_size_schedule=[0.20, 0.10]` + `max_trades_per_bar=2` lifecycle。

## 产物

- Baseline rerun: `research/probabilistic_v6_runs/walkforward_delay60_original_t2_feature60_postselect_gate_btc_fallback/summary.md`
- Summary JSON: `research/probabilistic_v6_runs/walkforward_delay60_original_t2_feature60_postselect_gate_btc_fallback/summary.json`
- No-trade gate scan: `research/probabilistic_v6_runs/walkforward_delay60_original_t2_feature60_postselect_gate_btc_fallback/no_trade_gate_scan_markov.md`

## Baseline 复算

当前 labeled dataset 覆盖的 execute months 为 2025-10 ~ 2026-03。

| Metric | Value |
|---|---:|
| Active_Silo_Sum | +6.0939% |
| Calendar_Normalized_Return | +0.507825% |
| Active Months | 5 |
| Empty Months | 1 |
| Active Silos | 5 |
| Trades | 51 |
| Worst Active Silo | -0.7900% |

BTCUSDT 2025-12 仍是核心坏点：validation topK 看起来通过，execute 月却亏损。

| Month | Symbol | TopK | Return | Val Return/DD | Val InitialSL | Val Markov |
|---|---|---:|---:|---:|---:|---:|
| 2025-12 | BTCUSDT | 20 | -0.7900% | 2.429064 | 0.2500 | 0.264106 |

## BTC fixed fallback

新增 BTCUSDT sizing fallback：

- `--btc-sizing-fallback-mode=fixed20_on_initial_sl`
- `--btc-dynamic-initial-sl-rate-max=0.30`

语义：BTC validation topK `InitialSL_rate <= 0.30` 时允许 `hybrid_markov`；若 `InitialSL_rate > 0.30`，runner 会额外保留 dynamic control summary，并用 fixed 20% CSV 作为 final execution。

本次复算中 BTC 2025-12 的 validation topK `InitialSL_rate=0.25`，因此没有触发 fixed fallback。结论是：只靠 InitialSL risk 不能识别 2025-12。

## Markov no-trade gate

新增 validation topK 内部特征摘要，包括：

- `validation_topk_sizing_markov_score_mean`
- `validation_topk_sizing_markov_score_std`
- `validation_topk_prob_success_mean`
- `validation_topk_prob_ev_atr_mean`
- `validation_topk_model_notional_share_mean`

在当前样本上，`min_validation_topk_markov_score >= 0.3` 能过滤 BTCUSDT 2025-12：

| Metric | Baseline | Markov Gate |
|---|---:|---:|
| Active_Silo_Sum | +6.0939% | +6.8839% |
| Calendar_Normalized_Return | +0.507825% | +0.573658% |
| Active Months | 5 | 4 |
| Trades | 51 | 40 |
| Worst Active Month | -0.7900% | +0.3375% |

保留 rows：

| Month | Symbol | TopK | Return | Val Markov |
|---|---|---:|---:|---:|
| 2025-11 | BTCUSDT | 15 | +0.4103% | 0.461801 |
| 2026-01 | BTCUSDT | 20 | +0.3375% | 0.625950 |
| 2026-02 | BTCUSDT | 10 | +3.0090% | 0.652205 |
| 2026-03 | ETHUSDT | 10 | +3.1271% | 0.445521 |

## 结论

概率模型没有丢弃，已经被拆成两个用途：`hybrid_markov` 做仓位评分，`validation_topk_sizing_markov_score_mean` 做 no-trade gate。当前 Markov gate 是有效诊断方向，但还不是 R2 晋级结论，因为 active months 从 5 降到 4，且 Active_Silo_Sum 仍只有 +6.8839%，没有达到 10% ~ 20% 实盘候选区间。

下一步应扩展输入样本与事件来源：至少补 2025-06 ~ 2026-04 完整 execute months，并把同样的 markov/no-trade gate 扫到 `baseline_plus_t3` 或 regime-specific event source 上，而不是继续只在这 5 个 active rows 上调阈值。

## Full-Window 更新

已补齐 full-window 数据并产出独立复盘：`research/20260510_probabilistic_v6_full_window_gate.md`。

关键变化：

- execute months 已扩到 2025-06 ~ 2026-04。
- full-window baseline Active_Silo_Sum 从小窗口 `+6.0939%` 降为 `+2.3601%`，暴露 2025-07 / 2025-08 / 2026-04 的额外亏损。
- Markov-only gate 只能提升到 `+2.7783%`，不够。
- R2 constrained gate (`validation_edge>=0.05`, `validation_return_over_dd<=10`, `validation_markov>=0.4`) 提升到 `+6.5516%`，PF `1.5197`，active months `6`，但仍低于 10% ~ 20% 实盘候选线。

## All-TopK 更新

已完成 all-topK matrix：`research/20260510_probabilistic_v6_alltopk_matrix.md`。

关键结论：

- 展开 `top_k=5/10/15/20` 后，全放行所有 gate-pass sleeve 为 `-7.1750%`。
- 可交易口径 `best_validation_per_symbol_month` 最好只有 `+8.3563%`，active months 只有 `4`；带 active-month 约束后降到 `+5.6883%`。
- 事后 oracle 每个 symbol-month 只挑正收益的上限也只有 `+10.4570%`，说明 Original_T2 单事件池的可交易空间不足。

因此下一阶段不再围绕当前池子继续微调 Markov 阈值，而是把概率模型接到更丰富的事件源和 regime/tick 特征上。

## R4 更新

`baseline_plus_t3` 已接入同一 full-window pipeline，但结果明显失败，详见 `research/20260510_probabilistic_v6_baseline_plus_t3_r4.md`：

- validation_best `-7.5453%`；
- all-topK `-50.7477%`；
- validation-only selector 最佳非空 `+5.7066%` 且不满足 worst-month 约束；
- post-hoc oracle positive per symbol-month `+7.6231%`，低于 `10%`。

当前判断：概率模型应继续使用，但不再消耗时间在 `baseline_plus_t3` 阈值上。

R4.2 增强特征复盘已完成，详见 `research/20260510_probabilistic_v6_r4_2_enhanced_features.md`：

- enhanced Original_T2 validation_best Active_Silo_Sum `-0.2361%`；
- all-topK Active_Silo_Sum `-57.7621%`；
- 可交易 selector 最佳非空 `+8.9897%`，active months 只有 `4`；
- qualified selector `+5.7496%`；
- oracle positive per symbol-month `+9.2568%`，仍低于 `10%`。

结论更新：小窗口输入增强没有把当前路线推到实盘候选线。下一阶段应进入 R4.3 regime classifier / 标签重构。
