# Issue #199 Runtime Runner 拆分工作规划

> Parent issue: [#199 拆分 live-runner 与 signal-runtime-runner](https://github.com/folgercn/bktrader/issues/199)
>
> Core protocol: [runtime-runner-decomposition-protocol.md](runtime-runner-decomposition-protocol.md)
>
> Risk & Testing: [runtime-runner-issue-199-risk-and-testing.md](runtime-runner-issue-199-risk-and-testing.md)

## 1. 目标

本规划用于把 #199 拆成可串行 review、可独立回滚的小 PR。实现时必须以 `runtime-runner-decomposition-protocol.md` 为核心协议，不允许把 runtime session 持久化、JetStream event bus、consumer 改造、进程拆分和 lease 防双活混在同一个 PR。

最终后台进程形态：

- `platform-api`: API、配置、查询、人工操作入口。
- `live-runner`: live session 生命周期、REST 对账、dispatch、order management、position/account sync。
- `signal-runtime-runner`: 行情 WebSocket、runtime state、runtime event publish。
- `notification-worker`: Telegram / 告警通知投递。

长期设计必须继续兼容更细拆分：

- `reconcile-runner`
- `dispatch-runner`
- `order-manager`
- `position-account-sync-runner`
- `market-ws-runner`
- `signal-bar-runner`
- `runtime-health-runner`
- `strategy-evaluation-runner`
- `alert-evaluator`

## 2. 全局约束

- 不改变 `dispatchMode=manual-review` 默认值。
- 不放宽 recovery / reconcile gate。
- 不把 WebSocket 或 JetStream event 当作交易事实源。
- 不把 lease owner 当作交易事实源。
- REST 仍是启动、恢复、接管和关键执行边界的权威校验入口。
- JetStream 是 at-least-once 消费模型，业务层必须做 idempotency。
- 每个 PR 必须独立合并、独立回滚。
- 每个 PR 必须保持当前 monolith / 当前拆分形态可运行。

## 3. Issue 顺序

### Step 1: #200 持久化 signal runtime session 状态

Issue: [#200](https://github.com/folgercn/bktrader/issues/200)

状态：

- ✅ **PR 已提交（2026-04-26）**：[#220 Persist signal runtime sessions (#200)](https://github.com/folgercn/bktrader/pull/220)
- ✅ 已完成：`signal_runtime_sessions` migration、store CRUD、memory/postgres 实现、service cache miss → store restore、`syncLiveSessionRuntime` 复用旧 runtime ID。
- ✅ 已按 review 修正：Create 语义改为真 upsert（同一 account+strategy 保留 runtime identity 但刷新 status/adapter/transport/subscription/state）、Start 使用运行占位避免并发双启动、Delete 改为 DB 删除成功后再取消本地 runner、not-found 改为 typed error。
- 🟡 待 review/merge：#220 合并后才能开始 #201。

目标：

- 将 `SignalRuntimeSession` identity / state / subscription plan 从 `Platform.signalSessions` 内存 map 中持久化到 store。
- `Platform.signalSessions` 降级为运行缓存。
- 新进程内存为空但 DB 有 runtime session 时，live session 可以复用旧 `signalRuntimeSessionId`。

建议范围：

- 新增 `signal_runtime_sessions` migration。
- 扩展 `store.Repository` runtime session CRUD。
- 实现 memory store 与 postgres store。
- 改造 `Create/Get/List/Update/Stop/DeleteSignalRuntimeSession`。
- 改造 `syncLiveSessionRuntime`，内存找不到时先从 store 恢复。

不做：

- 不引入 NATS JetStream。
- 不改 live evaluation 触发路径。
- 不新增独立 `signal-runtime-runner`。
- 不改 dispatch / reconcile / recovery gate。

最低测试：

- memory store runtime session CRUD。
- postgres store runtime session CRUD / upsert 语义测试。
- `syncLiveSessionRuntime` 在内存缓存为空但 store 存在时复用旧 ID。
- stop/delete 状态持久化正确。

### Step 2: #201 增加 NATS JetStream runtime event bus

Issue: [#201](https://github.com/folgercn/bktrader/issues/201)

目标：

- 增加 runtime event bus interface。
- 用 NATS JetStream 作为 production event bus。
- 用 in-memory fake 支持 unit test。
- 现有 WebSocket 消息路径继续工作，同时 side-publish 标准化 runtime event 到 JetStream。

建议范围：

- 新增 runtime event envelope 类型。
- 定义 stream：`BKT_RUNTIME_EVENTS`。
- 定义 subject：`bktrader.runtime.signal.v1.<account_id>.<strategy_id>.<symbol>.<stream_type>`。
- 定义 stable `id` / `fingerprint`。
- 增加 JetStream publisher。
- 增加 in-memory fake publisher。
- 可选增加 Postgres audit archive，但 audit 不能成为主消费队列。

不做：

- 不让 `live-runner` 消费 JetStream。
- 不新增独立 `signal-runtime-runner`。
- 不改变旧 callback fanout。
- 不把 event archive 当主 inbox。

最低测试：

- event envelope 字段完整。
- fingerprint 稳定且不含写入时间。
- duplicate event 幂等。
- JetStream stream / subject 配置测试。
- publish 失败不影响现有内存 fanout，但必须记录日志或状态。

### Step 3: #202 live-runner 消费 JetStream event

Issue: [#202](https://github.com/folgercn/bktrader/issues/202)

目标：

- `live-runner` 通过 JetStream durable consumer 消费 runtime event。
- event consumer 触发 live evaluation。
- 旧同进程 callback 路径只作为过渡兼容，不再是拆分后的真实边界。

建议范围：

- 新增 `SignalRuntimeEventConsumer`。
- 定义 durable consumer：`live-evaluation`。
- 按 event time / stream sequence 保持稳定处理语义。
- 消费成功后 ack。
- 消费失败不 ack，并保留 retry / error 可观测性。
- 对 dispatch proposal / order 提交做幂等防护。

不做：

- 不新增独立 `signal-runtime-runner`。
- 不改变 dispatch 默认值。
- 不放宽 recovery / reconcile gate。
- 不把 WebSocket / JetStream event 当 REST 事实源。

最低测试：

- JetStream event 触发 live evaluation happy path。
- duplicate event 不重复推进 proposal / order。
- consumer 失败时不 ack。
- stale source / readiness gate 仍然阻断。
- recovery / reconcile gate 不被绕过。

### Step 4: #203 新增独立 signal-runtime-runner 进程

Issue: [#203](https://github.com/folgercn/bktrader/issues/203)

目标：

- 新增 `BKTRADER_ROLE=signal-runtime-runner`。
- 行情 WebSocket 与 live trading runtime 分离部署。
- `signal-runtime-runner` 只负责 runtime session scanner、WS、runtime state、JetStream publish。

建议范围：

- 扩展 config role validation。
- 扩展 `platform-worker` role。
- 扩展 `RuntimeOptionsForRole`：`signal-runtime-runner` 启用 `WarmLiveMarketData`，`live-runner` 去掉 `WarmLiveMarketData`（行情预热随 WS 归属迁移）。
- 新增 runner scanner。
- 更新 compose / CD 部署范围。
- API start/stop 语义逐步转为 desired state。

不做：

- 不启动 live recovery。
- 不启动 live sync。
- 不执行 dispatch。
- 不引入多副本扩缩容。

最低测试：

- role option mapping。
- `signal-runtime-runner` 不启动 live sync / recovery。
- `live-runner` 不启动行情 WS。
- compose 服务定义包含正确 `BKTRADER_ROLE`。
- desired status 与 actual status 语义不混淆。

### Step 5: #204 runner lease / ownership 防双活

Issue: [#204](https://github.com/folgercn/bktrader/issues/204)

目标：

- 防止同一类 runner 多实例同时处理同一 resource。
- 为未来更细 runner 复用统一 ownership 协议。

建议范围：

- 新增 `runtime_leases` 表。
- 实现 acquire / heartbeat / release / expired takeover。
- resource type 至少覆盖：
  - `signal-runtime-session`
  - `live-session`
  - `account-sync`
- runner 处理 resource 前必须 acquire lease。

不做：

- 不做 leader election 系统。
- 不引入 Kubernetes primitives。
- 不改变 dispatch 默认值。
- 不把 lease 当交易事实源。

最低测试：

- acquire success。
- active lease blocks second owner。
- expired lease takeover。
- release only by owner。
- heartbeat extends lease。
- 未拿到 lease 的 runner 跳过。

## 4. GitHub Issue 描述同步

✅ **已完成（2026-04-26）**：#200–#204 的标题与描述已按本规划同步更新。

- #201 目标已改为 NATS JetStream runtime event bus。
- Postgres 只保留业务事实状态和可选 audit archive。
- #202 消费对象已明确为 JetStream durable consumer。

## 5. 推荐分支与 PR 策略

每个 step 单独分支：

- `codex/issue-200-persist-signal-runtime-sessions`
- `codex/issue-201-jetstream-runtime-event-bus`
- `codex/issue-202-live-runner-consume-runtime-events`
- `codex/issue-203-signal-runtime-runner`
- `codex/issue-204-runner-lease-ownership`

Review / merge 必须串行。后续 step 只能基于已合并的前序 step。

## 6. 开始代码前检查表

每个 step 开始前必须确认：

- 是否只解决当前 issue。
- 是否触碰 `internal/service/live*.go` 或 `execution_strategy.go`。
- 是否需要阅读 `docs/pr-lessons-learned.md`。
- 是否需要阅读 `docs/runtime-recovery-extension-coding-rules.md`。
- 本次事实源、缓存态、推导态分别是什么。
- 是否影响 auto-dispatch。
- 是否影响 final submit payload。
- 是否影响 reconcile / recovery gate。
- 是否有 failure path 测试。
