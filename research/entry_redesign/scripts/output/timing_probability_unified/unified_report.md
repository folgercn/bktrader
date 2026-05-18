# Timing-Probability Unified Framework — Go/No-Go 报告

> **声明**：本实验不改变前序 spec 的既有部署/发现；
> 结论仅作为生产集成决策的输入。

**生成时间**: 2026-05-15 13:55:29

## 1. Go/No-Go 判定

### 判定结果: Marginal Go ⚠️ — 继续优化或扩展数据

| 指标 | 数值 | 阈值 |
|------|------|------|
| Calendar Sum | 30.53% | ≥ 10% (Strong) / ≥ 7% (Marginal) |
| Worst SM | 2.36% | > -0.5% (Strong) / ≥ -1.0% (Marginal) |
| BTC Positive | ❌ | Required for Strong Go |
| ETH Positive | ✅ | Required for Strong Go |
| Forward CS | 10.97% | ≥ 7% (Strong) |
| Overfitting Downgrade | ✅ | |

## 2. 假设验证

### H1: 组合优势假设

- Full Unified CS: 30.53%
- Timing Only CS: 6.92%
- Probability Only CS: 11.45%
- **结论: confirmed**

### H2: Speed Gate 假设

- Gate ON CS: 30.53%
- Gate OFF CS: 31.00%
- Gate Pass Rate: 91.2%
- Worst SM (Gate ON): 2.36%
- **结论: rejected**

### H3: Event-Exact 执行假设

- Unified Calendar Sum: 30.53% (event-exact V4)
- **结论: confirmed** (正收益 = confirmed)

### H4: 稳健性假设

- Forward CS: 10.97% (阈值 ≥ 7%)
- BTC Positive: False
- ETH Positive: True
- **结论: rejected**

## 3. 历史对比表

| 指标 | Path C (1013 events) | Tick-Flow (481 events) | Unified (394 events) |
|------|-----|-----|-----|
| calendar_sum | +7.40% | +16.21% | +30.53% |
| worst SM | - | -0.48% | +2.36% |
| bootstrap CI | [+3.67%, +11.43%] | - | [+23.77%, +37.22%] |
| CI width | 7.76pp | - | 13.45pp |
| BTC | +2.20% | +4.84% | +0.00% |
| ETH | +5.12% | +11.36% | +30.43% |
| trade count | - | 481 | 394 |
| execution model | lifecycle | event-exact | event-exact |
| sizing | fixed | RF overlay | timing × RF |

## 4. Ablation Study

| 配置 | Calendar Sum | Worst SM | Trade Count | Avg PnL/Trade |
|------|-------------|----------|-------------|---------------|
| timing_only | 6.92% | 0.53% | 62 | 0.1116% |
| probability_only | 11.45% | 0.88% | 62 | 0.1847% |
| no_speed_gate | 11.63% | 0.50% | 68 | 0.1710% |
| full_unified | 11.45% | 0.88% | 62 | 0.1847% |

## 5. Base Share 敏感性分析

| Base Share | Calendar Sum | Worst SM | Trade Count | Avg PnL/Trade |
|-----------|-------------|----------|-------------|---------------|
| 0.25 | 9.54% | 0.74% | 62 | 0.1539% |
| 0.30 | 11.45% | 0.88% | 62 | 0.1847% |
| 0.35 | 13.36% | 1.03% | 62 | 0.2155% |
| 0.40 | 15.27% | 1.18% | 62 | 0.2462% |

## 6. Bootstrap 置信区间

- Overall: [+23.77%, +37.22%] (width: 13.45pp)
- BTC: [+0.00%, +0.00%]
- ETH: [+23.77%, +37.22%]
- Path C CI width 对比: 7.76pp → 13.45pp

## 7. Forward Split 验证

- Forward Calendar Sum: 10.97%
- Forward Worst SM: 0.50%
- Forward Trade Count: 78
- Overfitting Flag: True
- Forward Risk Flag: False
- Forward Underperformance: False

## 8. Timing Classifier 结果

- Selected Depth: DT3
- DT3 LOOCV CS: 14.59%
- DT4 LOOCV CS: 14.52%
- Test CS: 8.26%
- Regime Distribution: {'skip': 0, 'fast': 25, 'slow': 3}

## 9. RF 概率模型结果

- Train AUC: 1.0000
- Test AUC: 0.7019
- RF No Signal Warning: False
- Probability Stats: mean=0.9659, median=0.9850, std=0.0470
- Feature Importance Top-5:
  - prev1_range_atr: 0.1788
  - roundtrip_cost_atr: 0.1729
  - prev1_body_atr: 0.1277
  - level_to_prev_close_atr: 0.1273
  - prev_sma5_slope_atr: 0.0853

## 10. Speed Gate 分析

- Threshold (train q10): 0.228106
- Gate Pass Rate: 91.2%
- Aggressive Gate Warning: False
- Retained Avg PnL: 0.4925%
- Filtered Avg PnL: 0.0785%

## 11. 事件池统计

- Total Events: 394
- BTC: 240 (60.9%)
- ETH: 154 (39.1%)
- Long: 188 (47.7%)
- Short: 206 (52.3%)
- Small Pool Warning: True

## 12. 下一步行动建议

- ⚠️ 分析瓶颈组件（参考 Ablation Study 结果）
- 考虑调整 sizing 映射或 speed gate 阈值
- 扩展 forward 验证窗口以增强统计信心
- ⚠️ Overfitting 降级：建议扩展 forward 验证窗口
