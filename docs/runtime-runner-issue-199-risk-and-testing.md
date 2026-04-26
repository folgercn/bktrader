# Issue #199 风险、边界与测试详细补充

> 本文档是 [runtime-runner-issue-199-workplan.md](runtime-runner-issue-199-workplan.md) 的详细补充。
> 基于当前代码库实际状态（2026-04-26）评估。

---

## 一、现状评估

### 1.1 当前代码库关键事实

| 维度 | 现状 | 影响 |
|------|------|------|
| `signalSessions` | 纯内存 `map[string]domain.SignalRuntimeSession`，无持久化 | 进程重启丢失全部 runtime session，live session 被迫重建 |
| `store.Repository` | 无 `SignalRuntimeSession` CRUD 方法 | Step 1 需扩展接口，memory store 和 postgres store 都要实现 |
| NATS 依赖 | `config.go` 有 `NATS_URL` 字段，`docker-compose.prod.yml` 有 nats 容器，但 **Go 代码零使用** | Step 2 是从零引入，`go.mod` 需新增 `nats.go` 依赖 |
| 进程角色 | `RuntimeOptionsForRole` 支持 `monolith/api/live-runner/notification-worker` | Step 4 需新增 `signal-runtime-runner` 角色 |
| `syncLiveSessionRuntime` | L3377-3388：内存找不到时直接清除 `signalRuntimeSessionId`，重新创建 | Step 1 核心改造点：内存 miss → 先查 store → 复用旧 ID |
| `resolveLiveRuntimeSession` | 遍历内存 map 匹配 accountID+strategyID | Step 1 需同步改造：先查内存，miss 后查 store |
| callback fanout | `runSignalRuntimeWithRecovery` 直接调用 live evaluation | Step 2/3 的边界：side-publish 到 JetStream，不改变现有 callback |

### 1.2 全局依赖链

```
Step 1 (持久化) ← Step 2 (event bus) ← Step 3 (consumer) ← Step 4 (进程拆分) ← Step 5 (lease)
```

严格串行，每个 step 的前置条件是前序 step 已合并。

---

## 二、各 Step 风险分析

### Step 1: #200 持久化 signal runtime session

#### 高风险点

| 风险 | 描述 | 缓解措施 |
|------|------|----------|
| **R1-1 身份键变更** | 当前 ID 格式 `signal-runtime-{unixNano}`，持久化后跨重启复用。若 ID 生成逻辑变化，老 live session 的 `signalRuntimeSessionId` 找不到对应记录 | 保持 ID 格式不变；store 必须按 ID 精确查找，不依赖格式解析 |
| **R1-2 State JSON 膨胀** | `SignalRuntimeSession.State` 包含 `sourceStates`（含 200 根 bar 历史），直接 JSONB 写入可能导致行级锁竞争 | State 列使用 JSONB，但写入频率控制在状态变更时（start/stop/event），不做 ticker 级持久化 |
| **R1-3 双写竞态** | 内存 map 更新和 store 写入之间若有 goroutine 交错读，可能读到不一致状态 | 单一写入路径：先写 store，成功后更新内存 map。失败时不更新内存 |
| **R1-4 migration 幂等** | 生产环境已有数据的 DB 执行 migration 失败 | 使用 `CREATE TABLE IF NOT EXISTS`，列用 `ADD COLUMN IF NOT EXISTS` |
| **R1-5 零值语义** | `Status` 空字符串 vs `"READY"` vs `"STOPPED"` 在 store 查询时行为不同 | 明确定义：空字符串不合法，store 写入前校验 Status 非空 |

#### 边界条件

1. **进程重启恢复**：内存为空 → store 有 `RUNNING` 状态的 session → 必须恢复到内存但 **不自动启动 WS 连接**（实际连接由 `StartSignalRuntimeSession` 控制）
2. **并发创建**：两个 live session 同时调用 `syncLiveSessionRuntime` 对同一 account+strategy → store 层必须有 unique constraint 或 upsert 语义防止重复
3. **Delete 清理**：`DeleteSignalRuntimeSession` 必须同时清理 store 记录和内存 map，且先取消运行中的 goroutine
4. **store 不可用降级**：store 写入失败时，内存 map 仍可工作（降级为当前行为），但必须记录 error 日志

#### 测试矩阵

| 测试场景 | 类型 | 验证点 |
|----------|------|--------|
| memory store CRUD | 单元 | Create/Get/List/Update/Delete 基本语义 |
| postgres store CRUD | 集成 | 同上 + JSONB State 序列化/反序列化 |
| Create 唯一性 | 单元 | 同 account+strategy 不产生重复 session |
| 内存 miss → store 恢复 | 单元 | `syncLiveSessionRuntime` 内存无 → store 有 → 复用旧 ID |
| 内存 miss → store 也无 | 单元 | 走新建路径，与当前行为一致 |
| Stop 持久化 | 单元 | Status 变 `STOPPED`，store 和内存一致 |
| Delete 清理 | 单元 | store 记录删除 + 内存 map 删除 + cancel 调用 |
| store 写入失败 | 单元 | 不 panic，记录日志，内存 map 不更新 |
| State 含 NaN | 单元 | 写入前过滤 NaN/Inf（参考 PR#51 教训） |
| 并发 Get/Update | 单元 | `sync.Mutex` 保护下不 panic |

---

### Step 2: #201 NATS JetStream runtime event bus

#### 高风险点

| 风险 | 描述 | 缓解措施 |
|------|------|----------|
| **R2-1 新依赖引入** | `nats.go` 是项目首个外部消息队列依赖，增加运维复杂度 | NATS 已在 docker-compose.prod.yml；单元测试用 in-memory fake，不依赖 NATS broker |
| **R2-2 publish 阻塞 WS 路径** | 如果 JetStream publish 同步阻塞，WS event 处理延迟会影响 sourceStates 新鲜度 | publish 必须异步或有超时（≤100ms），失败只记日志不阻塞 |
| **R2-3 fingerprint 不稳定** | 如果 fingerprint 包含写入时间或随机数，consumer 无法去重 | fingerprint 只含 `barStart|barEnd|symbol|timeframe|streamType`，不含时间戳 |
| **R2-4 JetStream 连接断开** | NATS 不可用时 publish 全部失败 | 实现 circuit breaker：连续 N 次失败后暂停 publish M 秒，记录 metric |
| **R2-5 event 数据量** | signal bar 事件含完整 bar 数据，高频 tick 场景可能产生大量 event | payload 只放当前 bar 摘要，不放历史；tick 事件做节流（每 symbol 每秒最多 1 条） |

#### 边界条件

1. **双 publish 防护**：同一 signal bar close 事件被 WS 回调触发两次 → fingerprint 相同 → JetStream dedup 或 consumer 端去重
2. **event 顺序**：JetStream 保证同 subject 顺序，但跨 subject 不保证 → consumer 按 `event_time` 排序处理
3. **stream 配置**：`BKT_RUNTIME_EVENTS` retention 策略建议 `WorkQueuePolicy` + 7 天 `MaxAge`
4. **monolith 兼容**：monolith 模式下 publisher 和 consumer 在同进程，仍走 JetStream（不短路），确保行为一致

#### 测试矩阵

| 测试场景 | 类型 | 验证点 |
|----------|------|--------|
| event envelope 字段完整 | 单元 | 所有必填字段非空 |
| fingerprint 稳定性 | 单元 | 同输入多次调用 → 相同 fingerprint |
| fingerprint 不含时间 | 单元 | 不含 `createdAt` / `publishedAt` / 随机数 |
| in-memory fake publish | 单元 | 发布后可读到 |
| in-memory fake 幂等 | 单元 | 重复 fingerprint 不产生重复 event |
| publish 失败不阻塞 | 单元 | fake 注入 error → WS callback 仍正常返回 |
| publish 失败记日志 | 单元 | error 被记录到可观测 channel |
| JetStream stream 配置 | 集成 | stream name / subject / retention 符合预期 |
| 高频 tick 节流 | 单元 | 每 symbol 每秒 ≤1 条 tick event |

---

### Step 3: #202 live-runner 消费 JetStream event

#### 高风险点

| 风险 | 描述 | 缓解措施 |
|------|------|----------|
| **R3-1 重复触发 dispatch** | 同一 event 被消费两次（at-least-once）→ 重复生成 proposal → 重复下单 | 必须用 `decisionEventFingerprint` + `lastStrategyDecisionEventIntentSignature` 做幂等检查 |
| **R3-2 先 ack 后执行** | 如果先 ack 再执行 live evaluation，执行失败时 event 已被消费，无法重试 | **必须先执行成功再 ack**。失败 → 不 ack → JetStream 重投 |
| **R3-3 stale event 堆积** | consumer 长时间宕机后重启，JetStream 重放大量历史 event → 触发过时的 live evaluation | consumer 恢复后按 `event_time` 过滤，丢弃超过 `signalBarFreshnessSeconds` 的 stale event |
| **R3-4 callback 与 consumer 双路径** | 过渡期同一 event 可能同时走旧 callback 和 JetStream consumer → 双重触发 | 幂等防护是唯一保障；不能依赖"只走一条路"的假设 |
| **R3-5 recovery gate 绕过** | consumer 处理 event 时跳过 `isLiveSessionRecoveryCloseOnlyMode` 检查 | consumer 必须复用现有 `evaluateLiveSignalDecision` 路径，不允许绕过任何 gate |

#### 边界条件

1. **consumer group 隔离**：durable consumer `live-evaluation` 只被 `live-runner` 进程消费，`signal-runtime-runner` 不消费
2. **max deliver**：建议 `MaxDeliver=5`，超过后进入 dead-letter 或记录告警
3. **ack wait**：建议 30s，覆盖最慢的 live evaluation 路径
4. **consumer 启停**：`live-runner` 进程启动时创建 consumer，优雅关闭时 drain 后停止

#### 测试矩阵

| 测试场景 | 类型 | 验证点 |
|----------|------|--------|
| happy path 消费触发 evaluation | 单元 | event → evaluateLiveSignalDecision 被调用 |
| 重复 event 不重复 proposal | 单元 | 同 fingerprint event ×2 → 只生成 1 个 proposal |
| 消费失败不 ack | 单元 | evaluation 返回 error → ack 未调用 |
| stale event 丢弃 | 单元 | event_time 超过 freshnessSeconds → 跳过但 ack |
| recovery gate 阻断 | 单元 | session 在 close-only-takeover → evaluation 不执行 |
| reconcile gate 阻断 | 单元 | reconcile gate blocked → evaluation 不执行 |
| readiness gate 阻断 | 单元 | sourceStates stale → evaluation 阻断 |
| dispatchMode=manual-review | 单元 | 生成 proposal 但不自动 dispatch |

---

### Step 4: #203 独立 signal-runtime-runner 进程

#### 高风险点

| 风险 | 描述 | 缓解措施 |
|------|------|----------|
| **R4-1 WS 连接归属混乱** | 拆分后 `live-runner` 和 `signal-runtime-runner` 都可能尝试建立 WS 连接 | `RuntimeOptionsForRole("signal-runtime-runner")` 只启 WS；`live-runner` 不再启 WS |
| **R4-2 desired vs actual 状态混淆** | API `start` 写入 `desired_status=RUNNING`，但 WS 实际未连接 → UI 显示"运行中" | UI 必须展示 `desired_status` 和 `actual_status` 两个字段 |
| **R4-3 scanner 抢占** | 多个 `signal-runtime-runner` 实例扫描同一 session → 重复启动 WS | Step 5 的 lease 解决此问题，Step 4 暂限制单实例部署 |
| **R4-4 CD 配置遗漏** | docker-compose 新增服务但遗漏环境变量 | compose 配置必须在 PR 中包含，并有 review checklist |
| **R4-5 monolith 回退** | 拆分后发现问题需要回退到 monolith → 需保证 monolith 模式仍可运行 | `monolith` role 启动所有组件（含 WS + live sync + recovery） |

#### 边界条件

1. **角色验证扩展**：`config.Validate()` 的 switch 必须新增 `signal-runtime-runner`
2. **RuntimeActionsEnabled**：`signal-runtime-runner` 返回 `false`（不允许 dispatch/start live session）
3. **scanner 语义**：scanner 定期查 store 中 `desired_status=RUNNING` 的 session → 启动 WS → 更新 `actual_status`
4. **API 行为变更**：`StartSignalRuntimeSession` API 改为写 `desired_status`，返回时声明"请求已接受"而非"已启动"

#### 测试矩阵

| 测试场景 | 类型 | 验证点 |
|----------|------|--------|
| role option mapping | 单元 | `signal-runtime-runner` → WS=true, LiveSync=false, Recovery=false |
| live-runner 不启动 WS | 单元 | `live-runner` role → WS 相关 option=false |
| config validation | 单元 | `signal-runtime-runner` 通过验证 |
| RuntimeActionsEnabled | 单元 | `signal-runtime-runner` → false |
| monolith 全功能 | 单元 | `monolith` role → 所有 option=true |
| compose 服务定义 | 人工 | BKTRADER_ROLE 正确、环境变量完整 |
| desired vs actual | 单元 | 写 desired=RUNNING → actual 不立即变化 |

---

### Step 5: #204 runner lease / ownership

#### 高风险点

| 风险 | 描述 | 缓解措施 |
|------|------|----------|
| **R5-1 lease 当交易事实** | 开发者误把"持有 lease"等同于"可以交易" | 代码 review 强制检查：lease 只控制"谁处理"，不控制"能否执行" |
| **R5-2 heartbeat 停止但进程未退出** | 进程 hang 但未崩溃 → lease 过期 → 另一实例接管 → 原实例恢复 → 双写 | heartbeat goroutine 与业务 goroutine 绑定同一 context；heartbeat 失败 → cancel 业务 |
| **R5-3 时钟偏差** | 不同服务器时钟不同步 → lease 过期判定不一致 | 使用 DB 服务器时间 `NOW()` 作为基准，不依赖进程本地时间 |
| **R5-4 lease 表热点** | 所有 runner 高频 heartbeat 同一表 → 行级锁竞争 | heartbeat 间隔 ≥5s；使用 `UPDATE ... WHERE id = ? AND owner_id = ?` 避免锁升级 |
| **R5-5 orphan lease** | 进程崩溃未 release → lease 直到过期才释放 → 期间 resource 无人处理 | TTL 设 30s，heartbeat 间隔 10s，最多 30s 延迟 |

#### 边界条件

1. **acquire 竞争**：两个 runner 同时 acquire → 只有一个成功（DB `INSERT ... ON CONFLICT DO NOTHING` + 检查 `owner_id`）
2. **expired takeover**：lease `updated_at + ttl < NOW()` → 允许新 owner acquire
3. **release 校验**：只有 `owner_id` 匹配才能 release，防止误释放
4. **lease 失败处理**：acquire 失败 → 跳过该 resource，不 panic，不影响其他 resource 处理
5. **graceful shutdown**：进程收到 SIGTERM → release 所有持有的 lease → 退出

#### 测试矩阵

| 测试场景 | 类型 | 验证点 |
|----------|------|--------|
| acquire 成功 | 单元 | 无竞争时 acquire 返回 true |
| 重复 acquire 被拒 | 单元 | 已有活跃 lease → 第二个 owner acquire 返回 false |
| owner 自身 acquire 幂等 | 单元 | 同 owner 重复 acquire → 成功（续约语义） |
| expired takeover | 单元 | TTL 过期 → 新 owner acquire 成功 |
| release by owner | 单元 | owner release 成功 |
| release by non-owner | 单元 | 非 owner release 失败 |
| heartbeat 续约 | 单元 | heartbeat 更新 `updated_at` |
| heartbeat 失败 | 单元 | owner 不匹配 → heartbeat 失败 → 触发业务 cancel |
| 未获 lease 跳过 | 单元 | acquire 失败 → 业务逻辑不执行，不 panic |
| 并发 acquire 竞争 | 集成 | 10 goroutine 同时 acquire → 只有 1 个成功 |

---

## 三、跨 Step 系统性风险

| 风险 | 涉及 Step | 描述 | 缓解 |
|------|-----------|------|------|
| **全链路回滚** | 全部 | Step 3 合并后发现 Step 1 有 bug → 需要同时回滚 3 个 PR | 每个 step 保证 monolith 可运行；Step 2/3 有 feature flag 可关闭 JetStream 路径 |
| **NATS 单点故障** | 2-5 | NATS 不可用 → runtime event 无法传递 → live evaluation 停止 | 保留旧 callback 路径作为 fallback，NATS 不可用时自动降级 |
| **DB migration 顺序** | 1, 5 | Step 1 和 Step 5 各有 migration，顺序错误会失败 | migration 文件名严格递增，PR review 检查 |
| **内存使用增长** | 1 | 持久化后内存仍保留 map → 双份数据 | 内存 map 只做 hot cache，定期同步 store，不持久化 bar 历史 |
| **observability 缺失** | 2-5 | NATS/lease 问题在日志中不可见 | 每个 step 必须增加 structured log + health endpoint 暴露状态 |

---

## 四、安全边界强制约束（跨全部 Step）

以下约束在 **每个 Step 的 code review** 中必须逐条检查：

1. ❌ 任何 Step 不允许改变 `dispatchMode` 默认值 `manual-review`
2. ❌ 任何 Step 不允许让 JetStream event 直接触发 `dispatchLiveSessionIntent` 而不经过完整 gate 校验
3. ❌ 任何 Step 不允许在 `signalRuntimeSessionId` 变更时不更新 live session state
4. ❌ 任何 Step 不允许把 `lease.owner_id` 作为 reconcile/recovery gate 的判断依据
5. ❌ 任何 Step 不允许在 consumer 中先 ack 后执行副作用
6. ✅ 每个 Step 的 PR 必须包含至少 1 个 failure path 测试
7. ✅ 每个涉及 `live*.go` 修改的 PR 必须先读 `pr-lessons-learned.md`
8. ✅ 每个涉及 recovery 路径的 PR 必须先读 `runtime-recovery-extension-coding-rules.md`
