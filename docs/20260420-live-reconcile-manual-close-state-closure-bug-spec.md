# Live Reconcile / Manual Close / Session Refresh 闭环缺失问题说明

本文用于定义当前 `live session` 在手动平仓、成交回写、对账门禁与恢复态刷新之间的闭环缺失问题。

这不是“单条脏数据”问题，而是 `exchange facts`、`DB durable state`、`session cached state` 三层之间的状态机设计缺口。

---

## 1. 问题结论

当前系统已经具备以下能力：

- 交易所 REST 对账与 DB `Position` 覆盖
- `reconcile gate` 对 mismatch 的阻断
- recovery / takeover / close-only 状态
- reduce-only close 的执行边界保护
- fill 去重与 `exchange_trade_id` 幂等防护

但当前链路仍存在一个关键缺口：

> 当交易所侧订单已经成功成交，系统没有保证本地 `fill -> position -> session recovery state` 在同一个恢复闭环内完成收敛。

结果就是：

- 交易所已经 `FILLED`
- 本地 `fills` 未完整落库，或 `position` 未及时归零 / 删除
- `session.state.positionRecoveryStatus` 仍停在 `stale` / `conflict` / `closing-pending`
- `reconcile gate` 继续阻断执行
- 最终只能依赖人工擦库或手工改 DB 才能恢复

因此本问题的根因不是“SQL 没删干净”，而是：

1. 成交事实落库闭环不完整
2. reconcile gate 只有阻断，没有足够的自愈收敛
3. manual close 成功后，缺少强制的 recovery state 重算
4. session 内存态与 DB 持久态之间缺少可靠刷新 / 再判定机制

---

## 2. 背景与事实源分层

本问题必须严格区分三层状态：

### 2.1 交易事实层（authoritative facts）

- 交易所 REST 订单状态
- 交易所 REST 持仓状态
- 交易所成交明细 / trade fills

这是最终事实源。

### 2.2 本地持久层（durable ledger）

- `orders` 表
- `fills` 表
- `positions` 表
- `live session.state` 中需要跨重启保留的恢复 / gate 元数据

这层不是独立事实宇宙，而是本地 durable 副本与审计账本。

### 2.3 运行时缓存层（rebuildable cache / derived state）

- `session.state.recoveredPosition`
- `session.state.livePositionState`
- `session.state.virtualPosition`
- `p.livePlans[session.ID]`
- 各类 `last*` 缓存字段

这层只能是缓存与派生态，必须允许重算，不能长期作为最终裁决者。

---

## 3. 典型异常现象

当前线上 / testnet 症状可归纳为以下几类：

### 3.1 订单已成交，但本地 position / fill 没有原子同步完成

- 交易所侧订单已 `FILLED`
- 本地 `position` 仍存在
- 本地 `fills` 缺失或不完整
- `Order-Fill-Position` 三角未形成强一致

### 3.2 reconcile gate 过于刚性，缺少自愈闭环

- 一旦发现 `quantity-mismatch` 或 `db-position-exchange-missing`
- 系统直接进入 `stale` / `conflict` / `error`
- 但不会把本地状态自动收敛到交易所真实状态
- 最终需要人工擦库或手动改数据

### 3.3 manual close 流程没有形成闭环

- `reduceOnly MARKET` close 已发出并成交
- 但系统未稳定完成：
  - fill 落库
  - position 归零 / 删除
  - session recovery state 重算
  - session 从 gate / blocked 中恢复

### 3.4 session 缓存态与 DB 状态脱节

- 人工把 DB 修正到正确数量后
- `session.state` 仍停在 `conflict` / `stale`
- 说明恢复态结论被缓存住了，但没有可靠重算入口

---

## 4. 代表性复现链路

一条典型的失败链如下：

1. 系统存在 recovery / takeover 中的 `live session`
2. 本地 DB `position` 与交易所真实持仓发生偏差
3. `recoverRunningLiveSession()` 或后续刷新把 session 置入 reconcile gate blocked 模式
4. 人工或系统发出 `reduceOnly MARKET` manual close
5. 交易所已实际成交并 flat
6. 本地链路没有在同一闭环内完成：
   - 订单同步终态确认
   - fill 持久化
   - position 删除 / 归零
   - recovery gate 重算
   - session state 解锁
7. session 继续停留在 `closing-pending` / `stale` / `conflict`
8. 后续自动执行仍被 gate 卡死

这说明当前系统缺少“成交后 authoritative re-close / re-reconcile / re-refresh”的收口动作。

---

## 5. 代码锚点

以下函数是本问题的核心锚点。

### 5.1 成交回写与仓位落账

- `internal/service/order.go`
  - `finalizeExecutedOrder()`
    - 负责 fill 去重、写入 fills、累积 filled quantity、更新 order status
  - `applyExecutionFill()`
    - 负责依据 fill 更新 / 删除 `Position`

当前观察：

- `finalizeExecutedOrder()` 内部是“逐 fill 创建 + 逐 fill apply”，并非显式事务包裹
- 若 `CreateFill`、`applyExecutionFill`、`UpdateOrder`、`captureAccountSnapshot` 之间任一步失败，本地可能形成部分收敛状态
- `applyLiveSubmissionResult()` 可能直接把订单标记为 `FILLED`，但这一步并不落 `fills`、也不更新 / 删除 `Position`
- 因此 `orders.status = FILLED` 与 `positions` 仍存在可以同时出现

### 5.2 manual close / live order sync 后续刷新

- `internal/service/live_execution.go`
  - `syncLatestLiveSessionOrder()`
    - 订单终态同步后会尝试 `SyncLiveAccount()` 与 `refreshLiveSessionPositionContext()`

当前观察：

- 存在“尝试刷新”的动作，但没有把“exchange 已 flat -> DB 必须 flat -> session 必须重算为非 gate 阻断态”收敛成强约束
- 该链路对“最后一笔已派发订单”的刷新相对强，但对全量 reconcile 补回来的历史订单并不构成统一收口

### 5.3 启动恢复与 reconcile gate

- `internal/service/live.go`
  - `recoverRunningLiveSession()`
  - `reconcileLiveAccountPositions()`
  - `applyLivePositionReconcileGateState()`
  - `completeRecoveredLiveSessionMetadata()`

当前观察：

- `reconcileLiveAccountPositions()` 已经能识别 mismatch / adopted / verified 等场景
- 但 gate 更多是“判错并阻断”，不是“在 authoritative close 之后自动收口”
- `completeRecoveredLiveSessionMetadata()` 能在 flat 时清理部分 recovery / gate 字段，但依赖调用时机，不构成统一闭环
- `resolveLivePositionReconcileGate()` 的权威 gate 真相位于 `account.Metadata`，`session.state` 只是拷贝缓存
- 如果 account metadata 已变化，但没有经过 refresh fanout，session 中的 gate 结论仍会陈旧

### 5.4 session position context 刷新

- `internal/service/live_recovery.go`
  - `refreshLiveSessionPositionContext()`

当前观察：

- 该函数负责重算 `recoveredPosition`、`livePositionState`、`positionRecoveryStatus`
- 是当前最接近统一刷新入口的函数
- 但它目前更像“按当前快照派生状态”，不是“manual close 成功后的强制闭环 orchestrator”
- 目前只有 `SyncLiveAccount()` 一类路径会稳定 fanout 到 session；手工 DB 修正、部分 store 写入、reconcile 补单后的变化并不会自动回灌到 session cache

---

## 6. 根因判断

根因分四类：

### 6.1 成交事实与本地 durable state 之间缺少显式原子闭环

系统能拿到交易所成交结果，但没有把以下四步收敛为一个必须完成的闭环：

1. 持久化 fills
2. 更新 / 删除 position
3. 更新 order terminal status
4. 触发 session recovery state 重算

并且当前还存在一个更具体的断层：

- `submit` 返回 `FILLED` 不等于本地 settlement 已完成
- 订单终态更新与 `fill/position` 落账不是同一闭环

### 6.2 reconcile gate 缺少“自愈优先”路径

当前 gate 以“发现 mismatch 即阻断”为主。

问题不在阻断本身，而在于：

- 没有把“交易所已 flat / 已 authoritative close”作为优先收敛条件
- 没有自动把本地状态收敛到 `exchange truth`
- 没有在收敛成功后自动解除不再成立的 gate

### 6.3 manual close 成功不等于 session 收口成功

当前 manual close 更偏向“订单能不能打出去”，但系统缺少“订单成功后，本地 runtime 是否完成收口”的硬约束。

### 6.4 session.state 的缓存结论缺少主动失效与重算

`stale` / `conflict` / `closing-pending` 这类状态如果已经不再符合事实，应当被重新计算并清除，而不是长期残留。

更具体地说：

- `session.State` 是 cache / derived layer
- `UpdateAccount`、`SavePosition`、`UpdateLiveSessionState` 这类持久化写入本身不带 recompute hook
- `SyncLiveAccount()` 会 fanout refresh，但并不是所有影响事实的路径最终都会回到这里
- `ReconcileLiveAccount()` 在补全历史订单后也没有保证再次 session refresh

---

## 7. 目标行为

本问题修复后，系统应满足以下行为：

### 7.1 交易所事实优先

- 交易所订单终态与持仓状态是最终事实源
- DB 必须作为其 durable 副本收敛
- session 只能根据 `exchange + DB` 重新计算

### 7.2 manual close 成功后的强制闭环

若 `reduceOnly MARKET` 或其他 close-only close 已在交易所侧终态成交，则系统必须保证：

1. fill 落库
2. position 归零或删除
3. `positionRecoveryStatus` 重算
4. 若交易所已无仓位，不允许 session 继续卡在 `reconcile gate`

### 7.3 gate 应阻断未确认事实，但不能阻断已确认 flat 事实

允许：

- mismatch 时进入 `stale` / `conflict`
- 未确认时阻断 auto-dispatch

不允许：

- 交易所已无仓位、DB 已收敛 flat 后，session 仍长期卡在 blocked / stale / conflict

### 7.4 session.state 必须可重算

- 手工修 DB 不是标准修复路径
- 但即便 DB / exchange 已恢复一致，session 也必须有路径重算，而不是只能靠新建 session

---

## 8. 非目标

本次修复不是为了：

- 移除数据库
- 重写整套 live runtime
- 顺手重构执行策略链
- 放宽 reconcile gate 校验

本次目标是补齐“成交后 authoritative 收口闭环”，不是扩大修改范围。

---

## 9. 最小修复范围

建议按以下最小范围收敛：

### 9.1 新增“authoritative close settlement”闭环

在以下任一场景命中时触发统一收口：

- live order sync 确认订单已 `FILLED`
- manual close 返回终态成交
- authoritative account reconcile 发现交易所已 flat

该闭环至少执行：

1. 拉取并补齐 fills
2. 校准 / 删除本地 `Position`
3. 终结 `Order.Status`
4. 刷新 session recovery state
5. 如果当前 symbol 已 flat，清除过期 gate / takeover 阻断态

### 9.2 收敛 session 刷新入口

优先复用 `refreshLiveSessionPositionContext()`，但需要在其上层建立更明确的 orchestration：

- 先 authoritative sync
- 再 position settle
- 再 session refresh
- 最后清理不再成立的 gate / blocked 状态

### 9.3 明确 gate 的解除条件

至少补齐以下规则：

- `db-position-exchange-missing` 在 authoritative close 后若已 flat，应解除 blocking
- `quantity-mismatch` 在 authoritative reconcile 收敛后若已 match，应解除 blocking
- `close-only takeover` 在已 flat 后应退出 close-only 模式

### 9.4 不放宽执行边界

修复不能通过以下手段达成：

- 放宽 `reduceOnly` 校验
- 绕过 reconcile gate
- 取消 recovery state 限制

---

## 10. 必补回归测试

至少补以下测试：

1. `manual close filled -> fills persisted -> position deleted -> session becomes flat`
2. `exchange flat but db stale -> authoritative reconcile heals db and clears stale gate`
3. `quantity mismatch -> reconcile blocked -> manual close succeeds -> gate clears after close settlement`
4. `partial fill + restart + final close -> no duplicate exit, final position flat`
5. `close-only takeover -> exchange flat -> recovery status resets from close-only to flat`
6. `session stale/conflict state recomputes after authoritative DB+exchange convergence`

禁止只补 helper test，必须覆盖完整的 recovery / reconcile / settlement 行为链。

---

## 11. 建议的 Codex 拆分

由于涉及高风险 live/recovery 代码，建议按单问题域拆成串行小 PR，而不是一次混改。

### Worker A: authoritative close settlement

范围：

- `internal/service/order.go`
- `internal/service/live_execution.go`
- 对应测试

目标：

- 确保 live close 终态后，本地 `fill/order/position` 收敛完整

### Worker B: reconcile gate 自愈与解除条件

范围：

- `internal/service/live.go`
- `internal/service/live_recovery.go`
- 对应测试

目标：

- 明确 gate 在 authoritative flat / match 后如何解除

### Worker C: recovery/session state 重算

范围：

- `internal/service/live.go`
- `internal/service/live_recovery.go`
- 对应测试

目标：

- 保证 `stale/conflict/close-only` 在事实已变化后可重算退出

### Worker D: regression tests only

范围：

- `internal/service/live_test.go`
- `internal/service/order_test.go`
- 如有需要新增 `*_test.go`

目标：

- 补完整行为链回归测试

说明：

- Worker A/B/C 写入范围有重叠，不适合完全并行落代码
- 更合适的做法是先由一个 Codex owner 落主修复，再让测试 worker 补强回归
- 如果必须并行，应先冻结单一 ownership，避免互相覆盖 `live.go` / `live_recovery.go`

---

## 12. PR / 评审要求

该问题属于高风险 live runtime 修复，PR 描述必须明确写出：

- root cause
- 修改点
- 行为变化
- 新增测试

并且必须说明：

- 没有放宽 `manual-review` 默认行为
- 没有扩大到无关的执行策略 / 部署 / 默认参数修改
- 没有把 session cache 重新提升为事实源

---

## 13. 一句结论

这次要修的不是“某条脏数据”，而是：

> 当交易所已经成交并 flat 时，系统必须把 `exchange truth -> fills -> position -> session recovery state` 收敛成一个强制闭环，而不是把 session 长期留在过期的 gate / stale / conflict 状态里。
