# 2026-04-22 Binance REST 统一限流与 Live Account Sync 风暴治理计划

## 0. 实施状态

截至 2026-04-22，本专项已经按串行 PR 完成后端治理主线，并补了一版前端降载：

- PR #135 `codex/binance-rest-unified-rate-limit`
  - 已完成 Binance REST 统一 gate 收口。
  - signed REST、`exchangeInfo`、`klines` 等 Binance 请求统一经过 requester / limiter。
  - 共享 `429` / `Retry-After` backoff，避免单条链路绕过限流。
- PR #136 `codex/live-account-sync-coalescing`
  - 已完成同账户 `SyncLiveAccount` 并发合并。
  - 同一账户 in-flight sync 会复用同一次底层执行结果。
  - 增加 recent-result reuse 窗口，压制短时间串行重复 sync。
  - `authoritative-reconcile`、startup recovery、filled / settlement 等必须刷新的触发会绕过普通 recent reuse，但仍不并发轰炸。
- PR #137 `codex/live-sync-and-stale-observability`
  - 已补 live account sync 调度观测字段。
  - 已补 runtime source gate 的 stale / missing 明细日志。
  - `stale-source-states` 后续可以直接从日志定位具体 `sourceKey / streamType / symbol / lastEventAt / maxAgeSec`。
- PR #138 `codex/monitor-chart-runtime-fallback`
  - 已移除前端 dashboard 固定每 5 秒拉 `/api/v1/chart/candles?symbol=BTCUSDT&resolution=1` 的行为。
  - Monitor 主图优先使用 runtime `sourceStates.signal_bar`。
  - 只有 runtime bars 不足时，才按当前高亮会话请求 fallback candles。
  - fallback 最小粒度提升到 `5m`，避免展示型图表继续用 `1m` 造成 REST 压力。
- PR #141 `codex/monitor-chart-fallback-retry`
  - 补充 fallback candles 请求失败后的重试边界。
  - 请求失败时清空 request key，避免同一上下文被一次失败长期锁空。

专项当前结论：

- Telegram 高频告警不是根因，只是 stale / sync 风暴的外显噪声。
- 后端已从 REST requester、live account sync 调度、运行时观测三层收口。
- 前端已停止对展示型 K 线做固定高频 REST 轮询。
- 如果后续仍出现 stale，需要优先查看 PR #137 新增的 source gate 明细和 sync trigger 明细，而不是继续调 Telegram 去抖窗口。

## 1. 背景

2026-04-21 晚间的告警与系统日志显示，`runtime stale-source-states` / `runtime-stale-*` 反复出现，与 `service.live` 高频 `SyncLiveAccount` 以及 Binance REST `429` 限流在时间上高度重合。

已确认现象：

- Telegram 告警去抖逻辑已经生效，但仍然存在多轮独立 stale 事件。
- 同一账户在短时间内反复触发 `SyncLiveAccount`。
- Binance REST 日志中出现 `temporarily rate-limited until ...`。
- 一次 Binance 账户同步会连续请求：
  - `/fapi/v3/account`
  - `/fapi/v2/positionRisk`
  - `/fapi/v1/openOrders`

这意味着：

1. 问题不只是通知噪声。
2. 问题也不只是单个 sync 路径过频。
3. 根因更可能是“多个 live 路径共同触发账户同步 + Binance REST 统一治理不完整”，最终形成请求风暴并放大 stale 观测。

## 2. 目标

本专项只解决两个问题域：

1. 所有 Binance API 请求都必须经过统一限流治理。
2. `SyncLiveAccount` 必须改为统一调度，避免风暴式重复执行。
3. 前端展示型 K 线不能继续固定高频打 Binance REST。

本专项不做的事情：

- 不改变 live/runtime 健康判定语义。
- 不放宽 runtime readiness / recovery / reconcile 的安全边界。
- 不把 WS 当作最终事实源。
- 不修改 execution strategy 的交易决策语义。

## 3. 根因判断

### 3.1 账户同步触发入口过散

当前 `SyncLiveAccount` 会被多个链路直接触发：

- `syncActiveLiveAccounts` 周期刷新
- `triggerAuthoritativeLiveAccountReconcile`
- startup recovery
- filled order 后的账户刷新
- terminal order sync 后的账户刷新
- close position 目标解析前的账户刷新
- 其他订单 follow-up 流程

这些路径中，只有 dispatcher 周期刷新会经过 freshness 判断；其他路径多数会直接调用 `SyncLiveAccount`。

### 3.2 现有治理只有“互斥”，没有“调度”

当前 `SyncLiveAccount` 只有 per-account mutex：

- 可以避免并发同时执行
- 但不能合并请求
- 不能复用最近结果
- 不能表达优先级
- 不能在短时间内做冷却

结果是：

- 调用方大量收到“操作进行中”类错误或继续重试
- 同一账户在时间上形成连续 burst
- 一次 sync 里三次 Binance REST 请求会把 burst 放大

### 3.3 Binance REST 限流治理覆盖不完整

项目里已有 `binanceRESTLimiter`，但它当前主要包住了部分 signed REST 请求。

现状问题：

- 统一 gate 的治理范围不够明确
- 调用点分散，业务代码仍然容易直接走 Binance 请求
- 没有端点类别与优先级控制
- `429` backoff 虽然存在，但还没有形成统一 requester 语义

## 4. 事实源边界

本专项必须坚持以下边界：

- REST 对账结果是事实源。
- `account.Metadata`、`lastLiveSyncAt`、`healthSummary.accountSync`、session state 都是缓存态。
- 告警、runtime readiness、UI 汇总都属于推导态。

因此改造目标必须是：

- 改善 REST 请求治理
- 改善 sync 调度
- 增强观测

而不是：

- 通过放宽 stale 判定来掩盖真实问题
- 用缓存态假装账户已经同步

## 5. 实施策略

本专项原计划按 3 个串行 PR 推进，每个 PR 只解决一个问题域。实施过程中额外发现前端 dashboard 也存在固定 5 秒 K 线 REST 轮询，因此追加 PR4 / PR5 处理展示链路降载。

### PR1: Binance REST 统一治理层

目标：

- 所有 Binance REST 请求必须经过统一 requester / limiter。

改造方向：

- 收敛现有 `binanceRESTLimiter` 为统一请求入口。
- 把 signed / 需要权重治理的非 signed 请求都纳入统一治理策略。
- 统一处理：
  - 限速
  - burst
  - `429` backoff
  - `Retry-After`
  - 请求分类
  - 观测字段

建议的请求类别：

- `trade-critical`
  - submit order
  - cancel order
  - order status confirmation
  - immediate fill settlement
- `account-sync`
  - account snapshot
  - position risk
  - open orders snapshot
- `reconcile`
  - startup recovery
  - authoritative reconcile
  - takeover verification
- `history-read`
  - all orders
  - user trades
  - 历史补数
- `metadata-read`
  - symbol rules
  - exchange info

预期行为：

- 同一 `baseURL + credential key` 下共享总配额。
- 低优先级请求不能挤占关键交易请求。
- 一旦命中 `429`，后续请求共享同一个 backoff 窗口。
- 调用方不再自行解释限流状态。

边界：

- 此 PR 不改 `SyncLiveAccount` 触发路径。
- 只收敛请求治理与基础日志。

### PR2: Live Account Sync 调度层

目标：

- 同一账户的 sync 不再由多个业务路径直接打成风暴。

核心思想：

- `SyncLiveAccount` 从“直接执行”升级为“统一调度请求”。

建议新增统一入口（命名待定）：

- `RequestLiveAccountSync(accountID, reason, priority, policy)`

其中 policy 至少区分：

- `must-sync-now`
- `best-effort-refresh`

触发源分级建议：

- `must-sync-now`
  - startup recovery
  - reconcile-required
  - filled order 后事实刷新
  - submit/close 前必须的权威校验
- `best-effort-refresh`
  - dispatcher freshness 刷新
  - 一般性状态跟随刷新
  - 可延迟的 follow-up

调度器应具备的能力：

- 同账户请求合并
- in-flight 请求结果复用
- 冷却窗口
- 最近结果缓存
- 优先级区分
- 触发源记录

明确要求：

- `must-sync-now` 不允许被普通 freshness 逻辑吞掉。
- 但即使是 `must-sync-now`，也不能并发重复执行相同账户 sync。
- 调用方应拿到复用结果，而不是重复触发真实 Binance 请求。

替换范围：

- `syncActiveLiveAccounts`
- `triggerAuthoritativeLiveAccountReconcile`
- startup recovery
- `live_execution.go` 中 filled / terminal / settlement 后刷新
- `order.go` 中 close position / order follow-up 刷新

边界：

- 此 PR 不改交易策略与最终执行边界。
- 不改变 reconcile 是否必需的判断，只改变 sync 的调度方式。

### PR3: 观测与回归补强

目标：

- 下次再出问题时，日志能直接说明是哪条 source stale、是谁触发了 sync、为什么触发频率异常。

建议新增日志字段：

#### sync 调度日志

- `account_id`
- `trigger`
- `priority`
- `policy`
- `coalesced`
- `waited_for_inflight`
- `skipped_by_cooldown`
- `last_success_at`
- `last_attempt_at`

#### Binance requester 日志

- `request_category`
- `path`
- `gate_key`
- `blocked_until`
- `wait_ms`
- `retry_after`

#### stale source 明细日志

- `runtime_id`
- `source_key`
- `role`
- `stream_type`
- `symbol`
- `last_event_at`
- `max_age_sec`

边界：

- 此 PR 不扩大到业务语义修改。
- 以观测与测试为主。

### PR4: 前端 Monitor K 线降载

目标：

- 展示型 K 线不再固定每 5 秒请求 Binance candles。

实现结果：

- 移除 `useDashboard` 中固定 `BTCUSDT + 1m` 的 `/api/v1/chart/candles` 轮询。
- Monitor 主图优先使用 runtime `sourceStates.signal_bar`。
- runtime bars 不足时，才由 `MonitorStage` 按当前高亮会话发起 fallback candles 请求。
- fallback 请求会根据当前 monitor timeframe 选择原生周期，但最小不低于 `5m`。
- fallback 单次请求 `limit=240`，避免原先 `1m + limit=840` 的高频大窗口展示请求。

边界：

- 此 PR 不改变 runtime `sourceStates` 的生成方式。
- 不改变策略评估、dispatch、live sync 或后端 K 线接口语义。
- fallback candles 只用于图表展示，不作为交易事实源。

### PR5: Monitor fallback 重试边界

目标：

- 避免 fallback candles 一次请求失败后，同一上下文长期不再自动尝试。

实现结果：

- fallback 请求失败时清空 `fallbackRequestKeyRef`。
- 后续 dashboard / Monitor 重新渲染时，如果 runtime bars 仍为空，允许再次尝试 fallback。

边界：

- 不恢复固定轮询。
- 不在失败后立即 tight-loop 重试。
- 重试仍受 React effect 与上下文变化节奏约束。

## 6. 设计约束

### 6.1 不允许把“跳过”误当“成功”

如果 sync 因冷却窗口被跳过，必须在语义上明确是：

- skipped
- coalesced
- reused

而不是伪装成 fresh authoritative sync。

### 6.2 不允许影响未对账保护

恢复 / takeover / reconcile 场景下：

- 未完成权威对账前，仍然不能自动交易。
- sync 调度优化不能把风险状态“消音”成正常状态。

### 6.3 不允许让低优先级查询挤占下单路径

统一限流的目标不是“一刀切慢下来”，而是：

- 优先保证交易关键路径
- 压制重复 sync、历史补数、低价值轮询

## 7. 测试计划

### 7.1 PR1 测试

- 同一 limiter key 下多个请求共享总配额。
- `429` 后共享 backoff 生效。
- `Retry-After` 能正确影响 blocked-until。
- 低优先级请求不会导致关键交易请求永久饥饿。

### 7.2 PR2 测试

- 同一账户并发 sync 请求只执行一次底层 adapter sync。
- `best-effort-refresh` 命中冷却窗口时被跳过。
- `must-sync-now` 在已有 in-flight sync 时复用结果。
- startup recovery + dispatcher + filled-order follow-up 同时触发时，不会形成风暴式多次 sync。
- reconcile-required 不会被普通 cooldown 错误吞掉。

### 7.3 PR3 测试

- 新日志字段在关键路径上都有输出。
- stale source 明细能准确落日志。
- 关键测试覆盖不依赖文案匹配。

### 7.4 专项回归

至少补或确认以下回归：

- startup recovery
- exchange takeover
- reconcile-required
- duplicate exit 防护不变
- partial fill + restart
- immediate fill settlement
- passive close payload 不变

## 8. 验证清单

实施后按相关 PR 范围执行：

```bash
gofmt -w .
go test ./internal/service/...
go test ./...
go build ./cmd/platform-api
go build ./cmd/db-migrate
```

若涉及 live 实际闭环验证，追加：

```bash
bash scripts/testnet_live_session_smoke.sh
```

## 9. 风险说明

这是高风险改动，原因在于：

- 会碰 `internal/service/live*.go`
- 会碰 Binance live adapter / reconcile / recovery 相关路径
- 会影响 live 账户事实同步节奏

因此必须遵守：

- 一个 PR 只解决一个问题域
- 不顺手改 execution strategy
- 不顺手改 runtime readiness 判定
- 每个 PR 都单独说明 root cause / 修改点 / 行为变化 / 新增测试

## 10. 推荐执行顺序

1. 先做 PR1：统一 Binance REST requester / limiter。
2. 再做 PR2：统一 `SyncLiveAccount` 调度与合并。
3. 最后做 PR3：补观测与专项回归。

不建议颠倒顺序：

- 如果先改 sync 调度，但 REST requester 仍然分散，后续仍可能有漏网请求。
- 如果先大面积加日志而不先收口执行入口，日志只会更吵，不会先解决根因。

## 11. 后续排查手册

如果再次出现 `stale-source-states` 或 `数据源过期`：

1. 先看 runtime source gate 明细日志。
   - 重点字段：`runtime_id`、`source_key`、`role`、`stream_type`、`symbol`、`last_event_at`、`max_age_sec`。
   - 目标是确认到底是哪一路 source stale，而不是只看 Telegram 文案。
2. 再看 live account sync 触发日志。
   - 重点字段：`trigger`、`coalesced`、`waited_for_inflight`、`reused_recent_result`、`wait_ms`、`result_error`。
   - 目标是确认是否仍有某条路径绕开合并或短时间重复触发。
3. 再看 Binance requester 限流日志。
   - 重点字段：`category`、`path`、`blocked_until`、`retry_after`、`wait_ms`。
   - 目标是确认是否仍存在 `429` backoff 或低价值请求挤压。
4. 最后再看 Telegram。
   - Telegram 只负责通知去抖，不应该作为根因定位入口。

如果再次发现前端 K 线导致压力：

- 优先确认 Monitor 是否拿到了 runtime `signal_bar`。
- 只有 runtime bars 为空时才应该看到 `/api/v1/chart/candles` fallback 请求。
- fallback 请求的 resolution 不应低于 `5m`。
- 不应再出现 dashboard 每 5 秒固定拉 `BTCUSDT + 1m` 的请求。
