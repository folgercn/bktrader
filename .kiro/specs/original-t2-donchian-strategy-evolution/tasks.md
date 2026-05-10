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

- [ ] R4.4 filtered event-slice runner：
  - 将 R4.3 的 top event slices 固定成 train/validation 内可计算规则，生成 filtered event datasets。
  - 优先候选：`BTCUSDT_short & eff_60s_low`、`short & speed_60s_high`、`ETHUSDT_short & prev1_close_pos_side_mid_or_lower`、`BTCUSDT flow_ratio_15s_low`。
  - 按 R3 同口径复跑 validation_best / all-topK / selector scan；只有 validation-only selector 达到 `10% ~ 20%` 且不靠单月集中贡献时才继续 GRU / sequence model。
