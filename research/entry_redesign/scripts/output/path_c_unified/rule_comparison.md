# DT3 规则对比报告

生成时间：2026-05-13T15:42:36.250834+00:00

## Rule Stability Score

**Score: 0.50**

差异摘要：规则差异：节点 1: 一侧缺失（扩展=('prev1_close_pos_side', '≤', 0.27), 原始=None）；节点 2: 一侧缺失（扩展=('touch_extension_atr', '≤', 0.0), 原始=None）

## 原 A2 DT3 规则

```
（原 A2 DT3 规则文本未存储在 pretouch_refinement 输出中，无法进行精确规则对比。此处使用占位文本。若需精确对比，请重新运行 pretouch_refinement 实验并保存规则文本。）
```

## 扩展池 DT3 规则

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

## 深度对比（LOOCV）

| max_depth | LOOCV calendar_sum | 备注 |
|-----------|-------------------|------|
| 2 | +25.48% |  |
| 3 | +25.50% |  |
| 4 | +25.51% | ← best |
| 5 | +25.40% |  |

DT3 仍为最优深度: **False** (best_depth=4)
