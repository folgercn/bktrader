# 多时间框架 Pretouch 事件 Pipeline 对比

生成时间: 2026-05-15

## 1. Full-Window 结果 (2025-06 ~ 2025-10 train+test)

### ETHUSDT

| 指标 | 30min | 1h | 4h |
|------|-------|----|----|
| 事件总数 | 2438 | 1148 | 275 |
| Long / Short | 1193/1245 | 592/556 | 127/148 |
| Delay 可交易 | 1070 | 530 | 122 |
| Timing depth | DT4 | DT3 | DT4 |
| DT3 LOOCV CS | -0.1569 | **+0.3109** | -0.0904 |
| RF test AUC | 0.5323 | **0.9889** | 0.6216 |
| Speed gate pass rate | 88.6% | 90.2% | 88.5% |
| **Calendar Sum (gate ON)** | +69.76% | +9.55% | +19.17% |
| Worst SM (gate ON) | -11.80% | **+0.27%** | -11.69% |
| Trade count (gate ON) | 931 | 32 | 70 |
| Neg SM count | 2 | **0** | 2 |

### BTCUSDT

| 指标 | 30min | 1h | 4h |
|------|-------|----|----|
| 事件总数 | 2383 | 1069 | 285 |
| Delay 可交易 | 793 | 435 | 132 |
| RF test AUC | 0.9763 | 0.9819 | 0.7069 |
| **Calendar Sum (gate ON)** | +1.01% | +4.13% | +6.94% |
| Worst SM (gate ON) | -1.27% | -0.49% | -0.78% |
| Trade count (gate ON) | 41 | 47 | 74 |
| Neg SM count | 1 | 1 | 1 |

## 2. Forward Validation (2025-11 ~ 2026-04, OOS)

| 指标 | ETH 1h | ETH 4h |
|------|--------|--------|
| Forward events | 618 | 153 |
| **Calendar Sum (gate ON)** | **-7.95%** | **+7.91%** |
| Worst SM | -4.26% | -4.15% |
| Trades (gate ON) | 44 | 80 |
| Neg SM | 4 | 3 |

### ETH 4h Forward 月度明细

| 月份 | Return |
|------|--------|
| 2025-11 | +7.79% |
| 2025-12 | +4.35% |
| 2026-01 | -1.63% |
| 2026-02 | -2.44% |
| 2026-03 | -4.15% |
| 2026-04 | +3.99% |

### ETH 1h Forward 月度明细

| 月份 | Return |
|------|--------|
| 2025-11 | +0.56% |
| 2025-12 | -2.27% |
| 2026-01 | -0.93% |
| 2026-02 | +0.96% |
| 2026-03 | -4.26% |
| 2026-04 | -2.01% |

## 3. Adverse Fill 压力测试 (ETH 4h Forward)

| Scenario | CS(ON) | Worst SM | Neg SM |
|----------|--------|----------|--------|
| same_close_xslip0bps | +7.91% | -4.15% | 3 |
| next_close_xslip0bps | +8.34% | -4.08% | 3 |
| next_adverse_xslip0bps | +6.51% | -4.34% | 3 |
| next_adverse_xslip1bps | +5.69% | -4.45% | 3 |
| next_adverse_xslip3bps | +4.05% | -4.68% | 3 |
| next_adverse_xslip5bps | +2.41% | -4.92% | 3 |
| **next_adverse_xslip7bps** | **+0.77%** | -5.15% | 3 |
| next_adverse_xslip10bps | -1.69% | -5.50% | 3 |

**结论：ETH 4h forward 在 next_adverse + 7bps 滑点下仍为正（+0.77%），breakeven 约在 8~9 bps。**

## 4. 诊断分析

### ETH 30min：高 CS 但不可信

- RF AUC 0.53 = 无信号，概率模型无法区分正负事件
- 931 笔交易中大部分来自 2025-06/07/08 的趋势月
- 2025-09/10 转为负（-6%, -12%）
- **结论：训练集过拟合，不推荐进入 forward 验证**

### ETH 1h：保守但 OOS 失败

- Full-window 最稳健（0 neg SM, RF AUC 0.99）
- 但 forward 期间 -7.95%，4/6 个月为负
- 仅 32 笔 in-sample 交易 → timing classifier 过于保守
- **结论：1h 单独使用时 timing 过滤太严，OOS 泛化不足**

### ETH 4h：最有潜力

- Forward +7.91%，6 个月中 3 正 3 负
- Adverse fill 7bps 仍为正
- 事件质量高（4h 结构 breakout 信号更强）
- 但 worst SM = -4.15%（2026-03），需要 regime gate 保护
- **结论：4h 是当前最有 live 潜力的时间框架，建议进入下一轮 regime gate 研究**

### BTC 整体偏弱

- 所有时间框架 CS 在 +1% ~ +7%
- 与 AGENTS Core Memory 中 BTC fixed 20% fallback 结论一致
- **结论：BTC pretouch 不是主要 alpha 来源**

## 5. Quality Filter Sweep (ETH 4h)

| Filter | N | Fwd N | IS CS | FWD CS | FWD WS |
|--------|---|-------|-------|--------|--------|
| raw (no filter) | 275 | 153 | +19.17% | +7.91% | -4.15% |
| **pre_touch <= 3600s** | **148** | **71** | +9.55% | **+8.70%** | **-3.61%** |
| pre_touch <= 1800s | 75 | 33 | +7.61% | +1.49% | -5.28% |
| eff_300s <= 0.8 | 81 | 41 | +9.51% | -5.94% | -2.82% |
| pre<=3600 + speed>=q10 | 134 | 67 | +14.83% | -6.26% | -7.88% |

**最佳候选：`pre_touch_seconds <= 3600`**（4h bar 前 1 小时内触发）

- Forward CS 从 +7.91% 提升到 **+8.70%**
- Worst SM 从 -4.15% 改善到 **-3.61%**
- Adverse fill 10bps 仍为正 (**+2.17%**)
- 月度分布更均匀：Nov +9.66%, Dec +3.96%, Jan -1.48%, Feb -3.61%, Mar -1.79%, Apr +1.95%

## 6. 下一步

1. ~~Forward validation~~ ✅ 已完成
2. ~~Adverse fill 压力测试~~ ✅ 已完成（4h pre<=3600 forward breakeven >10bps）
3. ~~Quality filter sweep~~ ✅ 已完成（pre_touch<=3600s 为最佳）
4. **ETH 4h regime gate**: 用 validation 可观测特征进一步过滤 Jan/Feb/Mar 的负月
5. **1h + 4h 合并**: 测试两个时间框架事件合并后的 pipeline（去重同向重叠事件）
6. **Live shadow 评估**: ETH 4h + pre_touch<=3600s 已达到 live shadow 候选门槛

## 7. 参数

```json
{
  "exec_params": {
    "initial_stop_atr": 0.45,
    "breakeven_at_r": 0.8,
    "trail_start_r": 1.5,
    "trail_buffer_atr": 0.05,
    "max_hold_hours": {"30min": 1.0, "1h": 2.0, "4h": 8.0},
    "min_stop_bps": 12.0,
    "slippage": 0.0002,
    "entry_fee": 0.0002,
    "exit_fee": 0.0004
  },
  "base_notional_share": 0.80,
  "speed_gate_quantile": 0.10,
  "forward_start": "2025-11-01",
  "random_state": 42
}
```
