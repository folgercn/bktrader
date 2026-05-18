# 规则型分类器规则描述

> 基于 DecisionTree (max_depth=3) 提取的人类可读规则

## 分类器参数

- 模型类型: DecisionTree
- max_depth: 3
- 训练样本数: 52
- LOOCV calendar_sum: +3.2386%
- Train accuracy: 82.7%

## 特征重要性（规则型分类器）

1. **prev1_close_pos_side**: 0.3885
2. **level_to_prev_close_atr**: 0.3842
3. **signal_atr_percentile**: 0.2273

## 规则描述

（规则文本将在 runner 集成 model 对象后由 `extract_rules_text()` 生成）
