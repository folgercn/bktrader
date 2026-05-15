# Implementation Plan

- [x] R0.1 收敛 requirements 为 research-only 范围，移除当前 spec 内的 live shadow / 灰度 / 全量执行步骤。
- [x] R0.2 修正 `control-reset` 表述：当前 research spec 不得把 `bktrader-ctl live control-reset` 当作 PnL、parity 或 gate failure 的 rollback 流程。
- [x] R0.3 补 Scheme Semantic Contract，明确当前 V4/V6 runner 是 Baseline_Derived_Sizing，不是完整 zero-initial reentry-window lifecycle。
- [x] R0.4 将 Scheme B 升为主线，Scheme A / C 降为 secondary control / fail-fast。

- [x] R1.1 在 `research/probabilistic_v6_walkforward_runner.py` 增加报告字段：
  - `active_silo_sum_pct`
  - `calendar_normalized_return_pct`
  - `active_months`
  - `empty_months`
  - `scheme_semantic_contract`
  - per-row `sizing_mode` / `sizing_fallback_reason`
  - 已完成：当前实现仅扩展 report/schema，不改变 topK selection、模型训练或 execution 结果；BTCUSDT fixed fallback 仍留在 R1.2。

- [x] R1.2 为 Scheme B 增加 BTCUSDT sizing fallback 输出：
  - validation `InitialSL_rate <= 0.30` 时允许 `hybrid_markov`
  - validation `InitialSL_rate > 0.30` 时复跑或输出 fixed 20% 对照
  - 报告必须区分 dynamic result、fixed fallback result、final selected result
  - 已完成：runner 增加 `--btc-sizing-fallback-mode=fixed20_on_initial_sl` 与 `--btc-dynamic-initial-sl-rate-max=0.30`；fallback 触发时写入 dynamic control summary，并用 fixed 20% CSV 作为 final execution。

- [x] R1.3 固化 Scheme B baseline run：
  - `entry_delay_seconds=60`
  - `feature_horizon_seconds=60`
  - `top_k_policy=validation_best`
  - `top_k_selection_metric=return_over_drawdown`
  - `validation_topk_gate_stage=post_selection`
  - `min_validation_topk_return_over_dd=1.0`
  - `max_validation_topk_return_pct=7.0`
  - 覆盖至少 2025-06 至 2026-04 execute months
  - 已完成 full-window 复算：`research/probabilistic_v6_runs/walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback/summary.md`；execute months 覆盖 2025-06 ~ 2026-04。补充说明：旧 Jan-Apr cache 在 2026-04 全月 `trade_count=0`，因此 2026-04 用 fresh cache 重建。

- [x] R1.4 输出 Scheme B baseline markdown：
  - 和已归档 `+6.09%` active silo sum 同表比较
  - 单列 BTCUSDT 2025-12 validation pass / execute loss 的 regime failure
  - 同时报告 Active_Silo_Sum 与 Calendar_Normalized_Return
  - 已完成：`research/20260510_probabilistic_v6_full_window_gate.md` 汇总 full-window baseline、Markov-only gate、R2 constrained gate，并明确当前 runner 仍是 Baseline_Derived_Sizing。

- [x] R2.1 增加 regime/no-trade gate scanner：
  - 仅使用 validation 或 execute 前可观测字段
  - 候选包含 validation topK metrics、prior closed-bar volatility/trend/chop state、model confidence dispersion、InitialSL risk
  - 禁止使用 execute 月 label 或 signal bar 完整 OHLC
  - 已完成：runner 输出 validation topK 特征摘要，scanner 增加 `min_validation_topk_markov_score`；当前最佳诊断 gate 为 `markov_score>=0.3`，过滤 BTCUSDT 2025-12 后 Active_Silo_Sum 从 `+6.0939%` 到 `+6.8839%`。

- [x] R2.2 跑 Scheme B + regime gate full matrix：
  - Gate 目标：过滤 BTCUSDT 2025-12 或同类 validation-pass/execute-loss 月
  - Gate 通过条件：Active_Silo_Sum > `+6.09%`、Calendar_Normalized_Return 改善、PF >= `1.3`、MaxDD <= `3%`、active months >= 6、无 active month `< -2%`
  - 已完成：full-grid 输出 `no_trade_gate_scan_fullgrid.md`；最佳 R2 constrained gate 为 `validation_edge>=0.05` + `validation_return_over_dd<=10` + `validation_markov>=0.4`，Active_Silo_Sum `+6.5516%`、PF `1.5197`、active months `6`。注意：该结果仍未达到 10% ~ 20% 实盘候选目标。

- [x] R2.3 写中文 research 总结：
  - 参数快照
  - month/symbol attribution
  - failed-gate month 列表
  - regime gate 是否真正提升泛化
  - 是否允许另开 live migration spec
  - 已完成：`research/20260510_probabilistic_v6_full_window_gate.md`；结论是不允许进入 live migration spec，只能作为下一轮研究基线。

- [x] R2.4 验证：
  - `python3 -m py_compile` 覆盖改动的 research Python
  - smoke run 证明新增字段稳定
  - full run 后检查 `bars_cache/*.pkl` 不进入 git staging
  - 已完成：`python3 -m py_compile research/probabilistic_v6_walkforward_runner.py research/probabilistic_v6_no_trade_gate_analyzer.py research/probabilistic_v4_event_dataset.py research/probabilistic_v6_execution_labeler.py`；full-window run / gate scan 已产出 summary；当前缓存目录仍为 untracked，未 staging。

- [x] R3.1 跑 all-topK full-window execution matrix：
  - `top_k_values=[5,10,15,20]`
  - `top_k_policy=all`
  - `rank_by=prob_ev_atr`
  - 覆盖 2025-06 至 2026-04 execute months
  - 已完成：`research/probabilistic_v6_runs/walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix/summary.md`；全放行 Active_Silo_Sum `-7.1750%`，单一 topK 最好为 top10 `+0.2632%`。

- [x] R3.2 扫 all-topK validation-only selector：
  - 每个 `(execute_month, symbol)` 最多选择 1 个 sleeve 的可交易口径：`best_validation_per_symbol_month`
  - 目标：Active_Silo_Sum >= `10%`、active months >= `6`、trades >= `40`、worst month >= `-2%`
  - 已完成：`no_trade_gate_scan_alltopk_target.md` 与 `no_trade_gate_scan_alltopk_best_validation_only.md`；可交易口径最好 `+8.3563%` / active months `4`，带 active-month 约束后 `+5.6883%`，未达目标。

- [x] R3.3 写 all-topK 中文总结并更新结论：
  - 已完成：`research/20260510_probabilistic_v6_alltopk_matrix.md`
  - 结论：当前 Original_T2 单事件池的 post-hoc positive oracle 上限也只有 `+10.4570%`，可交易 validation selector 不足以达到 10% ~ 20%；继续阈值微调不再是主线。

- [x] R4.1 扩事件源进入同一 full-window pipeline：
  - 优先把 `baseline_plus_t3` 或其他结构事件接到 execution-labeled dataset / V6 walk-forward。
  - 要求输出和 R3 同口径的 `symbol_rows.csv`、`summary.md`、selector scan。
  - 已完成：`baseline_plus_t3` labeled dataset 共 `1679` 行；validation_best Active_Silo_Sum `-7.5453%`；all-topK Active_Silo_Sum `-50.7477%`；validation-only selector 最佳非空 `+5.7066%` 但 worst month `-2.5470%`；post-hoc oracle positive per symbol-month `+7.6231%`，低于 `10%`。结论：`baseline_plus_t3` 当前扩源失败，不进入下一轮阈值微调。

- [x] R4.2 增强概率模型输入：
  - 补更细的小窗口 tick/order-flow proxy 特征：5s/15s/60s flow transition、速度衰减、touch 后反抽深度、state_seq 压缩 Markov 状态。
  - 不把 execute 月 label 或完整未来 OHLC 放进 selector。
  - 已完成：`probabilistic_v4_event_dataset.py` 新增 `flow_ratio_5s/15s/30s`、`flow_delta_*`、`volume_ratio_*`、`speed_5s_atr`、`speed_decay_*`、`eff_15s`、`close_pos_15s`、`pullback_5s_atr`；`probabilistic_v5_ml_probability_model.py` 已接入这些列；smoke 路径 `research/probabilistic_v6_runs/r4_2_feature_smoke/`。
  - 已完成 full-window rebuild：`research/probabilistic_v6_runs/2025m03_2026apr_original_t2_delay60_r4_2_features/events_execution_labeled.csv`，`1303` 行、`83` 列，事件定义与旧 Original_T2 一致。
  - validation_best 结果：Active_Silo_Sum `-0.2361%`，active months `8`，trades `105`，低于旧 full-window baseline `+2.3601%`。
  - all-topK 结果：Active_Silo_Sum `-57.7621%`，固定 topK 全部为负；可交易 `best_validation_per_symbol_month` selector 最佳非空 `+8.9897%` / active months `4`，qualified `+5.7496%` / active months `6`，未达 `10%` 目标。
  - R4.2 结论：输入增强有诊断价值，但不能继续沿当前阈值路线打磨；下一步转 R4.3 regime classifier / 标签重构。

- [x] R4.3 regime classifier：
  - 在 event/symbol-month 选择前先做 no-trade / tradeable regime 判断。
  - 晋级条件仍是 full-window validation-only selector 达到 10% ~ 20%，且收益不只集中在单一月份。
  - 已完成第一版 symbol-row classifier：`research/probabilistic_v6_regime_classifier.py`，只消费已有 `symbol_rows.csv`，用历史月份训练、上一 execute month 选择模型和阈值、下一 execute month OOS 评分。
  - enhanced all-topK classifier 结果：Active_Silo_Sum `+2.5444%`，active months `4`，trades `40`，target false；old all-topK control 为 `-2.0012%`。
  - event-level feature slice 扫描产出更强上游线索：`BTCUSDT_short & eff_60s:q2<=0.949966` label return `+8.6851%`、14 months、11 positive months、worst month `-1.2413%`。
  - R4.3 结论：symbol-row classifier 不晋级；下一步应固定 event-level slice / regime label 后回到 V6 runner 做真实 execution 验证。

- [x] R4.4 filtered event-slice runner：
  - 将 R4.3 的 top event slices 固定成 train/validation 内可计算规则，生成 filtered event datasets。
  - 优先候选：`BTCUSDT_short & eff_60s_low`、`short & speed_60s_high`、`ETHUSDT_short & prev1_close_pos_side_mid_or_lower`、`BTCUSDT flow_ratio_15s_low`。
  - 按 R3 同口径复跑 validation_best / all-topK / selector scan；只有 validation-only selector 达到 `10% ~ 20%` 且不靠单月集中贡献时才继续 GRU / sequence model。
  - 已完成：4 个 slice 固定为分位数规则（`btc_short_eff60_low`、`short_speed60_high`、`eth_short_range_high`、`btc_flow15_low`），生成 filtered datasets 并复跑 V6 runner 3 种模式。最佳结果为 `short_speed60_high` top10 的 Active_Silo_Sum `+3.46%`（11 active months），仍低于 baseline `+6.09%`，远未达到 `10%` 目标。Label 复盘到 runner 执行的衰减率 61%~93%，证明 event-level slice 过滤不能替代 model selection。结论：不继续 slice 阈值微调；`short_speed60_high` 可回流到 R6.5 union expansion。产出：`research/filtered_event_slices/summary.json` + `research/filtered_event_slices/20260514_r4_4_filtered_event_slices.md`。

## R5 — Scheme D Portfolio Equity Simulator

- [x] R5.1 设计 Scheme D 输入契约与参数快照
  - 对齐 design `## Scheme D — Portfolio Equity Simulator` 输入节；Requirement 3.7。
  - 产出：`research/portfolio_equity_sim/README.md`，列出 lifecycle ledger 路径（主 `union_lifecycle_reentry_window_candidate_001` + `power0_fixed_1p30`、对照 `quality_edge_return_mult_1p20_cap_1p80`），以及 `input_runner_param_snapshot` 必填字段清单（`reentry_size_schedule`、`max_trades_per_bar`、三条 candidate_001 gate 阈值、sizing mode、symbols、execute month 集合）。
  - 验收：README 覆盖所有字段，并显式说明 Simulator 为 Full_Reentry_Window_Lifecycle 的下游审计层（Requirement 3.7、design Scheme Semantic Contract 表的 Scheme D 行）。

- [x] R5.2 实现 Portfolio_Equity_Simulator 最小可运行版本
  - 对齐 design `## Scheme D` 算法 7 步，选变体 B (close-event 瞬时结算) 作为保守默认。
  - 脚本路径：`research/portfolio_equity_simulator.py`（研究层 Python，不进入 `internal/`、`deployments/`、`.github/workflows/`，对齐 Requirement 7.1）。
  - 功能：读 lifecycle ledger CSV → 展开 open/close 事件 → 1s 时间轴聚合 `open_notional_share(t)` → 按 `capital_usage_cap ∈ {1.00, 0.80, 0.60}` 分别输出 CSV + JSON。
  - 跨 symbol 暴露不 net；若 `open_notional_share(t) + slot_share > cap + 1e-9` 则拒绝并累加 `rejected_by_capital_slot_count`。
  - 验收：Python 语法合法 (`python3 -m py_compile`)；对单 trade、跨 symbol 双 trade、slot0+slot1 同 bar 三个 minimal fixture smoke 通过；CSV 列 / JSON 键与 design 指定完全一致。

- [x] R5.3 计算 CAGR / realized_MaxDD / concurrency / 拒绝比
  - 对齐 design `## Scheme D` 算法步骤 6。
  - summary JSON 增补字段：`cagr_pct`、`realized_max_dd_pct`、`concurrency_p50`、`concurrency_p95`、`rejected_by_capital_slot_count`、`total_slot_count`、`rejected_by_capital_ratio`、`per_month_realized_return_pct`、`active_silo_sum_pct`、`active_silo_sum_vs_cagr_relative_diff_pct`。
  - 验收：smoke run 下所有字段非空；`|Active_Silo_Sum - CAGR| / max(|CAGR|, 1e-9)` 和 `rejected_by_capital_ratio` 通过单元测试验证计算正确（对齐 Requirement 6 P13）。

- [x] R5.4 集成 P12 / P13 invariant counters 与 fail-fast
  - 对齐 design `## Scheme D` Fail-Fast 段 + Requirement 6 P12 / P13。
  - summary JSON 增补 `invariant_violations.P12_count`、`invariant_violations.P13_count`、`paper_only`（布尔值，`cap=1.00` 下 `realized_max_dd_pct > 10%` OR `cagr_pct < 10%` 时置 true）。
  - 验收：单元测试覆盖三类 canary：超 cap 行（P12 应记 > 0）、高并发下 CAGR 偏离（P13 应记 > 0）、低收益高 MaxDD（`paper_only` 应为 true）。

- [x] R5.5 对主候选 + 对照各跑一次（cap ∈ {1.00, 0.80, 0.60}）
  - 对齐 design Primary Flow 步骤 6。
  - 运行：`research/portfolio_equity_simulator.py` 对 `power0_fixed_1p30` 主候选 + `quality_edge_return_mult_1p20_cap_1p80` 对照各 3 个 cap 共 6 组。
  - 产出：6 对 CSV + JSON，路径 `research/portfolio_equity_sim/<date>_<scheme>_<cap>.csv|json`。
  - 验收：`cap=1.00` 下主候选 `active_silo_sum_vs_cagr_relative_diff_pct <= 15%` AND `realized_max_dd_pct <= 10%` AND `cagr_pct >= 10%`；否则标 `paper_only=true` 并挂到 R1 gate failure，走 Requirement 4.2 rollback 回 R1。

- [x] R5.6 写 Scheme D research 总结
  - 对齐 design Primary Flow 步骤 6 + Requirement 4.2 R1 gate。
  - 产出：`research/20260512_scheme_d_portfolio_equity_sim.md`（文件名日期占位，tasks 执行日期后再定）。
  - 内容：6 组 run 的对照表（`cap_level`、`cagr_pct`、`realized_max_dd_pct`、`concurrency_p95`、`rejected_by_capital_ratio`、`active_silo_sum_vs_cagr_relative_diff_pct`、`paper_only`）、与 `Calendar_Sum +33.02%` 的差距分析、是否达到 Requirement 4.2 R1 gate。
  - 验收：中文总结明确结论；引用 `research/20260511_probabilistic_v6_calendar_holdout_validation.md` 与 requirements Requirement 3.7 / Requirement 4.2；markdown 文件头声明 "research-only portfolio equity audit, not a live PnL forecast"（对齐 Requirement 7.10）。

## R6 — Scheme B 子线 (B-1 ~ B-7)

- [x] R6.1 Gate_Sensitivity_Grid 扫描器 (B-1)
  - 对齐 design Primary Flow 步骤 7 + Requirement 3.8 / Requirement 6 P14。
  - 脚本：`research/gate_sensitivity_grid.py`，对 candidate_001 三阈值做 `5x5x5` 扫描（`validation_return_over_dd` 中心 `10` ±20%，`validation_topk_sizing_markov_score_mean` 中心 `0.9` ±20%，`validation_topk_sized_return_pct` 中心 `0.5` ±20%）。
  - 产出：`research/gate_sensitivity_grid/<date>_candidate_001.csv`（列：三维阈值 + Calendar_Sum + worst_silo + trade_count）+ `<date>_candidate_001.json`（键：`neighborhood_p5_pct / p50_pct / p95_pct`、per-dimension `partial_dependence_curve`、`invariant_violations.P14_count`）。
  - 验收：P14 counter 为 0 即通过；neighborhood P50 `>= +25%` AND worst-silo P5 `>= -1.0%` 方可作为 R2 gate 通过证据（Requirement 4.3）。

- [x] R6.2 Lifecycle Exit Sweep (B-2)
  - 对齐 design Primary Flow 步骤 8 + Requirement 3.9。
  - 扫描：在 `power0_fixed_1p30` lifecycle 之上对 `trail_start_r ∈ {0.7, 0.9, 1.1}` × `breakeven_at_r ∈ {0.6, 0.8, 1.0}` × `max_hold_hours ∈ {2, 4, 6}` 共 27 组重放 exit（entry / gate / sizing 保持不变）。
  - 产出：`research/lifecycle_exit_sweep/<date>_power0_fixed_1p30.csv|json`；Pareto 集满足 `worst_silo >= -0.20%` AND `calendar_sum_delta >= 0%`。
  - 验收：JSON 中 `pareto_set` 非空；每组输出须包含 `calendar_sum_pct` / `worst_silo_pct` / `trade_count`。

- [x] R6.3 BTC_Only_Regime_Cap 特征工程 + 审计 (B-3)
  - 对齐 design Primary Flow 步骤 9 + design Safety BTC_Only_Regime_Cap 特征审计条款 + Requirement 3.10 / Requirement 6 P4 / P5。
  - 特征仅使用 train/validation 窗口已观测值：`30d_realized_vol_percentile`、validation 期 `InitialSL_rate`、prior-month `realized_trend_pct`（具体列表在 design Primary Flow 步骤 9 已列出）。
  - 每条 per-event 特征导出 CSV MUST 额外写一列 `point_in_time_cutoff`（ISO8601）；禁止使用 execute 月 label 与当前未闭合 signal bar 完整 OHLC。
  - 产出：`research/btc_only_regime_cap.md` + per-event 特征 CSV（`research/btc_only_regime_cap/<date>_features.csv`）。
  - 验收：BTC worst silo 改善至 `>= -0.20%` AND 整体 Calendar_Sum 回撤 `<= 2pp`；否则 markdown 中必须写明量化豁免理由。

- [x] R6.4 Non-Overlapping Historical Extension (B-4)
  - 对齐 design Primary Flow 步骤 10 + Requirement 3.11 / Requirement 6 P10。
  - 在 `2024-01 ~ 2025-05` 复跑 candidate_001 gate 不变；必须 **先** 查 `research/probabilistic_v6_runs/lifecycle_bars_cache/` 与 `dataset/archive` 下 1s tick 是否可用。
  - 产出：`research/historical_extension.md`；若档案可用，附 Active_Silo_Sum / active months / worst active silo；若不可用，`unavailable=true` + 附 archive 查询证据（`bars_cache` 列表、`dataset/archive` zip 清单）。
  - 验收：若可用，要求 Active_Silo_Sum > `+10%` AND 无单 active silo `< -1.5%`；若不可用，证据清单存在且与现有 2025-06~2026-04 窗口非重叠（P10）。

- [x] R6.5 Event-Source Union Expansion (B-5)
  - 对齐 design Primary Flow 步骤 11 + Requirement 3.12。
  - 在 `combo_baseline + short_speed60` 之外，对 `short_speed60_high`、`eth_short_range_high_loose`、`btc_short_eff60_low_loose` 各自独立套用 candidate_001 同款 gate，然后事件级 OR union。
  - 产出：per-slice 单独 gate 后 diff 表 + union 后 calendar 表（`research/event_source_union_expansion/<date>_union.csv|md`）。
  - 验收：`flat_silos <= 8/22` AND worst silo `>= -0.40%`（相对当前 baseline 不劣化）。

- [x] R6.6 Flat_Month_Audit (B-6)
  - 对齐 design Primary Flow 步骤 12 + Requirement 3.13。
  - 对 candidate_001 gate 拒绝的 12 个 flat silos 逐月导出被拒候选事件（含 rejection reason、validation metrics、如直接按 realistic 成本执行的 PnL）。
  - 产出：`research/flat_month_audit.md`；若 realistic PnL 为正的 flat silos `>= 2`，新增旁路条款 `validation_return_over_dd > 10 AND validation_trades_count >= N`（N 按 audit 分布拟定）。
  - 验收：旁路后 worst silo 影响量化 `>= -0.20%`；否则不允许放行。

- [x] R6.7 Taker-Taker Cost Stress (B-7)
  - 对齐 design Primary Flow 步骤 13 + Requirement 3.14。
  - 在 summary 里并排输出 `realistic_taker_both_pct`（`8bps/side` 双边 taker + `2bps/side` slip）。
  - 产出：`research/taker_taker_cost_stress.md`；`cap=1.00` 下至少 `7 / 10` active silos 仍为正。
  - 验收：若少于 `7` active silos 为正，markdown 必须显式开题 maker-rebate / limit-on-touch 作为 R1 子问题，不允许直接推进 R2。

- [x] R6.8 R2 gate 综合评审
  - 对齐 Requirement 4.3 R2 gate criteria（Scheme B-1 邻域合格 + Scheme B-3 BTC worst silo 改善或量化豁免 + Scheme B-4 历史扩展通过或 `unavailable=true`）+ R5.5 Scheme D `cap=1.00` 通过。
  - 产出：`research/20260512_original_t2_r2_gate_review.md`（占位日期），汇总 B-1 ~ B-7 与 R5.5 的 pass/fail，附上所有 summary JSON 的 `invariant_violations.P{12,13,14}_count`。
  - 验收：IF 所有子项通过 AND `invariant_violations.P{12,13,14}_count` 均为 0 AND 主候选 `paper_only=false`, THEN 可在 markdown 中建议另开 live migration spec；否则明确回到 R1 重新 scope。不在本 spec 内启动 live 阶段。

## Task Dependency Graph

```
R5.1 → R5.2 → R5.3 → R5.4 → R5.5 → R5.6
R5.5 → R6.8
R6.1 → R6.8
R6.2 (依赖 R5.5 的 power0_fixed_1p30 lifecycle) → R6.8
R6.3 → R6.8
R6.4 → R6.8
R6.5 → R6.8
R6.6 → R6.8
R6.7 → R6.8
R4.4 (已挂起) 与 R6 并行，不阻塞 R6.8；但若 R4.4 产出新的 slice，可回流到 R6.5
R6.8 → R7.1 (rolling gate 纳入后重跑)
R7.1 → R7.2 → R7.3 → R7.4
```

## Phase R7: Rolling Regime Gate 纳入 candidate_001 Pipeline

> **⚠️ FALSIFIED 2026-05-11**: 路径已证伪，保留为研究过程记录。
> 详见 `research/20260511_remove_candidate_001_falsified.md`。
> 核心原因：rolling gate 在 BTC 2025-08（唯一负 silo）产生 0 FLAT bars，
> 无法替代 candidate_001 的功能。Phase 2 PnL 改善是计算 artifact，不是真实改善。
> Rolling gate 的合理定位是 AUGMENT 层，不是替代层。

~~基于 `.kiro/specs/rolling-regime-gate/` 的 Phase 1 + Phase 2 结论，把 rolling regime gate 正式作为 AUGMENT 层接入 candidate_001 pipeline，重跑 lifecycle 和 R2 gate 评审。~~

- [ ] R7.1 Rolling Gate AUGMENT 模式接入 lifecycle runner
  - 对齐 `rolling-regime-gate` spec Phase 2 结论：`single_24h_abstrend5_consol300` 配置在 AUGMENT 模式下 worst silo 从 -0.40% → -0.04%，Calendar_Sum 代价 -0.13%。
  - 实现：在 `research/probabilistic_v6_union_lifecycle_runner.py` 的 `_run_group()` 中，对 `breakout_gate` 做 rolling regime 过滤（调用 `research/rolling_regime_lifecycle_runner.py` 的 `filter_breakout_gate()`）。
  - 配置：`gate_mode=AUGMENT`，`window_span=24`，`thresholds={realized_trend_pct: (-5.0, 5.0), consolidation_score: (None, 300.0)}`。
  - Walk-forward 选参：每个 execute month 使用前一月 validation 期自动选择 (window_span, threshold)，fallback 为上述默认配置。
  - 产出：新的 lifecycle ledger 目录 `union_lifecycle_reentry_window_candidate_001_rolling_gate/power0_fixed_1p30/`。
  - 验收：新 lifecycle 的 Calendar_Sum >= baseline `+33.02%` 的 95%（即 >= `+31.37%`）AND worst_silo >= `-0.20%`。
  - _Requirements: 3.10 (B-3 BTC worst silo), 4.3 (R2 gate)_

- [ ] R7.2 Scheme D 重跑（rolling gate lifecycle）
  - 对齐 Requirement 3.7 / 4.2。
  - 运行：`research/portfolio_equity_simulator.py` 对 R7.1 产出的 rolling gate lifecycle ledger 跑 `cap ∈ {1.00, 0.80, 0.60}`。
  - 产出：`research/portfolio_equity_sim/20260512_power0_fixed_1p30_rolling_gate_cap_{1p00,0p80,0p60}.{csv,json}`。
  - 验收：`paper_only=false` AND `cagr_pct >= 10%` AND `realized_max_dd_pct <= 10%`。
  - _Requirements: 3.7, 4.2_

- [ ] R7.3 B-1 Gate_Sensitivity_Grid 重跑（rolling gate 口径）
  - 对齐 Requirement 3.8 / 6 P14。
  - 关键改进：把 B-1 scanner 从 per-sleeve one-shot proxy 改为 lifecycle 口径（使用 R7.1 的 rolling gate lifecycle），或把 25% 阈值按 proxy 口径重新标定。
  - 产出：`research/gate_sensitivity_grid/20260512_candidate_001_rolling_gate.{csv,json}`。
  - 验收：neighborhood_p50 >= 重新标定的阈值 AND worst_silo_p5 >= `-1.0%` AND `P14_count=0`。
  - _Requirements: 3.8, 4.3_

- [ ] R7.4 R2 Gate 综合评审（含 rolling gate）
  - 对齐 Requirement 4.3。
  - 汇总 R7.2（Scheme D）+ R7.3（B-1 重跑）+ R6.3（B-3，rolling gate 已替代）+ R6.4（B-4）+ R6.7（B-7）的 pass/fail。
  - 产出：`research/20260512_original_t2_r2_gate_review_v2.md`。
  - 验收：所有 R2 gate criteria 通过 → 建议另开 live migration spec；否则明确回到 R1 并列出具体 blocker。
  - _Requirements: 4.3_
