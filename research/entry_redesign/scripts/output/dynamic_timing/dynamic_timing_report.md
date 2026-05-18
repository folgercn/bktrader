# Dynamic Entry Timing 实验报告

> **判定结果**: ❌ No-Go（不采用，回退固定 delay）
>
> ⚠️ `dynamic_timing_not_superior=true`：动态模型未超越 Baseline A
>
> ⚠️ `pullback_benefit_marginal=true`：回调入场收益边际不足
>
> ⚠️ `small_sample_warning=true`：116 events 样本量有限，结论需谨慎
>

## 1. 实验设计

- **目标**：替代固定 delay（D=5s）入场，通过实时 tick 特征动态决定入场时机
- **数据**：V6 gate 筛选后 116 unique events（candidate_001）
- **验证方式**：time-split 60/40（train 70 events / test 46 events）
- **执行模型**：V4 execution model（initial_stop_atr=0.45, breakeven_at_r=0.8, trail_start_r=0.9, trail_buffer_atr=0.05, max_hold_hours=4）
- **Cost model**：slippage=2bps/side, entry_fee=2bps, exit_fee=4bps
- **最优参数**（train set grid search）：
  - max_steps=2
  - strong_momentum_threshold=0.1
  - strong_flow_threshold=0.55
  - moderate_momentum_threshold=0.05
  - extension_threshold=0.3
  - weak_flow_threshold=0.45
  - fading_threshold=0.01
  - min_steps_for_skip=2
  - pullback_target_atr=0.02
  - decision_window_seconds=60

## 2. 结果对比

| 指标 | Dynamic | Baseline A (D=5s) | Baseline B (D=60s) | Baseline C (D=0s) |
|------|---------|-------------------|--------------------|--------------------|
| Calendar Sum (%) | 2.40 | 2.74 | 2.56 | 2.88 |
| Trade Count | 46 | 47 | 47 | 47 |
| Win Rate | 0.80 | 0.83 | 0.74 | 0.83 |
| Avg Win (%) | 0.44 | 0.44 | 0.51 | 0.45 |
| Avg Loss (%) | -0.79 | -0.85 | -0.68 | -0.83 |
| Payoff Ratio | 0.56 | 0.52 | 0.76 | 0.54 |
| Skip Rate | 0.00 | 0.00 | 0.00 | 0.00 |
| Pullback Fill Rate | 0.67 | 0.00 | 0.00 | 0.00 |
| Per-Trade Quality (bps) | 20.01 | 22.34 | 20.91 | 23.48 |

**Dynamic vs Baseline A 改善**: -0.34%（绝对值）

> ❌ **dynamic_timing_not_superior**: 动态模型 calendar sum 未超越 Baseline A。
>
> 可能原因分析：
>   - regime 分类阈值可能不适合当前市场结构

## 3. Regime 分析

### 3.1 Regime 分布统计

| Regime | Count | Trade Count | Win Rate | Avg PnL (%) | Calendar 贡献 (%) |
|--------|-------|-------------|----------|-------------|-------------------|
| Default | 72 | 72 | 72.2% | 0.05 | 1.02 |
| Moderate Momentum | 40 | 40 | 77.5% | 0.19 | 1.98 |
| Over-Extended | 4 | 3 | 100.0% | 0.61 | 0.47 |

### 3.2 Train/Test 分布稳定性

| Regime | Train (%) | Test (%) | 差异 (pp) |
|--------|-----------|----------|-----------|
| Default | 65.2 | 57.4 | -7.8 |
| Moderate Momentum | 33.3 | 36.2 | +2.8 |
| Over-Extended | 1.4 | 6.4 | +4.9 |

## 4. 参数敏感性

| 参数 | Min Cal Sum (%) | Max Cal Sum (%) | Range (%) | 敏感性 |
|------|-----------------|-----------------|-----------|--------|
| decision_window_seconds | 1.08 | 1.08 | 0.00 | OK |
| extension_threshold | 0.73 | 1.08 | 0.35 | OK |
| fading_threshold | 1.08 | 1.08 | 0.00 | OK |
| max_steps | 0.79 | 1.08 | 0.29 | OK |
| min_steps_for_skip | 1.08 | 1.08 | 0.00 | OK |
| moderate_momentum_threshold | 0.86 | 1.08 | 0.22 | OK |
| pullback_target_atr | 1.08 | 1.08 | 0.01 | OK |
| strong_flow_threshold | 1.08 | 1.08 | 0.00 | OK |
| strong_momentum_threshold | 1.08 | 1.08 | 0.00 | OK |
| weak_flow_threshold | 1.08 | 1.08 | 0.00 | OK |

## 5. Bootstrap 置信区间

Bootstrap 重采样次数：1000

| Symbol | P5 (%) | P95 (%) | Mean (%) |
|--------|--------|---------|----------|
| BTC | 0.49 | 2.28 | 1.40 |
| ETH | -0.67 | 2.44 | 0.98 |
| Combined | 0.72 | 4.15 | 2.38 |

> ⚠️ `small_sample_warning=true`：116 events 样本量下，置信区间较宽，结论需谨慎解读。

## 6. 与 V6 Baseline 差距分析

- **V6 Baseline（reentry-trigger 入场）**: 33.02%
- **Dynamic Timing（真实 tick 入场）**: 2.40%
- **差距**: 30.62%

**差距来源分析**：

V6 的 33.02% 来自以下组合优势，entry timing 层无法弥补：

1. **Reentry-trigger 入场**：V6 使用 planned price 成交（乐观假设），dynamic timing 使用真实 tick 价格
2. **极紧止损（0.05 ATR）**：V6 止损极窄，avg loss 仅 -0.08%；V4 execution 使用 0.45 ATR 止损
3. **链式 Reentry**：V6 允许同一 signal bar 内多次 reentry，放大盈利 events 的收益
4. **宽 Trailing**：V6 trailing 策略允许更大的盈利空间

entry timing 优化仅能改善「何时入场」，无法弥补执行模型（止损宽度、reentry 次数、trailing 策略）的结构性差异。

## 7. Go/No-Go 判定

### 判定结果：❌ No-Go（不采用，回退固定 delay）

**判定依据**：

- Dynamic calendar_sum: 2.40%
- Baseline A calendar_sum: 2.74%
- 改善幅度: -0.34%
- Overfitting flag: False
- High sensitivity params: None

**判定规则**：

| 条件 | 判定 |
|------|------|
| 改善 > 1.0% 且无 overfitting 且无 high_sensitivity | ✅ Go |
| 改善 > 0 但 ≤ 1.0%，或有 high_sensitivity | ⚠️ Conditional Go |
| 改善 ≤ 0 或 overfitting_flag=true | ❌ No-Go |

**分 Symbol 验证**：

| Symbol | Calendar Sum (%) | Win Rate | Negative Flag |
|--------|-----------------|----------|---------------|
| BTCUSDT | 1.37 | 87.0% | No |
| ETHUSDT | 1.03 | 73.9% | No |

## 8. 下一步行动建议

### ❌ 建议：回退到静态参数优化路径

1. 回退到 `original-t2-entry-logic-redesign` spec 的静态参数优化
2. 在固定 delay 框架内优化 D 值（D=5s vs D=10s vs D=15s）
3. 聚焦执行模型优化（止损、trailing）而非入场时机
4. 分析 dynamic timing 失败原因，为后续迭代积累经验

---

*报告由 dynamic_entry_timing_runner 自动生成*
