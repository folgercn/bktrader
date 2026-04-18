# Live Session `plan-exhausted` 排查手册

本文用于排查以下典型现象：

- `live session` 长时间保持 `RUNNING`
- 运行日志持续出现 `strategy · plan-exhausted`
- 5m / 4h / 1d runtime 仍在继续进事件，但 session 一直不开仓

## 1. 先看 6 个关键字段

优先观察目标 `live session.state` 中的以下字段：

- `planLength`
- `planIndex`
- `lastStrategyEvaluationPlanLength`
- `lastStrategyEvaluationRemaining`
- `lastStrategyEvaluationStatus`
- `positionRecoveryStatus`

如果同时看到：

- `lastStrategyEvaluationStatus = plan-exhausted`
- `lastStrategyEvaluationRemaining = 0`
- `planIndex >= lastStrategyEvaluationPlanLength`

则说明当前评估链路已经没有可用的下一步计划，后续 heartbeat 不会再进入正常的策略决策与下单路径。

## 2. 再判定 session 是否还有继续运行的理由

继续看：

- `hasRecoveredPosition`
- `hasRecoveredVirtualPosition`
- `lastDispatchedOrderStatus`
- `lastSyncedOrderStatus`

### A. 仍有真实仓位 / virtual 仓位

这类 session 仍可能需要继续运行，用于：

- 仓位状态监控
- reduce-only 退出单跟踪
- watchdog fallback 风控退出

此时不应简单把 `RUNNING + plan-exhausted` 视为无害噪音，应该继续核查：

- 是否存在活动中的退出单
- `positionRecoveryStatus` 是否为 `protected-open-position` / `closing-pending`
- `watchdogExitStatus` 是否卡在 `intent-ready` / `order-working`

### B. 已经 `flat` 且无 virtual 仓位

这时如果仍看到：

- `positionRecoveryStatus = flat`
- `hasRecoveredPosition = false`
- `hasRecoveredVirtualPosition = false`
- session 仍然是 `RUNNING`

则需要继续区分两种情况：

- 若同时还持续停留在旧的 `planIndex >= lastStrategyEvaluationPlanLength`，说明 exhausted plan 没有被正确收口或 rollover。
- 若 `planIndex = 0` 且 `planLength = 0`，说明系统已经把旧 plan 清空，正在等待同一条 `RUNNING` session 进入下一轮 plan 周期。

## 3. `planLength` 与 `lastStrategyEvaluationPlanLength` 不一致意味着什么

常见异常组合：

- `planLength = 918`
- `lastStrategyEvaluationPlanLength = 914`
- `planIndex = 922`

这通常表示：

- session state 中缓存的计划长度不是当前正在评估的那份 plan
- 当前 plan 可能已重算或内存缓存已刷新
- 但 `planIndex` 没有同步回收或钳正

这种情况下，`planIndex` 可能已经落在当前 plan 之外，session 会稳定进入 `plan-exhausted`。

## 4. 修复后的预期行为

针对 `flat + no virtual position + no active orders + plan-exhausted`，系统现在会：

1. 把 `planIndex` 先收敛到当前 `len(plan)`
2. 记录 `completedAt`
3. 清掉进程内的 live plan 缓存
4. 把当前 session 的 plan 状态重置为待下一轮构建
5. 保持同一条 `live session` 继续 `RUNNING`

下一次 runtime heartbeat 到来时，系统会基于当前市场重新构建一份新的 live plan，并继续做策略判断。

### 修复后 rollover 前后的典型状态

- 刚命中 exhausted：
  - `lastStrategyEvaluationStatus = plan-exhausted`
  - `completedAt` 已写入
  - `lastPlanRolloverAt` 已写入
- rollover 已调度、等待下一轮 plan：
  - `planIndex = 0`
  - `planLength = 0`
  - `livePlans` 缓存已清空
- 下一轮 plan 已生成：
  - `planLength > 0`
  - `completedAt` 被清除
  - session 继续按新 plan 参与判断与开仓

因此修复后如果旧 plan 已经耗尽，不需要人工手动 stop / restart / 新建 session 才能进入下一轮。

## 5. 仍然不开仓时怎么继续排查

如果 session 已不再长期卡在 `plan-exhausted`，但仍未开仓，下一步请看：

- `lastStrategyDecision`
- `lastExecutionProposal.status`
- `lastExecutionProposal.reason`
- `lastStrategyEvaluationSourceGate`
- `lastStrategyEvaluationNextPlannedEventAt`

可按以下顺序判断：

1. `lastStrategyEvaluationStatus = waiting-source-states`
   - 说明不是没信号，而是运行时依赖数据还没准备好。
2. `lastStrategyEvaluationStatus = waiting-decision`
   - 说明策略 gate 没放行，需看 `lastStrategyDecision.reason`。
3. `lastStrategyEvaluationStatus = waiting-execution`
   - 说明有意图，但执行层没有给出 `dispatchable` proposal。
4. `lastStrategyEvaluationStatus = intent-ready`
   - 说明已经具备派单条件，此时应继续排查 `dispatchMode`、订单状态与自动派单门控。

## 6. 一句结论模板

适合在群里同步的简版结论：

`这不是单纯“5m 没机会”，而是 live session 的当前 plan 已经耗尽；如果同时已经 flat 且无 virtual position，系统应自动 rollover 到下一轮 plan，而不是长期卡在同一份 exhausted plan 上。`
