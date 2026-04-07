# 2026-04-07 MA Filter Research

## 目标

在不改动主体 breakout / reentry / exit 结构的前提下，优化外层方向过滤，让 `1d` 策略更早进入反转语境，同时保持噪声可控。

当前 canonical 语义：

- `Initial` 默认仅记录状态，`dir2_zero_initial=true` 时不实际开仓
- 真实资金暴露主要来自 `Reentry`
- 过滤器只决定当前 signal bar 是否进入 `long/short` 语境

## 研究脚本

- [`research/ma_filter_backtest.py`](/Users/wuyaocheng/Downloads/bkTrader/research/ma_filter_backtest.py)

## 已排除方向

- `EMA5/EMA8` 没有优于 `SMA5`
- `ADX` 单独使用或叠加 `EMA` 都会明显拉低收益
- `4h` 不适合加入 early reversal 窄门

## 当前最优候选

- 周期：`1d`
- 主过滤：`SMA5 hard`
- 补充过滤：`early reversal gate = 0.06 ATR`

Early reversal gate 语义：

- 仅在主过滤未放行时触发
- 允许 `close` 距离 `SMA5` 不超过 `0.06 * ATR`
- 同时要求结构已开始改善
  - long: `prev_high_2 > prev_high_1` 且 `prev_low_1 >= prev_low_2`
  - short: `prev_low_2 < prev_low_1` 且 `prev_high_1 <= prev_high_2`

## 长窗口结果

### 1D full window

- 窗口：`2023-01-01` 到 `2026-02-28`
- 基线 `SMA5 hard`
  - return: `+152.76%`
  - max drawdown: `-2.4867%`
- `SMA5 hard + early reversal 0.06 ATR`
  - return: `+156.75%`
  - max drawdown: `-2.4867%`

### 1D yearly slices

- `2023`
  - 基线：`+19.15%`
  - 候选：`+18.87%`
  - 结论：略差，回撤近似

- `2024`
  - 基线：`+51.46%`
  - 候选：`+52.60%`
  - max drawdown: `-1.39% -> -0.56%`
  - 结论：明显更优

- `2025`
  - 基线：`+32.62%`
  - 候选：`+34.02%`
  - max drawdown：相同
  - 结论：小幅更优

- `2026-01-01` 到 `2026-02-28`
  - 基线：`+2.01%`
  - 候选：`+2.01%`
  - 结论：当前样本内无差异

### 4H full window

- 窗口：`2023-01-01` 到 `2026-02-28`
- 基线 `SMA5 hard`
  - return: `+392.47%`
  - max drawdown: `-1.1114%`
- `SMA5 hard + early reversal 0.06 ATR`
  - return: `+385.56%`
  - max drawdown: `-1.2107%`

结论：

- `early reversal gate` 更像是 `1d` 专属优化
- 不建议直接迁移到 `4h`

## 当前判断

- `1d` 可继续把 `SMA5 hard + early reversal 0.06 ATR` 作为正式候选
- `4h` 维持 `SMA5 hard`
- 下一步更值得做的是：
  - 把该规则映射到 `research/backTest.py` 的正式候选参数集
  - 再对 `1d` 做滚动窗口或 walk-forward 检查
