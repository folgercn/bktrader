# bktrader PR 实战经验库

> **来源**：从项目 155 个 PR（#1 ~ #155）的 review 记录、comments、diff 中提炼。
> **维护者**：folgercn
> **最后更新**：2026-04-22
> **定位**：本文档是 [harness-engineering-部署方案.md](harness-engineering-部署方案.md) 的实战补充，记录从真实 PR review 中沉淀下来的踩坑模式、修复策略和 review 纪律。

---

## 1. 状态一致性陷阱

### 1.1 状态双写 / 重复计数

**典型案例**：PR#39（Add backend health snapshots and sync monitoring）

`SyncLiveAccount(...)` 扩展后，adapter 成功路径做了一次 `persistLiveAccountSyncSuccess(...)`，但 `syncLiveAccountFromBinance(...)` 内部又各自更新了 success health，导致：

- `syncCount` 被双计数
- `lastSuccessAt` 被二次覆盖
- `source` / `adapterKey` / `executionMode` 来源不一致

**修复模式**：

- 统一走 `persistLiveAccountSyncSuccess(...)` / `persistLiveAccountSyncFailure(...)` 两个收口点
- adapter 如果自行持久化 success，通过 `PersistsLiveAccountSyncSuccess() == true` 标记能力边界，平台层不再双写
- 所有失败出口（adapter resolve 失败、adapter sync 失败、local fallback 失败、store update 失败）都必须进入统一 failure accounting

**规则提炼**：
> 每条状态变更链路只允许有一个写入出口。如果有 N 个路径都可能写同一个字段，必须收敛到统一 helper，或通过 capability flag 显式声明谁负责写。

---

### 1.2 监控与告警语义不一致

**典型案例**：PR#39、PR#146（suppress transient runtime recovery alerts）

alerts 和 `/api/v1/monitor/health` 对 `quiet` / `stale` 使用了各自独立的判定条件，导致：

- alerts 里没问题，monitor/health 却在亮黄灯
- 或者反过来：告警响了，但 health 面板显示绿色

**修复模式**：

- 引入共享入口 `liveSessionEvaluationQuiet(mode, status, state)`
- `ListAlerts()` 和 `HealthSnapshot()` 都调用同一个入口
- 共享语义：只对 `LIVE + RUNNING` 生效；stopped / inactive session 不报 quiet；threshold `<= 0` 视为关闭

**规则提炼**：
> 同一个判定在系统中只能有一个入口。如果两个 API 需要表达同一个概念（如 "quiet"），必须走同一个函数，禁止各自维护平行条件。

---

### 1.3 配置开关全链路不一致

**典型案例**：PR#39

新增 `strategyEvaluationQuietSeconds` 和 `liveAccountSyncFreshnessSeconds` 两个阈值，支持 `<= 0` 表示关闭检测。但 `SetRuntimePolicy(...)` 中的 `> 0` 判断会忽略显式传入的 `0`，导致：

- API 看起来"配置成功了"
- 实际行为没有变

**修复模式**：

- HTTP patch payload、`UpdateRuntimePolicy(...)`、`SetRuntimePolicy(...)`、持久化、读取、`HealthSnapshot()` 返回、判断函数 — 全链路统一支持 `0 = disable`
- 补回归测试：设为 `0` → 重建 Platform → `LoadPersistedRuntimePolicy()` → 验证检测函数不再触发

**规则提炼**：
> 运维类阈值 / 开关必须全链路一致：API → 内存态 → 持久态 → 判断函数。如果支持 `0 = 关闭`，就必须在每一层都显式支持。

---

## 2. 零值与默认值语义

### 2.1 Go 零值 fallback 覆盖显式值

**典型案例**：PR#22（execution normalization, SL protection, watermark boundaries）

`adapterSubmission` fallback 逻辑中，`false` 和 `0` 被当成"未提供"而被旧值覆盖：

- 显式传入的 `reduceOnly = false` 被覆盖为 `true`
- 显式传入的 `normalizedPrice = 0.0` 被覆盖为旧的非零价格

**修复模式**：

- 引入 `executionSubmissionValuePresent(path, value)` 判断：
  - `bool`：任何值都算"已提供"
  - `float64`：`0` 对已知零值字段（如 `rawQuantity`、`normalizedQuantity`）不算"已提供"，对未知字段算"已提供"
- 通过 path-based 规则区分"这个零是有意义的"还是"这个零是默认值"

**规则提炼**：
> Go 的 `false` / `0` 与"未提供"无法区分。任何涉及 fallback / merge 的逻辑，必须对每类字段显式定义"零值是否有意义"。不能用简单的 `if value != 0` 来判断"是否已提供"。

---

### 2.2 SL protection 插值方向错误

**典型案例**：PR#22

`resolveAggressiveSLProtectionDecision()` 的 BUY-side 分支中，`coverage-weighted-cap` 的插值方向搞反了——partial top-of-book coverage 应该从 `spreadCapped` 向 `bestAsk` 单调移动，实际上反向了，导致滑点封顶失效。

**修复模式**：

- SELL 和 BUY 两个分支必须对称实现
- 加了 BUY-side 对称回归测试

**规则提炼**：
> 任何涉及 BUY / SELL 双向逻辑的函数，必须同时补双向测试。单向通过不等于双向都对。

---

## 3. 身份与生命周期管理

### 3.1 Watermark 身份泄漏

**典型案例**：PR#22、PR#26

`watermarkPositionKey` 只由 `side | entryPrice` 组成，无法区分"同方向、同入场价"的新旧仓位。实盘中平仓后在同价再次开仓时，会复用上一笔仓位遗留的 `hwm / lwm`，导致 trailing stop / protection 计算错误。

**修复模式**：

- watermark key 扩展为 `positionID | symbol | side | entryPrice`
- 活跃仓位优先使用稳定的 canonical key
- 非活跃仓位清理持久化 watermark 状态
- legacy key 仅用于兼容读取，之后升级到 canonical key

**规则提炼**：
> 任何按 identity 缓存的状态（watermark、HWM、LWM），其 key 必须能唯一标识"当前这笔仓位"。如果同一个 key 可能在平仓 → 再开仓后被复用，就会泄漏上一笔仓位的状态。

---

### 3.2 Legacy 数据隐式语义迁移

**典型案例**：PR#43（fix signal runtime bindings）

`signalBindingTimeframe(...)` 对 `binance-kline` 的空 timeframe 默认回填 `1d`。这看起来只是"格式化"，实际上改变了 binding 的身份键，导致：

- 老数据原来是"未定义 timeframe"，现在被无声解释成 `1d`
- match key、list 排序、binding 替换、runtime subscription channel 都受影响

**修复模式**：

- 补兼容性回归测试：老 binding 无 timeframe + 新 binding 显式 1d → 验证是否视为同一绑定
- 在代码注释中明确说明这是迁移语义

**规则提炼**：
> 任何给"空值"回填默认值的逻辑，如果这个值参与身份识别（match key、缓存 key、routing key），就不是"格式化"而是"语义迁移"。必须补兼容性测试。

---

## 4. 执行安全边界

### 4.1 自动 resume 过于激进

**典型案例**：PR#109（Fix live immediate fill settlement and stop defaults）

`resumeFlatBlockedLiveSession()` 的逻辑是：
- session 是 `BLOCKED`
- 没有 recovered position
- 没有 reconcile gate blocker
→ 自动 resume 到 `RUNNING`

问题：`BLOCKED` 不只是 recovery 原因造成的。如果有其他 block reason，这个 helper 会错误地解除阻塞。

**修复模式**：

- 去掉自动 resume
- BLOCKED session 必须由人工或显式状态转移条件解除

**规则提炼**：
> 自动 resume / auto-dispatch 必须有**充分且显式**的前置条件验证，而不是"已知的阻塞原因都不存在 → 就恢复"。因为你永远不知道未来会新增什么阻塞原因。

---

### 4.2 Reconcile 吃历史外部订单

**典型案例**：PR#126（Guard live reconcile against historical external orders）

reconcile 从交易所 REST 拉到历史外部订单（非本系统下的、已关闭的），试图据此 self-heal 本地仓位，导致错误的仓位状态。

**修复模式**：

- 如果 reconcile 只找到历史外部订单，保持 reconcile gate blocking，要求人工 review
- 增加 skip 规则：跳过 historical external terminal orders
- 收紧 reconcile symbol 选择逻辑

**规则提炼**：
> reconcile 只能信任**当前活跃的、本系统下达的**订单/仓位。历史外部订单不能作为 self-heal 的依据。

---

### 4.3 Immediate fill 双落账

**典型案例**：PR#124（fix immediate filled settlement idempotency）

订单立即成交（FILLED）后，settlement 尚未完成就触发了 `SyncLiveAccount` + position refresh，导致仓位被创建两次（settlement 一次 + reconcile 一次）。

**修复模式**：

- `immediateFillSyncRequired` 从 retry marker 收敛为"settlement 尚未完成"的 pending 状态
- 对 `FILLED && settlementPending` 的订单，不提前触发 account sync / position refresh
- 账户级 reconcile 中，如果同账户同 symbol 存在 pending settlement 订单，记录 `order-settlement-pending` gate，阻止直接 adopt

**规则提炼**：
> 订单状态变更的消费必须幂等。如果 settlement 还没完成，后续的 sync / reconcile 不能抢先落账。

---

## 5. 性能与限流

### 5.1 全表扫描在热路径

**典型案例**：PR#124

`liveSettlementPendingOrderSymbols(accountID)` 使用 `ListOrders()` 后内存过滤 pending settlement，在 live sync / reconcile 热路径上造成全表扫描。

**规则提炼**：
> live sync / reconcile 热路径上的查询必须有索引或按条件查询。`ListXxx()` + 内存过滤在数据量增长后会成为瓶颈。

---

### 5.2 Binance 429 限流

**典型案例**：PR#126、PR#135、PR#136、PR#138

多个 live session 并发 REST sync 打爆 Binance 请求限额，触发 429 限流。

**修复模式（跨多个 PR 逐步修复）**：

- PR#135：统一 Binance REST request gating
- PR#136：Coalesce live account sync requests（合并同一时间窗口内的重复 sync）
- PR#138：Reduce monitor chart Binance polling
- PR#143：Throttle terminal order account sync

**规则提炼**：
> 对外部 API 的调用必须有统一 gating 层。多个调用方（live session sync、monitor chart、reconcile）不能各自独立打 REST，必须经过统一的请求合并和限流。

---

### 5.3 NaN 污染 session state

**典型案例**：PR#51（block NaN from LiveSession state）

float 运算产生 NaN，写入 JSON state 后整个 session 不可恢复（JSON 不支持 NaN）。

**修复模式**：

- 在 `UpdateLiveSessionState(...)` 入口做 NaN 检查
- `json.Marshal` 错误要被捕获并处理，不能静默继续

**规则提炼**：
> 任何写入持久化的 float 值，必须在写入前检查 NaN / Inf。一旦 NaN 进入 JSON state，后续所有读取都会失败。

---

## 6. 监控与告警可信度

### 6.1 Telegram 告警 flap

**典型案例**：PR#146

告警 delivery 从 `recovered` / `failed` 状态再次变为活跃时，复用了上一轮的旧 `firstActiveAt`，导致新一轮短暂 stale 一进来就被认为超过了 45s grace window，直接发送（绕过了抑制窗口）。

**修复模式**：

- 当 flap-suppressed delivery 已经是 `recovered / failed` 后再次活跃，重置 `firstActiveAt`
- 补测试覆盖 recovered delivery 重新活跃不会立即发送

**规则提炼**：
> 告警的 flap suppression 状态机必须完整覆盖：`active → recovered → 再次 active` 这个循环。每次从 recovered 回到 active，grace 计时器必须重置。

---

### 6.2 Stale source 告警 debounce

**典型案例**：PR#130、PR#131

stale source 告警使用不稳定的 ID（包含动态内容），导致同一个 stale 条件反复创建新的 delivery，无法被 dedup / suppress。

**修复模式**：

- 使用 stable alert ID（如 `live-warning-stale-source-states-{accountID}`）
- debounce 逻辑基于 stable ID

**规则提炼**：
> 告警 ID 必须是稳定的、可预测的。不能在 ID 中包含时间戳、动态文案或频繁变化的状态值。

---

## 7. PR 协作纪律

### 7.1 PR 范围蔓延

**典型案例**：PR#22（经历了 16+ 轮 AI review + 修复循环），PR#39

PR#22 从"加 execution normalization telemetry"开始，逐步扩展到 SL protection 修复、watermark 身份重构、adapterSubmission fallback 语义调整——最终变成了一个横跨 7 个文件、增加了 1000+ 行新代码的巨型 PR。

PR#39 从"补 health snapshot"开始，最终碰到了 live account sync 主路径、runtime policy 更新语义、dispatcher retry 行为。

**规则提炼**：
> 一个 PR 只解决一个问题域。如果发现"顺手"需要改的东西已经超出原始目标，应该新开 issue + 新分支。"加监控"不应该变成"改 live sync 行为"。

---

### 7.2 AI Agent 特有的协作模式

从 155 个 PR 中观察到的 AI Agent（主要是 Codex / wuyaocheng 使用 Codex）特有的行为模式：

| 模式 | 描述 | 建议 |
|------|------|------|
| 倾向于"一次修完" | Agent 会在一个 PR 中反复叠加 fix，导致 PR 膨胀 | 限制单 PR 的 commit 次数和文件范围 |
| 零值语义盲区 | 反复在 bool/float 的零值 fallback 上出错 | 对 fallback / merge 逻辑必须人工 review |
| 测试覆盖偏正确路径 | 补的测试多是"基础正确路径"，少"边界错误路径" | review 时明确要求 failure path 测试 |
| 对 recovery 语义缺乏先验 | 不知道"恢复后什么能做什么不能做" | AGENTS.md §7 的显式状态机是必须的 |
| review 响应速度快但深度不够 | 每次 @codex review 都快速回应，但对深层语义问题发现率低 | 高风险 PR 仍需人工主审，AI review 做辅助 |

---

## 8. 部署与运维实战

### 8.1 Migration 安全

| 规则 | 说明 |
|------|------|
| 使用 `ADD COLUMN IF NOT EXISTS` | 保证 migration 幂等性（PR#39） |
| 不在热路径上加无界索引 | migration 对大表的 ALTER 必须评估锁和性能影响 |
| migration 文件命名必须唯一递增 | 生产环境靠文件名判断是否已执行（PR#121 alias migration 踩过坑） |

### 8.2 Binance REST 限流治理

跨 PR#126、PR#135、PR#136、PR#138、PR#143 逐步完成的限流治理全貌：

```
请求发起方           统一 gating 层          Binance REST
──────────          ──────────            ─────────
live session sync ─┐
monitor chart     ─┤─→ account-level gate ─→ coalesce ─→ rate limit ─→ REST API
reconcile         ─┤   (per accountID)       (时间窗口合并)  (全局频率)
terminal order    ─┘
```

关键设计：
- 按 `accountID` 加互斥锁，防止同一账户并发 REST
- 同一时间窗口内的多次 sync 请求合并为一次
- 失败后按 freshness window 节流重试，不是固定间隔重打
- monitor chart 降低 Binance polling 频率

### 8.3 日志与可观测性

- 容器重启后日志需持久化到 volume（PR#68）
- `healthDayKey()` 使用 `time.Local`，如果服务器时区与运营时区不一致，"今日"统计边界会漂移（PR#39 review 提醒）
- stale source 告警使用 stable alert ID + debounce（PR#130、PR#131）
- 运行时 healthSummary 按 section 分槽（`accountSync`、`strategyIngress`、`execution`、`tradeTick`、`orderBook`），而非往 metadata 随便堆 key（PR#39）

---

## 9. Review 黄金规则

从 folgercn 在 155 个 PR review 中反复强调的审查标准，提炼为 10 条可执行的规则：

1. **成功/失败路径必须统一 accounting** — 每个出口都走统一 helper，不允许散落多处
2. **不允许"失败装成功"** — fallback 失败必须真实返错，不能静默吞掉
3. **同一个判定只能有一个入口** — alerts 和 snapshot 不能各自维护平行条件
4. **全链路一致性** — 配置从 API → 内存 → 持久化 → 判断函数必须同语义
5. **缓存态 ≠ 事实** — WS 状态、内存快照、推导结果不能直接当交易事实
6. **未对账不允许自动执行** — recovery / takeover 后必须等 REST 对账完成
7. **PR 不能静默扩大范围** — "加监控"不能顺手变成"改 live sync 行为"
8. **Legacy 数据迁移需要兼容性测试** — 隐式改身份键必须补回归测试
9. **热路径不能全表扫描** — live sync / reconcile 路径上的查询必须有索引
10. **自动 resume / dispatch 必须有显式前置条件** — 不能靠"看起来没问题就恢复"

---

## 附录：PR 速查索引

按主题分类的关键 PR 索引，方便查找具体案例：

### 执行安全
| PR | 标题 | 关键教训 |
|----|------|----------|
| #22 | Execution normalization, SL protection, watermark | 零值 fallback、watermark 身份、BUY/SELL 对称 |
| #109 | Fix live immediate fill settlement and stop defaults | 自动 resume 危险、immediate fill settlement |
| #112 | Fix live close settlement closure | settlement 闭环 |
| #124 | Fix immediate filled settlement idempotency | 双落账防护 |
| #126 | Guard live reconcile against historical external orders | reconcile 安全边界 |

### 运行时恢复
| PR | 标题 | 关键教训 |
|----|------|----------|
| #92 | Fix live manual close and watchdog recovery gating | 手动平仓 + reconcile gate |
| #93 | Fix live recovery close-only fallback | recovery 被动平仓 |
| #95 | Fix runtime recovery passive-close metadata | 被动平仓 metadata 完整性 |
| #96 | Hard reconcile gate before takeover | 接管前强制对账 |
| #97 | Add recovered close execution boundary guard | 恢复关闭执行边界 |
| #98 | Define takeover state machine and action matrix | 接管状态机定义 |
| #99 | Enforce runtime recovery reconcile gates | reconcile gate 执行 |
| #100 | Restart/takeover/passive-close regression suite | 回归测试套件 |

### 监控与可观测
| PR | 标题 | 关键教训 |
|----|------|----------|
| #39 | Add backend health snapshots and sync monitoring | 状态双写、语义一致性、配置全链路 |
| #130 | Debounce telegram stale-source alerts | 告警 debounce |
| #131 | Use stable live stale alert IDs | 稳定告警 ID |
| #146 | Suppress transient runtime recovery alerts | 告警 flap suppression |

### Binance 限流
| PR | 标题 | 关键教训 |
|----|------|----------|
| #135 | Unify Binance REST request gating | 统一 gating |
| #136 | Coalesce live account sync requests | 请求合并 |
| #138 | Reduce monitor chart Binance polling | 降低轮询 |
| #143 | Throttle terminal order account sync | 终端订单节流 |
