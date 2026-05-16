# Runtime Supervisor 生产 executor 部署与权限模型

本文档对应 issue #419，是真实生产 container fallback executor 的设计/安全模型 PR。本文档只定义边界与分阶段方案，不落地 executor 代码，不修改 `deployments/`，不改变任何默认运行行为。

## 结论

优先采用 **node-agent executor**，Docker socket executor 只保留为高风险备选。

原因：

- Docker socket 等价于把宿主机 Docker 管理权限交给 supervisor 容器，权限边界过粗。
- node-agent 可以把 Docker 权限收敛在本机最小进程内，只开放固定 allowlist 动作。
- control-plane 不需要挂载 docker.sock，也不需要接收或拼接容器名、compose service 或 shell command。
- node-agent 的鉴权、allowlist、审计和回滚可以独立演进，适合先灰度到单机/单 target。

## 当前前提

当前 Runtime Supervisor 已具备策略层和审计层基础能力：

- service failure candidate / `containerFallbackPlan`
- `noop` dry-run executor
- allowlisted `command` executor 骨架
- `armed` gate
- manual submit
- auto-submit policy
- suppress / backoff / duplicate gate
- executor result audit

但生产默认仍必须保持安全状态：

- `SUPERVISOR_CONTAINER_RESTART_ENABLED=false`
- 不配置真实 executor
- 不 auto-submit
- 不挂载 Docker socket
- Dashboard fallback submit 默认不授予

## 非目标

本阶段不做以下事项：

- 不实现 Docker socket executor。
- 不实现 node-agent 进程。
- 不修改 `deployments/`、compose、CI/CD 或生产 env 模板。
- 不启用真实 container restart。
- 不改变 `dispatchMode`、testnet/mainnet、live runner、交易执行语义。
- 不扩展更多 runtime 类型。

## 分阶段 PR

### PR1：设计/安全模型

范围：

- 写清 node-agent 与 Docker socket 两种方案的权限边界。
- 定义 node-agent API、鉴权、allowlist、审计字段、失败/backoff 语义。
- 定义后续 executor 实现的测试矩阵。
- 明确 deployments 接入必须另起 PR。

验收：

- 文档能作为 PR2/PR3 的开发合同。
- 没有代码行为变化。
- 没有生产配置变化。

### PR2：node-agent executor 代码实现

范围：

- 新增 `node-agent` executor kind。
- supervisor 只向 node-agent 提交固定 target action，不传动态命令。
- executor 使用当前 `containerFallbackPlan` 的 gate 结果，不绕过现有 disabled / not armed / allowlist / suppressed / backoff / duplicate / safety gate。
- 实现结构化响应和 `containerFallbackActions` 审计。

不包含：

- 不修改生产 compose。
- 不默认启用 node-agent。
- 不 auto-submit。

最低测试：

- disabled
- executor missing
- not armed
- allowlist miss
- suppressed
- backoff active
- duplicate
- empty reason
- node-agent 401/403
- node-agent timeout
- node-agent non-2xx / failure result
- success audit

### PR3：部署接入与生产启用文档

范围：

- 只在 PR2 合并后处理。
- 新增 node-agent 的部署说明、env、Mac 本机进程/launchd 与 Linux systemd 接入方案。
- 写清生产启用、回滚、权限检查和 smoke test。
- 若必须选择 Docker socket，则必须单独 L3 PR，并声明 socket 权限风险。

默认：

- 仍不启用真实 executor。
- 仍不启用 auto-submit。
- 仍需人工显式设置 allowlist 和 armed gate。

## 推荐方案：node-agent executor

### 进程边界

node-agent 是每台宿主机本地的最小执行面。它负责把固定 target action 转换成本机 Docker/compose 操作。

推荐部署形态：

- 当前 Mac/Docker Desktop 场景下，首选 **宿主机本机 node-agent 进程**，由 launchd、手工守护脚本或开发期命令启动；它仍然通过本机 Docker CLI / compose 调 Docker Desktop，不绕开 Docker 依赖。
- Linux 生产场景下，首选 **独立二进制 + systemd unit**；它同样通过本机 Docker CLI / compose 执行固定 allowlist 动作。
- 独立容器或 sidecar 只能作为备选，因为 node-agent 的职责是在 Docker/compose 异常时触发恢复动作；若它本身也依赖同一套 Docker runtime，故障相关性更高。
- 不建议把 node-agent 嵌进 platform-api、live-runner 或 supervisor 进程；真实 Docker 操作权限应隔离在本机最小执行面内。

职责：

- 只监听本机或受控内网地址。
- 只接受 supervisor 的鉴权请求。
- 只执行配置文件中 allowlist 的固定 action。
- 只返回结构化执行结果。
- 只记录本机执行审计。

禁止：

- 禁止接受 shell command。
- 禁止接受动态 command path 或 args。
- 禁止从 HTTP body 接收容器名、compose service、project name。
- 禁止把 reason、target URL、Dashboard 字段、CLI 参数拼进命令。
- 禁止提供通用 Docker API proxy。

### 调用链

```text
Dashboard / bktrader-ctl
  -> platform-api supervisor submit endpoint
  -> RuntimeSupervisor containerFallbackPlan gate
  -> node-agent executor
  -> local fixed allowlist action
  -> structured result
  -> containerFallbackActions audit
```

Dashboard 和 CLI 只提交 `targetName`、`confirm=true`、`reason`。真实执行细节只能来自启动配置和 node-agent 本地 allowlist。

### node-agent API 草案

只定义一个 health API 和一个最小 mutating API。

Health API：

```http
GET /v1/health
Authorization: Bearer <agent-token>
```

响应体：

```json
{
  "status": "ok",
  "version": "dev",
  "executorKind": "node-agent",
  "tokenConfigured": true,
  "allowlistedTargets": ["api"],
  "checkedAt": "2026-05-16T03:01:30Z"
}
```

约束：

- health endpoint 也必须鉴权；不能对未授权调用暴露 allowlist。
- `tokenConfigured` 只表示 agent 启动时已加载 token，不返回 token 内容、长度、hash 或来源路径。
- `allowlistedTargets` 只返回 target name，不返回固定命令、service 列表或本机路径。
- PR3 smoke test 使用该 endpoint 确认 agent 可达、鉴权可用、allowlist 目标符合预期。

Mutating API：

```http
POST /v1/container-fallback/restart
Authorization: Bearer <agent-token>
Content-Type: application/json
```

请求体：

```json
{
  "requestId": "supervisor-generated-id",
  "targetName": "api",
  "action": "container-restart",
  "reason": "operator reviewed static restart plan",
  "planReason": "service probes failed 3/3",
  "episodeStartedAt": "2026-05-16T03:00:00Z",
  "candidateSince": "2026-05-16T03:01:00Z",
  "source": "dashboard",
  "operator": "folgercn"
}
```

字段约束：

- `targetName` 必须命中 supervisor target allowlist 和 node-agent 本地 allowlist。
- `action` 第一阶段只允许 `container-restart`。
- `reason` 必须非空，只用于审计，不参与命令构造。
- `source` 只能是 `dashboard`、`ctl`、`api`、`supervisor`。
- `operator` 只能来自后端 auth context，不能由 Dashboard/CLI 请求体伪造。

响应体：

```json
{
  "requestId": "supervisor-generated-id",
  "targetName": "api",
  "action": "container-restart",
  "executorKind": "node-agent",
  "executed": true,
  "exitCode": 0,
  "timedOut": false,
  "message": "compose restart accepted",
  "error": "",
  "startedAt": "2026-05-16T03:02:00Z",
  "finishedAt": "2026-05-16T03:02:03Z",
  "durationMs": 3000
}
```

失败响应也必须结构化；supervisor 收到网络错误、超时、非 2xx 或 `executed=false` + `error` 时，都必须写入 action audit 并进入 backoff。

### node-agent allowlist

node-agent 本地配置以 supervisor target name 为 key：

```json
{
  "targets": {
    "api": {
      "action": "container-restart",
      "executor": "docker-compose",
      "projectDirectory": "/opt/bktrader",
      "composeFiles": ["deployments/docker-compose.prod.yml"],
      "services": ["platform-api"],
      "timeoutSeconds": 30
    }
  }
}
```

约束：

- key 必须和 `SUPERVISOR_TARGETS` 的 target name 一致。
- `services` 必须是静态数组，不得来自请求。
- `projectDirectory` 和 compose 文件必须是本机固定配置。
- 第一阶段只允许 restart 单个或固定少量 service，不提供 scale、exec、logs、pull、up、down。
- node-agent 启动时必须校验重复 key、空 service、相对路径、超大 timeout。

### 鉴权

第一阶段使用 bearer token，后续可升级 mTLS。

要求：

- supervisor 到 node-agent 使用独立 token，不复用 `SUPERVISOR_BEARER_TOKEN`。
- token 必须来自环境或本机只读 secret 文件。
- node-agent 拒绝空 token、示例 token、过短 token。
- 鉴权失败返回 401/403，并写本机审计。
- supervisor 对 401/403 视为 executor failure，写 action audit 并进入 backoff。

### 审计

supervisor `containerFallbackActions` 至少保留：

- target name/base URL
- action
- request id
- reason
- plan reason
- source
- operator
- service failure episode start
- candidate since
- executor kind
- executor preview
- submitted/executed
- status code
- exit code
- timed out
- message/error
- backoff until
- requested/finished time

node-agent 本机审计至少保留：

- request id
- remote address
- authenticated principal
- target name
- action
- allowlist decision
- fixed service list
- command result
- duration
- error

禁止把 bearer token、完整 Authorization header、生产密钥写入审计。

### backoff 与重试

必须沿用 supervisor 当前 gate：

- 同一个 service failure episode 默认只提交一次。
- executor failure 后写 `containerFallbackBackoffUntil`。
- backoff 未清理前，plan 返回 `blockedReason=container-fallback-backoff-active`。
- 健康恢复后清理当前 episode 的 candidate/submitted/backoff 临时状态。
- operator 可通过 `clear-backoff` 显式清理 retry gate 后再次人工 submit。

自动提交仍必须额外要求：

- `SUPERVISOR_CONTAINER_FALLBACK_AUTO_SUBMIT=true`
- `SUPERVISOR_CONTAINER_RESTART_ENABLED=true`
- executor configured
- executor armed
- target allowlisted
- plan executable

PR2 初期建议只实现 manual submit；auto-submit 可在同 PR 测试覆盖完整后保留关闭默认，也可以另拆 follow-up。

## Docker socket 备选方案

Docker socket 方案只有在明确接受 L3 风险后才能进入实现 PR。

风险：

- 挂载 `/var/run/docker.sock` 基本等价宿主机 root 级控制能力。
- supervisor 容器一旦被打穿，可控制同宿主机其他容器。
- compose service allowlist 只能降低误操作风险，不能显著降低 socket 泄露风险。

若未来必须采用 Docker socket：

- 必须单独 PR 修改 `deployments/`。
- 必须在 PR 描述中声明 L3 风险。
- 必须只挂载到独立 supervisor 容器，不挂到 platform-api/live-runner。
- 必须保留默认不挂载。
- 必须使用固定 allowlist，不接受动态 container/service/command。
- 必须写清回滚：移除 env、移除 socket mount、重启 supervisor、确认 `containerExecutorConfigured=false`。

## 配置草案

PR2 可按以下配置命名落地；PR1 不新增这些 env：

```text
SUPERVISOR_CONTAINER_EXECUTOR=node-agent
SUPERVISOR_CONTAINER_EXECUTOR_ARMED=false
SUPERVISOR_CONTAINER_FALLBACK_AUTO_SUBMIT=false
SUPERVISOR_NODE_AGENT_BASE_URL=http://127.0.0.1:18081
SUPERVISOR_NODE_AGENT_TOKEN_FILE=/run/secrets/bktrader-supervisor-node-agent-token
SUPERVISOR_NODE_AGENT_TIMEOUT_SECONDS=30
```

保持不变量：

- 未设置 executor 时，`containerExecutorConfigured=false`。
- `ARMED=false` 时，非 dry-run executor 不能执行。
- `AUTO_SUBMIT=false` 时，只允许人工 submit。
- Dashboard submit 权限仍由 #420 的 capability model 控制。

## PR2 开发合同

实现 PR 必须满足：

- 新 executor 不改变现有 `noop` 和 `command` 行为。
- 所有动态输入只能用于审计，不能参与命令构造。
- `targetName` 同时通过 supervisor allowlist 和 node-agent allowlist。
- 提交前重新评估 plan，不能使用旧 snapshot 的缓存态绕过 gate。
- 成功/失败路径都写 action audit。
- 失败不能伪装成功。
- 超时、鉴权失败、非 2xx、结构化 error 都进入 backoff。
- 测试至少覆盖 #419 issue 中列出的 failure path。

## PR3 部署合同

部署 PR 必须包含：

- 生产启用步骤。
- 回滚步骤。
- 权限检查步骤。
- 本机 token/secret 生成方式。
- Mac 当前部署以宿主机本机进程/launchd 为主，明确依赖 Docker Desktop / Docker CLI / compose。
- Linux 生产部署以 systemd unit 为主，明确依赖本机 Docker CLI / compose。
- compose/sidecar 只能作为明确说明故障相关性的备选。
- node-agent 日志位置。
- supervisor 状态检查命令。
- Dashboard/CLI 人工 submit 验证步骤。

建议生产启用顺序：

1. 部署 node-agent，但不配置 supervisor executor。
2. 只读检查 node-agent health。
3. 配置 supervisor executor，但保持 `ARMED=false`。
4. 确认 `/api/v1/supervisor/status` preview 与 allowlist 正确。
5. 设置 `ARMED=true`，仍保持 `AUTO_SUBMIT=false`。
6. 用人工 submit 做单 target 验证。
7. 观察审计和 backoff 行为。
8. 另行评估是否允许 auto-submit。
