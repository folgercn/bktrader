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
- 当前统一控制 API 已落最小切片：`POST /api/v1/runtime/start`、`POST /api/v1/runtime/stop` 支持 `runtimeKind=signal` / `signal-runtime`、`runtimeKind=live-session` 以及 `runtimeKind=paper-session`；`POST /api/v1/runtime/restart`、`POST /api/v1/runtime/suppress-auto-restart`、`POST /api/v1/runtime/resume-auto-restart` 仍只支持 `runtimeKind=signal` / `signal-runtime`。请求必须显式传入 `confirm=true`；`start` / `stop` / `suppress-auto-restart` / `resume-auto-restart` 始终要求非空 `reason` 并写入 runtime state 审计字段，`restart` 在 `force=true` 时必须传入非空 `reason`。`live-session` start/stop 只复用既有 live control intent，不做 live session restart，不做 live session 自动 dispatch，也不做 Docker/container restart；`paper-session` start/stop 复用既有 paper runner 启停，并写入 `desiredStatus` / `actualStatus` 与 start/stop 审计字段。

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
- `SUPERVISOR_TARGETS` 使用逗号分隔，支持 `name=http://host:port` 或直接填写 `http(s)` base URL，最多 32 个 target；显式 `name=` 形式必须有非空 name、非空 base URL，且 name 必须唯一；name 只能包含 ASCII 字母、数字、`.`、`_`、`-`；所有 base URL 必须使用 `http://` 或 `https://` 并包含 host，不能包含 userinfo 凭证、query 或 fragment，且按 scheme/host/path 归一化后不能重复，避免后续按 `targetName` 执行 suppress/backoff 控制时出现 allowlist 歧义、静默跳过 target、重复 probe / restart，或把拼写错误延迟成运行期 probe 失败。
- `SUPERVISOR_BEARER_TOKEN` 可选；设置后 read-only collector 会对所有 targets 的 `/healthz` 与 `/api/v1/runtime/status` 请求附加 `Authorization: Bearer <token>`，用于采集受鉴权保护的内网 runtime API。platform-api 会在 `/api/v1/runtime/status` 上接受该 token 作为只读 supervisor probe，不创建用户会话、不放行其他 API。当 target 指向其他容器、其他服务或非 loopback 地址时，应使用该 token 路径，不依赖 loopback 豁免。
- 生产 compose 默认将 platform-api 配为 `SUPERVISOR_TARGETS=api=http://127.0.0.1:8080`；该 loopback 自采集只读取 `/healthz` 和 `/api/v1/runtime/status`，不会调用控制 API。启用鉴权时，`/api/v1/runtime/status` 仅在 `SUPERVISOR_TARGETS` 非空且请求来自 loopback 时免 token，用于 platform-api 的 supervisor 自采集；未配置 supervisor targets 时，loopback 请求仍需正常鉴权。
- 独立 `supervisor` 服务跨容器采集 platform-api 时，请在生产 `.env` 配置强随机 `SUPERVISOR_BEARER_TOKEN`；若部署脚本发现该变量缺失，会生成强随机 token 写入 `.env` 并导出给本次 compose 调用。prod compose 要求该变量非空，`scripts/deploy.sh` 会拒绝使用示例值发布生产环境。示例值只能用于本地开发。
- `SUPERVISOR_SERVICE_FAILURE_THRESHOLD=3` 为默认值；supervisor 会按 target 记录连续服务级失败次数，并在达到阈值后把该 target 标记为容器兜底候选，但当前阶段只暴露状态，不执行 Docker/container restart。
- `SUPERVISOR_CONTAINER_RESTART_ENABLED=false` 为默认值；未显式设为 `true` 时，容器兜底计划只会返回 `blockedReason=container-restart-disabled`，不会进入 executor 阶段。
- `SUPERVISOR_CONTAINER_EXECUTOR` 默认为空，表示不配置容器 executor；支持值为 `noop`、`command` 与 `node-agent`。`noop` 只用于 dry-run executor readiness 与提交审计。`command` 是第一版真实 executor 骨架，必须同时设置 `SUPERVISOR_CONTAINER_EXECUTOR_ARMED=true` 和 `SUPERVISOR_CONTAINER_EXECUTOR_COMMANDS_JSON`，否则启动配置校验失败；命令配置是 target name 到固定命令 spec 的 JSON allowlist，例如 `{"api":{"path":"/usr/bin/docker","args":["compose","restart","platform-api"],"timeoutSeconds":30}}`。command allowlist 的 key 必须匹配 `SUPERVISOR_TARGETS` 的有效 target name，重复或拼错会在启动配置校验阶段失败，避免真实 executor 因 target typo 长期处于运行期 block。command executor 不走 shell、不接受 HTTP/CLI/Dashboard 动态传入容器名或命令、不默认启用；compose/deploy 只透传 env，不挂载 Docker socket、不改变任何默认权限，生产若要让 command 调 Docker 仍需单独审查 socket/权限边界。`node-agent` executor 通过 `SUPERVISOR_NODE_AGENT_BASE_URL` 调用宿主机本机 node-agent 的 `POST /v1/container-fallback/restart`，使用 `SUPERVISOR_NODE_AGENT_TOKEN` 或 `SUPERVISOR_NODE_AGENT_TOKEN_FILE` 做 Bearer 鉴权，并通过 `SUPERVISOR_NODE_AGENT_TIMEOUT_SECONDS` 控制 HTTP 超时；它不接收动态命令，只把 target/action/reason/plan/source/operator 审计上下文提交给 node-agent，本机 Docker CLI / compose allowlist 仍由 node-agent 自己持有。`node-agent` 可在 `SUPERVISOR_CONTAINER_EXECUTOR_ARMED=false` 时完成配置和 preview，但 plan 会以 `blockedReason=container-executor-not-armed` 阻断真实 submit。即使同时设置 `SUPERVISOR_CONTAINER_RESTART_ENABLED=true` 和 `SUPERVISOR_CONTAINER_EXECUTOR=noop`，默认也只会显示 `executorKind=noop` / `executorDryRun=true` 与可人工提交的 eligible 计划，不会后台自动写 `containerFallbackActions`；若还显式设置 `SUPERVISOR_CONTAINER_FALLBACK_AUTO_SUBMIT=true`，collector 才会在 eligible 时提交一次 noop 审计。noop 不会调用 Docker、不会访问 docker.sock、不会重启容器。
- `SUPERVISOR_CONTAINER_FALLBACK_AUTO_SUBMIT=false` 为默认值；未显式设为 `true` 时，后台 collector 只生成 `containerFallbackPlan`，不会自动调用 executor。人工 `submit-container-fallback` / Dashboard submit 不受 auto-submit 开关阻断，但仍受 Dashboard submit 权限和当前 plan `executable=true` 约束。plan 会同步返回 `autoSubmitEnabled`、`autoSubmitEligible`、`manualSubmitRequired`，避免 operator 只看到 `executable=true` 却无法判断后台是否会自动提交。若设置为 `true`，配置校验要求同时启用 `SUPERVISOR_CONTAINER_RESTART_ENABLED=true` 并配置 `SUPERVISOR_CONTAINER_EXECUTOR`，避免 auto-submit policy 变成静默 no-op。
- Dashboard 操作权限由 `SUPERVISOR_DASHBOARD_CAN_VIEW`、`SUPERVISOR_DASHBOARD_CAN_RUNTIME_CONTROL`、`SUPERVISOR_DASHBOARD_CAN_CONTAINER_FALLBACK_GATE`、`SUPERVISOR_DASHBOARD_CAN_CONTAINER_FALLBACK_SUBMIT` 控制；前三者默认 `true`，fallback submit 默认 `false`，需要显式打开后 Dashboard/API 才能提交 container fallback executor。`GET /api/v1/supervisor/status` 会在 `policy.dashboardPermissions` 返回按钮可用性和 blocked reason，前端按该策略禁用 runtime control、fallback gate 和 submit 操作。
- 默认只采集 `/healthz` 和 `/api/v1/runtime/status`，不调用任何控制 API。
- `GET /api/v1/supervisor/status` 返回最近一次 read-only supervisor 采集快照，并在顶层 `policy` 中暴露 `applicationRestartEnabled`、`serviceFailureThreshold`、`containerRestartEnabled`、`containerFallbackAutoSubmit`、`containerExecutorConfigured`。这些字段只反映当前 supervisor policy，不代表已执行或允许执行容器 restart。
- `bktrader-ctl runtime status --json` 和 `bktrader-ctl supervisor status --json` 提供 CLI 结构化只读巡检入口；非 JSON 输出中，`bktrader-ctl runtime status` 会汇总 runtime kind、attention、最近 lifecycle/auto-restart 控制审计和每个 runtime 的 restart 状态，`bktrader-ctl supervisor status` 会汇总 policy、target reachability、runtime attention、control action 数量、最近 container fallback gate 控制审计、最近 container fallback executor 提交审计，以及 container fallback 的 `decision`、`enabled` / `executorConfigured` / `executorArmed` / `targetAllowed` / `executable` readiness、`autoSubmitEnabled` / `autoSubmitEligible` / `manualSubmitRequired` 提交模式、dry-run gates、command executor 的静态命令 preview 和当前失败 episode 的 decision audit。Dashboard 的 Supervisor Policy 与 target fallback plan 同步显示真实 executor 的 `armed` 与 `target allowed` gate，并在 target 已进入 fallback plan 时展示该 target 对应的固定 command preview，避免只看到 executor ready 却看不见实际执行门禁和将要提交的静态命令。
- `bktrader-ctl supervisor suppress-container-fallback <targetName> --confirm --reason "<原因>"` / `resume-container-fallback` 以及 Dashboard Supervisor target 行上的 gate 操作提供人工抑制/恢复入口；该入口只按已配置的 target name 切换 supervisor 内存态 gate 和审计字段，不接受 URL、container name、compose service 或 shell command，不调用 Docker API。
- `bktrader-ctl supervisor defer-container-fallback <targetName> --seconds <N> --confirm --reason "<原因>"` / `clear-container-fallback-backoff` 以及 Dashboard Supervisor target 行上的 retry-gate 操作提供人工 backoff 设置/清理入口；该入口只影响 `containerFallbackBackoffUntil`、submitted/dedupe gate 和审计字段，不执行容器操作；`--seconds` / Dashboard backoff seconds 必须在 `1..86400` 之间，避免长期静默阻断容器兜底候选。
- `bktrader-ctl supervisor submit-container-fallback <targetName> --confirm --reason "<原因>"` 以及 Dashboard Supervisor target 行上的 submit 操作提供显式人工提交入口；该入口只会复用当前 target 的 `containerFallbackPlan` 和启动配置中的固定 executor/allowlist，要求计划已经 `executable=true`，并把人工 reason/source 与只读 plan reason 一起写入 `containerFallbackActions` 审计。它不接受 URL、container name、compose service、shell command 或 command args 作为请求输入；若当前 plan 被 disabled、missing executor、未 armed、allowlist miss、suppressed、backoff、duplicate 或 safety gate 阻断，API 返回 409，不会调用 executor。
- `bktrader-ctl runtime start|stop` 是统一 runtime 控制入口，支持 signal runtime 和 live-session runtime；`restart|suppress-auto-restart|resume-auto-restart` 当前仍只支持 signal runtime。所有 mutating 命令都要求 `--confirm`，除普通 `restart` 外还要求 `--reason`，并支持 `--dry-run` 预览请求。
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

- `GET /api/v1/supervisor/status` 的每个 target 会返回 `serviceState`，包含连续失败次数、失败阈值、最近失败/恢复时间、当前连续失败 episode 起点 `serviceFailureEpisodeStartedAt`、是否已成为 `containerFallbackCandidate`、候选起点 `containerFallbackCandidateSince`，以及 dry-run decision audit：`containerFallbackSuppressed`、`containerFallbackBackoffUntil`、`containerFallbackAttemptCount`、`containerFallbackSubmitted*`、`lastContainerFallbackDecisionAt`、`lastContainerFallbackDecisionReason`。当前 `containerFallbackAttemptCount` 表示当前连续失败 episode 内进入容器兜底决策层的次数，健康恢复后清零；它不是 Docker restart 执行次数。`containerFallbackSubmitted*` 表示当前连续失败 episode 内已经向 dry-run executor 提交过一次 fallback 动作，并记录提交原因、message 或 error；健康恢复会清理 failure episode、candidate since、submitted/dedupe 和 backoff 状态。
- `GET /api/v1/supervisor/status` 的顶层 `serviceFailureEpisodes` 保留最近 50 个已恢复的服务级失败 episode，只在 supervisor 进程内存中保存，不写 DB，进程重启后清空。每条 episode 记录 target、开始/恢复时间、持续秒数、最大连续失败次数、最近失败原因/时间，以及该 episode 内的 fallback candidate、attempt、submitted、error、backoff、last decision 摘要。该历史用于恢复后复盘为什么曾经进入容器兜底候选或 dry-run 提交，不代表当前 target 仍处于失败状态，也不会触发 Docker API、docker.sock 或容器 restart。
- `GET /api/v1/supervisor/status` 的顶层 `containerFallbackControls` 保留最近 50 条人工 fallback gate 控制审计，包含 `action`、`targetName`、`targetBaseUrl`、`suppressed`、`backoffUntil`、`backoffSeconds`、`reason`、`source`、`operator`、`updatedAt`。人工 suppress/resume/defer/clear-backoff 成功后，`LastSnapshot()` 会立即把 supervisor 内存态 gate 覆盖到最近快照，并同步返回最新 `containerFallbackControls`，Dashboard / CLI 刷新不需要等待下一轮 collector 才能看到 gate 状态变化。
- 当 target 已成为容器兜底候选时，状态中会额外返回 `containerFallbackPlan`；当前 `action=container-restart` 只是计划语义。该计划显式返回 `decision=blocked|eligible`、`enabled`、`executorConfigured`、`executorKind`、`executorDryRun`、`executorArmed`、`targetAllowed`、`executorPreview`、`executable`、`autoSubmitEnabled`、`autoSubmitEligible`、`manualSubmitRequired` 以及 gates：`duplicate`、`suppressed`、`backoffActive`、`safetyGateOk`。command executor 的 `executorPreview` 只来自启动配置中的固定 allowlist，包含 `kind`、`commandPath`、`commandArgs`、`timeoutSeconds`，不会读取 HTTP body、Dashboard 字段或 CLI 参数。plan 可执行语义被固定为 `candidate && enabled && executorConfigured && executorArmed && targetAllowed && !suppressed && !backoffActive && !duplicate && safetyGateOk`；collector 是否自动提交还必须额外满足顶层 policy 的 `containerFallbackAutoSubmit=true`，此时同一个 plan 会返回 `autoSubmitEligible=true`；若 plan 已可执行但 auto-submit policy 未启用，则返回 `manualSubmitRequired=true`，人工 submit 仍只要求当前 plan `executable=true`。dry-run executor 配置后 `executorArmed=true` 仅表示无需真实执行 armed gate。默认 `enabled=false`、`executorConfigured=false`、`executorKind=none`、`executorDryRun=true`、`executorArmed=false`、`executable=false`、`autoSubmitEnabled=false`、`autoSubmitEligible=false`、`manualSubmitRequired=false`、`decision=blocked`，并返回 `blockedReason=container-restart-disabled`；即使 `SUPERVISOR_CONTAINER_RESTART_ENABLED=true`，在 executor 尚未配置前也只会返回 `enabled=true`、`executorConfigured=false`、`executorKind=none`、`executorDryRun=true`、`executorArmed=false`、`executable=false`、`decision=blocked`、`blockedReason=container-executor-not-configured`。非 dry-run executor 未显式 armed 会返回 `blockedReason=container-executor-not-armed`，target 未命中 allowlist 会返回 `blockedReason=container-executor-target-not-allowlisted`。同一连续失败 episode 内 executor 已提交过后，后续计划会返回 `duplicate=true`、`blockedReason=container-fallback-already-submitted`，避免重复提交。
- `internal/service/runtime_supervisor.go` 已定义 `ContainerFallbackExecutor` 接口和 `NoopContainerFallbackExecutor`，用于先跑通 executor readiness 与 decision/audit plumbing；`runtime_supervisor_command_executor.go` 提供 allowlisted command executor 作为真实 executor 的第一段落地。executor 必须声明 descriptor，并通过 `kind`/`dryRun` 区分 noop dry-run 与真实执行器；`GET /api/v1/supervisor/status` 的 policy 同步返回 `containerFallbackAutoSubmit`、`containerExecutorKind`、`containerExecutorDryRun`、`containerExecutorArmed`。默认 supervisor options 不配置 executor，且 `containerFallbackAutoSubmit=false`，因此生产路径仍返回 `containerExecutorConfigured=false`、`containerExecutorKind=none`、`containerExecutorDryRun=true`；只有显式设置 `SUPERVISOR_CONTAINER_EXECUTOR=noop` 时才会接入 noop executor。noop executor 的 `Restart` 只返回 `executed=false`，不触碰 Docker API、不访问 docker.sock、不执行容器 restart。command executor 只有在 `SUPERVISOR_CONTAINER_RESTART_ENABLED=true`、executor 已配置、`containerExecutorArmed=true`、target 命中 allowlist、未 suppressed、backoff 未生效、未重复提交且 `safetyGateOk=true` 时才可被人工 submit 或被 `SUPERVISOR_CONTAINER_FALLBACK_AUTO_SUBMIT=true` 的 collector 提交固定命令；缺少 armed gate 返回 `blockedReason=container-executor-not-armed`，target 不在 allowlist 返回 `blockedReason=container-executor-target-not-allowlisted`。command executor 的 allowlist key 在配置校验和 executor 构造时都会按 trim 后的 target name 去重；配置 key 还必须命中 `SUPERVISOR_TARGETS` 的有效 target name。command executor 同时实现只读 preview 接口，supervisor 会在计划和提交审计中复制固定 `commandPath` / `commandArgs` / `timeoutSeconds`，便于 operator 审核将要执行或已经提交的命令；preview 只是状态输出，不改变执行条件。`GET /api/v1/supervisor/status` 顶层 `containerFallbackActions` 保留最近 50 条 executor 提交审计，包含 target、reason、source、planReason、当前 action 所属的 `serviceFailureEpisodeStartedAt` / `containerFallbackCandidateSince`、executor kind/dry-run、executor preview、submitted、executed、exitCode、timedOut、message/error、error backoff 和 requestedAt；同一个连续失败 episode 内只提交一次，健康恢复后才允许下一轮失败 episode 再次提交。若 command executor 正常退出，会返回 `exitCode=0`；若固定命令返回非零退出码，action 会保留 `exitCode`、截断后的 stdout/stderr `message` 和 error；若命令超时，action 会返回 `timedOut=true` 并尽量保留超时前已捕获输出。若 executor 返回 error，supervisor 会写入 `containerFallbackBackoffUntil`、`containerFallbackBackoffReason="container fallback executor error: ..."`、`containerFallbackBackoffSource=supervisor`，并在 action 上返回 `backoffUntil` / `backoffSeconds=300`；后续 collect 先以 `blockedReason=container-fallback-backoff-active` 阻断。若 executor 已提交但 target 仍未恢复，人工 `clear-backoff` 会同时清理 backoff 与 submitted/dedupe 状态，允许 operator 在同一 failure episode 内审核后重试。
- 当前 service fallback 只把 `/healthz` 不可达或非 2xx、`/api/v1/runtime/status` 连接不可达视为服务级失败；`/runtime/status` JSON decode 失败不会触发容器兜底候选，避免把业务状态或响应格式问题误判成需要重启容器。
- `POST /api/v1/supervisor/container-fallback/suppress` 和 `resume` 是人工 gate，只接受 `targetName`、`confirm=true`、非空 `reason`，并写入 `containerFallbackSuppressed*` / `containerFallbackResumed*` 审计字段；Dashboard target 行的 suppress/resume 操作调用同一 API。它不依赖 `SUPERVISOR_CONTAINER_RESTART_ENABLED`，也不代表容器 restart 被授权执行。若存在重复 target name，请先修正 `SUPERVISOR_TARGETS`，API 会拒绝 ambiguous target。
- `POST /api/v1/supervisor/container-fallback/defer` 和 `clear-backoff` 是人工 retry-gate 控制，只接受 `targetName`、`confirm=true`、非空 `reason`，其中 `defer` 还要求 `backoffSeconds` 在 `1..86400` 之间；Dashboard target 行的 defer/clear-backoff 操作调用同一 API。`defer` 只写入 `containerFallbackBackoffUntil` 和 backoff 审计字段，健康恢复会清理当前 failure episode 的 backoff 状态。`clear-backoff` 会清理当前 failure episode 的 `containerFallbackBackoffUntil`、`containerFallbackSubmitted*` 和 submitted dedupe map，因此不只用于 executor error 后审核重试，也用于 executor 已提交但服务仍未恢复时，由 operator 显式清 gate 后在同一 failure episode 内再次人工 submit。
- `POST /api/v1/supervisor/container-fallback/submit` 是人工 executor submit gate，只接受 `targetName`、`confirm=true`、非空 `reason`。它在提交前重新评估当前 `containerFallbackPlan`，只有 `executable=true` 时才调用 executor；成功后 action `reason` 为人工 reason，`planReason` 保留服务失败/eligible 依据，`source` 标记 API/CLI/Dashboard 来源。
- 达到 `SUPERVISOR_SERVICE_FAILURE_THRESHOLD` 后只记录 `containerFallbackCandidate=true` 和原因；在显式开启 `SUPERVISOR_CONTAINER_RESTART_ENABLED=true` 且配置 noop dry-run executor 时，默认仍需要人工 submit 才会写 `containerFallbackActions` 审计。只有再显式设置 `SUPERVISOR_CONTAINER_FALLBACK_AUTO_SUBMIT=true` 时，collector 才会自动提交一次 noop executor 审计，但仍不调用 Docker API、不挂载 Docker socket、不执行容器 restart。
- 后续真正执行容器级 restart 前，仍需单独设计 executor、backoff、人工抑制、权限边界和部署安全审查。

真实 executor 的安全合同：

- Executor 必须保持 allowlist 语义，只能操作显式配置的 supervisor target 对应容器；不得接受任意容器名、镜像名、compose service 名或 shell command 作为 runtime 输入。
- Executor 入参只能来自 supervisor 已知 target、当前 service failure episode、只读 decision plan，以及显式人工 reason/source 审计；HTTP request body、Dashboard 字段或 CLI 参数不得直接穿透为容器名、compose service、shell command、command path 或 command args。
- `SUPERVISOR_CONTAINER_RESTART_ENABLED=true` 只是打开策略层；仍必须同时满足 executor 已配置、target 已成为候选、未被人工抑制、backoff 未生效、safety gate 通过，才允许形成可提交计划。后台自动进入真实 executor 还必须显式设置 `SUPERVISOR_CONTAINER_FALLBACK_AUTO_SUBMIT=true`；否则只能由人工 submit 入口触发。
- 非 dry-run executor 还必须满足 `containerExecutorArmed=true` 和 `targetAllowed=true`。command executor 的 allowlist 以 supervisor target name 为 key，必须和 `SUPERVISOR_TARGETS` 对齐；命令 path 必须为绝对路径，args 为静态数组，执行时不经过 shell，也不会把 reason、target URL、HTTP request body、Dashboard 字段或 CLI 参数拼进命令。不要把密钥、token 或其他敏感信息放入 command args；这些固定 args 会在受鉴权保护的 supervisor 状态和审计中作为 preview 返回。
- 真实 executor 必须在执行前后写结构化审计日志，至少包含 target、executor kind、decision reason、episode attempt、operator suppression state、backoff_until、executed、error；失败不能伪装成成功。
- 容器 restart 执行次数必须与当前 `containerFallbackAttemptCount` 区分：前者表示真实 executor 调用，后者仍只表示本轮连续失败 episode 进入决策层的次数。
- 任意真实执行失败后必须进入 backoff；健康恢复后才能清理当前 episode 的临时状态。反复失败不得形成无间隔 restart loop。
- 人工 stop、fatal suppressed、`desiredStatus=STOPPED`、target 未在 allowlist、reason 为空、最近一次真实执行仍在 backoff 内时，必须硬性阻断真实 executor。
- Docker socket 方案必须单独 PR 修改 `deployments/`，并在 PR 中声明 L2/L3 风险、socket 权限、网络暴露边界和回滚方式；node-agent 方案必须先定义 agent 鉴权、target allowlist 和审计格式。
- Dashboard 的 container fallback submit 按钮只能在当前计划 `executable=true` 时启用，并在确认弹窗中展示 plan decision、executor kind/dry-run 和固定 command preview；提交必须带 reason，成功/失败都进入 `containerFallbackActions` 审计，不得直接接收或拼接容器名、compose service、shell command、command path 或 command args。
- 第一个真实 executor PR 应优先实现 dry-run parity 测试和 blocked-path 测试，再考虑执行路径；测试至少覆盖 disabled、executor missing、suppressed、backoff active、allowlist miss、empty reason、executor failure。

### Dashboard 视图

前端 console 的 Runtime Supervisor 页面读取 `GET /api/v1/supervisor/status`，展示 supervisor policy、service target、runtime 状态、`applicationRestartPlan`、应用内控制动作、已恢复服务失败 episode 历史、container fallback gate 控制审计、container fallback dry-run 提交审计、`containerFallbackCandidate`、fallback `decision`、`containerFallbackAutoSubmit`、plan 级 `autoSubmitEligible` / `manualSubmitRequired`、executor `kind`/`dryRun` 和 dry-run audit 摘要。

当前页面开放两类人工控制入口。第一类是最小的应用内 runtime 控制：对 `runtimeKind=signal` / `signal-runtime` 的 runtime，可在显式确认并填写 reason 后调用现有 `POST /api/v1/runtime/start`、`POST /api/v1/runtime/stop`、`POST /api/v1/runtime/restart`、`POST /api/v1/runtime/suppress-auto-restart`、`POST /api/v1/runtime/resume-auto-restart`。start/stop/restart 入口固定 `confirm=true`，其中 stop/restart 固定 `force=false`，保留 active position/order 防护；suppress/resume 只切换 signal runtime 自动恢复抑制状态并写审计 reason。第二类是 target 行上的 container fallback 控制：suppress/resume/defer/clear-backoff 只写 supervisor 内存态 gate、reason/source/time 审计和最近 `containerFallbackControls` 记录，不调用 Docker API、不访问 docker.sock、不执行容器 restart；clear-backoff 在存在 active backoff、已提交或 duplicate gate 时开放，用于显式清理当前 failure episode 的 retry gate；submit 只在 plan executable 时开放，弹窗展示当前 decision/executor/preview，提交后复用固定 executor 并写入 `containerFallbackActions`。该页面不支持 live-session start/stop/restart；Docker socket 或 node-agent 权限边界仍必须单独 PR 审查。

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
6. Container Fallback Readiness PR：容器兜底候选、计划、noop executor readiness 和安全合同。
7. Docker Executor Design PR：确定真实 executor 的 allowlist、权限边界、backoff、审计、dry-run parity 和回滚策略。
8. Docker Executor PR：在严格条件和单独安全审查后加入容器级执行器。
9. Dashboard PR：增加统一运行态视图；容器操作入口需单独确认/审计 PR。

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
