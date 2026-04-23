# Reconcile Gate 自愈与启动恢复顺序修正治理计划
> **事故日期**: 2026-04-23
> **事故现象**: BTCUSDT LONG 仓位 DB=0.013 vs Exchange=0.0065 → `quantity-mismatch` → `reconcile-gate-blocked` → session 永久卡死
> **临时修复**: 手工改库 DB position 对齐交易所后 stop/start
---
## Root Cause 分析
### 直接原因
`reconcileLiveAccountPositions()` ([live.go:2113-2295](file:///Users/fujun/node/bktrader/internal/service/live.go#L2113-L2295)) 在检测到 `quantity-mismatch` 时，会写入 `conflict` + `blocking=true`。但下游所有恢复路径都没有针对 **"side 一致、交易所 authoritative、无 pending settlement 的纯 quantity 偏差"** 提供自动收敛逻辑。
### 代码级证据
1. **`livePositionReconcileGateCanSelfHeal()`** ([live.go:601-604](file:///Users/fujun/node/bktrader/internal/service/live.go#L601-L604)) 仅覆盖 `stale` + `db-position-exchange-missing`，不覆盖 `conflict` + `quantity-mismatch`
2. **`attemptLiveAccountReconcileSelfHeal()`** ([live.go:622-638](file:///Users/fujun/node/bktrader/internal/service/live.go#L622-L638)) 依赖上述函数，因此 `quantity-mismatch` 场景直接 `return account, false, nil`
3. **`StartLiveSession()`** ([live.go:1899-1933](file:///Users/fujun/node/bktrader/internal/service/live.go#L1899-L1933)) 和 **`recoverRunningLiveSession()`** ([live.go:2065-2088](file:///Users/fujun/node/bktrader/internal/service/live.go#L2065-L2088)) 都调用 `attemptLiveAccountReconcileSelfHeal`，但因为 `canSelfHeal` 返回 false，直接走到 `enterRecoveredLiveSessionReconcileGateBlocked`
4. 一旦进入 `reconcile-gate-blocked`，**没有任何定时重试或事件驱动的再次评估路径**
### 叠加原因：启动顺序
`recoverRunningLiveSession()` 在 source gate 检查之前就可能因 reconcile gate 触发 blocked，而 signal runtime WS 此时还没连上。日志表现为 "runtime source gate blocked" 先于 "signal runtime websocket connected"。
---
## Issue A: Reconcile Gate 自愈 / Adopt 路径
### 目标
把"可判定的单一事实冲突"从永久 `conflict` 收敛为 `adopted` → `verified`，不需要人工改库。
### 允许自动 Adopt 的最小边界（动作矩阵）
| 条件 | 要求 |
|------|------|
| 交易所 REST snapshot | `authoritative = true` |
| symbol | 唯一匹配（DB 与 exchange 同一 symbol） |
| side | **一致**（DB side == exchange side） |
| 冲突字段 | 仅限 `quantity` 和/或 `entryPrice`（不含 `side`） |
| pending settlement | **无**（`liveSettlementPendingOrderSymbols` 为空） |
| open orders | 无本系统未完成的 entry/exit order 影响仓位归因 |
| reconcile 证据来源 | 当前 authoritative position snapshot（非历史 external terminal orders） |
### 禁止自动 Adopt 的场景（继续 conflict + 人工处理）
| 场景 | 原因 |
|------|------|
| side mismatch | 方向不一致说明可能存在外部操作或系统 bug |
| multi-field-mismatch 含 side | 同上 |
| 存在 pending settlement | fill 未结算前 quantity 不可信 |
| 存在本系统 open order | 正在执行的订单可能导致仓位变化 |
| exchange snapshot 不是 authoritative | 弱事实不能覆盖 DB |
| symbol/account 归属不唯一 | 需要人工确认 |
### 具体改动点
#### 1. 扩展 `livePositionReconcileGateCanSelfHeal()` ([live.go:601](file:///Users/fujun/node/bktrader/internal/service/live.go#L601))
**现状**：只认 `stale` + `db-position-exchange-missing`
**改为**：增加以下判定：
- `conflict` + `quantity-mismatch` → 可自愈
- `conflict` + `entry-price-mismatch` → 可自愈
- `conflict` + `multi-field-mismatch` 但 mismatchFields 不含 `side` → 可自愈
> [!IMPORTANT]
> 要读 gate 里的 `mismatchFields`，检查是否包含 `side`。如果包含 `side`，必须返回 false。
#### 2. 新增 `adoptLivePositionFromExchangeTruth()` helper
**职责**（单一入口，禁止在多处各自改 DB）：
```
func (p *Platform) adoptLivePositionFromExchangeTruth(
    account domain.Account,
    symbol string,
    exchangePosition map[string]any,
    gate map[string]any,
) (domain.Account, error)
```
流程：
1. 前置检查：`side` 一致、无 pending settlement、无 open orders、authoritative
2. 从 gate 的 `exchangePosition` 读取 quantity/entryPrice/markPrice
3. 用 precision helper（`internal/service/precision_tolerance.go`）对齐 stepSize
4. 更新本地 `positions` 表（通过 `p.store.SavePosition`）
5. 重新调用 `refreshLiveAccountPositionReconcileGate(account)` 刷新 gate
6. 验证刷新后 gate status 变为 `verified` 或 `adopted`
7. 写 timeline event：`reconcile_adopt_applied`，携带完整上下文
> [!WARNING]
> 步骤 3 必须使用 `precision_tolerance.go` 的统一 helper，不能散落 `math.Abs` 比较。这是 PR review 黄金规则 #11。
#### 3. 修改 `reconcileLiveAccountPositions()` ([live.go:2225-2234](file:///Users/fujun/node/bktrader/internal/service/live.go#L2225-L2234))
**现状**：发现 `mismatchFields > 0` 就直接记 `conflict` + `blocking=true`
**改为**：
- 如果 mismatchFields **不含 `side`** 且 **无 pending settlement** 且 **无 open orders**：
  - 调用 `adoptLivePositionFromExchangeTruth()` 修正 DB
  - 记 gate 为 `adopted` + `blocking=false`
  - scenario 为 `exchange-truth-adopted-quantity` 或 `exchange-truth-adopted-entry-price`
- 否则保持现有 `conflict` + `blocking=true` 行为
> [!CAUTION]
> 这里有一个与你原始计划的**分歧点**。你建议在 `resolveLivePositionReconcileGate` 和 `ensureLiveExecutionPlan` 等入口处做 adopt。但我认为**正确的收口位置是 `reconcileLiveAccountPositions()`**，原因如下：
> 1. `reconcileLiveAccountPositions` 是唯一同时持有 DB position 和 exchange position 的完整上下文的位置
> 2. 在这里直接做 adopt，下游 `resolveLivePositionReconcileGate` 读到的就已经是 `adopted`/`verified`，不需要再二次处理
> 3. 避免了多入口各自改库的风险（review 黄金规则 #1：统一 helper）
>
> 如果你倾向于在 `StartLiveSession` 入口处做（作为"启动前最后一次尝试"），可以保留 `attemptLiveAccountReconcileSelfHeal` 的调用，但让它在内部触发一次完整的 `ReconcileLiveAccount`（已经在做了），然后由 `reconcileLiveAccountPositions` 内部的 adopt 逻辑完成修正。两者不矛盾。
#### 4. 清理 session state 残留
`enterRecoveredLiveSessionReconcileGateBlocked()` ([live.go:3630-3657](file:///Users/fujun/node/bktrader/internal/service/live.go#L3630-L3657)) 写入了大量 session state 字段。当 adopt 成功后，必须确保这些字段被清除：
- `recoveryMode`
- `recoveryBlockedReason` / `recoveryBlockedDetail` / `recoveryBlockedAt`
- `positionReconcileGateStatus` → 改为 `verified`
- `positionReconcileGateBlocking` → false
- `lastStrategyEvaluationStatus` → 清除 `reconcile-gate-blocked`
**现有代码 `completeRecoveredLiveSessionMetadata()` ([live.go:3505-3553](file:///Users/fujun/node/bktrader/internal/service/live.go#L3505-L3553)) 在 position quantity ≤ 0 时已有清理逻辑**，但 adopt 后 position 仍 > 0，不会走这个分支。需要在 adopt 成功路径上补一个显式的 state cleanup。
---
## Issue B: 启动恢复顺序修正
### 目标
把 "先 blocked 再等 WS ready" 改成 "先 warmup → 再 gate ready → 最后 session RUNNING"。
### 当前启动顺序（问题所在）
```
StartLiveSession / recoverRunningLiveSession
  ├─ triggerAuthoritativeLiveAccountReconcile (REST sync)
  ├─ completeRecoveredLiveSessionMetadata
  ├─ resolveLivePositionReconcileGate ← 可能因 stale source 提前 blocked
  ├─ syncLiveSessionRuntime
  │   └─ bootstrapSignalRuntimeSourceStates ← 依赖 liveMarketSnapshot
  │       └─ liveMarketSnapshot ← 可能还没 warm cache
  ├─ ensureLiveSessionSignalRuntimeStarted
  │   └─ StartSignalRuntimeSession
  │       └─ runSignalRuntimeWithRecovery (goroutine) ← WS 此时才开始连接
  ├─ awaitLiveSignalRuntimeReadiness (轮询等待)
  └─ UpdateLiveSessionStatus("RUNNING")
```
**问题**：`bootstrapSignalRuntimeSourceStates()` ([signal_runtime_sessions.go:280-324](file:///Users/fujun/node/bktrader/internal/service/signal_runtime_sessions.go#L280-L324)) 调用 `liveMarketSnapshot()` 取缓存数据。如果缓存为空（冷启动/docker 重启后），bootstrap 返回空 sourceStates，导致 source gate 在 WS 连接之前就被判定为 stale/blocked。
### 目标顺序
```
StartLiveSession / recoverRunningLiveSession
  ├─ 1. triggerAuthoritativeLiveAccountReconcile (REST sync)
  ├─ 2. completeRecoveredLiveSessionMetadata
  ├─ 3. reconcile gate resolve + adopt (如适用)
  ├─ 4. 显式 market warmup (refreshLiveMarketSnapshot)  ← 新增
  ├─ 5. syncLiveSessionRuntime
  │   └─ bootstrapSignalRuntimeSourceStates (此时缓存已有)
  ├─ 6. ensureLiveSessionSignalRuntimeStarted
  │   └─ StartSignalRuntimeSession + WS connect
  ├─ 7. awaitLiveSignalRuntimeReadiness
  └─ 8. UpdateLiveSessionStatus("RUNNING")
```
### 具体改动点
#### 1. 在 `StartLiveSession()` 和 `recoverRunningLiveSession()` 中插入显式 warmup
在 `syncLiveSessionRuntime()` 之前，增加：
```go
// 显式预热 market data cache，确保 bootstrap 有数据可用
symbol := NormalizeSymbol(firstNonEmpty(
    stringValue(session.State["symbol"]),
    stringValue(session.State["lastSymbol"]),
))
if symbol != "" {
    if _, warmupErr := p.refreshLiveMarketSnapshot(symbol); warmupErr != nil {
        logger.Warn("market data warmup failed", "symbol", symbol, "error", warmupErr)
        // 非致命：继续执行，bootstrap 会用空数据但不会崩
    }
}
```
> [!NOTE]
> `refreshLiveMarketSnapshot` 已存在于 `live_market_data.go`，它会拉历史 bars 填充缓存。这里只是把调用时机前移。
#### 2. 增强 `bootstrapSignalRuntimeSourceStates()` 的容错
**现状** ([signal_runtime_sessions.go:291-298](file:///Users/fujun/node/bktrader/internal/service/signal_runtime_sessions.go#L291-L298))：snapshot 取不到就 `continue`，导致 sourceStates 为空
**改为**：如果 `liveMarketSnapshot` 返回空，主动调用 `refreshLiveMarketSnapshot` 做一次回填尝试：
```go
snapshot, err := p.liveMarketSnapshot(symbol)
if err != nil || len(snapshot.SignalBars) == 0 {
    // fallback: 尝试主动刷新
    snapshot, err = p.refreshLiveMarketSnapshot(symbol)
    if err != nil {
        continue
    }
}
```
#### 3. 日志区分 bootstrap-pending vs source-gate-blocked
在 `logRuntimeSourceGateState()` ([live.go:340-374](file:///Users/fujun/node/bktrader/internal/service/live.go#L340-L374)) 中，增加一个标记来区分：
- `bootstrap-pending`：启动预热阶段，数据尚未就绪，不是故障
- `source-gate-blocked`：预热完成后，数据源仍不满足要求
通过检查 session state 中是否存在 `startedAt` 来判断。
---
## 与原始计划的差异和补充意见
### 同意的部分
1. ✅ 拆成两个连续 issue，不做"大自愈重构"
2. ✅ gate 状态语义 `stale → conflict → adopted → verified` 的定义
3. ✅ 单一 adopt helper 收口，禁止多入口改库
4. ✅ 禁止 side mismatch 自动 adopt
5. ✅ 禁止 pending settlement 时 adopt
6. ✅ 启动顺序改为固定流水线
7. ✅ 不做项清单（不改 dispatchMode、不放宽未对账限制等）
### 补充建议
#### 1. Adopt 的收口位置建议下沉到 `reconcileLiveAccountPositions`
你的原始计划建议改 `resolveLivePositionReconcileGate` → `ensureLiveExecutionPlan` → `StartLiveSession` 这条链。我建议**核心修改下沉到 `reconcileLiveAccountPositions()`**，因为：
- 这是唯一同时持有双方完整数据的位置
- 上游 `ReconcileLiveAccount` → `refreshLiveAccountPositionReconcileGate` 已经调用它
- 改这一处，所有上游调用者（包括 `triggerAuthoritativeLiveAccountReconcile`）都自动受益
- `attemptLiveAccountReconcileSelfHeal` 需要的唯一改动是扩展 `livePositionReconcileGateCanSelfHeal` 的判定范围
#### 2. 需要增加"open orders 检查"作为 adopt 前置条件
你的原始计划提到了"当前没有本系统未完成 open order / exit order 会影响仓位归因"，但没有指出具体的实现位置。
建议在 `reconcileLiveAccountPositions` 的 adopt 路径上，复用 `liveSettlementPendingOrderSymbols` 的逻辑，扩展为同时检查任何 non-terminal 状态的 order：
```go
// 除了 settlement pending，还要检查是否有 working orders
workingOrderSymbols, err := p.liveWorkingOrderSymbols(account.ID)
```
#### 3. Adopt 后是否自动恢复 RUNNING 需要明确
你的不做项里写了 "不把 session 在 reconcile 成功后自动从 BLOCKED 恢复成 RUNNING"。但在 `recoverRunningLiveSession()` 流程中（docker 重启自动恢复），adopt 如果在 `triggerAuthoritativeLiveAccountReconcile` 内部完成，下游 `resolveLivePositionReconcileGate` 返回的 gate 就不再 blocking，session 会自然走完启动流程到 `RUNNING`。
**建议行为**：
- `recoverRunningLiveSession`：adopt 后继续正常流程，最终到 `RUNNING` ← 无需人工干预
- `StartLiveSession`：同上
- 如果 session 已经处于 `BLOCKED` 状态：需要显式 `stop?force=true` + `start` ← 不自动恢复
这与现有语义一致：`recoverRunningLiveSession` 本来就是为了自动恢复设计的。
#### 4. entryPrice mismatch 的 adopt 需要额外考量
`quantity-mismatch` 的 adopt 逻辑比较直观（交易所是事实源）。但 `entryPrice-mismatch` 更微妙：
- 交易所的 entryPrice 可能因为 ADL、资金费率等原因与 DB 不同
- 但 DB 的 entryPrice 可能是"加权平均入场价"，有策略语义
**建议**：
- `quantity` adopt：直接以交易所为准
- `entryPrice` adopt：以交易所为准，但在 timeline event 中额外记录 `dbEntryPrice`（保留审计线索）
- 如果未来需要保留策略侧的加权入场价，可以在 position metadata 中增加 `strategyEntryPrice` 字段（不在本次 scope 内）
---
## 回归测试矩阵
### Issue A: Reconcile Gate Adopt
| # | 测试场景 | DB Position | Exchange Position | 前置条件 | 期望结果 |
|---|---------|-------------|-------------------|----------|----------|
| A1 | quantity-mismatch adopt 正例 | LONG 0.013 | LONG 0.0065 | authoritative, 无 pending settlement, 无 open orders | DB→0.0065, gate→adopted→verified, start 成功 |
| A2 | entryPrice-mismatch adopt 正例 | LONG 0.0065 @ 60000 | LONG 0.0065 @ 59500 | 同上 | DB entryPrice→59500, gate→verified |
| A3 | side-mismatch 反例 | LONG 0.0065 | SHORT 0.0065 | - | 仍 conflict, DB 不改, session BLOCKED |
| A4 | multi-field 含 side 反例 | LONG 0.013 | SHORT 0.0065 | - | 仍 conflict |
| A5 | pending settlement 护栏 | LONG 0.013 | LONG 0.0065 | 有 pending settlement order | 不 adopt, 保持 stale/conflict |
| A6 | working orders 护栏 | LONG 0.013 | LONG 0.0065 | 有 non-terminal open order | 不 adopt, 保持 conflict |
| A7 | 非 authoritative 护栏 | LONG 0.013 | LONG 0.0065 | snapshot source = platform-live-reconciliation | 不 adopt |
| A8 | 已有 self-heal 路径不回退 | DB 有仓, exchange 无仓 | - | `db-position-exchange-missing` | 现有 self-heal 路径正常工作 |
| A9 | precision/stepSize 边界 | LONG 0.00650001 | LONG 0.0065 | precision helper 判定为相等 | gate→verified（不触发 adopt，直接 match） |
| A10 | docker 重启端到端 | LONG 0.013（旧） | LONG 0.0065 | docker restart | 自动 adopt→verified→session RUNNING |
### Issue B: 启动顺序
| # | 测试场景 | 前置条件 | 期望结果 |
|---|---------|----------|----------|
| B1 | 冷启动正例 | 无 market cache | 先 warm snapshot→bootstrap 有数据→source gate ready→start 成功 |
| B2 | warmup 失败容错 | market data 不可用 | start 不因 warmup 失败崩溃，降级为空 sourceStates |
| B3 | 不先记录 blocked source gate | 冷启动 | 日志中不出现 "runtime source gate blocked" 先于 WS connected |
| B4 | bootstrap-pending 日志区分 | 启动阶段 | 日志标记为 bootstrap-pending 而非 source-gate-blocked |
---
## 日志/可观测性要求
每个关键路径必须携带以下字段：
| 事件名 | 必需字段 |
|--------|----------|
| `reconcile_adopt_attempted` | session_id, account_id, symbol, db_qty, exchange_qty, db_side, exchange_side, mismatch_fields |
| `reconcile_adopt_applied` | 同上 + gate_status_before, gate_status_after, reason |
| `reconcile_adopt_rejected` | 同上 + rejection_reason（side-mismatch / pending-settlement / working-orders / non-authoritative） |
| `start_bootstrap_market_warmup` | session_id, symbol, cache_hit, bars_count |
| `start_bootstrap_runtime_ready` | session_id, source_states_count, missing_count |
| `start_source_gate_ready` | session_id, ready, stale_count |
---
## 不做项
- ❌ 不改默认 `dispatchMode`
- ❌ 不放宽未对账自动交易限制
- ❌ 不把 WS 当最终事实源
- ❌ 不新增 "side mismatch 也自动 adopt"
- ❌ 不靠历史 external terminal orders 自愈当前仓位
- ❌ 不改 `internal/service/execution_strategy.go`
- ❌ 不改 WS 重连策略
- ❌ 不改 dispatch 默认值
- ❌ 不改被动平仓语义
---
## 交付物与执行顺序
### Phase 1: Issue A（Reconcile Gate Adopt）
1. 扩展 `livePositionReconcileGateCanSelfHeal` 判定范围
2. 在 `reconcileLiveAccountPositions` 中增加 safe adopt 路径
3. 新增 `adoptLivePositionFromExchangeTruth` helper
4. 补充 open orders 检查
5. 补 A1-A10 回归测试
6. 确保 `go test ./internal/service/...` 通过
7. 确保 `go build ./cmd/platform-api` 通过
### Phase 2: Issue B（启动顺序）
1. 在 `StartLiveSession` 和 `recoverRunningLiveSession` 中插入显式 market warmup
2. 增强 `bootstrapSignalRuntimeSourceStates` 容错
3. 日志区分 bootstrap-pending vs source-gate-blocked
4. 补 B1-B4 回归测试
5. 全量 `go test ./internal/service/...`

### 当前执行状态（2026-04-24）

#### 已完成

- `livePositionReconcileGateCanSelfHeal` 已扩展到 `quantity-mismatch` / `entry-price-mismatch` / 不含 `side` 的 `multi-field-mismatch`。
- `reconcileLiveAccountPositions` 已新增 safe adopt 路径，收口到 `adoptLivePositionFromExchangeTruth`。
- adopt 前置护栏已覆盖：authoritative、pending settlement、working orders、exchange open orders、side mismatch。
- `StartLiveSession` / `recoverRunningLiveSession` 已在 `syncLiveSessionRuntime` 前增加 market snapshot warmup。
- `bootstrapSignalRuntimeSourceStates` 已支持冷缓存时主动刷新 market snapshot。
- 已补回归测试：
  - quantity mismatch 自动 adopt 到交易所数量。
  - pending settlement 时禁止 adopt。
  - side mismatch 继续保持 conflict 并阻断。
  - 冷缓存 bootstrap 会主动刷新 market data。
- 已通过验证：
  - `go test ./internal/service`
  - `go test ./...`
  - `go build ./cmd/platform-api`
  - `go build ./cmd/db-migrate`

#### 本轮未完全覆盖

- A2 entryPrice mismatch 单独测试尚未补，当前逻辑已支持，但缺独立断言。
- A6 working orders 护栏尚未补独立测试，当前逻辑已支持，但缺独立断言。
- A7 非 authoritative 护栏尚未补独立测试，当前逻辑已支持，但缺独立断言。
- B3 / B4 的日志顺序与 `bootstrap-pending` 细分尚未完整落地；当前只完成 warmup 前移和冷缓存刷新。

#### 建议 PR 拆法

- PR 1：先合并当前 safe adopt + warmup 前移，解决本次线上卡死主问题。
- PR 2：补齐剩余边界测试和日志细分，尤其是 entryPrice、working orders、non-authoritative、bootstrap-pending/source-gate-blocked 的可观测性。

### 一句话摘要（给执行 LLM）
> 只修 authoritative REST 下的 quantity/entryPrice mismatch auto-adopt（收口在 `reconcileLiveAccountPositions`）和 start bootstrap ordering（warmup 前移），不放宽 unresolved recovery gate，不把历史 external orders 当自愈依据，side mismatch 必须继续 block，先补 service-level regression tests 再改逻辑。adopt 核心改动在 `live.go` 的 `reconcileLiveAccountPositions` 和 `livePositionReconcileGateCanSelfHeal` 两处，启动顺序改动在 `StartLiveSession` / `recoverRunningLiveSession` 中增加 `refreshLiveMarketSnapshot` warmup。
