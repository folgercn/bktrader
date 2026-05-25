# ETH Pretouch (T2) 静态 Downsize 规则优化回测研究与对齐实施方案

本文档记录了针对 `original_t2` ETHUSDT 1h 独立 Downsize 规则在 2022-07 至 2026-04 长周期回测（包含 Canary 推广验证段）中的优化研究成果、具体规则定义、详细回测表现及后续研究对齐方向。

> 2026-05-25 更新：本文最初的规则 A / B 来自全量数据高维 grid search，因此只能视为
> research hypothesis，不能直接作为 Go live 对齐依据。正式晋级口径以后续
> `t2_shadow_candidate_sweep_20260525` 的 rolling walk-forward / frozen train-window
> 诊断为准。当前主候选是
> `static_optimal_or_doc_a_ctx12h_range_le350_scale025_downsize`，不是本文的 `static_rule_a` /
> `static_rule_b` 直接 `scale=0.0` 版本。

---

## 1. 优化背景与动机

在此前的优化尝试中，由于未引入 1h 棒线的 Context 时序上下文特征，所使用的纯局部突破特征（如 `prev1_range_atr`）对于潜在恶劣行情的规避能力受限。旧方案在 4 年长周期回测中仅实现了约 **+2.2%** 的增幅，无法满足通过 Canary 影子系统高确定性晋级门槛（$\ge +0.5pp$ 且回撤不恶化）。

为此，我们引入了 1h 时序上下文特征（如 12小时波动区间宽度 `ctx12h_range_atr`），并使用高维网格搜索（Grid Search）在全量数据集上挖掘极佳的静态 Downsize 拦截规则，获得了两个表现极其优异的候选方案。

---

## 2. 优化规则定义

优化方案包含 **前置避险（Profit Protection）判定** 和 **静态 Downsize 条件判定** 两个核心步骤。

### 2.1 前置避险机制（Profit Protection - PP）
为防止在超强单边大行情中错失收益，不论规则 A 还是规则 B，在触发前都必须先通过 PP 判定。如果满足以下任一条件，系统判定为“高胜率/高盈亏比避险安全区”，**不执行 Downsize**（照常足额进场）：
1.  **趋势强弱**：$ctx12h\_side\_return\_atr \ge 1.45$ （过去 12 小时的单向趋势极强）
2.  **波动状态**：$ctx4h\_range\_atr \ge 2.36$ 或 $ctx12h\_range\_atr \ge 3.98$ （波动极度活跃）
3.  **模型胜率**：$rf\_probability \ge 0.965$ （随机森林预测胜率极高）

### 2.2 规则 A (收益最大化 - Profit Maximization)
在未触发前置避险机制时，若同时满足以下拦截条件，则**将仓位下调至 0.0（即放弃本次交易）**：
*   **突破触及偏离度**：$touch\_extension\_atr \le -0.112263$（突破偏离关键点较少，极易形成假突破假触及）
*   **中期波动宽度**：$ctx12h\_range\_atr \ge 3.006207$（过去 12 小时波动偏宽，背景市场洗盘剧烈）

### 2.3 规则 B (回撤改善 - Drawdown Mitigation)
在未触发前置避险机制时，若同时满足以下拦截条件，则**将仓位下调至 0.0（即放弃本次交易）**：
*   **突破触及偏离度**：$touch\_extension\_atr \ge -0.1147$（突破偏离关键点较多）
*   **中期波动宽度**：$ctx12h\_range\_atr \le 3.39274$（过去 12 小时整体处于震荡收缩或低波动状态）

---

## 3. 回测指标总体对比

以下为 **Baseline (基准策略)**、**规则 A** 与 **规则 B** 在长周期回测中的核心指标对比：

| 指标维度 | Baseline 基准 | 规则 A (收益最大化) | 规则 B (回撤改善) |
| :--- | :---: | :---: | :---: |
| **下调规则** | 无 | `touch_ext <= -0.112263` & `ctx12h_range >= 3.0062` | `touch_ext >= -0.1147` & `ctx12h_range <= 3.3927` |
| **四年总收益 (PnL)** | 50.4347% | **58.3023%** (**+7.8676pp**) | **56.4308%** (**+5.9961pp**) |
| **历史段收益 (Long-cycle)** | 16.5805% | **22.4697%** (**+5.8892pp**) | **20.5990%** (**+4.0185pp**) |
| **测试段收益 (Canary)** | 33.8542% | **35.8326%** (**+1.9800pp** / 过 promozione 关) | **35.8326%** (**+1.9776pp** / 过 promozione 关) |
| **最大回撤 (Max DD)** | -7.4096% | -7.4096% (持平) | **-6.2758%** (**显著降低 1.1338pp，回撤改善 15.3%**) |
| **最惨单月收益** | -4.4311% | -4.4311% (持平) | **-2.5768%** (**大幅改善，亏损压缩 41.8%**) |
| **负收益月份数** | 19 个 | **18 个** (减少 1 个) | **18 个** (减少 1 个) |
| **下调生效率 (Hits / 总事件)**| - | 11 / 146 = 7.5% | 13 / 146 = 8.9% |
| **平均尺寸比例 (Avg Scale)** | 1.0000 | 0.9247 | 0.9110 |

---

## 4. 月度收益明细表 (Monthly PnL Table)

以下列出各月 PnL（百分比 %）及两种规则相对于 Baseline 的收益改变值（Delta %）：

| 月份 | Baseline PnL | 规则 A PnL | 规则 A Delta | 规则 B PnL | 规则 B Delta |
| :--- | :---: | :---: | :---: | :---: | :---: |
| **2022-07** | +1.5655 | +1.5655 | 0.0000 | +1.8734 | +0.3079 |
| **2022-08** | -2.5768 | -2.2655 | +0.3113 | -2.5768 | 0.0000 |
| **2022-09** | +7.6652 | +7.6652 | 0.0000 | +6.0704 | -1.5948 |
| **2022-10** | +0.4971 | +0.8119 | +0.3148 | +0.4971 | 0.0000 |
| **2022-11** | -0.6353 | -0.6353 | 0.0000 | -0.6353 | 0.0000 |
| **2022-12** | -1.0254 | +0.5176 | +1.5430 | -0.7142 | +0.3111 |
| **2023-01** | +4.5332 | +4.5332 | 0.0000 | +4.5332 | 0.0000 |
| **2023-02** | -0.6256 | -0.3143 | +0.3113 | -0.6256 | 0.0000 |
| **2023-03** | -0.0888 | -0.0888 | 0.0000 | -0.0888 | 0.0000 |
| **2023-04** | -0.6292 | -0.6292 | 0.0000 | -0.6292 | 0.0000 |
| **2023-05** | -0.9287 | -0.9287 | 0.0000 | -0.9287 | 0.0000 |
| **2023-06** | -0.1217 | -0.1217 | 0.0000 | +0.2296 | +0.3513 |
| **2023-07** | -2.4652 | -2.4652 | 0.0000 | -2.4652 | 0.0000 |
| **2023-08** | -0.6285 | -0.6285 | 0.0000 | -0.3165 | +0.3121 |
| **2023-09** | +0.5351 | +0.5351 | 0.0000 | +0.5351 | 0.0000 |
| **2023-10** | +0.5580 | +0.8724 | +0.3145 | +0.5580 | 0.0000 |
| **2023-11** | -0.3740 | -0.3740 | 0.0000 | -0.6386 | -0.2646 |
| **2023-12** | +1.9853 | +1.9853 | 0.0000 | +1.9853 | 0.0000 |
| **2024-01** | -0.1594 | -0.1594 | 0.0000 | -0.1594 | 0.0000 |
| **2024-02** | -0.7366 | -0.7366 | 0.0000 | -0.7366 | 0.0000 |
| **2024-03** | +4.6652 | +4.6652 | 0.0000 | +2.9518 | -1.7135 |
| **2024-04** | +5.0971 | +5.0971 | 0.0000 | +5.0971 | 0.0000 |
| **2024-05** | -0.7112 | -0.7112 | 0.0000 | -0.7112 | 0.0000 |
| **2024-06** | -3.0323 | -0.6677 | +2.3647 | -0.6677 | +2.3647 |
| **2024-07** | +5.1810 | +5.4933 | +0.3123 | +5.1810 | 0.0000 |
| **2024-08** | -0.8780 | -0.8780 | 0.0000 | -0.8780 | 0.0000 |
| **2024-10** | +1.3655 | +1.7829 | +0.4174 | +1.3655 | 0.0000 |
| **2024-11** | +2.9801 | +2.9801 | 0.0000 | +3.6057 | +0.6256 |
| **2024-12** | -4.4311 | -4.4311 | 0.0000 | -1.1124 | +3.3187 |
| **2025-06** | -0.9478 | -0.9478 | 0.0000 | -0.9478 | 0.0000 |
| **2025-07** | +4.6047 | +4.6047 | 0.0000 | +4.6047 | 0.0000 |
| **2025-08** | +10.4234 | +10.4234 | 0.0000 | +10.4234 | 0.0000 |
| **2025-09** | -1.6000 | -1.6000 | 0.0000 | -1.6000 | 0.0000 |
| **2025-10** | +1.6242 | +1.6242 | 0.0000 | +1.6242 | 0.0000 |
| **2025-11** | +6.7450 | +6.7450 | 0.0000 | +7.0577 | +0.3127 |
| **2025-12** | +1.3152 | +1.3152 | 0.0000 | +1.3152 | 0.0000 |
| **2026-01** | +3.0880 | +3.0880 | 0.0000 | +3.0880 | 0.0000 |
| **2026-02** | +3.7128 | +4.0263 | +0.3135 | +3.7128 | 0.0000 |
| **2026-03** | +1.8302 | +1.8302 | 0.0000 | +1.8302 | 0.0000 |
| **2026-04** | +3.0581 | +4.7230 | +1.6649 | +4.7230 | +1.6649 |

---

## 5. 拦截事件审计明细 (Event Audit)

### 5.1 规则 A 拦截明细 (共 11 次)
规则 A 在长周期内所拦截的 **11次事件全部为负收益交易（胜率 100% 规避）**，没有任何一次正收益误伤：
*   **2022-08**：拦截 1 次 PnL 为 `-0.3113%` 的交易。
*   **2022-10**：拦截 1 次 PnL 为 `-0.3148%` 的交易。
*   **2022-12**：拦截 2 次 PnL 分别为 `-0.3111%` 和 `-1.2319%` 的交易。
*   **2023-02**：拦截 1 次 PnL 为 `-0.3113%` 的交易。
*   **2023-10**：拦截 1 次 PnL 为 `-0.3145%` 的交易。
*   **2024-06**：拦截 1 次 PnL 为 `-2.3647%` 的恶劣重仓亏损。
*   **2024-07**：拦截 1 次 PnL 为 `-0.3123%` 的交易。
*   **2024-10**：拦截 1 次 PnL 为 `-0.4174%` 的交易。
*   **2026-02**：拦截 1 次 PnL 为 `-0.3135%` 的交易（测试段）。
*   **2026-04**：拦截 1 次 PnL 为 `-1.6649%` 的大亏损单（测试段）。

### 5.2 规则 B 拦截明细 (共 13 次)
规则 B 拦截了 13 次事件，其中有 3 次为正收益误伤，但其通过**完美规避 2024-12 月的超级黑天鹅大单**（避开 `-3.3187%` 巨亏），实现了极致的最大回撤压缩：
*   **2022-07**：拦截 1 次 PnL 为 `-0.3079%` 的交易。
*   **2022-09**：误伤 1 次 PnL 为 `+1.5948%` 的正收益交易。
*   **2022-12**：拦截 1 次 PnL 为 `-0.3111%` 的交易。
*   **2023-06**：拦截 1 次 PnL 为 `-0.3513%` 的交易。
*   **2023-08**：拦截 1 次 PnL 为 `-0.3121%` 的交易。
*   **2023-11**：误伤 1 次 PnL 为 `+0.2646%` 的正收益交易。
*   **2024-03**：误伤 1 次 PnL 为 `+1.7135%` 的正收益交易。
*   **2024-06**：拦截 1 次 PnL 为 `-2.3647%` 的亏损大单。
*   **2024-11**：拦截 2 次 PnL 分别为 `-0.3137%` 和 `-0.3119%` 的交易。
*   **2024-12**：**拦截 1 次 PnL 为 `-3.3187%` 的超级黑天鹅亏损（全周期最差月份核心源头）**。
*   **2025-11**：拦截 1 次 PnL 为 `-0.3127%` 的交易（测试段）。
*   **2026-04**：拦截 1 次 PnL 为 `-1.6649%` 的亏损大单（测试段）。

---

## 6. 后续研究对齐方向

### 6.1 正式 runner 对齐

后续不再把本文的 A/B 全量搜索结果直接接入 live。研究推进顺序为：

*   用 `t2_static_downsize_deep_dive.py` 做固定规则、命中事件、集中度、scale ladder 诊断。
*   用 `t2_shadow_candidate_sweep.py` 做正式 rolling walk-forward / frozen train-window gate。
*   只有候选通过人工 review 后，才讨论 Go metadata wiring。

### 6.2 当前 under-review 候选

当前更稳的候选不是 `scale=0.0` 的规则 A / B，而是：

```text
static_optimal_or_doc_a_ctx12h_range_le350_scale025_downsize

if NOT profit_protection
   and ctx12h_range_atr <= 3.500000
   and (
       (eff_300s >= 0.925057 and ctx12h_side_return_atr <= -0.282982)
       or
       (touch_extension_atr <= -0.112263 and ctx12h_range_atr >= 3.006207)
   ):
    scale = 0.25
else:
    scale = 1.0
```

2026-05-25 rolling walk-forward 结果：

| Candidate | All delta | Canary delta | Avg scale | Selected | Gate |
| --- | ---: | ---: | ---: | ---: | --- |
| `static_optimal_or_doc_a_ctx12h_range_le350_scale025_downsize` | `+7.374277pp` | `+1.718344pp` | `0.958904` | `8/146` | rolling shadow pass |

Stability audit 显示 `ctx12h_range_atr <= 3.40/3.45/3.50` 是同一收益平台，不是
`3.50` 单点命中；`3.25..3.60` 区间内有 `8/19` 个阈值同时满足 all/canary/avg-scale/
frozen-holdout 约束。Frozen train-window 诊断进一步显示：train `+8.291409pp`、
holdout `+1.718344pp`、all `+10.009754pp`、holdout avg scale `0.956731`。因此它已从原始 OR 候选的
`research_candidate_under_review` 提升为 `research_candidate_review_ready`。在完成人工 review、
事件级审计和 live 可重建性确认前，仍不能直接进入 live 实现。

2026-05-25 追加的 purged nested threshold selection 进一步降低了“事后固定 3.50”的过拟合担忧：
每个 forward month 只允许使用更早月份训练，并空出 1 个 purge month；阈值选择不是取训练收益最高
尖点，而是取训练窗内达到 best delta `95%` 的阈值平台中位数。该过程在 `28/40` 个 forward month
可激活，自动选择的阈值分布为 `3.35, 3.45, 3.60, 3.70, 3.75, 3.80`，不是单点 `3.50`。结果：

| Validation | All delta | Long delta | Canary delta | Avg scale | Selected |
| --- | ---: | ---: | ---: | ---: | ---: |
| fixed `ctx12h_range_atr <= 3.50` rolling | `+7.374277pp` | `+5.655933pp` | `+1.718344pp` | `0.958904` | `8` |
| purged nested plateau threshold | `+7.687331pp` | `+5.968986pp` | `+1.718344pp` | `0.953767` | `9` |

这说明当前收益不是依赖 `3.50` 的单点调参；更合理的解释是存在一个中等 `ctx12h_range_atr`
坏桶平台。不过 nested selector 仍共享同一条 OR 结构与 PP 规则，因此它只证明 range cap
不敏感，不能证明整条 OR 规则已经可上线。

2026-05-25 继续追加 selected-event reconstructability audit：

- Script:
  `research/entry_redesign/scripts/timing_probability_unified/t2_downsize_selected_event_reconstructability.py`
- Output:
  `research/entry_redesign/scripts/output/timing_probability_unified/t2_downsize_selected_event_reconstructability_20260525/`
- Result:
  `8/8` selected events 的 closed-bar context 可从 ETHUSDT 1s cache 重新聚合复算；
  `8/8` 复算后的 `profit_protection` / branch / final downsize decision 与当前候选命中一致；
  max feature abs diff 为 `4.4408920985e-16`。

事件分支分布为：`static_optimal` 命中 `5` 次、`doc_rule_a` 命中 `2` 次、
`static_optimal+doc_rule_a` 同时命中 `1` 次；所有 selected event 的 PP 均为 `False`。
这说明候选依赖的 4h/12h closed-bar context 在研究数据路径上是 live-decision-time
可重建的。剩余缺口是 intrabar touch 特征（`eff_300s`、`touch_extension_atr`）仍来自既有事件 ledger；
进入任何 Go metadata wiring 前，还需要确认 live metadata 中已有同口径字段，或补 shadow-only
telemetry 后再 review。

2026-05-25 metadata readiness audit 结论：

- Script:
  `research/entry_redesign/scripts/timing_probability_unified/t2_downsize_live_metadata_readiness_audit.py`
- Output:
  `research/entry_redesign/scripts/output/timing_probability_unified/t2_downsize_live_metadata_readiness_20260525/`
- Static code read:
  `rf_probability`、`eff_300s`、`touch_extension_atr` 已在 advance-plan 路径可用；
  `ctx12h_side_return_atr`、`ctx4h_range_atr`、`ctx12h_range_atr` 目前不是 Go live decision metadata 字段；
  signalBarDecision 只保留 `current` / `prevBar2` 快照，未保留完整 12h closed-bar context。
- Production read-only spot-check:
  `bktrader-ctl order list --limit 20 --json` 中最近 ETH entry order 的
  `intent.metadata.pretouchEvent` / `executionProposal.metadata.pretouchEvent`
  均包含 `eff300s`、`touchExtensionAtr`、`speed300sAtr`、`preTouchSeconds`、`atr` 等字段；
  未看到 4h/12h context 字段。

2026-05-25 根据人工推进意见，下一步不再停留在 shadow-only telemetry，而是直接进入
testnet shadow submitted quantity downsize：

*   只在 `pretouchShadowMode=testnet_shadow_collect` 下计算并记录
    `t2StaticDownsizeCandidate`。
*   只有 `pretouchShadowT2StaticDownsize=true`、候选规则 selected，且既有 testnet risk-on guard
    已允许提交 shadow quantity 时，才把 `submittedQuantityAfterShadow` 乘以 `0.25`。
*   非 sandbox / guard 未通过 / mainnet 路径只记录 would-downsize 与 block reason，submitted
    quantity 保持原 production sizing。
