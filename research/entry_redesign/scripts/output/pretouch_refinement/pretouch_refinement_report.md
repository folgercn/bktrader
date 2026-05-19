# Pretouch Classifier Refinement 报告

生成时间: 2026-05-13 08:36:30 UTC

> **⚠️ 声明**
>
> 本 refinement 不改变 `pre-breakout-timing-classifier` 的既有部署/发现。
> 结论仅作为下一阶段 Path C unified spec 的决策输入。
>
> `small_sample_warning=true`：样本量有限（116 events），所有结论附带 bootstrap 置信区间。

---

## Go/No-Go 判定

**判定结果: Marginal Go**

- 最优 arm: **A2** (RuleBased_DT3)
- Test calendar_sum: **+3.3941%**
- 相对 Baseline_Legacy 改善: **+0.4119pp**
- Overfitting flag: False
- LOOCV degradation flag: False
- No refinement superior: False

**理由**: 最优 arm A2 (RuleBased_DT3)：test calendar_sum = +3.39% 处于 Marginal Go 区间 (+3.10%, +3.50%]。相对 Baseline_Legacy 改善 +0.41pp。建议等更多 V6 gate events 积累或 Path C 扩大 events 池。

- Bootstrap 90% CI: [1.5960%, 5.1824%]

## 假设验证

### H1（特征假设）：增强特征是否显著提升分类器表现？

- A1 (原+增强特征, 5-regime) test calendar_sum = +2.88%
- Baseline test calendar_sum = +2.98%
- 差异 = +-0.10pp
- **H1 不成立**：差异 +-0.10pp ≤ 0.5%，增强特征未能显著提升分类器表现。

### H2（Regime 假设）：Regime 简化是否显著提升分类器表现？

- A2 (原特征, 3-regime) test calendar_sum = +3.39%
- A3 (原特征, 2-regime) test calendar_sum = +2.79%
- Baseline test calendar_sum = +2.98%
- 最优 Regime 简化 arm: A2 (3-regime)，差异 = +0.41pp
- **H2 不成立**：差异 +0.41pp ≤ 0.5%，Regime 简化未能显著提升分类器表现。

### H3（联合假设）：仅联合路径达到 Strong Go？

- A1 test calendar_sum = +2.88% (≤ Strong Go 阈值 +3.50%)
- A2 test calendar_sum = +3.39% (≤ Strong Go 阈值)
- A3 test calendar_sum = +2.79% (≤ Strong Go 阈值)
- A4 (原+增强, 3-regime) test calendar_sum = +3.36% (≤ Strong Go 阈值)
- A5 (原+增强, 2-regime) test calendar_sum = +2.96% (≤ Strong Go 阈值)

- **H3 不成立**：A4/A5（联合路径）均未达到 Strong Go 阈值，联合路径也未能突破。

## 消融分析摘要

消融实验已跳过（最优 arm 不含增强特征）。

## Oracle 实现率对比

Oracle calendar_sum 不可用，无法计算实现率。

## 与 Baseline_Legacy 数值对照表

| Arm | Best Classifier | Test CS | vs Legacy (pp) | vs Legacy (%) | Oracle 实现率 |
|-----|----------------|---------|----------------|---------------|--------------|
| Baseline_Legacy | LogisticRegression | +2.9822% | — | — | 0.00% |
| Baseline | LogisticRegression | +2.9822% | +0.0000 | +0.00% | 0.00% |
| A1 | RandomForest | +2.8775% | -0.1047 | -3.51% | 0.00% |
| A2 | RuleBased_DT3 | +3.3941% | +0.4119 | +13.81% | 0.00% |
| A3 | RuleBased_DT3 | +2.7875% | -0.1947 | -6.53% | 0.00% |
| A4 | RuleBased_DT3 | +3.3604% | +0.3781 | +12.68% | 0.00% |
| A5 | RuleBased_DT3 | +2.9567% | -0.0255 | -0.85% | 0.00% |

## 声明与保护

> **⚠️ 重要声明**
>
> 本 refinement 不改变 `pre-breakout-timing-classifier` 的既有部署/发现。
> 结论仅作为下一阶段 Path C unified spec 的决策输入。

### 保护标志

- `small_sample_warning=true`：样本量有限（116 events），所有结论附带 bootstrap 置信区间。
- `refinement_noop=false`：最优 arm 与 Baseline_Legacy 差异 ≥ 0.05%。
- `accuracy_calendar_decoupled=false`：accuracy 与 calendar_sum 未出现显著脱钩。
- `no_refinement_superior=false`：存在 treatment arm 超越 Baseline_Legacy。

## 下一步建议

1. 等更多 V6 gate events 积累或 Path C 扩大 events 池
2. 保留当前 Conditional Go 部署不变
3. 考虑在 Path C 中验证最优 arm 的组合
