# Runtime Runner 拆分协议基线

> 适用范围：Issue #199 及后续把 `live-runner`、`signal-runtime-runner` 继续细拆的所有 runtime 工作。

## 1. 目标

本文件先定义进程拆分前必须稳定下来的协议边界，避免只把当前业务拆成几个更大的黑箱。

短期目标是完成 #199：

- `platform-api`
- `live-runner`
- `signal-runtime-runner`
- `notification-worker`

长期目标是让以下业务能力可以继续独立拆分，而不再做大规模语义迁移：

- live session 生命周期
- REST reconcile / recovery gate
- dispatch proposal / final submit
- order management / fills / settlement
- position/account sync
- market WebSocket
- signal bar / event normalization
- runtime health / sourceStates
- strategy evaluation
- alert evaluation
- notification delivery

因此 #199 的重点不是容器数量，而是稳定以下协议：

- runtime session identity
- NATS JetStream runtime event bus
- consumer acknowledgement
- command / desired-state boundary
- runner lease / ownership
- fact / cache / derived state hierarchy

## 2. 标准化依赖策略

本项目不应该手搓通用基础设施和行业通用交易语义。拆分 runtime 时，优先使用成熟协议、成熟基础设施和行业标准，再在其上实现 bktrader 自己的安全语义。

### 2.1 Runtime Event Bus

Runtime event bus 目标采用 NATS JetStream：

- 官方文档：[NATS JetStream](https://docs.nats.io/nats-concepts/jetstream)
- Consumer 文档：[NATS JetStream Consumers](https://docs.nats.io/nats-concepts/jetstream/consumers)
- 开发模型：[NATS JetStream Developer Guide](https://docs.nats.io/using-nats/developer/develop_jetstream)

使用原则：

- JetStream 负责 durable stream、replay、durable consumer、ack 和 consumer progress。
- Postgres 仍负责业务事实状态，例如 runtime session、orders、fills、positions、reconcile gate。
- JetStream event 不能替代 REST 事实源，也不能替代 final submit guard。
- JetStream 是 at-least-once 消费模型，业务层必须保留 fingerprint、idempotency key 和重复消费防护。
- 单元测试应通过 event bus interface + in-memory fake 覆盖；需要 broker 行为时再跑 JetStream integration test。

### 2.2 交易消息语义

内部订单、成交、执行报告等语义应尽量向 FIX 概念对齐，而不是重新发明一套交易词汇。

参考：

- 官方组织：[FIX Trading Community](https://www.fixtrading.org/about/)
- 在线规格：[FIX Online Specification](https://dev.fixtrading.org/online-specification/introduction/)
- Go FIX engine 参考：[quickfixgo/quickfix](https://github.com/quickfixgo/quickfix)

当前不要求接入 FIX session，但内部 domain/event 命名和状态语义应参考常见 FIX 概念，例如：

- order intent / new order
- execution report
- order status
- execution type
- client order id
- exchange order id
- cumulative quantity
- leaves quantity

### 2.3 交易所 Adapter 与量化库

交易所 API 抽象可以参考成熟 crypto schema，但不能放弃本项目自己的执行安全边界。

参考：

- 多交易所 crypto API schema：[CCXT](https://github.com/ccxt/ccxt)
- 技术指标库：[TA-Lib](https://ta-lib.org/)

使用原则：

- 交易所 market/order/balance schema 可以参考成熟库。
- Binance futures 的 `reduceOnly`、`positionSide`、HEDGE/ONE_WAY、step/tick/notional 精度边界必须保留本项目 final guard。
- research / indicator 侧优先使用成熟库，避免重复实现常见指标。
- live runtime 的 recovery、reconcile、dispatch safety 不能交给普通量化库。

### 2.4 Durable Workflow

未来如果 runner command、order settlement、reconcile workflow 变成长生命周期流程，可以评估 durable workflow 引擎。

参考：

- [Temporal Documentation](https://docs.temporal.io/)

当前 #199 不引入 workflow engine；本文件只要求 event、ack、lease、desired state 协议不要阻塞未来迁移。

## 3. 核心原则

### 3.1 先协议，后进程

进程拆分只能发生在协议稳定之后。任何 runner 不应该依赖另一个 runner 的内存 map、goroutine callback、cancel function 或私有 struct。

### 3.2 外部 WS 是行情接入，JetStream 是内部事件总线，REST 是事实边界

WebSocket 仍然需要，但它只负责连接交易所或外部行情源，把实时行情接入系统。NATS JetStream 负责系统内部 runner 之间的持久事件分发、replay 和 durable consumer progress。

边界必须明确：

- external WebSocket: exchange / market data source -> bktrader。
- NATS JetStream: bktrader runner -> bktrader runner。
- REST: exchange truth verification -> bktrader fact state。

WebSocket event 和 JetStream event 都只允许作为实时触发器、可重放输入和缓存更新输入。

以下行为仍必须以 REST 或已确认本地事实为准：

- recovery / takeover 是否允许继续
- reconcile gate 是否解除
- final dispatch submit 是否安全
- position/account truth 是否可信

### 3.3 一个业务事实只有一个写入口

同一类状态不能由多个 runner 平行维护。需要共享时，必须通过 store、event、ack、lease 或明确的 command/desired state 协议表达。

### 3.4 每一步都可回滚

#199 每个 step 都必须保持当前生产形态可运行，并且可以独立合并、独立回滚。

## 4. 未来业务单元

这些不是 #199 必须一次拆出的进程，而是协议设计必须提前兼容的边界。

| 候选单元 | 职责 | 禁止承担 |
| --- | --- | --- |
| `live-session-runner` | live session 状态机、启动/停止/恢复意图 | 直接下单、直接信任 WS 为仓位事实 |
| `reconcile-runner` | REST 对账、recovery/takeover gate、exchange truth 校验 | 生成策略 intent、提交订单 |
| `dispatch-runner` | dispatch proposal 消费、manual-review/auto-dispatch 边界、final submit 前二次校验 | 维护行情 WS、绕过 reconcile gate |
| `order-manager` | 订单同步、terminal order、fills、settlement、close verification | 策略评估、行情订阅 |
| `position-account-sync-runner` | 持仓/账户权益快照、account sync、REST rate limit gating | 策略 intent、行情聚合 |
| `market-ws-runner` | 行情 WS 连接、订阅、重连、原始事件接入 | live dispatch、REST reconcile |
| `signal-bar-runner` | signal bar 标准化、闭合、补洞、continuity | 下单、账户状态修改 |
| `runtime-health-runner` | sourceStates、readiness、stale/quiet health summary | 交易事实判定 |
| `strategy-evaluation-runner` | 将稳定 signal event 转成 strategy decision/proposal | final submit、REST truth 判定 |
| `alert-evaluator` | 告警判定 | 通知投递副作用 |
| `notification-worker` | Telegram/通知投递 | 告警语义判定、交易执行 |

## 5. 状态分层

| 层级 | 示例 | 可用于执行决策吗 |
| --- | --- | --- |
| 事实源 | REST 对账结果、已确认订单、已确认成交、已确认持仓 | 可以，但仍需 final submit guard |
| 缓存态 | `SignalRuntimeSession.State`、`sourceStates`、WS last event、`session.State` 临时字段 | 不可以直接作为交易事实 |
| 推导态 | readiness、runtime health、strategy decision、proposal、intent | 不可以直接作为事实，只能进入下一层校验 |

任何 runner 拆分都必须说明自己读取、写入的是哪一层。

## 6. Runtime Session 协议

`signal_runtime_sessions` 表示稳定的 runtime identity，不表示某个进程内的 goroutine。

建议字段：

- `id`
- `account_id`
- `strategy_id`
- `status`
- `desired_status`
- `runtime_adapter`
- `transport`
- `subscription_count`
- `state`
- `created_at`
- `updated_at`

短期 #200 可以先不引入 `desired_status`，但 #203 API start/stop 语义改为 runner scanner 后，应避免把“期望运行”和“实际已连接”混在一个字段里。

语义约束：

- `id` 是跨进程身份，不因 runner 重启改变。
- `state.sourceStates` 是缓存态，不是交易事实。
- `state.subscriptions` 是运行计划快照，更新它不等价于运行中连接已热切换。
- `Platform.signalSessions` 只能作为运行缓存，不能继续作为事实源。

## 7. Runtime Event Bus 协议

Runtime event bus 使用 NATS JetStream。`signal-runtime-runner`、未来的 `market-ws-runner` 或 `signal-bar-runner` 发布标准化 runtime event；`live-runner` 和后续更细 runner 通过 durable consumer 消费。

建议 stream：

- `BKT_RUNTIME_EVENTS`

建议 subject：

- `bktrader.runtime.signal.v1.<account_id>.<strategy_id>.<symbol>.<stream_type>`
- `bktrader.runtime.health.v1.<runtime_session_id>`
- `bktrader.runtime.command.v1.<resource_type>.<resource_id>`（未来 command 化时使用）

Runtime signal event 建议 envelope：

- `id`
- `runtime_session_id`
- `account_id`
- `strategy_id`
- `source_key`
- `role`
- `stream_type`
- `symbol`
- `timeframe`
- `event_type`
- `event_time`
- `fingerprint`
- `payload`
- `created_at`

幂等约束：

- publish 方必须提供 stable `id` 与 `fingerprint`。
- `fingerprint` 不得包含写入时间、runner owner、随机数。
- signal bar 应包含 bar identity，例如 `barStart/barEnd`。
- tick/order book 应使用交易所事件 id；没有事件 id 时使用 `event_time + summary hash`。
- consumer 必须按 `fingerprint` 或业务 idempotency key 去重，不能假设 JetStream exactly-once。

payload 约束：

- payload 只放本事件需要的摘要，不放无限历史。
- 大历史留在 runtime session state 或后续专门的 snapshot 表。
- payload 必须能被 future consumer 理解，不能绑定某个 Go 私有 struct。

Postgres audit 约束：

- 如需查询和审计，可增加 event archive 表。
- archive 不是主消费队列，不能让业务消费路径再次依赖 DB polling。

## 8. Consumer Ack 协议

Consumer ack 以 JetStream durable consumer 为主，不再以单表 `processed_at` 作为主协议。

建议 durable consumer：

- `live-evaluation`
- `runtime-health`
- `signal-bar-continuity`
- `alert-evaluation`
- `audit-replay`

如果需要业务级审计，可另建 consumer ack archive：

- `event_id`
- `consumer_group`
- `consumer_owner_id`
- `status`
- `attempt_count`
- `last_error`
- `processed_at`
- `updated_at`

约束：

- 处理失败不得标记成功。
- 重复消费不得导致重复 dispatch proposal 或重复订单。
- durable consumer 的 ack policy、ack wait、max deliver、dead-letter 或 retry 策略必须在集成测试中覆盖。
- 业务成功和 JetStream ack 的顺序必须明确；不能先 ack 后执行关键副作用。

## 9. Command / Desired State 协议

API 不应长期直接启动其他 runner 的 goroutine。

短期兼容：

- monolith / current live-runner 可以继续直接 `StartSignalRuntimeSession`。

拆分后目标：

- API 写 `desired_status` 或 command。
- runner scanner 根据 desired state 启停实际连接。
- UI 必须区分 desired status 与 actual status，避免“请求成功”等同于“WS 已连接”。

如果未来 command 表落地，命令必须满足：

- stable command id
- idempotency key
- target resource type/id
- requested by / requested at
- status / error
- ack by owner

## 10. Lease / Ownership 协议

lease 用来防双活，不是交易事实源。

统一 resource type 建议：

- `signal-runtime-session`
- `live-session`
- `account-sync`
- `order-sync`
- `strategy-evaluation`
- `dispatch`
- `alert-evaluation`

约束：

- runner 处理资源前必须 acquire lease。
- 未拿到 lease 必须跳过，不得继续处理。
- heartbeat 停止并过期后允许 takeover。
- release 只能由 owner 执行。
- lease 失败应记录并跳过，不应 panic 整个 runner。
- lease 不能替代 reconcile gate、reduce-only guard 或 final submit checks。

## 11. #199 分阶段协议要求

### Step 1: 持久化 runtime session

只做 identity/state 持久化。

不做：

- runtime event bus
- 独立 runner
- live evaluation 触发路径改变
- dispatch/reconcile gate 改动

必须保证：

- 内存缓存为空但 DB 有 session 时，live session 复用旧 `runtime_session_id`。
- stop/delete 状态持久化正确。

### Step 2: NATS JetStream runtime event bus

只做 WS event publish 到 JetStream，并可选写 audit archive。

不做：

- live-runner 消费 JetStream
- 独立 runner
- 改变旧 callback fanout

必须保证：

- publish 失败不影响现有内存路径，但必须可观测。
- 重复 fingerprint 幂等。
- stream / subject / event envelope 有测试覆盖。

### Step 3: live-runner 消费 JetStream event

这是 live runtime 行为变更。

必须保证：

- 消费成功后才 ack。
- 消费失败不假装成功。
- stale source / readiness / recovery gate 不放宽。
- duplicate event 不重复推进 dispatch proposal 或 order。

### Step 4: 独立 signal-runtime-runner

只拆行情 runtime。

必须保证：

- `signal-runtime-runner` 不启动 live recovery / live sync / dispatch。
- `live-runner` 不启动行情 WS。
- API start/stop 语义明确区分 desired 与 actual。

### Step 5: runner lease / ownership

统一防双活协议。

必须保证：

- 两个同类 runner 同时启动时，同一 resource 只有一个 owner 处理。
- 过期 lease 可 takeover。
- 未获得 lease 的 runner 跳过。

## 12. 禁止耦合

后续拆分中禁止新增以下耦合：

- 跨 runner 读取对方内存 map。
- 跨 runner 调用对方 goroutine cancel function。
- event payload 使用某个进程私有 struct 作为协议。
- 用 WS reconnect 成功直接解除 REST reconcile gate。
- 用 lease owner 作为交易事实。
- 在同一个 PR 同时改 session 持久化、event 消费、进程角色、lease。

## 13. 最低测试矩阵

协议相关 PR 至少覆盖：

- memory store 与 postgres store 行为一致。
- 幂等写入和重复消费。
- 失败路径不 ack / 不 processed。
- JetStream stream / subject / durable consumer 配置。
- runner role option mapping。
- 未获得 lease 时跳过。
- stale / readiness / recovery gate 不因事件消费被绕过。
- 默认 `dispatchMode=manual-review` 不变。

涉及 final submit、reconcile、recovery、dispatch 的 PR，还必须遵守 `runtime-recovery-extension-coding-rules.md` 和 `pr-lessons-learned.md`。
