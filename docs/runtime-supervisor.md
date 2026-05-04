# Runtime Supervisor / Service Supervisor 分层规划

> 对应 GitHub Issue #270：分层推进统一 Runtime Supervisor / 容器控制面。

## 1. 目标

本文件定义 bktrader 后续统一 Runtime Supervisor / Service Supervisor 的推进边界。它的目标不是把已经拆开的业务重新合并到一个进程，而是在现有多进程、多容器形态之上增加统一控制面。

核心目标：

- 统一长期运行任务的状态语义：期望状态、实际状态、健康状态、重启计划、最后错误、自动恢复抑制。
- 区分服务级健康和业务 runtime 健康，避免把容器存活误判为策略或 live session 健康。
- 优先应用内 runtime 自愈，只有服务不可访问或内部恢复失败时才进入容器级兜底。
- 先做只读可观测，再做业务级恢复，最后才做容器级控制。
- 保留 bktrader 的交易安全边界：未对账不得自动交易，人工 stop 不得被自动拉起，fatal 错误不得无限重启。

## 2. 分层模型

### 2.1 应用内 Runtime Supervisor

应用内 Runtime Supervisor 处理业务逻辑层面的异常，不直接重启 Docker 容器。

适用对象：

- signal runtime
- live session runtime
- strategy worker
- market websocket runner
- runtime health / source state scanner
- 后续拆出的 order manager、reconcile runner、dispatch runner

典型异常：

- WebSocket 断连或订阅失败。
- runtime 进入 ERROR。
- signal bar continuity 缺口。
- live session 运行态异常。
- 业务 runtime 需要按照 desiredStatus 自动恢复。

这一层可以调用服务内部的 start / stop / restart 入口，但不能越过交易安全门禁。恢复后是否允许继续交易，仍然取决于 reconcile gate、事实源校验和 final submit guard。

### 2.2 容器级 Service Supervisor

容器级 Service Supervisor 处理基础设施层面的异常。

适用对象：

- `platform-api`
- `platform-worker`
- 独立 signal runtime runner
- 独立 live runner
- 后续拆出的 supervisor / control-plane service

典型异常：

- 容器进程退出。
- `/healthz` 连续失败。
- 服务完全不可访问。
- 服务响应超时。
- 应用内 `/runtime/status` 无法读取。

容器级恢复只能作为兜底，不能替代应用内 runtime 自愈。即使容器重启成功，也不能自动解除业务层的 reconcile / recovery / manual-review 约束。

## 3. 当前样板

PR #268 已经在 signal runtime 中落下第一版应用内自愈样板，后续抽象应保持现有行为不回退。

当前样板包括：

- `desiredStatus` 与 `actualStatus` 分离。
- `desiredStatus=RUNNING` 时，scanner 可以重新拉起未运行的 runtime。
- ERROR 后按 backoff 自动恢复，当前为 first `1m`、repeat `3m`。
- fatal 错误设置 `autoRestartSuppressed`，避免无限重启。
- 成功恢复后清理 supervisor restart 临时字段。
- `desiredStatus=STOPPED` 或关联 live session 已停止时，不允许 scanner 自动拉起。

当前 signal runtime 内部字段和推荐统一字段的关系：

| 当前字段 | 推荐统一字段 | 说明 |
| --- | --- | --- |
| `desiredStatus` | `desiredStatus` | 用户或系统期望状态。 |
| `actualStatus` | `actualStatus` | 当前 runtime 实际状态。 |
| `supervisorRestartAttempt` | `restartAttempt` | supervisor 计划内恢复次数。 |
| `nextAutoRestartAt` | `nextRestartAt` | 下一次允许恢复的时间。 |
| `supervisorRestartBackoff` | `restartBackoff` | 当前 backoff 间隔。 |
| `supervisorRestartReason` | `restartReason` | 恢复原因。 |
| `supervisorRestartSeverity` | `restartSeverity` | 错误分类。 |
| `lastSupervisorError` | `lastRestartError` | 最近一次恢复错误。 |
| `restartRequestedAt` / `restartRequestedReason` / `restartRequestedSource` / `restartRequestedForce` | 同名字段 | 最近一次统一 restart 控制请求的审计时间、原因、来源和 force 标记。 |
| `autoRestartSuppressed` | `autoRestartSuppressed` | 是否抑制自动恢复。 |
| `startRequestedAt` / `startRequestedReason` / `startRequestedSource` | 同名字段 | 最近一次统一 start 控制请求的审计时间、原因和来源。 |
| `stopRequestedAt` / `stopRequestedReason` / `stopRequestedSource` / `stopRequestedForce` | 同名字段 | 最近一次统一 stop 控制请求的审计时间、原因、来源和 force 标记。 |
| `autoRestartSuppressedAt` / `autoRestartSuppressedReason` / `autoRestartSuppressedSource` | 同名字段 | 最近一次 suppress 审计时间、原因和来源。 |
| `autoRestartResumedAt` / `autoRestartResumedReason` / `autoRestartResumedSource` | 同名字段 | 最近一次 resume 审计时间、原因和来源。 |

后续抽 helper 时应先提供兼容映射，不应在同一个 PR 中同时改字段名、改 scanner 行为和改前端展示。

## 4. 状态语义

### 4.1 desiredStatus

`desiredStatus` 表示用户或系统期望状态，不表示 runtime 当前真实状态。

推荐值：

| 值 | 语义 |
| --- | --- |
| `RUNNING` | 期望持续运行，允许应用内 supervisor 在安全条件满足时自愈。 |
| `STOPPED` | 人工或系统要求停止，不允许自动拉起。 |
| `PAUSED` | 暂停态，默认不自动恢复。 |

约束：

- 人工 stop 必须写入 `desiredStatus=STOPPED`。
- `desiredStatus=STOPPED` 时，应用内和容器级 supervisor 都不得主动恢复业务 runtime。
- 未显式声明 desiredStatus 的 legacy 记录必须有兼容策略，不能把 Go 零值静默解释成允许自动恢复。

### 4.2 actualStatus

`actualStatus` 表示 runtime 当前实际状态。

推荐值：

| 值 | 语义 |
| --- | --- |
| `STARTING` | 正在启动或连接。 |
| `RUNNING` | 业务 runtime 已运行。 |
| `RECOVERING` | 正在恢复中。 |
| `ERROR` | 已进入错误态。 |
| `STOPPED` | 已停止。 |
| `UNKNOWN` | 状态不可判定。 |

约束：

- 控制 API 返回 accepted 后，只表示期望状态已写入，不表示 actualStatus 已收敛。
- CLI 和前端必须以 actualStatus 收敛作为最终确认。
- 不能用容器健康直接覆盖 actualStatus。

### 4.3 health

`health` 表示 supervisor 视角下的健康摘要。

推荐值：

| 值 | 语义 |
| --- | --- |
| `healthy` | 服务和业务 runtime 均符合期望。 |
| `recovering` | 正在恢复或等待 backoff 到期。 |
| `error` | 业务 runtime 错误。 |
| `stale` | 缓存态过期或重连后缺少连续性校验。 |
| `unreachable` | 服务级接口不可访问。 |
| `suppressed` | 自动恢复被抑制。 |

### 4.4 restartSeverity

`restartSeverity` 用于决定是否允许自动恢复。

推荐值：

| 值 | 语义 | 自动恢复 |
| --- | --- | --- |
| `transient` | 临时网络、连接或订阅异常。 | 可以按 backoff 恢复。 |
| `kicked` | 被断开、限流或连接被挤掉。 | 谨慎恢复，可提高 backoff。 |
| `fatal` | 认证、权限、封禁、配置错误。 | 不允许自动恢复，必须 suppressed。 |
| `unknown` | 未分类错误。 | 默认保守处理。 |

## 5. 接口契约

### 5.1 服务级 `/healthz`

`/healthz` 只表达服务进程是否可响应，不表达业务 runtime 是否健康。

建议响应：

```json
{
  "service": "platform-api",
  "status": "ok",
  "checkedAt": "2026-04-28T12:30:00Z"
}
```

约束：

- `/healthz` 适合负载均衡、容器 healthcheck 和 Service Supervisor 探活。
- `/healthz=ok` 不代表 live session、signal runtime 或交易链路健康。
- `/healthz` 不应执行昂贵查询或外部交易所请求。

### 5.2 业务级 `/runtime/status`

`/runtime/status` 表达服务内 runtime 的业务状态。它可以是单 runtime，也可以返回多个 runtime 的集合。

当前平台 API 落点为 `GET /api/v1/runtime/status`，保持在现有鉴权中间件覆盖范围内。后续独立 runner 或 supervisor service 可以在内网暴露服务本地 `/runtime/status`，但不得裸露到公网。

建议单 runtime 响应：

```json
{
  "service": "signal-runtime",
  "runtimeId": "runtime-001",
  "runtimeKind": "signal",
  "desiredStatus": "RUNNING",
  "actualStatus": "ERROR",
  "health": "recovering",
  "restartAttempt": 2,
  "nextRestartAt": "2026-04-28T12:33:00Z",
  "restartBackoff": "3m0s",
  "restartReason": "runtime-error",
  "restartSeverity": "transient",
  "lastRestartError": "websocket timeout",
  "restartRequestedAt": "2026-04-28T12:22:00Z",
  "restartRequestedReason": "operator requested rebinding",
  "restartRequestedSource": "api",
  "restartRequestedForce": true,
  "autoRestartSuppressed": true,
  "autoRestartSuppressedAt": "2026-04-28T12:24:00Z",
  "autoRestartSuppressedReason": "operator paused runtime recovery during maintenance",
  "autoRestartSuppressedSource": "api",
  "lastHealthyAt": "2026-04-28T12:20:00Z",
  "lastCheckedAt": "2026-04-28T12:30:00Z"
}
```

建议多 runtime 响应：

```json
{
  "service": "platform-api",
  "checkedAt": "2026-04-28T12:30:00Z",
  "runtimes": []
}
```

约束：

- `/runtime/status` 返回缓存态和推导态，不是交易事实源。
- supervisor 可以用它判断是否需要恢复，但不能用它替代 REST 对账。
- 字段命名必须在不同 runtime 间保持一致。
- `restartRequested*`、`startRequested*`、`stopRequested*`、`autoRestartSuppressed*` / `autoRestartResumed*` 只暴露人工控制审计元数据，便于 CLI 和控制台确认最近一次控制来源；resume 后会清理 suppress 字段并写入 `autoRestartResumedAt` / `autoRestartResumedReason` / `autoRestartResumedSource`，这些字段不改变自动恢复判定语义。

### 5.3 控制接口

推荐后续统一接口：

```http
POST /runtime/start
POST /runtime/stop
POST /runtime/restart
POST /runtime/suppress-auto-restart
POST /runtime/resume-auto-restart
```

约束：

- 控制接口必须鉴权或仅内网暴露。
- `start` / `restart` 只能写入期望状态或触发应用内恢复，不得绕过业务安全校验。
- `stop` 必须写入 `desiredStatus=STOPPED`，使 scanner 和 supervisor 不再自动拉起。
- `resume-auto-restart` 只能解除 suppressed，不代表立即允许交易。
- 当前统一控制 API 已落最小 signal 切片：`POST /api/v1/runtime/start`、`POST /api/v1/runtime/stop`、`POST /api/v1/runtime/restart`、`POST /api/v1/runtime/suppress-auto-restart`、`POST /api/v1/runtime/resume-auto-restart` 均只支持 `runtimeKind=signal` / `signal-runtime`。请求必须显式传入 `confirm=true`；`start` / `stop` / `suppress-auto-restart` / `resume-auto-restart` 始终要求非空 `reason` 并写入 runtime state 审计字段，`restart` 在 `force=true` 时必须传入非空 `reason`。该接口复用现有 signal runtime 安全边界，不做 live session restart，不做 live session 自动 dispatch，也不做 Docker/container restart。

## 6. 推进阶段

### 阶段 1：统一可观测

目标：只读采集，不自动重启容器。

任务：

- 梳理当前长期运行 service / runtime 类型。
- 统一 `/healthz` 与 `/runtime/status` 的职责边界。
- 文档化 `desiredStatus`、`actualStatus`、`health`、restart 字段语义。
- 建立 service alive 和 runtime healthy 的区分。
- supervisor 周期性读取状态，但只记录，不执行恢复。

验收标准：

- 能区分“容器活着但业务 runtime ERROR”和“容器不可访问”。
- 能看到每个 runtime 的 desiredStatus、actualStatus、health。
- 没有新增容器级 restart 行为。

### 阶段 2：应用内 runtime 统一恢复

目标：把 signal runtime 样板抽成小 helper，并让至少一个新 runtime 或 service 接入。

当前 helper 落点：`internal/service/supervisor_restart.go`。推荐接口：

```go
type SupervisorBackoffPolicy struct {
    First  time.Duration
    Repeat time.Duration
    Max    time.Duration
}

func RestartBackoff(policy SupervisorBackoffPolicy, attempt int) time.Duration
func RestartAttempt(state map[string]any, key string) int
func ParseRestartTime(state map[string]any, key string) (time.Time, bool)
func ClearRestartState(state map[string]any, keys []string)
```

验收标准：

- signal runtime 现有自愈行为不回退。
- fatal 错误仍然 suppressed。
- 成功恢复到 RUNNING 后，restart 临时字段被清理。
- 至少覆盖一个 failure path 测试。

### 阶段 3：统一控制 API

目标：不同服务暴露一致的 runtime 控制接口。

验收标准：

- supervisor 不理解具体业务，也能读取状态和提交控制意图。
- 控制 API 有鉴权或网络隔离。
- control intent accepted 和 actualStatus converged 明确分离。

### 阶段 4：轻量 supervisor service

目标：新增独立 supervisor / control-plane service。

职责：

- 周期性检查服务 `/healthz`。
- 周期性读取 `/runtime/status`。
- 维护统一状态视图。
- 在业务安全条件满足时调用服务内部 `/runtime/restart`。
- 为未来前端 Runtime / Service 面板提供数据源。

当前只读骨架：

- `internal/service/runtime_supervisor.go` 提供 read-only collector。
- `BKTRADER_ROLE=api` 在显式配置 `SUPERVISOR_TARGETS` 时会启动 read-only supervisor collector，用于让现有 console 继续通过 platform-api 读取 `GET /api/v1/supervisor/status`。
- `BKTRADER_ROLE=supervisor` 只启动 read-only supervisor，不启动 live / signal / dashboard / notification 业务组件。
- 生产 compose 已提供独立 `supervisor` 服务：使用 `platform-api` 二进制、`BKTRADER_ROLE=supervisor`、`HTTP_ADDR=:8081`，默认 `SUPERVISOR_STANDALONE_TARGETS=api=http://platform-api:8080`。该服务可通过 `DEPLOY_SERVICES=supervisor` 单独发布，挂掉不会停止 platform-api / live-runner / signal-runtime-runner。
- `SUPERVISOR_TARGETS` 使用逗号分隔，支持 `name=http://host:port` 或直接填写 base URL。
- `SUPERVISOR_BEARER_TOKEN` 可选；设置后 read-only collector 会对所有 targets 的 `/healthz` 与 `/api/v1/runtime/status` 请求附加 `Authorization: Bearer <token>`，用于采集受鉴权保护的内网 runtime API。platform-api 会在 `/api/v1/runtime/status` 上接受该 token 作为只读 supervisor probe，不创建用户会话、不放行其他 API。当 target 指向其他容器、其他服务或非 loopback 地址时，应使用该 token 路径，不依赖 loopback 豁免。
- 生产 compose 默认将 platform-api 配为 `SUPERVISOR_TARGETS=api=http://127.0.0.1:8080`；该 loopback 自采集只读取 `/healthz` 和 `/api/v1/runtime/status`，不会调用控制 API。启用鉴权时，`/api/v1/runtime/status` 仅在 `SUPERVISOR_TARGETS` 非空且请求来自 loopback 时免 token，用于 platform-api 的 supervisor 自采集；未配置 supervisor targets 时，loopback 请求仍需正常鉴权。
- 独立 `supervisor` 服务跨容器采集 platform-api 时，请在生产 `.env` 配置强随机 `SUPERVISOR_BEARER_TOKEN`；prod compose 要求该变量非空，`scripts/deploy.sh` 会拒绝使用示例值发布生产环境。示例值只能用于本地开发。
- `SUPERVISOR_SERVICE_FAILURE_THRESHOLD=3` 为默认值；supervisor 会按 target 记录连续服务级失败次数，并在达到阈值后把该 target 标记为容器兜底候选，但当前阶段只暴露状态，不执行 Docker/container restart。
- `SUPERVISOR_CONTAINER_RESTART_ENABLED=false` 为默认值；未显式设为 `true` 时，容器兜底计划只会返回 `blockedReason=container-restart-disabled`，不会进入 executor 阶段。
- 默认只采集 `/healthz` 和 `/api/v1/runtime/status`，不调用任何控制 API。
- `GET /api/v1/supervisor/status` 返回最近一次 read-only supervisor 采集快照，并在顶层 `policy` 中暴露 `applicationRestartEnabled`、`serviceFailureThreshold`、`containerRestartEnabled`、`containerExecutorConfigured`。这些字段只反映当前 supervisor policy，不代表已执行或允许执行容器 restart。
- `bktrader-ctl runtime status --json` 和 `bktrader-ctl supervisor status --json` 提供 CLI 只读巡检入口；`bktrader-ctl supervisor status` 的人类可读输出会汇总 policy、target reachability、runtime attention、control action 数量，以及 container fallback 的 `decision`、`enabled` / `executorConfigured` / `executable` readiness、dry-run gates 和当前失败 episode 的 decision audit。
- `bktrader-ctl runtime start|stop|restart|suppress-auto-restart|resume-auto-restart` 是统一 runtime 控制入口，当前只支持 signal runtime；所有 mutating 命令都要求 `--confirm`，除普通 `restart` 外还要求 `--reason`，并支持 `--dry-run` 预览请求。
- `SUPERVISOR_APPLICATION_RESTART_ENABLED=false` 为默认值；只有显式设为 `true` 时，supervisor 才会对满足全部条件的 signal runtime 提交应用内 `POST /api/v1/runtime/restart`：
  - 目标服务 `/healthz` 可达且成功。
  - `/api/v1/runtime/status` 可读。
  - runtimeKind 为 `signal`；外部 target 可返回 `signal-runtime` 作为兼容 alias，语义等同 `signal`，不代表新增 runtime class。
  - `desiredStatus=RUNNING` 且 `actualStatus=ERROR`。
  - `autoRestartSuppressed=false` 且 `restartSeverity` 不是 `fatal`。
  - `nextRestartAt` 存在且已经到期。
  - 提交 restart 时固定 `force=false`、`confirm=true`，并带上 supervisor reason；同一个 target/runtime/`nextRestartAt` 只提交一次。
- 当 runtime 进入 restart 关注范围（例如 `actualStatus=ERROR`、存在 `nextRestartAt`、fatal/suppressed）时，supervisor 会在对应 runtime 上附加只读 `applicationRestartPlan`，显式返回 `decision=blocked|eligible`、`enabled`、`healthzOk`、`supported`、`due`、`duplicate`、`blockedReason` / `eligibleReason`。该计划用于解释为什么某个 runtime 会或不会进入应用内 restart；当前 `supported=true` 仍只覆盖 `signal` 及其兼容 alias `signal-runtime`，因此 `live-session` 等 runtime 即使 ERROR 也只会显示 `blockedReason=runtime-restart-unsupported-kind`，不会被 supervisor 自动拉起。

验收标准：

- supervisor 独立运行，不影响现有服务启动。
- supervisor 挂掉不会导致业务服务停止。
- 初期只调用应用内控制 API，不直接操作 Docker。

### 阶段 5：容器级兜底恢复

目标：服务完全不可访问或应用内恢复无法执行时，才进入容器级恢复。

触发条件必须保守：

- `/healthz` 连续 N 次失败。
- `/runtime/status` 连续超时或连接失败。
- 容器状态为 exited / unhealthy。
- 应用内 restart 无法提交。

验收标准：

- 只有服务级健康失败才允许容器 restart。
- 容器 restart 有 backoff、日志和人工抑制机制。
- `desiredStatus=STOPPED`、fatal suppressed 和人工 stop 都不会被容器级 supervisor 反复拉起。
- Docker socket 或 node-agent 权限边界有单独安全审查。

当前只读候选状态：

- `GET /api/v1/supervisor/status` 的每个 target 会返回 `serviceState`，包含连续失败次数、失败阈值、最近失败/恢复时间、是否已成为 `containerFallbackCandidate`，以及 dry-run decision audit：`containerFallbackSuppressed`、`containerFallbackBackoffUntil`、`containerFallbackAttemptCount`、`lastContainerFallbackDecisionAt`、`lastContainerFallbackDecisionReason`。当前 `containerFallbackAttemptCount` 表示当前连续失败 episode 内进入容器兜底决策层的次数，健康恢复后清零；它不是 Docker restart 执行次数。
- 当 target 已成为容器兜底候选时，状态中会额外返回 `containerFallbackPlan`；当前 `action=container-restart` 只是计划语义。该计划显式返回 `decision=blocked|eligible`、`enabled`、`executorConfigured`、`executorKind`、`executorDryRun`、`executable` 以及 dry-run gates：`suppressed`、`backoffActive`、`safetyGateOk`。当前完整执行语义被固定为 `candidate && enabled && executorConfigured && !suppressed && !backoffActive && safetyGateOk`；默认 `enabled=false`、`executorConfigured=false`、`executorKind=none`、`executorDryRun=true`、`executable=false`、`decision=blocked`，并返回 `blockedReason=container-restart-disabled`；即使 `SUPERVISOR_CONTAINER_RESTART_ENABLED=true`，在 executor 尚未配置前也只会返回 `enabled=true`、`executorConfigured=false`、`executorKind=none`、`executorDryRun=true`、`executable=false`、`decision=blocked`、`blockedReason=container-executor-not-configured`，用于明确“候选”不等于“已允许执行”。
- `internal/service/runtime_supervisor.go` 已定义 `ContainerFallbackExecutor` 接口和 `NoopContainerFallbackExecutor`，用于先跑通 executor readiness 与 decision/audit plumbing。executor 必须声明 descriptor，并通过 `kind`/`dryRun` 区分 noop dry-run 与未来真实执行器；`GET /api/v1/supervisor/status` 的 policy 同步返回 `containerExecutorKind`、`containerExecutorDryRun`。默认 supervisor options 不配置 executor，因此生产路径仍返回 `containerExecutorConfigured=false`、`containerExecutorKind=none`、`containerExecutorDryRun=true`；当前 app/config/env 也没有把 noop executor 接到部署配置中。noop executor 的 `Restart` 只返回 `executed=false`，不触碰 Docker API、不访问 docker.sock、不执行容器 restart。
- 当前 service fallback 只把 `/healthz` 不可达或非 2xx、`/api/v1/runtime/status` 连接不可达视为服务级失败；`/runtime/status` JSON decode 失败不会触发容器兜底候选，避免把业务状态或响应格式问题误判成需要重启容器。
- 达到 `SUPERVISOR_SERVICE_FAILURE_THRESHOLD` 后只记录 `containerFallbackCandidate=true` 和原因，不调用 Docker API，不挂载 Docker socket，不执行容器 restart。
- 后续真正执行容器级 restart 前，仍需单独设计 executor、backoff、人工抑制、权限边界和部署安全审查。

### Dashboard 视图

前端 console 的 Runtime Supervisor 页面读取 `GET /api/v1/supervisor/status`，展示 supervisor policy、service target、runtime 状态、`applicationRestartPlan`、应用内控制动作、`containerFallbackCandidate`、fallback `decision`、executor `kind`/`dryRun` 和 dry-run audit 摘要。

当前页面只开放最小的手动应用内控制入口：对 `runtimeKind=signal` / `signal-runtime` 的 runtime，可在显式确认并填写 reason 后调用现有 `POST /api/v1/runtime/start`、`POST /api/v1/runtime/stop`、`POST /api/v1/runtime/restart`、`POST /api/v1/runtime/suppress-auto-restart`、`POST /api/v1/runtime/resume-auto-restart`。start/stop/restart 入口固定 `confirm=true`，其中 stop/restart 固定 `force=false`，保留 active position/order 防护；suppress/resume 只切换 signal runtime 自动恢复抑制状态并写审计 reason。该页面不支持 live-session start/stop/restart，不触碰 Docker/container fallback。真正执行容器级 restart 前仍需单独 PR 设计 executor、权限边界和部署安全审查。

## 7. 安全边界

必须遵守：

- 缓存态不等于事实源；runtime status 不能替代交易所 REST 对账。
- 未完成对账不得 auto-dispatch、不得被动平仓自动提交、不得推进正常策略计划。
- WebSocket 恢复不等于事实恢复；重连后仍需 REST 级校验或 continuity 校验。
- final submit 前必须重新校验订单类型、reduce-only、positionSide 和当前状态是否允许执行。
- 人工 stop 后不得自动拉起。
- fatal 错误必须进入 suppressed。
- 容器级 restart 是兜底，不是业务恢复主路径。

## 8. 非目标

本 issue 不要求一次性完成：

- Kubernetes 化。
- 分布式调度。
- 把所有业务重新合并到单进程。
- 一次性重构所有 runtime。
- 复杂 DAG 编排。
- 容器级 Docker API 控制。
- Dashboard 全量实现。

## 9. 后续 PR 拆分建议

1. 文档 PR：新增本文件并接入文档导航。
2. Helper PR：抽出 backoff / attempt / time parse / clear state helper。
3. Status API PR：统一 `/healthz` 与 `/runtime/status` 返回结构。
4. Read-only Supervisor PR：只读采集状态，不执行恢复。
5. Application Restart PR：调用服务内部 `/runtime/restart` 执行业务级恢复。
6. Docker Fallback PR：在严格条件下加入容器级兜底。
7. Dashboard PR：增加统一运行态视图。

## 10. 开发检查表

任何后续改动开始前，先回答：

- 本 PR 属于哪个阶段？
- 本 PR 是否只解决一个问题域？
- 是否修改了 `internal/service/live*.go` 或 `execution_strategy.go`？
- 是否会改变默认 dispatch / auto-restart 行为？
- `desiredStatus=STOPPED` 是否仍然阻断自动恢复？
- fatal 错误是否仍然 suppressed？
- 成功恢复后 restart 临时字段是否清理？
- 新逻辑是否至少覆盖一个 failure path？
- runtime status 是否仍然只是缓存态 / 推导态，而不是交易事实源？
