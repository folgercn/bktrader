# Pre-Breakout Timing Classifier 实验报告

> 生成时间: 2026-05-13 02:18:04
> 样本量: 116 events (small_sample_warning=true)
> 决策建议: **Conditional Go**

## 1. 实验设计

### 1.1 核心假设

不同 pre-breakout 特征组合的 event，其最优入场延迟不同。
通过在 breakout 触发前就确定入场策略（conditional delay），
避免 post-breakout tick 观察的噪声问题。

### 1.2 方法

1. 对每个 V6 gate event 在 D=0/5/10/15/pullback 下模拟 V4 执行
2. 基于 PnL 确定每个 event 的 Optimal_Delay_Label
3. 使用 pre-breakout 特征训练分类器预测最优 delay regime
4. Time-split 验证（前 60% train，后 40% test）

### 1.3 使用特征

- 使用特征数: 10
- 特征列表: signal_atr_percentile, roundtrip_cost_atr, prev1_body_atr, prev1_range_atr, prev1_close_pos_side, prev_sma5_gap_atr, prev_sma5_slope_atr, level_to_prev_close_atr, level_to_signal_open_atr, touch_extension_atr
- 排除特征（缺失率 > 50%）: state_frac_0, state_frac_1, state_frac_2, state_frac_3, state_entropy

## 2. Optimal_Delay_Label 分布

| Label | 数量 | 占比 |
|-------|------|------|
| D0 | 62 | 53.4% |
| D5 | 12 | 10.3% |
| D10 | 6 | 5.2% |
| D15 | 3 | 2.6% |
| pullback | 8 | 6.9% |
| skip | 25 | 21.6% |

**Oracle calendar_sum（理论上限）**: +9.4422%
**Baseline B (D=5s) 全量**: +3.7186%
**Baseline B (D=5s) test set**: +2.7366%
**Oracle vs Baseline B 差异**: +5.7236%

## 3. 分类器对比

| 分类器 | LOOCV calendar_sum | Train calendar_sum | Test calendar_sum | Train Accuracy |
|--------|--------------------|--------------------|-------------------|----------------|
| RuleBased_DT3 | +3.2386% | +3.4101% | +0.0000% | 82.7% |
| DecisionTree | +3.2832% | +3.4174% | +0.0000% | 80.8% |
| RandomForest | +3.2986% | +3.6819% | +0.0000% | 90.4% |
| LogisticRegression ★ | +3.4959% | +3.5945% | +2.9822% | 82.7% |

**最优分类器**: LogisticRegression (LOOCV=+3.4959%)

## 4. 结果对比（Baselines vs Classifier）

| 策略 | Calendar Sum | Trade Count | Win Rate | Avg PnL |
|------|-------------|-------------|----------|---------|
| Baseline D0 | +4.2919% | 116 | 77.6% | 0.001419 |
| Baseline D5 | +3.7186% | 116 | 75.0% | 0.001229 |
| Baseline D10 | +3.5402% | 116 | 74.1% | 0.001171 |
| Baseline D15 | +3.5070% | 116 | 72.4% | 0.001160 |
| Baseline pullback | +2.3337% | 116 | 68.1% | 0.000773 |
| **Classifier (LogisticRegression)** | **+2.9822%** | — | — | — |
| Oracle | +9.4422% | — | — | — |
| Reference (dynamic-entry-timing) | +2.40% | — | — | — |

**Classifier vs Baseline B (test set) 改善**: +0.2456%
**Baseline B test set calendar_sum**: +2.7366%
**Oracle 实现率**: 31.6%

## 5. Bootstrap 置信区间

- 重采样次数: 1000
- random_state: 42

**Overall**: calendar_sum=+2.9822%, CI [5th, 95th] = [+1.1561%, +4.7285%]
**BTC** (24 events): CI [+0.5750%, +2.2613%]
**ETH** (23 events): CI [-0.0535%, +2.9442%]

## 6. 分 Symbol 验证

### BTC

- Test events: 24
- Classifier calendar_sum: +1.4587%
- Baseline B calendar_sum: +1.3500%
- Classifier win_rate: 87.5%
- ✓ Classifier superior for BTC

### ETH

- Test events: 23
- Classifier calendar_sum: +1.5235%
- Baseline B calendar_sum: +1.3866%
- Classifier win_rate: 78.3%
- ✓ Classifier superior for ETH

## 7. 过拟合检查

- Train calendar_sum: +3.5945%
- Test calendar_sum: +2.9822%
- LOOCV calendar_sum: +3.4959%
- Drop (train→test): +17.03%
- overfitting_flag: False
- loocv_degradation_flag: False

## 8. Regime 稳定性分析

- label_distribution_shift: False
- Max proportion difference: 9.9pp

| Regime | Train% | Test% | Δ(pp) |
|--------|--------|-------|-------|
| D0 | 56.5% | 48.9% | 7.6 |
| D5 | 7.2% | 14.9% | 7.6 |
| D10 | 4.3% | 6.4% | 2.0 |
| D15 | 4.3% | 0.0% | 4.3 |
| pullback | 2.9% | 12.8% | 9.9 |
| skip | 24.6% | 17.0% | 7.6 |

## 9. Feature Importance

| Rank | Feature | Importance |
|------|---------|------------|
| 1 | prev1_close_pos_side | 0.5019 |
| 2 | prev_sma5_slope_atr | 0.4584 |
| 3 | signal_atr_percentile | 0.4122 |
| 4 | prev_sma5_gap_atr | 0.3916 |
| 5 | prev1_range_atr | 0.3658 |
| 6 | touch_extension_atr | 0.2335 |
| 7 | level_to_signal_open_atr | 0.1720 |
| 8 | level_to_prev_close_atr | 0.1709 |
| 9 | prev1_body_atr | 0.1277 |
| 10 | roundtrip_cost_atr | 0.0000 |

## 10. Go/No-Go 决策

### 判定结果: **Conditional Go**

判定依据:
- Classifier test calendar_sum: +2.9822%
- Baseline B (D=5s) test set calendar_sum: +2.7366%
- 改善幅度: +0.2456%
- overfitting_flag: False
- loocv_degradation_flag: False

判定规则:
- **Go**: classifier > baseline_b + 0.5% 且无 overfitting_flag
- **Conditional Go**: classifier > baseline_b 但改善 < 0.5%，或存在 loocv_degradation_flag
- **No-Go**: classifier ≤ baseline_b 或 overfitting_flag=true

## 11. 下一步行动建议

~ **有条件采用，需进一步验证**

- 等更多 V6 gate events 积累后重新验证
- 尝试简化为 2-regime 分类（fast vs slow）
- 关注 LOOCV 退化和 symbol 差异

## 12. 与 dynamic-entry-timing 实验对比

| 维度 | dynamic-entry-timing | pre-breakout-timing-classifier |
|------|---------------------|-------------------------------|
| 特征时间点 | Post-breakout（touch 后 5-60s） | Pre-breakout（signal bar 时刻） |
| 特征来源 | 1s bar 实时计算 | events_execution_labeled.csv 已有列 |
| 决策方式 | 步进式规则匹配 | 一次性分类 |
| Calendar Sum | +2.40% | +2.9822% |
| 已知问题 | flow_imbalance 为 placeholder | 116 events 样本量小 |
| 结论 | No-Go | Conditional Go |

**分析**: Pre-breakout 方法使用 V6 gate 已验证有效的特征，
避免了 post-breakout tick 特征的噪声问题。
但受限于 116 events 的小样本量，统计显著性有限。

## 附录: 回调入场参数

- pullback_target_atr: 0.05
- pullback_window_seconds: 120
- start_offset_seconds: 5
