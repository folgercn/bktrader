# Design Document

## Scope

本 design 只覆盖 research-only 的 R0/R1/R2。它不修改 `internal/`、`live`、`deployments/`、`.github/workflows/`，不生成 live `session.config`，不定义 live shadow / 灰度 / 全量流程。若 R2 通过，后续再另开 live migration spec。

AGENTS §2 Core Memory 的 Research_Baseline 固定为：`dir2_zero_initial=true`、`zero_initial_mode=reentry_window`、`reentry_size_schedule=[0.20, 0.10]`、`max_trades_per_bar=2`。当前 V4/V6 概率 runner 默认不是完整 reentry-window lifecycle，而是 event selection + 单次 1s execution + fixed `notional_share=0.20` 或 `model_notional_share` dynamic sizing；所有报告必须显式标注这一点。

当前已知 research baseline：`delay60 + feature60 + post_selection gate` 在 5 个 active months silo sum 为 `+6.09%`，尚未达到可下沉 live 的要求。本 design 的目标是用 Scheme B 的 regime/no-trade gate 去解释并过滤 BTCUSDT 2025-12 这类 validation pass 但 execute loss 的状态。

## Scheme Semantic Contract

| Scheme | Priority | Entry Source | Feature Horizon | Execution | Sizing | Lifecycle Claim | Current Gap |
|---|---:|---|---|---|---|---|---|
| B: `delay60 + feature60 + post_selection + regime_gate` | 1 | true `original_t2` intrabar touch | `feature_horizon_seconds=60 <= entry_delay_seconds=60` | V4 1s execution runner | ETH `hybrid_markov`; BTC dynamic only if validation InitialSL gate passes, otherwise fixed 20% | Baseline_Derived_Sizing | Need per-symbol fallback output and normalized portfolio metrics |
| A: `pretouch fast-clean + V4 probability` | 2 | true `original_t2` pre-touch state band | 5s family only | V4 1s execution runner | ETH optional dynamic; BTC fixed 20% | Baseline_Derived_Sizing | Secondary control only; not main path |
| C: `pretouch fast-clean + structure exit` | 3 | same as A | n/a | structure exit replay | fixed 20% only | Baseline_Derived_Sizing | Fail-fast only; small-sample overfit risk |
| D: `Portfolio_Equity_Simulator + Real Capital Concurrency` | 1 (validator) | Scheme B lifecycle ledger (candidate_001 + `power0_fixed_1p30` 主 / `quality_edge_return_mult_1p20_cap_1p80` 对照) | n/a（下游审计，不再做 entry 判定） | 1s 时间轴模拟 open/close 事件 + `capital_usage_cap ∈ {1.00, 0.80, 0.60}` 约束 | 继承输入 ledger 的 `reentry_size_schedule=[0.20, 0.10]` / `max_trades_per_bar=2` slot 约束，跨 symbol 暴露不互相抵消 | Full_Reentry_Window_Lifecycle 下游审计层（**不是** Baseline_Derived_Sizing，其输入就是完整 lifecycle ledger） | 当前 runner 只输出 Calendar_Sum 与 per-silo return，没有 1s 级资金占用 / 并发上限 / 真实 MaxDD 计算，Calendar_Sum `+33.02%` 无法解读为组合级 CAGR |

No scheme in this spec may claim Full_Reentry_Window_Lifecycle unless the runner explicitly implements current + next signal-bar reentry windows and slot0/slot1 lifecycle. Scheme D 本身不产生 lifecycle，而是对 Full_Reentry_Window_Lifecycle 的 ledger 做 capital-concurrency audit（对齐 Requirement 3.7、Requirement 6 P12/P13）。

## Scheme D — Portfolio Equity Simulator

本节对齐 Requirement 3.7、Requirement 4.2 (R1 gate)、Requirement 6 P12 / P13 与 Requirement 7.10，回答一个具体问题：Calendar_Sum `+33.02%`（源 `research/20260511_probabilistic_v6_calendar_holdout_validation.md`，`power0_fixed_1p30` 主候选；对照 `quality_edge_return_mult_1p20_cap_1p80` Calendar_Sum `+33.41%`）在资金占用 / 并发 / 真实 MaxDD 约束下是否仍可解读为组合级 CAGR。Calendar_Sum 是 22 symbol-months 的 simple sum，不是权益曲线。

### 输入

- Lifecycle ledger：`research/probabilistic_v6_union_lifecycle_runner.py` 产出的 `union_lifecycle_reentry_window_candidate_001` 目录下的 `execute_<month>/<symbol>/lifecycle_ledger.csv`，主候选 `power0_fixed_1p30`、对照 `quality_edge_return_mult_1p20_cap_1p80`。
- Runner 参数快照：Scheme D 的 `portfolio_equity_summary.json` MUST 内嵌来自 runner 的 `input_runner_param_snapshot`，至少包含 `reentry_size_schedule`、`max_trades_per_bar`、`gate` (三条 candidate_001 阈值)、sizing mode（`power0_fixed_1p30` / `quality_edge_return_mult_1p20_cap_1p80`）、`symbols`（`BTCUSDT` / `ETHUSDT`）、execute month 集合（当前默认 `2025-06 ~ 2026-04`）。这是审计溯源要求，不允许省略。
- Scheme B-4 Historical Extension 可后置注入同格式 ledger（`2024-01 ~ 2025-05`，或标注 `unavailable=true`，见 Primary Flow 步骤 10）。

### 算法（pseudocode-level，不是代码）

1. 构建 1s 时间轴 `T = [2025-06-01T00:00:00Z, 2026-04-30T23:59:59Z]`（与 lifecycle ledger 的 execute window 一致；B-4 启用时再向左延伸，保持 P10 walk-forward 非重叠）。
2. 对每个 lifecycle ledger 中的 trade，生成 `open_event(ts_open, symbol, slot_index ∈ {0,1}, notional_share)` 与 `close_event(ts_close, symbol, slot_index, realized_r_multiple)`；`slot_index=0` 对应 `reentry_size_schedule[0]=0.20`，`slot_index=1` 对应 `reentry_size_schedule[1]=0.10`，与输入 ledger 的 slot 约束对齐（同一 signal bar 内 real-entry <= 2，对齐 Requirement 6 P2）。
3. 维护聚合 `open_notional_share(t) = Σ 所有当前 open slot 的 notional_share`，跨 BTCUSDT + ETHUSDT **不互相抵消**（跨 symbol 暴露不 net）。
4. 资金门控：对每个 `open_event`，若 `open_notional_share(t) + slot_share > capital_usage_cap`，则 **拒绝** 该 slot，`rejected_by_capital_slot_count += 1`，并在 per-event trace 中标注 `rejected_by=capital`；若未超，则放行，`accepted_slot_count += 1`。
5. Equity 积累（两种变体，design 层明确选 **保守** 变体）：
   - 变体 A（按 1s 连续复利）：把每个 accepted slot 的 realized R-multiple 均匀分摊到其 `[ts_open, ts_close]` 的每秒，等价于对 `(1 + per-second return)` 做离散复利。
   - 变体 B（在 close 事件瞬时结算）：`equity(t_close) = equity(t_close^-) * (1 + slot_share * realized_r_multiple * per_r_dollar_ratio)`，在 open 期间 equity 不动。
   - 本 design 选用 **变体 B（close-event 瞬时结算）** 作为默认；原因：realized R-multiple 已经包含完整持仓期 PnL，变体 A 需要额外分解 intrabar PnL 曲线，容易把 1s 噪声 artifact 当成组合波动，偏乐观；close-event 瞬时结算在 MaxDD 方向更保守，符合 AGENTS §9 的 "成功/失败路径必须统一 accounting" 精神。变体 A 作为对照，只在 R2 阶段需要与实盘 intraday exposure 比对时开启。
6. 记录 `realized_max_dd`（按权益曲线 rolling max）、`cagr_pct`（按 `T` 总长度年化）、`concurrency_p50 / p95`（基于 accepted slot 的 1s 并发计数分布）。
7. 对 `cap_level ∈ {1.00, 0.80, 0.60}` 各跑一次，每个 cap 产出独立的 CSV + JSON（不要在同一个 CSV 混 cap）。

### 输出

- CSV：`research/portfolio_equity_sim/<date>_<scheme>_<cap>.csv`，列：`ts, cap_level, equity, open_notional_share, rejected_slot_count_cumulative, realized_month_return_to_date`。
- JSON：`research/portfolio_equity_sim/<date>_<scheme>_<cap>.json`，键：`scheme_id`、`input_ledger_path`、`input_runner_param_snapshot`、`cap_level`、`cagr_pct`、`realized_max_dd_pct`、`concurrency_p50`、`concurrency_p95`、`rejected_by_capital_slot_count`、`total_slot_count`、`rejected_by_capital_ratio`、`per_month_realized_return_pct`、`active_silo_sum_pct`、`active_silo_sum_vs_cagr_relative_diff_pct`、`invariant_violations.P12_count`、`invariant_violations.P13_count`。
- 路径为 design-level 约定，最终 tasks 固化时可追加 `scheme_id` / `ledger_hash` 等字段，但 CSV 列与 JSON 键 **只能扩展不能删减**（对齐 Requirement 6 P9 round-trip）。

### 安全约束（Safety）

- Scheme D Simulator **MUST NOT** 生成 `session.config`、sleeve multiplier、dispatch 配置或任何 control-plane 命令（对齐 Requirement 7.10 与 Requirement 6 P8）。
- Scheme D Simulator **MUST NOT** 被解读为 live PnL 预测；输出文件头 / markdown 总结 MUST 显式写 "research-only portfolio equity audit, not a live PnL forecast"。
- 文件 MUST 只写入 `research/portfolio_equity_sim/` 目录，禁止写入 `internal/`、`deployments/`、`.github/workflows/` 或任何 live `session.config` 路径（AGENTS §3 高风险禁区）。

### Fail-Fast

IF `cap=1.00` 下 `realized_max_dd_pct > 10%` OR `cagr_pct < 10%`, THEN Scheme D 输出 MUST 在 summary JSON 中标注 `paper_only=true`，并阻塞 Requirement 4.3 的 R2 晋级（对齐 Requirement 3.7 fail-fast 条款）。

### 引用不变量

- Requirement 6 P12 (Capital Concurrency Bound)：`open_notional_share(t) <= capital_usage_cap`，违反计入 `invariant_violations.P12_count`。
- Requirement 6 P13 (Active_Silo_Sum vs CAGR Consistency)：`|Active_Silo_Sum - CAGR| / max(|CAGR|, 1e-9) <= 0.15`，违反计入 `invariant_violations.P13_count`；两者必须在 summary markdown 中同时报告。

## Primary Flow

1. R0 hardening: keep requirements/design/tasks aligned on research-only scope, scheme contract, metric definitions, and non-goals.
2. R1 implementation: update the Scheme B runner/reporting surface so each run emits Active_Silo_Sum, Calendar_Normalized_Return, active/empty months, per-symbol sizing mode, and regime gate details.
3. R1 run: rerun Scheme B over at least 2025-06 through 2026-04 execute months with the current `+6.09%` post-selection gate as the baseline row.
4. R2 regime gate: add validation-only or pre-execute observable regime/no-trade gates, then rerun and compare against baseline. Gate candidates may use validation top-K metrics, prior closed-bar volatility state, prior-month realized trend/chop statistics, and model confidence dispersion. They must not use execute labels or complete current signal-bar OHLC.
5. R2 decision: promote only if Active_Silo_Sum and Calendar_Normalized_Return improve, PF >= 1.3, MaxDD <= 3%, active months >= 6, and no active month is below -2%.
6. **Scheme D — Portfolio Equity Simulator（对齐 Requirement 3.7 / Requirement 4.2 R1 gate）**：对 Scheme B 当前主候选 `candidate_001 + power0_fixed_1p30` 以及对照 `candidate_001 + quality_edge_return_mult_1p20_cap_1p80` 的 lifecycle ledger（来自 `research/probabilistic_v6_union_lifecycle_runner.py` 的 `union_lifecycle_reentry_window_candidate_001` 输出），在 `cap_level ∈ {1.00, 0.80, 0.60}` 下各跑一次 Portfolio_Equity_Simulator，得到 `portfolio_equity_curve.csv` 与 `portfolio_equity_summary.json`；输出挂在 R1 gate 检查（对齐 Requirement 4.2：`Active_Silo_Sum` 与 Scheme D 输出的 `CAGR` 相对差必须 <= `15%`，否则 lifecycle 视为 "paper-only"，不允许进入 R2）。
7. **Scheme B-1 Gate_Sensitivity_Grid（对齐 Requirement 3.8 / 6 P14）**：以 candidate_001 的三阈值（`validation_return_over_dd <= 10`、`validation_topk_sizing_markov_score_mean <= 0.9`、`validation_topk_sized_return_pct >= 0.5`）为中心做 `5x5x5` 扫描（每维 ±20%），输出 `gate_sensitivity_grid.csv`（列：三维阈值 + Calendar_Sum + worst_silo + trade_count）以及在对应 summary JSON 中追加 `neighborhood_p5_pct / p50_pct / p95_pct` 与 per-dimension `partial_dependence_curve`；用于判定 Calendar_Sum `+33.02%` 不是单点过拟合。
8. **Scheme B-2 Lifecycle Exit Sweep（对齐 Requirement 3.9）**：在 `power0_fixed_1p30` lifecycle 之上对 `trail_start_r ∈ {0.7, 0.9, 1.1}` × `breakeven_at_r ∈ {0.6, 0.8, 1.0}` × `max_hold_hours ∈ {2, 4, 6}` 做 3x3x3 扫描（entry/gate/sizing 保持不变），在 summary 中列出满足 `worst_silo >= -0.20%` AND `calendar_sum_delta >= 0%` 的 Pareto 集，并按 Calendar_Sum/worst_silo 双目标排序归档。
9. **Scheme B-3 BTC_Only_Regime_Cap（对齐 Requirement 3.10 / Requirement 4.3 R2 gate）**：仅对 BTCUSDT 增加 per-symbol cap 或 regime no-trade；候选特征仅使用 train/validation 窗口已观测量，例如 `30d_realized_vol_percentile`、validation 期 `InitialSL_rate`、prior-month `realized_trend_pct`；**显式禁用** execute 月标签以及当前未闭合 signal bar 的完整 OHLC（对齐 AGENTS §2 Breakout Structure Semantics、Requirement 6 P4/P5）。输出 `btc_only_regime_cap.md`，量化 BTC worst silo delta 与整体 Calendar_Sum 回撤（必须 <= `2pp`）；若无法满足，MUST 在 design/报告中给出量化豁免理由，不允许留白。
10. **Scheme B-4 Non-Overlapping Historical Extension（对齐 Requirement 3.11 / Requirement 4.3 R2 gate）**：在 `2024-01 ~ 2025-05` 非重叠窗口上保持 candidate_001 gate 不变复跑；若 `research/probabilistic_v6_runs/.../bars_cache/` 或 `dataset/archive` 下的 1s tick 在该窗口不可用，MUST 附 archive 查询证据（`bars_cache` 目录列表、`dataset/archive` zip 清单）并在 `historical_extension.md` 中显式标注 `unavailable=true`，不允许用与现有 calendar 重叠的窗口冒充外推。
11. **Scheme B-5 Event-Source Union Expansion（对齐 Requirement 3.12）**：在现有 `combo_baseline + short_speed60` 之外，对 `short_speed60_high`、`eth_short_range_high_loose`、`btc_short_eff60_low_loose` 等 slice 各自独立套用 candidate_001 同款 gate，然后在事件级做 OR union；目标 `flat_silos <= 8/22` AND worst silo 不劣化（相对 `power0_fixed_1p30` baseline 的 `-0.40%`，即新 worst silo 不低于 `-0.40%`）。输出包含 per-slice 单独 gate 后的 diff 表与 union 后 calendar 表。
12. **Scheme B-6 Flat_Month_Audit（对齐 Requirement 3.13）**：对 candidate_001 gate 拒绝的 12 个 flat silos 逐月导出被拒候选事件（含 rejection reason / validation metrics / 直接 realistic 执行 PnL），产出 `flat_month_audit.md`；若 realistic PnL 为正的 flat silos >= 2，则在 design 中新增旁路条款（建议 `validation_return_over_dd > 10 AND validation_trades_count >= N`，N 在 audit 中按样本分布拟定），并量化旁路后对 worst silo 的影响，要求保持 `>= -0.20%`，否则不允许放行。
13. **Scheme B-7 Taker-Taker Cost Stress（对齐 Requirement 3.14）**：在 summary 中并排输出 `realistic_taker_both_pct`（`8bps/side` 双边 taker + `2bps/side` slip），要求 `cap=1.00` 下至少 `7 / 10` active silos 仍为正；若少于 7 个 active silos 为正，MUST 回到 R1 开题 maker-rebate / limit-on-touch 子问题（挂入独立 research PR），不允许直接推进 R2。报告归档为 `taker_taker_cost_stress.md`。

## Metrics

All Scheme B reports must include:

- `active_silo_sum_pct`: simple sum across active symbol-month silos.
- `calendar_normalized_return_pct`: fixed calendar/symbol grid return with empty silos counted as 0%.
- `active_months`: number of execute months with at least one active symbol.
- `empty_months`: execute months with no active symbols.
- `symbol_silo_rows`: per symbol/month/topK rows with gate reason, sizing mode, selected events, trades, realistic return, PF, win rate, MaxDD, validation topK fields, and regime gate fields.
- `baseline_comparison`: delta versus the archived `delay60 + feature60 + post_selection gate` result (`+6.09%` active silo sum over 5 active months).

**Scheme B 每次 run（含 B-1 ~ B-7 与主 lifecycle）额外 MUST 输出**（对齐 Requirement 3.4、Requirement 4.2 / 4.3、Requirement 6 P12 / P13 / P14，基准 `+33.02%` 语境来自 `research/20260511_probabilistic_v6_calendar_holdout_validation.md`）：

- `cagr_pct`：来自 Scheme D `portfolio_equity_summary.json`，按 cap 分组。
- `realized_max_dd_pct`：来自 Scheme D，按 cap 分组；替代旧的 per-silo MaxDD 汇总读法。
- `concurrency_p95`：1s 并发 slot 数 95 分位，来自 Scheme D。
- `rejected_by_capital_ratio`：`rejected_by_capital_slot_count / total_slot_count`，来自 Scheme D；P12 失败信号。
- `calendar_sum_pct`：22 symbol-months simple sum（或 B-4 扩展后的新 calendar 总长），保留与 `+33.02%` 的可比性。
- `flat_silo_count`：当前值基准为 `12/22`（主候选）；B-5 / B-6 目标收敛至 `<= 8/22`。
- `worst_silo_pct`：最差 active silo 收益，基准 `-0.40%`（`2025-08 BTCUSDT`，主候选）。
- `taker_both_realistic_pct`：B-7 双边 taker + `2bps` slip 的 realistic sum；cap=1.00 下 active 正 silo 数必须 `>= 7`。
- `active_silo_sum_vs_cagr_relative_diff_pct`：`|Active_Silo_Sum - CAGR| / max(|CAGR|, 1e-9)`；R1 gate 要求 `<= 15%`（Requirement 4.2 / Requirement 6 P13）。
- `gate_sensitivity_neighborhood_p50_pct`：B-1 Gate_Sensitivity_Grid 邻域 P50；R2 gate 要求 `>= +25%`（Requirement 3.8 / Requirement 6 P14）。
- `gate_sensitivity_worst_silo_p5_pct`：B-1 邻域 worst-silo P5；R2 gate 要求 `>= -1.0%`（Requirement 3.8 / Requirement 6 P14）。

**File contract paths**（design-level 固化，tasks 只能在其下追加 `scheme_id`/`date` 前缀，不得迁出该根目录）：

- `research/portfolio_equity_sim/<date>_<scheme>_<cap>.csv` + `.json`（Scheme D 主产物，对齐 Primary Flow 步骤 6）。
- `research/gate_sensitivity_grid/<date>_<scheme>.csv` + `.json`（B-1，对齐 Primary Flow 步骤 7）。
- `research/lifecycle_exit_sweep/<date>_<scheme>.csv` + `.json`（B-2，对齐 Primary Flow 步骤 8）。
- `research/btc_only_regime_cap.md`（B-3，对齐 Primary Flow 步骤 9）。
- `research/historical_extension.md`（B-4，对齐 Primary Flow 步骤 10；若 archive 不可用，文件内 `unavailable=true`）。
- `research/flat_month_audit.md`（B-6，对齐 Primary Flow 步骤 12）。
- `research/taker_taker_cost_stress.md`（B-7，对齐 Primary Flow 步骤 13）。

## Safety

The research harness must not emit live config or operational commands. In particular, `bktrader-ctl live control-reset` is out of scope; it is an exceptional production repair tool, not a research rollback primitive.

- **Scheme D 输出与 live 产物严格隔离**：Scheme D Simulator 的 CSV / JSON / markdown MUST 只写入 `research/portfolio_equity_sim/` 目录；design 层保证本 spec 的任何工作流都 NOT 在 `internal/`、`deployments/`、`.github/workflows/`、或任何 live `session.config` 路径下写入文件；同时 Scheme D 输出 NOT 等价于 live PnL 预测（对齐 Requirement 7.10、Requirement 6 P8、AGENTS §3 高风险禁区）。
- **BTC_Only_Regime_Cap 特征审计**：Scheme B-3 的特征工程 MUST 经过显式审计，确认没有使用 execute 月标签或当前未闭合 signal bar 的完整 OHLC（对齐 AGENTS §2 Breakout Structure Semantics、Requirement 6 P4 / P5）；每条 per-event 特征导出文件 MUST 额外写入一列 `point_in_time_cutoff`（ISO8601 timestamp，等于特征可用截止时间），供审计直接查表，不允许留白。
- **Historical Extension 非重叠约束**：Scheme B-4 的 `2024-01 ~ 2025-05` 历史扩展 MUST NOT 与现有 `2025-06 ~ 2026-04` walk-forward window 合并或叠加（对齐 Requirement 6 P10）；若档案不可用，MUST 以 `unavailable=true` 明确标注，而不是拿重叠窗口冒充外推。

## Gate Mapping

本节把 requirements.md 新增 / 修订的条款一一映射到本 design 中的落地位置，便于 R0 review 对齐。基准 `+33.02%` 的引用均回到 `research/20260511_probabilistic_v6_calendar_holdout_validation.md`。

| Requirement 条款 | Design 落地 | 判定条件 / 产物 |
|---|---|---|
| R3.7 Scheme D | `## Scheme D — Portfolio Equity Simulator` + Primary Flow 步骤 6 | `portfolio_equity_curve.csv` + `portfolio_equity_summary.json`，cap ∈ `{1.00, 0.80, 0.60}`，`input_runner_param_snapshot` 必须内嵌 |
| R3.8 B-1 Gate_Sensitivity_Grid | Primary Flow 步骤 7 | `research/gate_sensitivity_grid/*.csv` + summary JSON 中 `neighborhood_p5/p50/p95` 与 per-dimension `partial_dependence_curve` |
| R3.9 B-2 Lifecycle Exit Sweep | Primary Flow 步骤 8 | `research/lifecycle_exit_sweep/*.csv|json`，含 Pareto 集（`worst_silo >= -0.20%` AND `calendar_sum_delta >= 0%`） |
| R3.10 B-3 BTC_Only_Regime_Cap | Primary Flow 步骤 9 | `research/btc_only_regime_cap.md`，含特征 `point_in_time_cutoff` 审计列；BTC worst silo delta 与 Calendar_Sum 回撤量化 |
| R3.11 B-4 Non-Overlapping Historical Extension | Primary Flow 步骤 10 | `research/historical_extension.md`；若档案不可用，`unavailable=true` + `bars_cache` / archive 清单证据 |
| R3.12 B-5 Event-Source Union Expansion | Primary Flow 步骤 11 | per-slice 单独 gate 后 diff 表 + union 后 calendar 表；`flat_silos <= 8/22` AND worst silo `>= -0.40%` |
| R3.13 B-6 Flat_Month_Audit | Primary Flow 步骤 12 | `research/flat_month_audit.md`，含旁路条款规格（`validation_return_over_dd > 10 AND validation_trades_count >= N`）与 worst-silo 影响量化 |
| R3.14 B-7 Taker-Taker Cost Stress | Primary Flow 步骤 13 | `research/taker_taker_cost_stress.md`，`cap=1.00` 下 active 正 silo `>= 7` |
| R4.2 R1 gate | Scheme D `cap=1.00` 输出 + `active_silo_sum_vs_cagr_relative_diff_pct <= 15%` | summary JSON + markdown 同表比较，否则 `paper_only=true` |
| R4.3 R2 gate | B-1 邻域合格（P50 `>= +25%` AND worst-silo P5 `>= -1.0%`）+ B-3 BTC worst silo `>= -0.20%`（或 design 量化豁免）+ B-4（或 `unavailable=true`） | 对应 B-1 / B-3 / B-4 产物集中汇总 |
| R6.12 P12 Capital Concurrency Bound | Scheme D summary JSON `invariant_violations.P12_count` + `rejected_by_capital_ratio` | 每次 run P12_count 必须为 0 才能通过 gate |
| R6.13 P13 Active_Silo_Sum vs CAGR Consistency | Scheme D summary JSON `invariant_violations.P13_count` + `active_silo_sum_vs_cagr_relative_diff_pct` | 每次 run P13_count 必须为 0 才能通过 gate |
| R6.14 P14 Gate Sensitivity Neighborhood | B-1 summary JSON `invariant_violations.P14_count` + `neighborhood_p50/p5` | 每次 run P14_count 必须为 0 才能通过 gate |
| R7.10 非目标：Scheme D 非 live PnL | `## Scheme D — Portfolio Equity Simulator` Safety 段 + 本 design Safety 段 Scheme D 隔离条款 | 输出目录限定 `research/portfolio_equity_sim/`；markdown / JSON 文件头强制声明 research-only |

## Property Tests

本 spec 不强制 research harness 实现完整 PBT 基础设施，但 design 层给出 P12 / P13 / P14 的 property-based 测试骨架，供后续 tasks 阶段具体落地时参考；测试均 research-only，运行于 `research/` 下，不触及 `internal/` / `live`。

### P12 Capital Concurrency Bound

- **被测函数**：Portfolio_Equity_Simulator 的 1s gate 阶段（`open_notional_share(t) + slot_share` vs `capital_usage_cap` 比较）。
- **输入 fuzz**：随机生成若干 lifecycle ledger 条目 `(symbol ∈ {BTCUSDT, ETHUSDT}, slot_index ∈ {0, 1}, notional_share ∈ (0, 0.3], ts_open, ts_close)`，同时 fuzz `capital_usage_cap ∈ {0.20, 0.40, 0.60, 0.80, 1.00, 1.20}`，`max_trades_per_bar <= 2` 约束保持。
- **属性**：For all accepted slot stream, for all 1s timestamps `t`，`open_notional_share(t) <= capital_usage_cap + 1e-9`；违反时 summary JSON `invariant_violations.P12_count > 0`。
- **负向 canary**：刻意插入一条 `notional_share = capital_usage_cap + 0.05` 的 "越界" slot，如果被接受，测试必须 FAIL。

### P13 Active_Silo_Sum vs CAGR Consistency

- **被测函数**：Scheme D 的 `active_silo_sum_vs_cagr_relative_diff_pct` 计算。
- **输入 fuzz**：参数化 (ledger 大小 ∈ `[10, 500]` trades、mean per-trade R ∈ `[-0.5, 1.2]`、concurrency 密度 ∈ `[低, 中, 高]`、cap ∈ `{1.00, 0.60}`)。
- **属性**：在 **低并发 + 小 ledger + 中性 R** 的区域，`|Active_Silo_Sum - CAGR| / max(|CAGR|, 1e-9) <= 0.15`；summary JSON `invariant_violations.P13_count` 必须为 0。
- **负向 canary**：在 **高并发 p95** 区间（例如 `concurrency_p95 >= 4`），测试显式期望 `relative_diff` 明显放大（> 0.15），以证明 P13 能 **捕获** 不可叠加的情况；若此 canary 依然小于 0.15，说明 CAGR 或 Active_Silo_Sum 的计算有 silent rounding，测试必须 FAIL（这是反过来验证 P13 不是恒真 tautology）。

### P14 Gate Sensitivity Neighborhood

- **被测函数**：B-1 Gate_Sensitivity_Grid 的 neighborhood `P5 / P50 / P95` 与 per-dimension partial-dependence 计算。
- **输入 fuzz**：合成 gate scoring surface，参数化三维阈值网格（`5x5x5`）上的 Calendar_Sum 值；fuzz surface 平滑度（从平坦到尖峰）。
- **属性**：对 **平缓 surface**（max - min <= 5pp），`neighborhood_p50_pct >= center - 2pp` 且 `worst_silo_p5_pct >= center_worst - 0.5pp`；summary JSON `invariant_violations.P14_count` 为 0。
- **负向 canary**：对 **尖峰 surface**（仅中心点 `+33%`、邻域瞬间跌到 `< +10%`），测试必须让 P14 失败（`neighborhood_p50_pct < +25%` 或 `worst_silo_p5_pct < -1.0%`），即 `invariant_violations.P14_count > 0`；若尖峰 surface 仍被判为通过，说明 P14 等效于常数，测试必须 FAIL。

## Verification

R0 verification is static: check the Kiro docs for forbidden live migration wording and required contract fields.

R1/R2 verification is research-run based:

- Python compile for changed research scripts.
- A smoke run over a short period to prove output schemas.
- Full Scheme B matrix after smoke passes.
