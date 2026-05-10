# Requirements Document

## Introduction

本 spec 基于 `codex/research-original-t2-donchian-20260508` 分支下的最新研究（BTC/ETH 1h 时间框架，2026 Jan-Apr 与 2025 全年样本），以及 `20260508_original_t2_donchian_hybrid_findings.md`、`20260508_original_t2_arm_donchian_confirm_findings.md`、`20260508_probabilistic_v4_plan.md`、`20260509_probabilistic_v5_v6_execution_aware.md` 的汇总结论，梳理 "Original T2 + Donchian Hybrid / Distance / Structure / Probabilistic v4-v6 execution-aware" 这条研究演进线，并把当前可执行范围收敛为 research-only 的"收益增强策略演进方案"。

本 spec 的输出物只覆盖 R0/R1/R2 research 阶段的需求、可验证 gate 与演进路径，不改动 live execution 核心禁区（AGENTS §3），不引入已移除的 Go live-aligned replay 模块（AGENTS §2 Strategy Semantic Sources），不默认打开 `auto-dispatch`（AGENTS §3），也不在本 spec 内直接改生产默认仓位比例（即仍固定 research baseline `reentry_size_schedule=[0.20, 0.10]`、`max_trades_per_bar=2`，AGENTS §2 Core Memory）。Live shadow、灰度和全量发布必须延后到独立 live migration spec，经本 spec 的 R2 research gate 通过后再讨论。

### Research 演进脉络（Background）

研究 baseline 的历史演进按时间顺序：

1. **2026-04-16 breakout_reentry baseline**：30min 时间框架，long 用 `prev_low_1 + 0.1ATR`、short 用 `prev_high_1` 作为 reentry 锚点，reclaim / pullback 两种触发，使用 `re_p` 作为 planned fill 价格（`research/20260416_breakout_reentry_experiments.md`）。
2. **2026-04-19 zero_initial_reentry_window 引入**：`dir2_zero_initial=true` + `zero_initial_mode=reentry_window`，`Initial` 突破不再创建持久 zero-notional synthetic position，而是开启 current + next signal bar 的 reentry window。第一次真实下单变为 `Zero-Initial-Reentry`。
3. **2026-04-20 固定 `reentry_size_schedule=[0.20, 0.10]` 与 `max_trades_per_bar=2`**：成为长期 research baseline；此时仍以 30min 1h 为主，planned-price fill 被保留。
4. **2026-04-27 向后 T3 / SMA5 / low-vol gate 优化**：在同一 zero-initial reentry-window baseline 上叠加 breakout shape / T3 / entry-quality gate。
5. **2026-05-05 virtual SL + 0.55ATR + turn 0.1 + entry_sl 0.3**：研究 baseline 再 evolve 为 virtual SL decoupled stop（`research/20260505_research_baseline_evolution.md` 归档）。
6. **2026-05-08 direct_breakout (1d → 1h) on true `original_t2`**：ETH 2026 Jan-Apr 1h 940 笔，raw `+5.63%`，但 2bps slip 后 `-0.79%`，`6bps` 手续费后 realistic `-9.71%`，过度交易 + 成本压迫的问题被正式识别。
7. **2026-05-08 impulse_bar_run / micro_breakout_structure / micro filter / one-shot 降频**：`very_strict_oneshot` 把 2bps slip 拉到 `+0.46%`，但手续费仍吞掉收益，realistic 仍 `-7.03%`；"只调 speed/efficiency 阈值不是解" 被明确。
8. **2026-05-08 pretouch / posttouch_quality / entry**：post-touch confirm best `c05_f03_one` realistic `-1.72%`，虽把单笔质量从 `3.76bps/notional` 提到 `4.85bps/notional`，但仍远不到 `~10bps/notional` 的成本线。pre-touch fast_clean（ETH Jan-Apr）realistic `+0.44%`，是真正意义上的第一批成本后为正的候选。
9. **2026-05-08 original_t2 + Donchian hybrid/distance/structure**：`donchian_gap_bucket` 作为 headroom 特征，ETH 2026 Jan-Apr `edge10_d8_near_structure1p0_b4` 25 笔 realistic `+2.03%`，BTC 同期 `-1.86%`；ETH 2025 全年 `fast_clean` realistic `-4.57%`，表明"分箱 edge 在 2026 局部成立但跨期不稳"。
10. **2026-05-08 original_t2 arm + Donchian confirm**：ETH 2026 `b55_loose_structure1p0_b4` realistic `+1.77%`，BTC `+0.10%`，但 ETH 2025 全年 `b55_loose_structure1p0_b4` realistic `-15.87%`，彻底否定把该组合作为新 baseline。
11. **2026-05-08 probabilistic v4 plan**：将 event / quality / execution 拆成三层（`probabilistic_v4_event_dataset.py` / `probabilistic_v4_quality_model.py` / `probabilistic_v4_execution_runner.py`）。global probability + `delay5/be0.8/trail0.9` execution 让 ETH 2026 Jan-Apr 等权到约 `+1.39%`，但 BTC 2026 Mar OOS 仅 `-0.04%`。
12. **2026-05-09 probabilistic v5/v6 execution-aware**：ML 模型族 + Markov 多窗口 + execution-aware label。ETH 2026 Mar dynamic `+1.55%`，BTC dynamic `-1.11%`；2025 Dec 未通过。
13. **delay60 + feature60 + post_selection gate**：合法 point-in-time 语义下，走到 5 个 active months 合计 `+6.09%`，仍未到 `10%~20%` 级别实盘候选。

### 关键结论（Research 层有 α / 无 α）

- **有方向 α**：pre-touch `fast_clean`（dist 0.10-0.15 / speed300>=0.20 / pullback 0-0.02 ATR）、donchian gap `0.40+` 的 headroom pretouch 状态、post-touch `TrailingSL + be 0.8 / trail 0.9`、execution-aware probability（尤其 ETH）。
- **无稳定 α 或跨期崩溃**：direct breakout、single speed/efficiency micro filter、structure exit 移植到 arm+d8 confirm、signal-bar 完整 OHLC 作为模型特征（lookahead 已作废）。
- **跨币种不对称**：几乎所有候选在 BTC/ETH 上表现不对称；BTC 目前不能默认启用 aggressive dynamic sizing，ETH 也只能在 research gate 通过后继续测试；portfolio-level gate 是必须的。

### 当前实施决策（2026-05-10）

- **主线只推进 Scheme B**：`delay60 + feature60 + validation_best + post_selection gate` 是当前唯一 implementation-first 候选；下一步重点是补 regime gate / no-trade gate，而不是继续在 entry5 或已知弱事件源上调阈值。
- **Scheme A / Scheme C 降级**：Scheme A 仅保留为 V4/entry5 对照，Scheme C 仅保留 fail-fast 小样本复核；二者不能阻塞 Scheme B，也不能作为 live 候选。
- **当前 spec 不做 live migration**：R3/R4/R5 只作为后续独立 spec 的占位，不在本 spec 内产生 live session 配置、sleeve multiplier、control-reset 流程或 dispatch 行为变更。
- **必须补 Scheme Semantic Contract**：每个候选必须明确自己是 "baseline-derived sizing" 还是 "完整 reentry-window lifecycle"。当前 V4/V6 runner 默认是 event selection + 单次 1s execution + `notional_share=0.20`，不是完整 `slot0=20% / slot1=10%` reentry lifecycle；设计与报告必须避免把这两件事混写。

## Glossary

- **Research_Ledger**: research 层的回测产物，包含 trade ledger、summary JSON、月度归因、OOS 报告。
- **Live_Session**: 生产实盘运行的 session（AGENTS §2 Strategy Semantic Sources 中"live"这条事实源）。
- **Original_T2_Structure**: AGENTS Breakout Structure Semantics 定义的三根 bar 结构，long level = `prev_high_2`，short level = `prev_low_2`，当前 1h bar 未闭合，由当前 bar 内 `1s high/low` 对 level 触发。
- **Donchian_Hybrid**: 使用 `prev_high_8 / prev_low_8` 作为 headroom / confirm 特征，但 entry level 仍使用 `prev_high_2 / prev_low_2`。
- **Donchian_Confirm**: 真正 entry level 改为 `prev_high_8 / prev_low_8`；`original_t2` 仅作为 arm 条件。研究已证明 ETH 2025 全年不稳，本 spec 不再把它作为候选晋级路径。
- **Probabilistic_V4**: `research/probabilistic_v4_event_dataset.py` + `research/probabilistic_v4_quality_model.py` + `research/probabilistic_v4_execution_runner.py` + `research/probabilistic_v4_matrix_runner.py` 组成的三层架构。
- **Probabilistic_V5**: 在 V4 之上扩展 ML 模型族（logistic / random_forest / extra_trees / gradient_boosting / svm_rbf）、多窗口 Markov 特征、`hybrid_markov` dynamic sizing。
- **Probabilistic_V6**: execution-aware label（每个 event 用 1s OHLC 独立标注真实执行收益）+ walk-forward runner + validation top-K + no-trade gate。
- **Execution_Aware_Label**: 每个 touch event 用统一 1s execution runner（`delay5 / initial_stop_atr=0.45 / breakeven=0.8R / trail_start=0.9R / max_hold=4h` 或 `delay60 / feature_horizon=60s / ...`）独立标注的真实执行 PnL 和 exit reason。
- **Point_In_Time_Feature**: 只使用 touch 瞬间及之前已闭合 bar / 已观察到的 1s bar 特征；禁止使用当前未闭合 signal bar 的完整 `signal_close/signal_high/signal_low` 或完整当前 bar ATR。
- **Parity_Error**: 同一输入下 research summary 与 live audit log 在关键 metric（PnL / trade count / exit reason 比例 / 参数快照）上的差异。
- **Research_Baseline**: AGENTS §2 定义的长期 research baseline：`dir2_zero_initial=true` + `zero_initial_mode=reentry_window` + `reentry_size_schedule=[0.20, 0.10]` + `max_trades_per_bar=2`。本 spec 不改该 baseline。
- **Baseline_Derived_Sizing**: 使用 Research_Baseline 的固定 sizing 约束（slot0=20%、slot1=10%、`max_trades_per_bar=2`）作为资金暴露上限，但不声明已实现完整 zero-initial reentry-window 生命周期。
- **Full_Reentry_Window_Lifecycle**: 完整实现 `Initial` 打开 current + next signal bar reentry window、slot0/slot1 真实下单、同 signal bar real-entry count <= 2、以及后续 SL/PT-Reentry 状态流转的 research baseline 语义。
- **Scheme_Semantic_Contract**: 每个 Candidate_Scheme 在 design / tasks 中必须填写的语义契约，至少包含 entry source、breakout semantic、feature horizon、execution model、sizing mode、是否 Full_Reentry_Window_Lifecycle、当前 runner 差距和输出 ledger。
- **Dispatch_Mode**: AGENTS §3 中 live 控制面的 dispatch 模式；默认必须 `manual-review`，禁止隐式 `auto-dispatch`。
- **Portfolio_Silo**: research 层按 symbol（BTCUSDT / ETHUSDT）和月份切分的独立执行单元，跨 silo 汇总时使用等权或明确权重。
- **Active_Silo_Sum**: 只对通过 gate 且实际交易的 symbol-month silo 做简单相加的 return，用于快速比较 research run；不能直接等同于全年 portfolio return。
- **Calendar_Normalized_Return**: 把未交易月份按 0% 计入后，以固定 symbol/month 权重汇总的 return；R1/R2 gate 必须同时报告它，避免只看 active silo。
- **Research_Harness**: 本 spec 讨论的研究脚本与其输入/输出约束，对应 `research/` 目录内的 runner 与 markdown 报告。
- **Candidate_Scheme**: 本 spec Requirement 3 中列出的"收益增强策略候选"，每个 Candidate_Scheme 必须在 research 层产出可归档的 Research_Ledger 才能进入 live shadow。
- **Evolution_Gate**: Requirement 4 定义的分阶段准入门槛。
- **Intrabar_Breakout_Semantic**: AGENTS Breakout Structure Semantics 要求的语义：breakout 必须使用 intrabar `1s high/low` 对 `prev_high_2 / prev_low_2` 的关系判定，不能写成 "闭合 signal bar 收盘确认"。

## Requirements

### Requirement 1: Research 演进脉络与最新研究结论归档

**User Story:** As a research engineer, I want a single requirements-first 文档把 `codex/research-original-t2-donchian-20260508` 的演进线和最新结论梳理清楚, so that 后续所有候选方案、gate 设计、live 迁移都有唯一的事实基线。

#### Acceptance Criteria

1. THE Research_Harness SHALL 在本 spec、对应 design 文档、对应 tasks 文档中各至少一处显式引用 AGENTS §2 定义的 Research_Baseline，且该引用 MUST 同时逐字符串出现以下四个参数：`dir2_zero_initial=true`、`zero_initial_mode=reentry_window`、`reentry_size_schedule=[0.20, 0.10]`、`max_trades_per_bar=2`，并在同一段落附带 "AGENTS §2 Core Memory" 的章节指针；所有 Candidate_Scheme（Requirement 3）在 design 阶段 MUST 以该 Research_Baseline 作为起点，不得以旧的 `10%/5%/2.5%` 方案或 `position` 方案替代。
2. WHEN 本 spec / design / tasks 描述 breakout 触发逻辑, THE Research_Harness SHALL 使用 Intrabar_Breakout_Semantic，即 long 触发条件写为"当前未闭合 signal bar 内存在至少一个 `1s high >= prev_high_2` 的 1s bar"、short 触发条件写为"当前未闭合 signal bar 内存在至少一个 `1s low <= prev_low_2` 的 1s bar"；同一文档 MUST NOT 出现"闭合 signal bar 收盘确认"或任何等价表述（含"1h 收盘后确认 breakout"、"close > prev_high_2 后入场"等措辞）。
3. THE Research_Harness SHALL 在本 spec 的 Introduction / Background 中为下列每一个里程碑各列一条独立条目，且每条条目 MUST 同时包含 (a) 日期标签、(b) 一句中文结论摘要、(c) 至少一个 `research/` 目录下的 markdown 文件名引用：`20260416_breakout_reentry_experiments.md`、`20260419_zero_initial_reentry_window(_enhanced).md`、`20260420_eth_q1_reentry_window_second_bar_replay*.md`、`20260427` T3 / SMA5 系列、`20260505_research_baseline_evolution.md`、`20260508` direct_breakout / micro_filter / posttouch_quality / pretouch_continuation(_entry) / donchian_distance / donchian_structure / donchian_hybrid / arm_donchian_confirm、`20260508_probabilistic_v4_plan.md`、`20260509_probabilistic_v5_v6_execution_aware.md`。
4. THE Research_Harness SHALL 在本 spec 同一文档内相邻章节同时记录"有 α 方向"与"无 α 或跨期崩溃方向"，每个方向条目 MUST 同时给出：(a) 方向名称（例如 pre-touch `fast_clean`、direct breakout 等）、(b) 代表性样本范围（symbol + 时间窗口，例如 "ETH 2026 Jan-Apr 1h"）、(c) 至少一个 realistic PnL 百分比或跨期崩溃关键数值、(d) 至少一个 `research/` 目录下的 markdown 文件名作为出处。
5. IF 本 spec 的 design / tasks / PR 在 active 审阅阶段（design review、tasks review、PR review）引用已移除的 Go live-aligned replay 模块或其历史输出作为事实源, THEN THE Research_Harness SHALL 在该审阅阶段给出拒绝结论，拒绝结论 MUST 附带 AGENTS §2 Strategy Semantic Sources 的章节指针，且在该引用被修正或移除之前不得获得审阅通过；个人笔记或非审阅阶段的临时稿件不在此约束范围。
6. THE Research_Harness SHALL 在本 spec 对应的 design 与 tasks 文档中各至少出现一处显式标注，标注 MUST 同时包含：(a) 数值 `+6.09%`、(b) 样本范围 "5 个 active months silo sum"、(c) 参数组合 "delay60 + feature60 + post_selection gate"、(d) 明确陈述该结果尚未达到本 spec R2 research promotion gate，且不能直接作为 live shadow / 灰度 / 全量发布依据。

### Requirement 2: 20260508 最新研究结论的量化总结

**User Story:** As a research engineer, I want 本文档逐条记录 arm+donchian confirm、distance / structure / hybrid pretouch、posttouch quality、micro filter、pretouch entry、probabilistic v4/v5/v6 的信号质量与对照 direct_breakout 的关键指标, so that 后续演进方案可以直接基于这些量化结论决策而不用再回读每一份 markdown。

#### Acceptance Criteria

1. THE Research_Harness SHALL 在 design 阶段固化 research 成本模型为 `slip=2bps/side` + `maker_entry=2bps` + `taker_exit=4bps`（来源为本轮 research 报告口径，而非 AGENTS §2 的项目 baseline 章节），并产出下列对照清单，清单中每条 MUST 显式记录 `symbol / timeframe / 时间窗 / variant / trade_count / realistic_pct / raw_pct / 源 markdown`，任一字段缺失时 MUST 在同一行标注 `n/a (源未提供)`：
   - `ETHUSDT / 1h / 2026-01 ~ 2026-04 / original_t2 direct_breakout`：trade_count=940，realistic=`-9.71%`，raw=`+5.63%`；源 `research/20260508_eth_2026_jan_apr_1h_original_t2_direct_breakout.md`。
   - `ETHUSDT / 1h / 2026-01 ~ 2026-04 / micro very_strict_oneshot`：trade_count=727，realistic=`-7.03%`，raw (2bps slip, no fee)=`+0.46%`；源 `research/20260508_eth_2026_jan_apr_1h_original_t2_micro_strict_oneshot.md`。
   - `ETHUSDT / 1h / 2026-01 ~ 2026-04 / posttouch c05_f03_one`：trade_count=166，realistic=`-1.72%`，raw=`+1.60%`，per-trade quality ≈ `4.85 bps/notional`；源 `research/20260508_eth_2026_jan_apr_1h_original_t2_pretouch_continuation.md`。
   - `ETHUSDT / 1h / 2026-01 ~ 2026-04 / pretouch fast_clean`：trade_count=85，realistic=`+0.44%`，raw=`n/a (源未提供)`；源 `research/20260508_eth_2026_jan_apr_1h_original_t2_pretouch_continuation.md`。
   - `ETHUSDT / 1h / 2026-01 ~ 2026-04 / pretouch edge10_c1f03`：trade_count=169，realistic=`+0.51%`，raw=`n/a (源未提供)`；源 `research/20260508_eth_2026_jan_apr_1h_original_t2_pretouch_continuation.md`。
   - `ETHUSDT / 1h / 2026-01 ~ 2026-04 / donchian_hybrid fast_clean_tp1p0`：trade_count=`n/a (源未提供)`，realistic=`+0.61%`，raw=`n/a (源未提供)`；源 `research/20260508_original_t2_donchian_hybrid_findings.md`。
   - `ETHUSDT / 1h / 2026-01 ~ 2026-04 / donchian_structure edge10_d8_near_structure1p0_b4`：trade_count=25（触发 criterion 3 小样本标注），realistic=`+2.03%`，raw=`n/a (源未提供)`；源 `research/20260508_eth_2026_jan_apr_1h_original_t2_donchian_structure_exit_sweep.md`。
   - `BTCUSDT / 1h / 2026-01 ~ 2026-04 / pretouch fast_clean`：trade_count=103，realistic=`-0.53%`，raw=`+1.54%`；源 `research/20260508_btc_2026_jan_apr_1h_original_t2_donchian_hybrid_pretouch_continuation.md`。
   - `BTCUSDT / 1h / 2026-01 ~ 2026-04 / pretouch fast_clean_d8_exact`：trade_count=21（触发 criterion 3 小样本标注），realistic=`+0.03%`，raw=`n/a (源未提供)`；源 `research/20260508_btc_2026_jan_apr_1h_original_t2_donchian_hybrid_pretouch_continuation.md`。
   - `ETHUSDT / 1h / 2025-01 ~ 2025-12 / pretouch fast_clean`：trade_count=355，realistic=`-4.57%`，raw=`n/a (源未提供)`；源 `research/20260508_eth_2025_1h_original_t2_donchian_hybrid_pretouch_continuation.md`。
   - `ETHUSDT / 1h / 2025-01 ~ 2025-12 / pretouch fast_clean_d8_exact_structure1p0_b4`：trade_count=62，realistic=`-1.83%`，raw=`n/a (源未提供)`；源 `research/20260508_eth_2025_1h_original_t2_donchian_structure_exit_sweep.md`。
   - `ETHUSDT / 1h / 2026-01 ~ 2026-04 / arm+donchian_confirm b55_loose_structure1p0_b4`：trade_count=148，realistic=`+1.77%`，raw=`n/a (源未提供)`；源 `research/20260508_original_t2_arm_donchian_confirm_findings.md`。
   - `BTCUSDT / 1h / 2026-01 ~ 2026-04 / arm+donchian_confirm b55_loose_structure1p0_b4`：trade_count=136，realistic=`+0.10%`，raw=`n/a (源未提供)`；源 `research/20260508_original_t2_arm_donchian_confirm_findings.md`。
   - `ETHUSDT / 1h / 2025-01 ~ 2025-12 / arm+donchian_confirm b55_loose_structure1p0_b4`：trade_count=489，realistic=`-15.87%`，raw=`n/a (源未提供)`；源 `research/20260508_original_t2_arm_donchian_confirm_findings.md`。
   - `V4 / 1h / 2026-01 ~ 2026-03 / rule_global + delay5_be0.8_trail0.9 / 分 silo`：BTCUSDT trade_count=58, realistic=`+2.07%`；ETHUSDT trade_count=55, realistic=`-0.15%`；raw=`n/a (源未提供)`；源 `research/20260508_probabilistic_v4_plan.md`。
   - `V4 / 1h / 2026-01 ~ 2026-03 / probability_global + delay5_be0.8_trail0.9 / 分 silo`：BTCUSDT trade_count=56, realistic=`+1.92%`；ETHUSDT trade_count=49, realistic=`+0.86%`；raw=`n/a (源未提供)`；源 `research/20260508_probabilistic_v4_plan.md`。
   - `V4 OOS / 1h / 2026-03 / probability_global / 分 silo`：BTCUSDT realistic=`-0.04%`；ETHUSDT realistic=`+0.60%`；trade_count/raw=`n/a (源未提供)`；源 `research/20260508_probabilistic_v4_plan.md`。
   - `V4 OOS (relaxed) / 1h / 2025 Q4 / probability_global / 分 silo`：BTCUSDT realistic=`+0.04%`；ETHUSDT realistic=`+0.86%`；trade_count/raw=`n/a (源未提供)`；源 `research/20260508_probabilistic_v4_plan.md`。
   - `V6 OOS / 1h / 2026-03 / execution-aware per-symbol dynamic / 分 silo`：BTCUSDT realistic=`-1.11%`；ETHUSDT realistic=`+1.55%`；trade_count/raw=`n/a (源未提供)`；源 `research/20260509_probabilistic_v5_v6_execution_aware.md`。
   - `V6 walk-forward / 1h / delay60 + feature60 + post_selection gate / 分 silo-月合计`：active_months=5, 累计 trade_count=51, Active_Silo_Sum realistic=`+6.09%`；其中 BTCUSDT 2025-12 active 月 validation gate pass 但 execute realistic=`-0.79%`，属于未解决的 regime shift；raw=`n/a (源未提供)`；源 `research/20260509_probabilistic_v5_v6_execution_aware.md`。
2. WHEN design / tasks / Research_Ledger 讨论"最有效参数带"时, THE Research_Harness SHALL 记录以下具名取值带并标注对应证据源；任何引用 MUST 使用以下精确数值区间，不允许再出现 "合适 / 适中 / 较强" 等定性描述：
   - pretouch state band：`distance_bucket ∈ [0.10, 0.15] ATR` AND `speed300_bucket >= 0.20 ATR` AND `pullback_bucket ∈ [0.00, 0.02] ATR`；源 `research/20260508_eth_2026_jan_apr_1h_original_t2_pretouch_continuation.md`。
   - donchian headroom band：`donchian_gap_bucket >= 0.40`（上界开放，单位 ATR）；源 `research/20260508_original_t2_donchian_hybrid_findings.md`。
   - execution band (delay5 家族)：`entry_delay_seconds=5`, `initial_stop_atr=0.45`, `breakeven_at_r=0.8`, `trail_start_r=0.9`, `max_hold_hours=4`；源 `research/20260508_probabilistic_v4_plan.md`。
   - delay60 band：`entry_delay_seconds=60`, `feature_horizon_seconds=60`（MUST 满足 `feature_horizon_seconds <= entry_delay_seconds` 以维持 Point_In_Time_Feature 约束）, `top_k_policy=validation_best`, `top_k_selection_metric=return_over_drawdown`, sizing=`hybrid_markov`, gate=`post_selection`；源 `research/20260509_probabilistic_v5_v6_execution_aware.md`。
3. WHEN Research_Ledger 引用任何分箱, THE Research_Harness SHALL 把 `sample_size <= 30`（含 `sample_size = 0` 的空样本）统一标注为 "小样本，需跨期 (>= 2 个时间上不相交的窗口) + 跨品种 (BTCUSDT 与 ETHUSDT 同时复核) 复核"，且该分箱 MUST NOT 直接作为 Requirement 4 Evolution_Gate R1 的晋级候选。
4. THE Research_Harness SHALL 在 design / tasks 中显式记录 BTCUSDT 与 ETHUSDT 的非对称性，并声明：BTCUSDT 在 R0 / R1 / R2 research 阶段默认 sizing = Baseline_Derived_Sizing 的 fixed 20% 对照；`hybrid_markov` 或任何 aggressive dynamic sizing 在 BTCUSDT 上 MUST 先通过 validation 期间 `InitialSL_rate <= 0.30` 的 gate 才允许启用，否则回落 fixed 20% 并单独归档。ETHUSDT 在 Candidate_Scheme 已满足 R1 gate 的前提下允许启用 `hybrid_markov`。如果当前 runner 尚不支持 per-symbol fallback 复跑，tasks MUST 先实现该能力，不能在报告中手工声称已回落。
5. IF 后续 design / tasks / PR 引用任何早于 2026-05-08 的 proxy 研究结论（包含但不限于 `prev_high_8` 8-bar Donchian closed-bar proxy 的历史强结果，或任何未使用 Intrabar_Breakout_Semantic 的闭合 bar 收盘确认口径）来论证 true `original_t2` 收益, THEN THE Research_Harness SHALL 在 design review 阶段把该引用标注为 "作废，不可作为 true `original_t2` 结论使用"，并 SHALL 要求引用方改用 Requirement 2.1 清单中对应行作为事实源；拒绝后方可继续 review 流程。

### Requirement 3: 候选策略演进方案（Candidate Schemes）

**User Story:** As a research engineer, I want 明确、可测试的候选策略集合, so that 每个 Candidate_Scheme 都能在 research 层产出可归档的 Research_Ledger，再按 Requirement 4 的 Evolution_Gate 逐级下沉。

本 Requirement 列出三组 Candidate_Scheme（A / B / C）。每组 MUST 在 design 阶段固化输入参数、输入样本、输出 ledger 路径、gate 指标阈值与 Scheme_Semantic_Contract。当前实施优先级为 B > A > C，其中 A/C 只能作为对照或 fail-fast 复核，不能阻塞 Scheme B 的 regime gate 推进。

#### Acceptance Criteria

1. **Scheme A (Pretouch Fast-Clean + V4 Global Probability, secondary control)**: THE Candidate_Scheme_A SHALL 满足以下定义：
   - Entry: true Original_T2_Structure pre-touch state band（`distance_bucket=0.10-0.15 ATR` AND `speed300_bucket>=0.20 ATR` AND `pullback_bucket=0-0.02 ATR`），可选叠加 `donchian_gap_bucket=0.40+` headroom。
   - Quality: V4 `probability global` (`prob_min=0.55`, `ev_atr_min>=0`) 或 V4 `rule global` 作为对照。
   - Execution: `entry_delay_seconds=5`, `initial_stop_atr=0.45`, `breakeven_at_r=0.8`, `trail_start_r=0.9`, `max_hold_hours=4`。
   - Sizing: ETHUSDT 允许 `hybrid_markov` dynamic sizing（训练+验证 calibration），BTCUSDT 固定 fixed 20%。
   - Scope: 1h timeframe, BTCUSDT + ETHUSDT 分 silo 执行，portfolio 按等权合成并在 summary 中单列。
   - Priority: 仅作为 entry5/V4 对照，不作为当前主线；若 2025 Q4 / 2026 Q1 OOS 任一窗口低于 V4 OOS baseline，则停止投入。
   - Expected improvement：PnL 目标 > V4 OOS baseline（2026 Mar silo sum `+0.28%`、2025 Q4 relaxed `+0.45%`），PF >= `1.2`，MaxDD <= `2.5%`，每 silo 月 trade count >= 8。
2. **Scheme B (delay60 + Feature60 + Post-Selection Gate + Regime Gate, primary)**: THE Candidate_Scheme_B SHALL 满足以下定义：
   - Entry: Original_T2_Structure，`entry_delay_seconds=60`，`feature_horizon_seconds=60`（合法 point-in-time，禁止 > delay）。
   - Quality: V6 walk-forward runner 的 `top_k_policy=validation_best`，`top_k_selection_metric=return_over_drawdown`。
   - Gate: `--validation-topk-gate-stage=post_selection` + `--min-validation-topk-return-over-dd=1.0` + `--max-validation-topk-return-pct<=7.0`，gate 失败则该 silo-月空仓。
   - Regime Gate: design/tasks MUST 增加 validation-only 或 execute 前可观测的 regime/no-trade gate，目标是识别 BTCUSDT 2025-12 这类 validation pass 但 execute 月亏损的状态；不得使用 execute 月标签或未来 signal bar 完整 OHLC。
   - Sizing: `hybrid_markov` dynamic sizing；BTCUSDT 必须在 validation 期间额外满足 `InitialSL_rate<=0.30` 才启用 dynamic，否则 fixed 20%。如果 runner 当前不支持该 fallback，implementation MUST 先补 runner 能力并输出 dynamic-vs-fixed 对照。
   - Scope: 1h timeframe，BTCUSDT + ETHUSDT walk-forward（train 2 个月 / validation 1 个月 / execute 1 个月），覆盖至少 2025-06 至 2026-04 共 11 个 execute 月。
   - Expected improvement：Active_Silo_Sum > `+6.09%`（当前 baseline）且 Calendar_Normalized_Return 同步改善，月度 MaxDD <= `3%`，PF >= `1.3`，并显式列出 active months / empty months。
3. **Scheme C (Pretouch Fast-Clean + Structure Exit Small-Sample Confirmation, fail-fast only)**: THE Candidate_Scheme_C SHALL 满足以下定义：
   - Entry: 与 Scheme A 相同 pre-touch state band，可叠加 `donchian_gap_bucket` 特征。
   - Exit: `structure_start_atr=1.0`, `structure_bars=4`, `structure_buffer_atr=0.05`（来自 `edge10_d8_near_structure1p0_b4`）。
   - Scope: 1h timeframe，必须同时跑 ETH 2026 Jan-Apr、ETH 2025 全年、BTC 2026 Jan-Apr；只有三个样本同时 realistic 为正才能进入下一阶段。
   - Sizing: fixed 20% 或 Scheme A 的同款 sizing（不得使用 aggressive dynamic 放大小样本收益）。
   - Expected improvement：对于每个样本集，realistic >= `+0.5%`，trade count >= 50。
   - Priority: 仅用于验证小样本结构退出是否完全作废；不得使用 aggressive dynamic sizing 放大小样本收益。
   - Fail-Fast: IF 任一样本集 realistic < 0 OR trade count < 30, THEN THE Research_Harness SHALL 将 Scheme C 标注为 "failed"，不允许继续投入 research 时间。
4. **Candidate_Scheme 共性**: THE Candidate_Scheme_A / B / C SHALL 在 design 阶段指定：
   - 输入 symbol（BTCUSDT / ETHUSDT，可扩展但必须显式列出）。
   - 输入时间窗口（walk-forward split）。
   - 成本模型：slip `2bps/side`、maker entry `2bps`、taker exit `4bps`（research 报告口径）。
   - 输出 Research_Ledger 文件路径（`research/tmp_*_ledger.csv` + `research/*_summary.json` + `research/<date>_<scheme>.md`）。
   - Predicted metrics 维度：PnL / ProfitFactor / MaxDD / TradeCount / WinRate / Slot 或 notional share 贡献 / 月度归因 / OOS split / Active_Silo_Sum / Calendar_Normalized_Return。
   - Scheme_Semantic_Contract：必须说明该 Scheme 是 Baseline_Derived_Sizing 还是 Full_Reentry_Window_Lifecycle；若只是当前 V4/V6 event-selection runner，必须标注 "not full reentry-window lifecycle"。
5. IF 任一 Candidate_Scheme 在后续 design 阶段被发现使用 signal bar 完整 OHLC 或当前 bar 完整 ATR 作为 point-in-time 特征, THEN THE Research_Harness SHALL 在 design review 阶段拒绝该 Scheme（参考 V7 lookahead 修正教训，`20260509_probabilistic_v5_v6_execution_aware.md`）。
6. WHEN Candidate_Scheme 需要跨 symbol 组合时, THE Research_Harness SHALL 按 Portfolio_Silo 分 symbol / 月份独立执行，portfolio 级指标必须单独列出，不允许用"某一 silo 正收益"来代表组合收益。

### Requirement 4: Research-Only 分阶段演进路线图（Evolution Plan）

**User Story:** As a research engineer / risk owner, I want 一个 research-only 的 R0-R2 Evolution_Gate, so that 当前 Candidate_Scheme 的晋级决策先在研究层可复现、可审计、可失败退出，再决定是否另开 live migration spec。

#### Acceptance Criteria

1. **Phase R0 — Spec / Design / Tasks 收敛**:
   - Gate criteria: requirements、design、tasks 均显式声明 research-only；均包含 Research_Baseline、Scheme_Semantic_Contract、cost model source、Active_Silo_Sum 与 Calendar_Normalized_Return 定义。
   - Outputs: `.kiro/specs/original-t2-donchian-strategy-evolution/requirements.md`、`design.md`、`tasks.md`。
   - Safety: 仅写入 `.kiro/specs/`，不触及 `research/` 代码、`internal/`、`live` 配置。
   - Rollback: 若发现 live migration、control-reset、auto-dispatch 或 sleeve multiplier 混入当前 spec，必须移回后续独立 live migration spec。
2. **Phase R1 — Scheme B Research Implementation**:
   - Gate criteria: Scheme B 覆盖至少 2025-06 至 2026-04 的 execute 月；输出 Active_Silo_Sum、Calendar_Normalized_Return、active months、empty months、PF、MaxDD、trade count、symbol/month attribution；必须和当前 `+6.09%` post_selection baseline 同表比较。
   - Outputs: Research_Ledger CSV + summary JSON + markdown 总结 + 月度归因 + runner 参数快照。
   - Safety: 仅写入 `research/` 目录；不触及 `internal/`、`live`、`deployments/`、`.github/workflows/`。
   - Rollback: 未超过 `+6.09%` 或无法解释 BTCUSDT 2025-12 亏损时，不允许进入 R2；回到 regime gate / no-trade gate 设计。
3. **Phase R2 — Robustness / OOS / Regime Gate 验证**:
   - Gate criteria: Scheme B 在额外 OOS 或扩展 walk-forward 中 Active_Silo_Sum > `+6.09%` AND Calendar_Normalized_Return 改善 AND PF >= `1.3` AND MaxDD <= `3%` AND active months >= 6 AND 无单 active month `<-2%`。若 BTCUSDT 2025-12 或同类 validation-pass/execute-loss regime 未被 gate 识别，R2 不通过。
   - Outputs: OOS report markdown + walk-forward summary.md + `validation_topk_*` 字段 + regime gate 归因 + failed-gate month 列表。
   - Safety: 禁止使用已执行样本的 label 进行 in-sample 再调参；禁止使用 signal bar 完整 OHLC；禁止把 active silo 的单点正收益解释成组合可实盘。
   - Rollback: 如 OOS 不通过，Candidate_Scheme 回到 R1 重新 scope；不得进入 live shadow。
4. **Post-R2 — Live Migration Placeholder Only**:
   - IF R2 通过, THEN 才允许另开独立 live migration spec 讨论 live shadow / 灰度 / 全量；该后续 spec 必须重新审查 AGENTS §3 / §7 / §10，并使用 `bktrader-ctl --json` 做只读事实源。
   - THE current spec SHALL NOT 定义 R3/R4/R5 的 live 操作步骤、control-reset 流程、sleeve multiplier 或 session.config 输出。
5. THE Evolution_Gate SHALL 在 design / tasks 阶段把 R0-R2 的 gate 阈值沉淀为显式配置项或明确 CLI 参数，供 Research_Harness 在生成 Research_Ledger 时自动校验。

### Requirement 5: Research / Live Parity 与语义源约束

**User Story:** As a risk owner, I want 本 spec 的语义源约束显式写出, so that 后续 design / tasks / 执行阶段不会把 research 结果错误地映射到 live，也不会引用已移除的 replay 模块作为事实源。

#### Acceptance Criteria

1. THE Research_Harness SHALL 只使用两条事实源："research"（research 脚本与 ledger）和 "live"（`bktrader-ctl --json` + live audit log）。当前 spec 只使用 research；live 事实源仅作为后续独立 live migration spec 的约束占位。
2. IF 任何产物（design / tasks / PR）引用已移除的 Go live-aligned replay 模块或旧图谱节点作为事实源, THEN THE Research_Harness SHALL 拒绝合并（AGENTS §2 Strategy Semantic Sources）。
3. WHEN 后续独立 live migration spec 让 Live_Session 与 Research_Ledger 在同一输入窗口上运行, THEN 该后续 spec SHALL 定义 Parity_Error 容差；本 spec 不定义 live shadow 执行流程。
4. WHEN 任何 Candidate_Scheme 或 Research_Harness 流程测试 breakout 触发, THE Research_Harness SHALL 只允许 Intrabar_Breakout_Semantic 实现，即 "long 触发条件 = `1s_high >= prev_high_2`"、"short 触发条件 = `1s_low <= prev_low_2`"；即使其他语义实现能保持相同不变量，也不允许作为 breakout 触发器替代。
5. THE Research_Harness SHALL 在 Candidate_Scheme 运行期间保证 `max_trades_per_bar` 不超过 AGENTS baseline 定义的 `2`，且同一 signal bar 内的 real-entry count <= `2`。
6. THE Research_Harness SHALL 在 Candidate_Scheme 涉及 sizing 时，默认标注 Baseline_Derived_Sizing 或 Full_Reentry_Window_Lifecycle。若使用当前 V4/V6 event-selection runner，则报告必须写明默认 `notional_share=0.20` 是 fixed 20% event sizing，不等同于完整 slot0=`20%`、slot1=`10%` lifecycle；只有 Requirement 3.1 Scheme A / 3.2 Scheme B 的 dynamic sizing 在 research 层允许超出，且不下沉到 live 层默认配置。

### Requirement 6: 可作为 property-based 测试的 Correctness Properties

**User Story:** As a research engineer, I want 把关键 invariants 写成 property-based testable 约束, so that Research_Harness 的行为在参数 / 样本变化下仍保持一致。

#### Acceptance Criteria

1. THE Research_Harness SHALL 保持不变量 **P1 (Intrabar Breakout Trigger)**: FOR ALL (signal_bar, side), 若 side = long AND bar_is_unclosed AND `t2_ready(prev_high_2, prev_high_1)`, 则 entry 仅在存在至少一个 `1s_high_i >= prev_high_2` 的 1s bar 时触发；short 镜像条件成立。
2. THE Research_Harness SHALL 保持不变量 **P2 (Trades Per Bar Bound)**: FOR ALL signal_bar, 同一 signal bar 内的 real-entry count <= `max_trades_per_bar = 2`（AGENTS Research_Baseline）。
3. THE Research_Harness SHALL 保持不变量 **P3 (Cost Model Monotonicity)**: FOR ALL trade ledger, realistic_pnl <= `2bps_slip_pnl` <= raw_pnl（在相同 entry / exit 时间戳下），因为 realistic 在 slip 上额外叠加 `6bps` 往返手续费。
4. THE Research_Harness SHALL 保持不变量 **P4 (Point-in-Time Features Only)**: FOR ALL event in event dataset, 任何用于 quality / probability 模型的特征 MUST NOT 使用 `signal_close`、`signal_high`、`signal_low` 或当前未闭合 bar 的完整 ATR。测试时可以通过在 event dataset 中加入 "signal bar 完整 OHLC feature" 作为 canary，并断言 quality / probability 训练流水线不会读取该列。
5. THE Research_Harness SHALL 保持不变量 **P5 (Feature Horizon <= Entry Delay)**: FOR ALL execution variant with `entry_delay_seconds=D` AND `feature_horizon_seconds=H`, H <= D（避免 post-entry leakage）。
6. THE Research_Harness SHALL 保持不变量 **P6 (Research Metric Normalization)**: FOR ALL walk-forward summary, Active_Silo_Sum 与 Calendar_Normalized_Return MUST 同时输出，且报告 MUST 明确空仓月份按 0% 计入 Calendar_Normalized_Return。
7. THE Research_Harness SHALL 保持不变量 **P7 (Gate Stage Idempotency)**: FOR ALL walk-forward run with `--validation-topk-gate-stage=post_selection`, 两次运行相同输入参数和相同 events 下，输出 active rows / trades / silo sum 必须相同（后续 property-based fuzz 可以作为回归测试）。
8. THE Research_Harness SHALL 保持不变量 **P8 (No Live Config Output)**: 当前 spec 的 Research_Harness MUST NOT 生成 live `session.config`、sleeve multiplier、dispatch 配置或 control-plane 操作建议。若后续独立 live migration spec 需要生成 `session.config`，必须重新要求 `dispatchMode=manual-review`。
9. THE Research_Harness SHALL 保持不变量 **P9 (Round-Trip Serialization)**: FOR ALL `rules.json`、`summary.json`、event CSV header, 解码后再编码 SHALL 产生等价内容（字段顺序可忽略），以保证 research harness 的序列化器正确（AGENTS §通用: parser / serializer round-trip）。
10. THE Research_Harness SHALL 保持不变量 **P10 (Walk-Forward Window Non-Overlap)**: FOR ALL walk-forward split `(train, validation, execute)`, 三个窗口在时间轴上 MUST 两两不相交，且 execute window 的起点 >= validation window 的终点。
11. THE Research_Harness SHALL 保持不变量 **P11 (Scheme Semantic Contract Present)**: FOR ALL Candidate_Scheme outputs, summary markdown MUST include Scheme_Semantic_Contract and MUST explicitly state whether the run is Baseline_Derived_Sizing or Full_Reentry_Window_Lifecycle.

### Requirement 7: 显式非目标 / 暂不在范围内（Explicit Non-Goals）

**User Story:** As a risk owner, I want 把本 spec 明确 **不包含** 的修改写出来, so that 后续 design / tasks / PR 不会扩大范围、不会触动高风险禁区、不会默认改动 live 仓位配置。

#### Acceptance Criteria

1. THE Research_Harness SHALL NOT 在本 spec 或其 design / tasks 中修改 AGENTS §3 的高风险禁区：`internal/service/live*.go`、`internal/service/execution_strategy.go`、`deployments/`、`.github/workflows/`。任何涉及这些目录的改动 MUST 走独立 spec 与显式人工 approval。
2. THE Research_Harness SHALL NOT 引用已移除的 Go live-aligned replay 模块作为事实源（AGENTS §2 Strategy Semantic Sources）。
3. THE Research_Harness SHALL NOT 在本 spec 内把 `dispatchMode` 默认改为 `auto-dispatch`；所有 Candidate_Scheme 在 live shadow / live 灰度阶段默认 `manual-review`。
4. THE Research_Harness SHALL NOT 在本 spec 内修改 AGENTS §2 的 Research_Baseline（`reentry_size_schedule=[0.20, 0.10]`、`max_trades_per_bar=2`），也不在本 spec 内直接改生产默认仓位比例。任何 sleeve multiplier 或 live 仓位灰度都属于后续独立 live migration spec。
5. THE Research_Harness SHALL NOT 在本 spec 内引入新的 live adapter、订单路由或 reconcile 策略；live shadow / 灰度 / 全量发布均不在当前 spec 范围。
6. THE Research_Harness SHALL NOT 在本 spec 内把 V5/V6 的 ML 概率模型默认启用到 live；概率模型仅用于 research quality gating 与 research-side sizing 校准。
7. WHERE 某个 Candidate_Scheme 依赖高风险禁区改动, THE Research_Harness SHALL 允许该 Scheme 在本 spec 的 design / tasks 阶段完成 research-only 设计与验证（不修改任何高风险目录），并 SHALL 在 design 文档中显式声明"依赖高风险禁区改动，延后到独立 spec 实现"；真正的高风险修改 MUST 延后到独立 spec + 独立 PR 按 AGENTS §9 / §10 拆分实现，不在本 spec 内直接落地。
8. THE Research_Harness SHALL NOT 在 R1/R2 research 阶段触发任何 WS 重连、REST 对账、auto_resume 路径；这些路径只允许在后续独立 live migration spec 中基于 `bktrader-ctl --json` 的只读观测讨论（AGENTS §10 核心禁令）。
9. THE Research_Harness SHALL NOT 在本 spec 中建议或要求 `bktrader-ctl live control-reset`。该命令只用于异常修复，不能作为普通 PnL / parity / research gate 失败的 rollback 流程。
