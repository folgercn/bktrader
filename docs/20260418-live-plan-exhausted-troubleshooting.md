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
- 但 session 仍然是 `RUNNING`

则说明这条 session 已经失去继续运行的业务意义，应收口为完成或停止态。

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

1. 把 `planIndex` 收敛到当前 `len(plan)`
2. 刷新 `planLength`
3. 记录 `completedAt`
4. 自动把 `live session` 收口为 `STOPPED`
5. 清掉进程内的 live plan 缓存

因此修复后如果旧 session 已经耗尽，不会再无限期保持 `RUNNING` 并重复打印 exhausted 日志。

## 5. 仍然不开仓时怎么继续排查

如果 session 已不再 `plan-exhausted`，但仍未开仓，下一步请看：

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

`这不是单纯“5m 没机会”，而是 live session 的当前 plan 已经耗尽；如果同时已经 flat 且无 virtual position，那它应被视为已完成并收口，而不是继续 RUNNING。`
