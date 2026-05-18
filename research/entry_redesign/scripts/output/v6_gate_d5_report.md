# V6 Gate D=5 Tick 回测报告

## 实验目标

验证假设：将 V6 candidate_001 gate 过滤后的高质量 events，用 D=5 tick 执行（V4 execution model），能否超过 V6 baseline 的 33.02% calendar sum。

## 实验设计

- **事件来源**: V6 `power0_fixed_1p30` lifecycle ledger → 116 unique events（Zero-Initial-Reentry）
- **执行模型**: V4 tick execution（1s bar 级别模拟）
- **Entry**: touch_time + 5s 后第一个 1s bar close 入场
- **止损**: initial_stop_atr=0.45, stop_buffer_atr=0.05, stop_cap_atr=0.80, min_stop_bps=12
- **保护**: breakeven_at_r=0.8 (cost_lock_bps=10), trail_start_r=0.9, trail_buffer_atr=0.05
- **最大持仓**: 4 小时
- **Sizing**: [0.20, 0.10] × scale=1.3 → [0.26, 0.13]，max_trades_per_bar=2
- **Reentry**: InitialSL 后 1s reentry（slot1=0.13）
- **费用**: slippage=2bps/side, entry_fee=2bps, exit_fee=4bps
- **Calendar sum**: silo-based（每个 symbol×month 独立从 100k 开始）

### 与 V6 baseline 的关键差异

| 维度 | V6 Baseline | 本实验 (D=5) |
|------|-------------|-------------|
| 入场方式 | reentry-trigger（价格回到 level 附近） | touch_time + 5s 固定延迟 |
| 止损 | 0.05 ATR（极紧，~2-4 bps） | 0.45 ATR（宽，~50-200 bps） |
| Trailing | 0.3 ATR @ 0.5 ATR activation | 0.05 ATR @ 0.9R activation |
| Reentry 限制 | max 2/bar，跨 bar 可继续（实际 avg 2.67 次/event） | max 2 次/event 总计 |
| 总 trades | 310 | 140 |

## 结果

| 指标 | D=5 (V4 model) | V6 Baseline |
|------|----------------|-------------|
| **Calendar Sum** | **3.17%** | **33.02%** |
| Total Trades | 140 | 310 |
| Active Silos | 10 | 10 |
| Win Rate | 71.4% | — |
| Avg Win | +0.31% | ~1-5% |
| Avg Loss | -0.52% | — |
| Median Hold | 1279s (~21min) | ~minutes |
| Avg MFE_R | 0.78 | — |

### Silo 对比

| Symbol | Month | D=5% | V6% | Delta | Trades | Win% |
|--------|-------|------|-----|-------|--------|------|
| ETHUSDT | 2025-06 | -0.05% | +0.84% | -0.89% | 4 | 50% |
| BTCUSDT | 2025-07 | -0.01% | +2.36% | -2.37% | 22 | 59% |
| BTCUSDT | 2025-08 | -0.28% | -0.40% | +0.12% | 7 | 57% |
| ETHUSDT | 2025-09 | +0.04% | +0.73% | -0.69% | 6 | 67% |
| BTCUSDT | 2025-11 | +0.05% | +2.16% | -2.11% | 8 | 75% |
| ETHUSDT | 2025-12 | +1.14% | +7.91% | -6.77% | 29 | 79% |
| BTCUSDT | 2026-01 | +0.16% | +1.45% | -1.29% | 19 | 84% |
| BTCUSDT | 2026-02 | +1.08% | +5.58% | -4.50% | 17 | 76% |
| ETHUSDT | 2026-02 | +0.02% | +8.12% | -8.10% | 16 | 56% |
| ETHUSDT | 2026-03 | +1.02% | +4.27% | -3.25% | 12 | 83% |

### Exit Reason 分布

| Reason | Trades | Avg PnL |
|--------|--------|---------|
| TrailingSL | 80 (57%) | +0.38% |
| InitialSL | 38 (27%) | -0.53% |
| BreakevenSL | 18 (13%) | +0.02% |
| MaxHoldExit | 4 (3%) | -0.03% |

## 分析

### 假设被证伪

D=5 tick 执行 calendar sum 仅 3.17%，远低于 V6 baseline 的 33.02%（delta = -29.85%）。

### V6 Reentry 机制梳理

V6 `run_second_bar_replay` 的 reentry 规则：

1. **`max_trades_per_bar=2`**：每个 1h signal bar 内最多 2 次入场
2. **`reentry_timeout=1`**：exit 后 reentry window = exit 所在 bar + 下一个 bar
3. **链式续期**：每次 SL exit 更新 `last_exit_bar_index`，开启新 window。只要持续被 SL，window 就链式延续
4. **`trades_in_bar` 每个新 bar 重置**：跨 bar 后 sizing schedule index 也重置
5. **Reentry-trigger 入场**：不是固定延迟，而是价格回到 reentry level（wick anchor + ATR offset）时触发
6. **实际数据**：194 次 SL-Reentry 中 191 次在同 bar 内（1s 后），3 次跨 1 bar，0 次跨 2+ bars

### 根因：两个执行模型不可比

| 指标 | V6 Baseline | D=5 (V4 params) |
|------|-------------|-----------------|
| Mean PnL/trade | **+0.73%** | +0.13% |
| Win rate | **87.7%** | 71.4% |
| Avg win | +0.84% | +0.37% |
| Avg loss | **-0.08%** | -0.46% |
| Median hold | 414s (~7min) | 1279s (~21min) |

1. **入场机制不同**：V6 用 reentry-trigger（价格回到 level 附近时入场），确保入场方向和动量一致，win rate 87.7%。D=5 是 touch_time 后固定 5s 盲入，win rate 71.4%。

2. **止损哲学相反**：V6 用 0.05 ATR 极紧止损（avg loss 仅 -0.08%），错了立刻走。D=5 用 0.45 ATR 宽止损（avg loss -0.46%），给更多空间但亏损更大。

3. **Trailing 不同**：V6 用 0.3 ATR trailing @ 0.5 ATR activation，让利润跑到 avg win +0.84%。D=5 用 0.05 ATR trail_buffer @ 0.9R activation，trailing 太紧，avg win 只有 +0.37%。

4. **Reentry 贡献**：V6 链式 reentry 产出 310 trades（每次 SL 后立即 reentry），D=5 限制 max=2/event 只有 140 trades。

### 结论

- NEXT_EXPERIMENT.md 的假设基于错误前提：以为 V6 的 33% 只是因为 delay=60s
- 实际上 V6 的 33% 来自于 reentry-trigger 入场 + 极紧止损 + 宽 trailing + 链式 reentry 的组合
- D=5 固定延迟入场 + V4 execution params 是完全不同的执行哲学，不能直接对比 calendar sum
- 如果要在 D=5 入场上追求更高收益，需要：
  - (a) 缩紧止损（但需要更好的入场精度来维持 win rate）
  - (b) 加宽 trailing（让利润跑更远）
  - (c) 增加 reentry 次数（链式 reentry）

## 复现

```bash
python research/entry_redesign/scripts/v6_gate_d5_tick_backtest.py
```
