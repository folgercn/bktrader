# 2026-05-10 Probabilistic V6 All-TopK Matrix 复盘

## 范围

本轮只覆盖 `research`，不改 `live` / `internal` 执行路径。目标是验证一个关键问题：

> 在保留概率模型的前提下，把 `top_k=5/10/15/20` 全部展开成可选 sleeve 后，验证月字段能否稳定筛出 `10% ~ 20%` 级别候选？

产物：

- matrix run: `research/probabilistic_v6_runs/walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix/summary.md`
- symbol rows: `research/probabilistic_v6_runs/walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix/symbol_rows.csv`
- selector scan: `research/probabilistic_v6_runs/walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix/no_trade_gate_scan_alltopk_target.md`
- best-validation-only scan: `research/probabilistic_v6_runs/walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix/no_trade_gate_scan_alltopk_best_validation_only.md`

参数仍沿用 Scheme B：

- `entry_delay_seconds=60`
- `feature_horizon_seconds=60`
- `top_k_values=[5,10,15,20]`
- `top_k_policy=all`
- `rank_by=prob_ev_atr`
- `validation_topk_gate_stage=post_selection`
- BTC sizing fallback: `fixed20_on_initial_sl`，阈值 `validation_topk_initial_sl_rate > 0.30`

当前 runner 仍是 `Baseline_Derived_Sizing`：event selection + 1s execution + `hybrid_markov` / fixed 20% event sizing，不是完整 `dir2_zero_initial=true` + `zero_initial_mode=reentry_window` lifecycle。

## Matrix 结果

全部 gate-pass sleeve 直接相加是负收益：

| Metric | Value |
|---|---:|
| Active_Silo_Sum | -7.1750% |
| Calendar_Normalized_Return | -0.326136% |
| Active Months | 10 |
| Active Silos | 64 |
| Trades | 589 |
| Worst Active Silo | -3.9965% |
| Negative Active Silos | 42 |

按固定 topK 聚合：

| TopK | Active_Silo_Sum | Trades | Worst Active Silo |
|---:|---:|---:|---:|
| 5 | -3.1564% | 80 | -2.1443% |
| 10 | +0.2632% | 142 | -2.7931% |
| 15 | -0.8732% | 175 | -2.7931% |
| 20 | -3.4086% | 192 | -3.9965% |

结论：扩 sleeve 本身不是收益来源。`top10` 稍好，但也只是接近 0%；不能靠“多选一点/少选一点”把策略推到实盘候选。

## 验证选择器扫描

扫描只使用 validation 月字段，包括：

- `validation_edge`
- `validation_topk_sized_return_pct`
- `validation_topk_initial_sl_rate`
- `validation_topk_max_dd_pct`
- `validation_topk_return_over_dd`
- `validation_topk_sizing_markov_score_mean`

目标约束：

- Active_Silo_Sum >= `10%`
- active_months >= `6`
- trades >= `40`
- worst_month >= `-2%`
- 可交易口径最多每个 `(execute_month, symbol)` 选 1 个 sleeve，即 `best_validation_per_symbol_month`

结果：

| Selector | Active_Silo_Sum | Active Months | Trades | Worst Month | Target Hit |
|---|---:|---:|---:|---:|---|
| best-validation-only unrestricted | +8.3563% | 4 | 48 | -0.7250% | False |
| best-validation with active_months>=6 | +5.6883% | 6 | 59 | -1.3744% | False |
| all_sleeves diagnostic | +18.9059% | 5 | 175 | -3.9965% | False / invalid for trading |
| oracle positive per symbol-month | +10.4570% | 6 | 74 | +0.0469% | post-hoc only |

解释：

- `all_sleeves` 会在同一 `(month, symbol)` 同时计算 top10/top15/top20，实际会重复使用同一批事件，不能作为可交易组合。
- `oracle positive per symbol-month` 使用 execute 月真实收益挑正收益，属于事后上限诊断，不是验证月可执行选择器。
- 可交易口径下，即使放宽 active-month 约束，验证字段最好的组合也只有 `+8.3563%`。
- 当前事件池的 post-hoc 正收益上限也只刚过 `10%`，离 `10% ~ 20%` 稳健候选不够远。

## 关键归因

1. 2025-07 / 2025-08 的 regime failure 仍然会被 validation 字段误选。
2. 2026-03 / 2026-04 出现明显 validation spike：例如 2026-04 ETHUSDT validation edge 很强，但 execute 月为负。
3. `validation_topk_sizing_markov_score_mean` 能帮助过滤一部分坏点，但不是充分条件。
4. BTC fixed fallback 对 2025-08、2025-12、2026-04 这类 dynamic 亏损有减亏价值，但不会创造足够收益。
5. 当前 Original_T2 单事件池里，收益主要集中在 2026-02 / 2026-03；样本覆盖不够宽。

## 结论

概率模型不应丢弃，但当前使用方式要升级：

- 保留概率模型作为 `sleeve selection + sizing + no-trade gate` 的组件。
- 停止在当前 Original_T2 单事件池上继续微调阈值；这个池子的可交易上限已经偏低。
- 下一阶段应扩事件来源和输入特征，而不是继续围绕 `validation_return_over_dd` / Markov 阈值微调。

下一阶段优先级：

1. 将 `baseline_plus_t3` 或其他结构事件纳入同一个 full-window execution-labeled pipeline，扩大可选正收益 sleeve 池。
2. 给 V5/V6 特征补更细的小窗口 tick/盘口代理特征，例如 5s/15s/60s flow transition、速度衰减、touch 后反抽深度、state_seq 压缩 Markov 状态。
3. 将 no-trade gate 从单月 validation topK 指标推进到 regime classifier：先判断月/周/事件簇是否可交易，再决定 topK 和 sizing。
4. 只有当新事件池的 validation-only selector 在 full-window 达到 `10% ~ 20%`，且 cross-asset/year 不靠单月集中贡献时，才允许另开 live migration spec。

## R4.1 更新：baseline_plus_t3 事件源失败

已完成 `baseline_plus_t3` full-window pipeline，详见 `research/20260510_probabilistic_v6_baseline_plus_t3_r4.md`。

关键结果：

- labeled dataset 扩到 `1679` 行，但 validation_best Active_Silo_Sum 为 `-7.5453%`。
- all-topK 全展开为 `-50.7477%`，固定 topK 全部为负。
- all-topK validation-only selector 最佳非空为 `+5.7066%`，但 worst month `-2.5470%`，不满足目标约束。
- post-hoc oracle positive per symbol-month 只有 `+7.6231%`，低于 `10%`。

结论：`baseline_plus_t3` 扩源当前不是收益提升方向。概率模型继续保留，但下一步应切到 R4.2 的小窗口 tick/flow/regime 输入增强。

## R4.2 输入增强结果

已完成增强特征 full-window 复盘，详见 `research/20260510_probabilistic_v6_r4_2_enhanced_features.md`。

关键结果：

- Original_T2 enhanced dataset 保持 `1303` 行，列数从 `67` 增到 `83`；事件定义未变。
- validation_best Active_Silo_Sum 从旧 baseline `+2.3601%` 降到 `-0.2361%`。
- all-topK Active_Silo_Sum 为 `-57.7621%`，固定 topK 全部为负。
- 可交易 `best_validation_per_symbol_month` selector 最佳非空为 `+8.9897%`，但 active months 只有 `4`；满足 active_months>=6 后为 `+5.7496%`。
- post-hoc oracle positive per symbol-month 为 `+9.2568%`，仍低于 `10%`。

结论：R4.2 输入增强有诊断价值，但没有把收益上限推到 `10% ~ 20%`。下一阶段应转 R4.3 regime classifier / 标签重构，而不是继续阈值微调。
