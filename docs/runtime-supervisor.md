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
| `autoRestartSuppressed` | `autoRestartSuppressed` | 是否抑制自动恢复。 |

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
  "autoRestartSuppressed": false,
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
- `BKTRADER_ROLE=supervisor` 只启动 read-only supervisor，不启动 live / signal / dashboard / notification 业务组件。
- `SUPERVISOR_TARGETS` 使用逗号分隔，支持 `name=http://host:port` 或直接填写 base URL。
- 当前只采集 `/healthz` 和 `/api/v1/runtime/status`，不调用任何控制 API。
- `GET /api/v1/supervisor/status` 返回最近一次 read-only supervisor 采集快照。

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
