# 2026-04-23 Live Trade Pairs 落地方案

## 1. 结论先行

`Trade Pairs` 以后按下面三层职责重构：

1. **订单/成交主链路**
   - `orders + fills + positions`
   - 负责生成 trade pair 本体
   - 必须始终可用，不能被增强信息拖死

2. **Decision Event 增强层**
   - 通过 `decisionEventId` 点查 `strategy_decision_events`
   - 负责补：
     - `entryReason`
     - `exitReason`
     - `exitClassifier`
   - 允许缺失；缺失时主结果照常返回

3. **平仓核验增强层**
   - 不再直接依赖 `position_account_snapshots`
   - 为以后新数据单独建设"按 `orderId` 可点查"的平仓核验数据结构
   - 负责补：
     - `exitVerdict`
     - `notes` 中的 post-exit verification 信息
   - 历史数据不回填；旧数据没有核验记录时只返回基础 pair

一句话：**订单负责追溯，decision event 负责标签，单独的新结构负责平仓核验。**

---

## 2. 背景与根因

当前 `Trade Pairs` 的主聚合逻辑在：

- `internal/service/live_trade_pairs.go`

生产环境已确认：

- 某个 live session 的 `strategy_decision_events` 超过 `147 万` 行
- 同 session 的 `position_account_snapshots` 超过 `138 万` 行
- 旧实现按 `liveSessionID` 全量读取这两张表，导致接口超时

当前问题不是"前端获取失败"，而是后端主查询路径设计过重：

- 本应只靠 `orders + fills` 计算的追溯结果
- 被海量遥测表绑进了主链路

这违背了 `Trade Pairs` 的产品本意。

### 2.1 主链路自身的全表扫描问题

除了遥测大表，主查询自身也存在全表扫描：

- `p.store.ListOrders()` -- 无条件读取全部 orders（Postgres: `SELECT ... FROM orders ORDER BY created_at ASC`）
- `p.store.ListFills()` -- 无条件读取全部 fills
- `p.store.ListPositions()` -- 无条件读取全部 positions

读取后再在内存中按 `metadata->>'liveSessionId'` 过滤。
随着系统运行时间增长，这三张表也会成为瓶颈。
本次重构必须同步解决此问题。

---

## 3. 产品边界

### 3.1 必须保证的能力

`Trade Pairs` 必须稳定返回以下字段，即使增强数据不可用：

- `id`
- `liveSessionId`
- `accountId`
- `strategyId`
- `symbol`
- `status`
- `side`
- `entryOrderIds`
- `exitOrderIds`
- `entryAt`
- `exitAt`
- `entryAvgPrice`
- `exitAvgPrice`
- `entryQuantity`
- `exitQuantity`
- `openQuantity`
- `realizedPnl`
- `unrealizedPnl`
- `fees`
- `netPnl`
- `entryFillCount`
- `exitFillCount`

### 3.2 可选增强字段

增强字段允许缺失，不影响主结果返回：

- `entryReason`
- `exitReason`
- `exitClassifier`
- `exitVerdict`
- `notes`

### 3.3 当前阶段不处理的事项

- 不做历史数据回填
- 不要求旧数据一定具备 `exitVerdict` / `notes`
- 不把"平仓核验"强行塞回现有 `position_account_snapshots` 主查询

---

## 4. 目标状态

### 4.1 查询分层

后续代码必须严格遵守：

1. **第一层：主 pair 查询**
   - 仅使用：
     - `orders`（必须按 `liveSessionID` 过滤查询，不能全表读取）
     - `fills`（必须按已筛选的 `orderID` 关联查询，不能全表读取）
     - 必要时 `positions`（按 `accountID` 过滤）
   - 输出最近 `limit` 个 pair

2. **第二层：Decision Event 标签增强**
   - 只对第一层返回的少量 pair 做增强
   - 使用这些 pair 相关订单上的 `decisionEventId`
   - 通过主键点查 `strategy_decision_events`

3. **第三层：平仓核验增强**
   - 只对第一层返回的少量 exit order 做增强
   - 通过 `orderId` 点查新的"平仓核验结构"

### 4.2 主链路查询收敛要求

当前 `ListOrders()` / `ListFills()` / `ListPositions()` 是全表扫描。
第 1 PR 必须将其收敛：

1. 新增或使用按 `liveSessionID` 过滤的 orders 查询方法
   - 通过 `metadata->>'liveSessionId'` 索引过滤，或新增专用查询方法
2. fills 查询改为 `WHERE order_id IN (...)` -- 只查与当前 session 相关的 orders 的 fills
3. positions 查询改为 `WHERE account_id = $1` -- 只查当前 session 的 account

### 4.3 多 Symbol 追溯假设

当前 pair 追溯逻辑使用单一 `pnlState` 追踪净持仓。
**本系统假设一个 live session 只交易一个 symbol**。

第 1 PR 必须加防御性断言：
- 如果同一 session 的订单出现多个不同 symbol，记录 warning 日志
- pair 追溯按 symbol 分组，确保即使出现多 symbol 也不会产生错误的 pair

### 4.4 必须避免的实现方式

以下模式后续一律视为错误实现：

- 按 `liveSessionID` 全量读取 `strategy_decision_events`
- 按 `liveSessionID` 全量读取 `position_account_snapshots`
- 无条件全表读取 `orders` / `fills` / `positions`
- 为了补标签而让 `Trade Pairs` 主结果超时
- 让增强字段成为接口成败的前提条件

---

## 5. 新数据结构设计

## 5.1 推荐方案

新增一张专门的订单关闭核验表，例如：

`order_close_verifications`

建议字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | text | 主键 |
| `live_session_id` | text | 对应 live session |
| `order_id` | text | 对应 exit order |
| `decision_event_id` | text null | 可选，对应决策事件 |
| `account_id` | text | 账户 |
| `strategy_id` | text | 策略 |
| `symbol` | text | 交易对 |
| `verified_closed` | boolean | 是否确认已平干净 |
| `remaining_position_qty` | double precision | 核验时剩余仓位数量 |
| `verification_source` | text | `reconcile` / `rest-sync` / `manual-review` 等 |
| `event_time` | timestamptz | 事实发生时间 |
| `recorded_at` | timestamptz | 记录写入时间 |
| `metadata` | jsonb | 扩展信息 |

### 5.2 建议索引

至少包含：

- `primary key (id)`
- `index on (order_id, event_time desc)`
- `index on (live_session_id, event_time desc)`
- 如未来按 session + order 联查较多，可加：
  - `index on (live_session_id, order_id, event_time desc)`

### 5.3 零值语义精确定义

`remaining_position_qty` 的零值必须显式区分：

| 状态 | `verified_closed` | `remaining_position_qty` | 含义 |
|------|-------------------|--------------------------|------|
| 正常关闭 | `true` | `<= 1e-9` | 已确认平仓干净 |
| 仓位不一致 | `false` | `> 1e-9` | 平仓后仍有残留 |
| 逻辑矛盾 | `false` | `<= 1e-9` | 不应出现，必须记录 warning |

- 浮点精度 epsilon 统一为 `1e-9`，与现有代码保持一致
- "无核验记录" 不等于 "remaining_position_qty = 0"，查询时必须通过记录是否存在来区分

### 5.4 Store 接口层变更

新增 `order_close_verifications` 表意味着以下接口层必须同步更新：

1. **Store interface** -- 在 store 接口中新增：
   - `CreateOrderCloseVerification(item domain.OrderCloseVerification) (domain.OrderCloseVerification, error)`
   - `QueryOrderCloseVerifications(query domain.OrderCloseVerificationQuery) ([]domain.OrderCloseVerification, error)`

2. **双后端实现** -- `memory.Store` 和 `postgres.Store` 必须同时实现

3. **domain 类型** -- 新增 `domain.OrderCloseVerification` 和 `domain.OrderCloseVerificationQuery` 结构体

4. **db-migrate** -- 新增 migration 文件

### 5.5 为什么不继续复用 `position_account_snapshots`

原因很明确：

- 它是快照/遥测表，不是订单级核验事实表
- 生产库没有 `order_id` 索引
- 同一个 `order_id` 会对应大量快照，语义并不收敛
- 继续复用会让查询设计与建模定位长期错位

---

## 6. 写入时机设计

## 6.1 只服务以后新数据

本方案**只要求以后新增的 exit order** 写入关闭核验数据。

历史数据处理规则：

- 没有 `order_close_verifications` 记录时
- `Trade Pairs` 仍返回基础结果
- `exitVerdict` / `notes` 允许为空或退化为默认值

## 6.2 建议写入节点

建议只在关键节点写，不做高频刷写：

1. **exit order 成交后首次核验**
   - 目的：记录首次关闭确认结果

2. **reconcile / authoritative REST 校验后**
   - 目的：记录最终 authoritative verdict

3. **人工接管/人工确认后**
   - 目的：为 `manual-review` / `mismatch resolved` 提供明确结论

## 6.3 幂等规则

必须避免同一笔 exit order 被无意义高频追加记录。

建议二选一：

1. **Upsert 最新状态**
   - 每个 `order_id + verification_source` 保留最新一条

2. **Append only，但主查询只取最新事件**
   - 允许保留历史变化
   - 查询时必须按 `order_id` 取最新一条或最新 authoritative 一条

如果没有明确审计需求，推荐先用 **Append only + 查询取最新**，实现更简单、语义更稳。

---

## 7. 查询实现方案

## 7.1 Trade Pairs 主查询

后续 LLM 实现时，必须按以下顺序组织：

1. 读取当前 session 的相关订单
2. 基于订单关联 fills
3. 仅用订单/成交生成 pair
4. 在 pair 数量排序后先应用 `limit`
5. 只对结果集做增强查询

关键点：

- **先 pair，后增强**
- **先 limit，后增强**

## 7.2 Decision Event 增强查询

增强过程：

1. 从结果集的相关订单提取 `decisionEventId`
2. 去重
3. 用主键点查 `strategy_decision_events`
4. 补：
   - `entryReason`
   - `exitReason`
   - `exitClassifier`

查询规则：

- 不允许按 `liveSessionID` 全量查
- 必须按 `decisionEventId` 逐个点查或批量 `IN (...)`
- 增强失败不得阻塞主结果返回

## 7.3 平仓核验增强查询

增强过程：

1. 从结果集提取 exit order IDs
2. 去重
3. 按 `orderId` 查询 `order_close_verifications`
4. 每个 order 只取最新有效核验记录
5. 补：
   - `exitVerdict`
   - `notes`

建议映射规则：

- `verified_closed = true` 且 `remaining_position_qty <= epsilon`
  - `exitVerdict = normal`
- `verified_closed = false` 且 `remaining_position_qty > epsilon`
  - `exitVerdict = mismatch`
  - `notes += post-exit-position-still-open`
- 若 `verification_source = manual-review`
  - 可在 `notes` 标注来源

## 7.4 无核验记录时的默认行为

如果新表查不到该 exit order 的核验记录：

- 主结果照常返回
- 不要报错
- 不要卡接口
- `exitVerdict` 可保留现有默认值或置空策略，但必须稳定一致

推荐：

- `closed` pair 默认 `exitVerdict = normal`
- 只有拿到明确负向核验时才升级为 `mismatch`

---

## 8. 接口字段策略

## 8.1 保持现有 API 兼容

当前前端已依赖这些字段：

- `entryReason`
- `exitReason`
- `exitClassifier`
- `exitVerdict`
- `notes`

因此第一阶段不建议删字段，而是：

- 保持字段存在
- 允许增强值缺失
- 把字段语义从"总能给出"改成"尽力补充"

## 8.2 前端兼容要求

前端必须接受：

- `entryReason == ""`
- `exitReason == ""`
- `exitClassifier == ""`
- `notes == nil`

前端不应把这些字段当成必有值。

---

## 9. 迁移方案

## 9.1 第一阶段

最小可上线版本：

1. `Trade Pairs` 主查询退回订单/成交主链路
2. **主链路查询收敛** -- 将 `ListOrders()` / `ListFills()` / `ListPositions()` 全表扫描改为按 session 过滤的查询
3. Decision Event 增强保留
4. Snapshot 增强移除
5. 文档与测试更新
6. API handler 增加响应时间日志（elapsed-time log），方便上线后验证效果

### 9.1.1 Snapshot 增强移除清单

第一阶段必须清理以下代码：

| 文件 | 内容 | 操作 |
|------|------|------|
| `live_trade_pairs.go` L645 | `enrichLiveTradePairs` 中的 `populateTradePairSnapshotTelemetry` 调用 | 删除 |
| `live_trade_pairs.go` L673-680 | `enrichLiveTradePairs` 中的 snapshot verdict 覆写逻辑 | 删除 |
| `live_trade_pairs.go` L708-729 | `populateTradePairSnapshotTelemetry` 函数 | 删除 |
| `live_trade_pairs.go` L291-304 | `latestPositionSnapshotByOrderID` 函数 | 删除（当前未被调用，属于死代码） |
| `live_trade_pairs.go` L383 | `finalizeClosed` 的 `snapshotByOrderID` 参数 | 简化为无参或移除 snapshot 逻辑，保留 `hasUnsafeExitOrder` / `recoveryExit` 等本地判定 |

`finalizeClosed` 中 L392-397 的 snapshot 核验逻辑在第一阶段删除。
该功能将在第三阶段通过 `order_close_verifications` 重新实现。

这一阶段不需要历史回填。

## 9.2 第二阶段

新增数据库迁移：

1. 建 `order_close_verifications` 表
2. 建索引
3. 新增 `domain.OrderCloseVerification` + `domain.OrderCloseVerificationQuery` 类型
4. 在 store interface 增加 `CreateOrderCloseVerification` / `QueryOrderCloseVerifications`
5. 在 `memory.Store` 和 `postgres.Store` 同时实现
6. 在 `db-migrate` 工具中增加 migration 文件

## 9.3 第三阶段

在写入链路接入：

1. exit order 成交后的首次核验写入
2. reconcile 后写入 authoritative 结果
3. `Trade Pairs` 查询接入新表
4. 写入代码必须封装为独立 helper，不能散落在 `live.go` 主循环中

---

## 10. 测试与验收

## 10.1 必须覆盖的后端测试

至少新增/保留以下用例：

1. **基础 pair 计算**
   - 无增强数据也能返回 pair

2. **Decision Event 增强**
   - 有 `decisionEventId` 时能补：
     - `entryReason`
     - `exitReason`
     - `exitClassifier`

3. **增强缺失容错**
   - event 查不到时不影响主结果

4. **关闭核验增强**
   - `verified_closed = true` -> `normal`
   - `remaining_position_qty > 0` -> `mismatch`

5. **无核验记录**
   - 不报错
   - 返回基础 pair

6. **只对结果集做增强**
   - 不能再出现按 `liveSessionID` 全量扫大表的实现

7. **failure path**
   - 新表缺失 / 查询失败时接口仍返回主结果

## 10.2 性能验收标准

针对生产同量级数据，目标是：

- `Trade Pairs` 主接口响应不再受上百万 decision/snapshot 记录影响
- 主链路查询不再全表扫描 `orders` / `fills` / `positions`
- Decision Event 增强查询应保持毫秒级到几十毫秒级
- 新的关闭核验查询必须能稳定走 `order_id` 索引
- API handler 必须输出 elapsed-time 日志，上线后可直接在生产日志中验证效果

## 10.3 验证命令

代码改完后至少执行：

```bash
gofmt -w .
go test ./...
go build ./cmd/platform-api
go build ./cmd/db-migrate
```

如涉及前端展示微调，必须使用全量构建验证（不要只检查单个文件）：

```bash
cd web/console
npm run build
```

trade pairs 相关的前端组件分布在多个文件中，单文件 tsc 检查无法覆盖：

- `src/components/live/LiveTradePairsCard.tsx`
- `src/hooks/useLiveTradePairs.ts`
- `src/components/layout/DockContent.tsx`
- `src/pages/MonitorStage.tsx`
- `src/pages/AccountStage.tsx`

---

## 11. 给后续 LLM 的执行顺序

后续 LLM 必须按以下顺序推进，禁止一步大包揽：

1. **第 1 PR**
   - 收敛 `Trade Pairs` 主查询（包括将 `ListOrders` / `ListFills` / `ListPositions` 改为按 session 过滤）
   - 按 9.1.1 清单移除 Snapshot 增强及相关死代码
   - 加多 Symbol 防御性断言
   - 保留 Decision Event 增强
   - 增加 API 响应时间日志
   - 补测试

2. **第 2 PR**
   - 新增 `order_close_verifications` migration
   - 新增 domain 类型
   - 实现 store interface + memory.Store + postgres.Store
   - 补 migration/索引测试

3. **第 3 PR**（高风险 -- 涉及 `live*.go` 修改）
   - 在 exit / reconcile 路径写入关闭核验记录
   - 接入 `Trade Pairs` 增强读取
   - 补端到端回归测试
   - **写入代码必须封装为独立 helper，不能散落在 live 主循环中**
   - **必须标记为 L2/L3 级别 PR**
   - **AI review 只做辅助，必须等待人工主审通过**
   - 遵守 `AGENTS.md` 第 10 节的运行时恢复/接管专项规则

### 11.1 明确禁止

- 不允许在一个 PR 里同时重写主查询、改 live 执行链、再做前端改版
- 不允许为了通过测试继续按 `liveSessionID` 全量扫遥测大表
- 不允许把旧 snapshot 表继续当作订单级核验事实表
- 不允许 `ListOrders()` / `ListFills()` 继续保持全表扫描

---

## 12. 风险与注意事项

1. **高风险边界**
   - 如果写入核验记录时涉及 `internal/service/live*.go`
   - 必须遵守 `AGENTS.md` 与 `docs/pr-lessons-learned.md` 的高风险规则
   - 第 3 PR 必须标记为 L2/L3 级别，等待人工主审

2. **语义一致性**
   - `exitVerdict` 的定义必须收敛，不允许多处平行维护

3. **零值语义**
   - `remaining_position_qty = 0` 与"无核验记录"不是一回事
   - 查询与序列化时必须显式区分
   - 浮点精度 epsilon 统一为 `1e-9`（见 5.3 节）

4. **性能约束**
   - 任何后续增强查询都必须建立在可索引点查之上
   - 主链路查询（orders / fills / positions）不允许全表扫描

5. **回滚策略**
   - 第 1 PR 上线后如果 Decision Event 增强本身也出现性能问题
   - 需要有快速降级能力：增加配置项或特性开关，允许完全跳过增强层
   - 降级时接口只返回基础 pair，所有增强字段为空

6. **前端轮询机制**
   - 当前 `useLiveTradePairs.ts` 只在 `sessionId` / `limit` 变化时请求一次
   - 对于 `open` 状态 pair 的 `unrealizedPnl`，没有定时刷新
   - 本次重构不强制要求加轮询，但后续如需实时性需补充 polling interval
   - 前端必须能容忍所有增强字段为空（参见 8.2 节）

---

## 13. 最终落地结果

本方案完成后，系统行为应为：

- `Trade Pairs` 在任何情况下都能先返回基础追溯结果
- 主链路查询只读取当前 session 相关的 orders / fills / positions，不做全表扫描
- `reason/classifier` 通过 `decisionEventId` 低成本补充
- `verdict/notes` 通过新的订单级关闭核验结构补充
- 增强层可通过配置降级，不影响主结果返回
- 历史数据不回填
- 以后新数据逐步具备完整增强能力

这就是后续代码工作的目标态。
