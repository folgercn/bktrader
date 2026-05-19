# Path C Unified Pretouch — 实验报告

生成时间：2026-05-13T15:42:36.248011+00:00

> **声明**：本实验不改变前两个 spec（`pre-breakout-timing-classifier`、
> `pretouch-classifier-refinement`）的既有部署/发现。
> 结论仅作为生产集成决策的输入。

## Go/No-Go 判定

**决策：Marginal Go**

- test calendar_sum: **+7.40%**
- bootstrap CI lower (5th): **+3.67%**
- bootstrap CI upper (95th): **+11.43%**
- overfitting_flag: **True**

**判定理由**：test calendar_sum = +7.40% 处于 Marginal Go 区间。原因：存在 overfitting flag。

**下一步**：分析瓶颈（gate 质量问题 vs 模型泛化问题）；考虑进一步扩展时间范围或引入新 symbol；或等待更多 V6 gate events 积累后重新验证。

## 假设验证

### H1_sample_stability: ❌ 未通过

证据：test_cs = +7.40% > +3.10%；CI width = 7.76pp ≥ 3.0pp

### H2_gate_quality: ✅ 通过

证据：原 116 events 平均 pnl = 0.0023%，新增 events 平均 pnl = 0.0003%，差异 = 0.0020%，p-value = 0.0000，degradation = False

### H3_rule_convergence: ✅ 通过

证据：Rule Stability Score = 0.50 ≥ 0.5；规则差异：节点 1: 一侧缺失（扩展=('prev1_close_pos_side', '≤', 0.27), 原始=None）；节点 2: 一侧缺失（扩展=('touch_extension_atr', '≤', 0.0), 原始=None）

## 历史对比

| 指标 | Baseline Legacy (116 events) | Refinement A2 (116 events) | Path C |
|------|-----|-----|-----|
| test calendar_sum | +2.98% | +3.39% | +7.40% |
| bootstrap CI | [1.16%, 4.73%] | [1.60%, 5.18%] | [3.67%, 11.43%] |
| CI width | 3.57pp | 3.59pp | 7.76pp |
| classifier | LogisticRegression | RuleBased_DT3 | RuleBased_DT3 |
| regime schema | 5-regime | 3-regime | 3-regime |

## 事件池概况

- 最终事件池大小: **1013** events
- 包含原 116 events: **True**
- 达到 200 events 目标: **True**
- 选定策略: 选择维度 C：直接使用 events_execution_labeled.csv（original_t2_delay60），按 bars_cache 覆盖时间范围筛选。产出 1013 events，包含全部 116 个原始 events。理由：最简单直接，事件数量充足，保证向后兼容。

## Simulation 统计

- 总 events: 1013
- 成功模拟: 1013
- 跳过: 0
- Drift check: n_compared=580, max_diff=0.000000, warning=False

## 标签分布

### Train set
- fast: 304 (50.1%)
- slow: 67 (11.0%)
- skip: 236 (38.9%)

### Test set
- fast: 233 (57.4%)
- slow: 45 (11.1%)
- skip: 128 (31.5%)

- label_shift_vs_original: True (原 skip=21.6%, 扩展 skip=35.9%)

## DT3 训练结果

- LOOCV calendar_sum: +25.50%
- Train calendar_sum: +25.64%
- Test calendar_sum: +7.40%
- Accuracy (train): 83.02%

### 决策规则

```
规则 1: 若 prev1_body_atr ≤ -1.36
  → slow
  [训练样本: 2 个, 置信度: 100.0%]

规则 2: 若 prev1_body_atr > -1.36 且 prev1_close_pos_side ≤ 0.27 且 touch_extension_atr ≤ 0.00
  → fast
  [训练样本: 6 个, 置信度: 66.7%]

规则 3: 若 prev1_body_atr > -1.36 且 prev1_close_pos_side ≤ 0.27 且 touch_extension_atr > 0.00
  → fast
  [训练样本: 45 个, 置信度: 97.8%]

规则 4: 若 prev1_body_atr > -1.36 且 prev1_close_pos_side > 0.27 且 prev1_body_atr ≤ -0.54
  → slow
  [训练样本: 6 个, 置信度: 66.7%]

规则 5: 若 prev1_body_atr > -1.36 且 prev1_close_pos_side > 0.27 且 prev1_body_atr > -0.54
  → fast
  [训练样本: 312 个, 置信度: 81.4%]

```

## 规则对比

- Rule Stability Score: **0.50**
- Instability Warning: False
- 规则差异：节点 1: 一侧缺失（扩展=('prev1_close_pos_side', '≤', 0.27), 原始=None）；节点 2: 一侧缺失（扩展=('touch_extension_atr', '≤', 0.0), 原始=None）

## 深度对比

- max_depth=2: LOOCV calendar_sum = +25.48%
- max_depth=3: LOOCV calendar_sum = +25.50%
- max_depth=4: LOOCV calendar_sum = +25.51% ← best
- max_depth=5: LOOCV calendar_sum = +25.40%
- DT3 仍为最优: **False**

## 稳健性验证

- Overall bootstrap CI: [+3.67%, +11.43%] (width=7.76pp)
- CI width 对比: 原=3.58pp, 扩展=7.76pp
- Overfitting flag: True
- LOOCV degradation flag: False
- Label distribution shift: False
- Gate quality degradation: False
- Small sample warning: False

### Per-symbol Bootstrap

- BTCUSDT: mean=+2.20%, CI=[+-0.26%, +4.60%]
- ETHUSDT: mean=+5.12%, CI=[+1.95%, +8.20%]
