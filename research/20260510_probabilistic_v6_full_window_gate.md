# 2026-05-10 Probabilistic V6 Full-Window Gate 复盘

## 范围

本轮只覆盖 `research`，不改 `live` / `internal` 执行路径。目标是把概率模型继续保留在 Scheme B 中，并把样本从小窗口扩到完整 walk-forward：

- labeled dataset: `research/probabilistic_v6_runs/2025m03_2026apr_original_t2_delay60/events_execution_labeled.csv`
- baseline run: `research/probabilistic_v6_runs/walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback/summary.md`
- Markov-only scan: `research/probabilistic_v6_runs/walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback/no_trade_gate_scan_markov_only.md`
- full-grid scan: `research/probabilistic_v6_runs/walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback/no_trade_gate_scan_fullgrid.md`
- R2 constrained summary: `research/probabilistic_v6_runs/walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback/r2_constrained_gate_summary.json`

参数仍沿用 Scheme B：

- `entry_delay_seconds=60`
- `feature_horizon_seconds=60`
- `top_k_policy=validation_best`
- `top_k_selection_metric=return_over_drawdown`
- `validation_topk_gate_stage=post_selection`
- `min_validation_topk_return_over_dd=1.0`
- `max_validation_topk_return_pct=7.0`
- BTC sizing fallback: `fixed20_on_initial_sl`，阈值 `validation_topk_initial_sl_rate > 0.30`

当前 runner 仍是 Baseline_Derived_Sizing：event selection + 1s execution + `hybrid_markov` / fixed 20% event sizing，不是完整 `dir2_zero_initial=true` + `zero_initial_mode=reentry_window` lifecycle。

## 数据覆盖

补充了早期和 4 月数据后，combined labeled dataset 共 `1303` 行：

| Window | Rows |
|---|---:|
| 2025-03 ~ 2025-06 fresh | 405 |
| 2025-Q3 existing | 314 |
| 2025-Q4 existing | 293 |
| 2026-01 ~ 2026-03 existing | 194 |
| 2026-04 fresh | 97 |

注意：旧 `2026_jan_apr_original_t2_delay60` 依赖的 Jan-Apr bars cache 在 2026-04 全月 `trade_count=0`，但原始 4 月 zip 有交易。fresh rebuild 后 BTCUSDT 4 月产出 168 个事件，ETHUSDT 4 月产出 157 个事件，因此这次 full-window 用 fresh 4 月标签补齐。

walk-forward 覆盖的 execute months 为：

`2025-06, 2025-07, 2025-08, 2025-09, 2025-10, 2025-11, 2025-12, 2026-01, 2026-02, 2026-03, 2026-04`

## Full-Window Baseline

| Metric | Value |
|---|---:|
| Active_Silo_Sum | +2.3601% |
| Calendar_Normalized_Return | +0.107277% |
| Active Months | 7 |
| Empty Months | 4 |
| Active Silos | 11 |
| Trades | 93 |
| PF | 1.1245 |
| Worst Active Month | -2.8693% |
| Worst Row MaxDD | -2.5014% |

月度归因：

| Month | Return |
|---|---:|
| 2025-07 | -2.8693% |
| 2025-08 | -1.3744% |
| 2025-12 | -0.6031% |
| 2026-01 | +0.2418% |
| 2026-02 | +5.7099% |
| 2026-03 | +2.4720% |
| 2026-04 | -1.2168% |

结论：扩展窗口后，原 Scheme B 明显不是 10% ~ 20% 实盘候选；新增早期月份暴露了 2025-07 / 2025-08 的 regime failure。

## Markov-Only Gate

只用 `validation_topk_sizing_markov_score_mean` 扫 no-trade gate 时，最佳非空结果为：

| Gate | Active_Silo_Sum | Active Months | Trades | Worst Month |
|---|---:|---:|---:|---:|
| `markov >= 0.4` | +2.7783% | 7 | 84 | -2.8693% |

结论：Markov 分数能过滤一部分坏点，但单独使用不够。它没有识别 2025-07 / 2025-08 的主要亏损，也保留了 2026-04 的亏损。

## R2 Constrained Gate

full-grid 中，满足当前 R2 约束的最佳组合是：

- `validation_edge >= 0.05`
- `validation_return_over_dd <= 10`
- `validation_topk_sizing_markov_score_mean >= 0.4`
- 其他字段保持 baseline post-selection 约束

结果：

| Metric | Value |
|---|---:|
| Active_Silo_Sum | +6.5516% |
| Calendar_Normalized_Return | +0.297800% |
| Active Months | 6 |
| Active Silos | 7 |
| Trades | 68 |
| PF | 1.5197 |
| Worst Active Month | -1.3744% |
| Worst Row MaxDD | -2.3154% |

保留 rows：

| Month | Symbol | TopK | Model | Return | Val Edge | Val Return/DD | Val Markov |
|---|---|---:|---|---:|---:|---:|---:|
| 2025-07 | BTCUSDT | 20 | gradient_boosting | -0.7250% | 0.114524 | 4.006239 | 0.565093 |
| 2025-08 | BTCUSDT | 5 | gradient_boosting | -1.3744% | 0.121069 | 5.473764 | 0.519884 |
| 2025-12 | ETHUSDT | 20 | logistic | -0.1849% | 0.126700 | 1.178069 | 0.620777 |
| 2026-01 | BTCUSDT | 10 | gradient_boosting | +0.2418% | 0.185802 | 2.976014 | 0.400074 |
| 2026-02 | BTCUSDT | 10 | logistic | +2.3794% | 0.118453 | 3.023881 | 0.410474 |
| 2026-02 | ETHUSDT | 5 | logistic | +3.3305% | 0.126830 | 4.152484 | 0.427991 |
| 2026-03 | ETHUSDT | 5 | logistic | +2.8842% | 0.374316 | 5.396308 | 0.715808 |

这个 gate 通过当前 R2 表内条件：

- Active_Silo_Sum > +6.09%
- Calendar_Normalized_Return 相对 full-window baseline 改善
- PF >= 1.3
- MaxDD <= 3%
- active months >= 6
- 无 active month < -2%

但它仍然不是实盘候选：Active_Silo_Sum 只有 +6.5516%，离 10% ~ 20% 目标还有明显差距，而且收益主要集中在 2026-02 / 2026-03。

## 结论

概率模型应该保留，但当前用途需要调整：

1. `hybrid_markov` sizing 有价值，但不能单独解决 regime failure。
2. `validation_topk_sizing_markov_score_mean` 可以作为 no-trade gate 的一部分，单独使用太弱。
3. 当前最有效组合是 Markov + validation edge + validation Ret/DD 上限，用来过滤过拟合式 validation spike。
4. 下一阶段不应继续只微调阈值。要冲 10% ~ 20%，需要引入更强输入源或更强模型族，例如：
   - 盘口 / tick 小窗口聚合状态作为 Markov 或 RF/SVM 输入；
   - 按 regime 分层训练，而不是所有月份共用同一选择逻辑；
   - 将 sizing 从单事件 notional share 推进到 reentry-window lifecycle 近似；
   - 将 `baseline_plus_t3` 或更丰富 event source 纳入同一 full-window validation。

当前结论：R2 constrained gate 可以作为下一轮研究基线，但不能进入 live migration spec。

## 下一阶段更新：All-TopK Matrix

已完成后续 all-topK 矩阵复盘：`research/20260510_probabilistic_v6_alltopk_matrix.md`。

关键结果：

- 将 `top_k=5/10/15/20` 全部展开后，直接放行所有 gate-pass sleeve 为 `-7.1750%`。
- 单一 topK 固定组合中，最好的是 top10，但 Active_Silo_Sum 也只有 `+0.2632%`。
- 可交易口径 `best_validation_per_symbol_month` 的验证选择器最好只有 `+8.3563%`，active months 只有 `4`。
- 加上 `active_months>=6`、`trades>=40`、`worst_month>=-2%` 后，最好结果降到 `+5.6883%`。
- 事后 oracle 每个 symbol-month 只挑正收益 sleeve 的上限为 `+10.4570%`，但这是 execute 月 hindsight，不能作为可交易 selector。

结论更新：当前 Original_T2 单事件池的可交易上限太窄，继续调阈值没有意义。概率模型继续保留，但下一阶段应扩事件源和输入特征，优先把 `baseline_plus_t3` 或更丰富的 tick/order-flow regime 特征接入同一 full-window pipeline。

## R4 更新

已完成 `baseline_plus_t3` 扩事件源复盘：`research/20260510_probabilistic_v6_baseline_plus_t3_r4.md`。

结果显示扩源本身失败：

- validation_best Active_Silo_Sum `-7.5453%`；
- all-topK Active_Silo_Sum `-50.7477%`；
- validation-only selector 最佳非空 `+5.7066%`，但 worst month `-2.5470%`；
- oracle positive per symbol-month `+7.6231%`，事后上限也低于 `10%`。

因此下一步不再继续 `baseline_plus_t3` 阈值搜索。

R4.2 增强特征复盘也已完成，详见 `research/20260510_probabilistic_v6_r4_2_enhanced_features.md`：

- enhanced Original_T2 dataset 仍为 `1303` 行，但列数从 `67` 增到 `83`；
- validation_best Active_Silo_Sum 为 `-0.2361%`；
- all-topK Active_Silo_Sum 为 `-57.7621%`；
- 可交易 selector 最佳非空 `+8.9897%` 但 active months 只有 `4`，qualified 为 `+5.7496%`；
- oracle positive per symbol-month `+9.2568%`，低于 `10%`。

结论更新：概率模型继续保留，但当前路线不应继续阈值打磨；下一阶段应转 R4.3 regime classifier / 标签重构。
