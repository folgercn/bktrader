# 2026-04-23 前端 Orders 查询与轮询治理专项计划

## 1. 背景

2026-04-23 16:42 到 16:52 左右，生产环境出现“前端长时间无响应、随后手工重启容器”的现象。只读排查结果表明，问题核心不是“真实下单风暴”，而是：

- 前端存在全局定时轮询；
- `GET /api/v1/orders` 在轮询链路中被无条件全量读取；
- 后端 `/api/v1/orders` 当前没有服务端分页、没有默认 limit、没有轻量模式；
- 单条 `orders.metadata` 体积较大，导致单次响应体积可达到 `17MB` 到 `43MB`；
- 大响应 + 高频轮询共同拖慢 API，后续进一步诱发容器 OOM / 重启。

这份文档目标不是直接给出代码，而是提供一份可执行的完整改造计划，供后续其他 LLM 或人工按阶段落地。

## 2. 已确认事实

### 2.1 前端轮询事实

- `web/console/src/App.tsx` 顶层启动 `useDashboard()`，不是某个局部页面单独使用。
- `web/console/src/hooks/useDashboard.ts` 中存在 `window.setInterval(load, 5000)`，当前轮询间隔为硬编码 `5000ms`。
- `loadDashboard()` 每次会通过 `Promise.all` 全量请求多组接口。
- 其中包括 `fetchJSON<Order[]>("/api/v1/orders")`。

结论：

- 只要前端已登录且页面打开，就会触发整套 dashboard 轮询。
- 当前不是“进入订单页才拉订单”，而是“全局每 5 秒拉一次订单”。

### 2.2 前端分页事实

- 前端订单分页已经存在，但发生在全量数据加载之后。
- 典型位置：`web/console/src/components/layout/DockContent.tsx`
- 当前逻辑是：
  - 先加载全量 `orders`
  - 再在前端 `sort + slice`
  - 只显示当前页

结论：

- 这是展示分页，不是服务端分页。
- 无法降低后端查询开销、网络传输成本、JSON 解析成本、浏览器内存占用。

### 2.3 后端 Orders 查询事实

- `/api/v1/orders` 当前 GET handler 直接调用 `platform.ListOrders()`
- `platform.ListOrders()` 直接透传 `store.ListOrders()`
- `store.ListOrders()` 当前 SQL 为全表 `select ... from orders order by created_at asc`

结论：

- 后端当前没有分页、limit、offset、cursor、时间窗口、账户过滤、session 过滤、轻量字段投影。

### 2.4 Orders 体积事实

排查时观察到：

- 单次 `/api/v1/orders` 响应可达 `17MB` 到 `43MB`
- 单次请求耗时可达 `20s` 到 `66s`
- 单条订单 `metadata` 当前样本可达约 `9KB` 到 `10KB`
- `metadata` 中包含大量执行上下文与嵌套 JSON，例如：
  - `intent`
  - `executionContext`
  - `executionProposal`
  - `adapterSubmission`
  - `adapterSync`
  - `runtimePreflight`
  - `orderLifecycle`

结论：

- Orders 问题不是单点，而是“高频拉取 + 全量返回 + 单条记录过胖”的组合问题。

## 3. 现阶段问题清单

需要系统性解决以下五类问题：

1. 前端轮询间隔硬编码，无法按环境调优。
2. Dashboard 全局轮询范围过大，静态数据和动态数据没有分层。
3. Orders 当前没有服务端分页，只能全量拉取。
4. 前端类似的“全量读取”接口不止 `/orders`，需要统一排查。
5. 中长期需要评估将部分实时数据从轮询改为 WS / 订阅模型。

## 4. 总体目标

本次专项建议按“先止血，再优化，再演进”的节奏推进。

### 4.1 短期目标

- 立即消除 `/api/v1/orders` 全量高频拉取带来的生产压力。
- 将轮询频率从硬编码改为环境可配置。
- 将前端全量读取链路分层治理，避免静态配置也跟着高频刷新。

### 4.2 中期目标

- 为高体积列表接口建立统一分页/筛选/轻量视图协议。
- 将 dashboard 拆分为“轻量首页数据”和“按需详情数据”。

### 4.3 长期目标

- 将真正实时的数据面板改为 WS / SSE / 订阅推送。
- 将前端状态从“定时全量同步”改为“首屏快照 + 事件增量更新”。

## 5. 分阶段实施计划

## Phase 0: 先做排查基线

目标：在不改行为前，先把现状量化，后续改造有验证基线。

建议输出：

- 统计当前前端所有 `fetchJSON(...)` 接口清单。
- 标注哪些接口属于：
  - 高频动态数据
  - 中频状态数据
  - 低频静态配置数据
- 统计生产上这些接口的：
  - 请求频率
  - 平均耗时
  - 响应体积
  - 调用页面 / 触发 hook

建议关注接口：

- `/api/v1/orders`
- `/api/v1/fills`
- `/api/v1/positions`
- `/api/v1/live/sessions`
- `/api/v1/account-summaries`
- `/api/v1/accounts`
- `/api/v1/alerts`
- `/api/v1/notifications`
- `/api/v1/monitor/health`
- `/api/v1/live/launch-templates`
- `/api/v1/signal-runtime/*`
- `/api/v1/signal-sources`
- `/api/v1/strategies`
- `/api/v1/backtests`

交付物：

- 一份接口清单表
- 一份“高风险全量接口”名单

## Phase 1: Orders 止血改造

目标：优先解决生产已暴露的 `/api/v1/orders` 查询风暴。

### 5.1 后端接口改造

新增或改造 `/api/v1/orders` 查询协议，至少支持：

- `limit`
- `offset` 或 `cursor`
- `accountId`
- `liveSessionId`
- `status`
- `symbol`
- `from`
- `to`
- `sort`

建议默认行为：

- 未显式指定时，默认只返回最近 `N` 条，例如 `50` 或 `100`
- 默认按 `created_at desc`
- 默认拒绝“无限制全量返回”

建议增加响应结构：

```json
{
  "items": [],
  "page": {
    "limit": 50,
    "offset": 0,
    "total": 1234,
    "hasMore": true
  }
}
```

可选增强：

- 增加 `view=summary|full`
- `summary` 只回前端列表需要字段
- `full` 仅用于订单详情或调试场景

### 5.2 存储层改造

`internal/store/postgres/store.go` 需要从 `ListOrders()` 全表读取，转向基于 `OrderQuery` 的分页查询。

建议：

- 扩展 `domain.OrderQuery`
- 明确 limit 上限，例如 `maxLimit=500`
- 为常见过滤维度补索引或确认已有索引

至少检查索引：

- `orders(created_at)`
- `orders(account_id, created_at)`
- `orders((metadata->>'liveSessionId'))`
- `orders(status, created_at)`

如果 `metadata->>'liveSessionId'` 是高频条件，应考虑：

- 保持 JSON 过滤但补表达式索引
- 或中长期迁移为显式列

### 5.3 前端订单列表改造

订单列表页面必须从“全量拉取后前端分页”改为“翻页即发请求”的模式。

具体要求：

- 订单列表独立请求自己的分页接口
- 切页时请求下一页
- 切页大小时重新请求
- 不再依赖 dashboard 全量加载出的 `orders`

建议前端只在以下场景拉订单列表：

- 用户进入订单 tab
- 用户切换页码
- 用户切换筛选器
- 用户主动刷新

不建议继续：

- dashboard 全局每 5 秒拉全量订单

## Phase 2: Dashboard 轮询拆分

目标：把当前“大一统 loadDashboard”拆成按数据域分层加载。

### 5.4 按数据特征拆分接口

建议将 dashboard 请求拆为三层：

#### A. 高频动态数据

建议包括：

- live sessions
- positions
- fills
- alerts
- notifications
- monitor health

这些数据可以保留轮询，但频率应可配置，且不应和静态数据绑定在同一个 `Promise.all` 里。

#### B. 中频状态数据

建议包括：

- account summaries
- accounts
- signal runtime sessions

这些数据更新频率通常低于实时流水，可采用更长周期轮询。

#### C. 低频静态/半静态数据

建议包括：

- strategies
- live adapters
- signal sources
- signal source types
- runtime policy
- telegram config
- live launch templates
- backtests options

这些数据不应每 5 秒刷新。

建议刷新策略：

- 首屏加载一次
- 用户打开相关 modal 时刷新
- 用户执行修改动作后刷新
- 或采用长周期轮询，例如 60s / 300s

### 5.5 前端架构要求

建议将 `useDashboard()` 拆为多个 hook，例如：

- `useDashboardRealtime()`
- `useDashboardState()`
- `useDashboardConfig()`
- `useOrdersPageQuery()`

目标：

- 降低一次请求链路的耦合
- 某一组数据失败时，不拖垮整个 dashboard
- 更容易引入独立频率、独立缓存、独立重试

## Phase 3: 全量读取专项排查与整改

目标：不仅修 `/orders`，还要找出所有类似的“全量读取 + 高频访问”组合。

### 5.6 排查原则

前端和后端联合排查以下模式：

- “列表接口没有分页”
- “在 App 顶层或全局 hook 中自动高频请求”
- “返回内容包含大块 metadata / nested JSON”
- “页面只显示前几条，但接口返回全部”

### 5.7 初步高风险候选

按当前代码，优先排查：

- `/api/v1/fills`
- `/api/v1/live/sessions`
- `/api/v1/alerts`
- `/api/v1/notifications`
- `/api/v1/account-summaries`
- `/api/v1/signal-runtime/sessions`

说明：

- 它们未必像 `orders` 一样重，但如果也走全量 + 高频，后面会成为第二个瓶颈。

### 5.8 整改优先级

优先级建议：

1. `/orders`
2. `/fills`
3. `/live/sessions`
4. `/alerts` / `/notifications`
5. 剩余配置类接口

## Phase 4: 配置化轮询阈值

目标：所有前端轮询周期都可由 `.env` 控制，不再写死在源码中。

### 5.9 环境变量建议

建议至少引入：

- `VITE_DASHBOARD_REALTIME_POLL_MS`
- `VITE_DASHBOARD_STATE_POLL_MS`
- `VITE_DASHBOARD_CONFIG_POLL_MS`
- `VITE_ORDERS_POLL_MS`
- `VITE_FILLS_POLL_MS`
- `VITE_ALERTS_POLL_MS`
- `VITE_MONITOR_HEALTH_POLL_MS`

原则：

- 所有 polling 周期必须集中定义
- 代码里不得再出现散落的 magic number `5000`
- `.env.example` 必须同步更新

### 5.10 配置策略建议

建议默认值大致分层：

- 实时：`3000ms - 5000ms`
- 状态：`10000ms - 30000ms`
- 配置：`60000ms - 300000ms`

注意：

- 默认值不宜为了“看起来实时”而压得太低
- 前端页面打开数、会话并发数、响应体积都要计入

## Phase 5: 中长期 WS / 订阅化演进

目标：将真正实时的数据从轮询切换为推送。

### 5.11 哪些数据值得优先改为 WS / 订阅

优先推荐：

- live sessions
- orders
- fills
- positions
- alerts
- notifications
- monitor health

原因：

- 这些数据是“事件流”
- 适合首屏快照 + 增量更新
- 继续轮询会越来越贵

### 5.12 哪些数据不适合优先 WS 化

暂不优先：

- strategies
- backtests
- runtime policy
- adapters
- launch templates
- signal source catalog

原因：

- 更新不频繁
- 用低频拉取或按需刷新更简单可靠

### 5.13 推荐演进路线

不要一步把所有接口改成 WS。

建议路线：

1. 先完成服务端分页和轮询拆分
2. 保留 HTTP 首屏快照接口
3. 为实时数据补 WS / SSE 增量通道
4. 前端采用：
   - 首次进入页面：HTTP 拉快照
   - 页面存活期间：订阅增量更新
   - 断线后：自动重连 + 回补快照

### 5.14 需要提前考虑的问题

- 事件幂等
- 重连补偿
- 游标 / sequence
- 单页多订阅去重
- store 状态合并策略
- 断线退化到轮询

## 6. 建议的数据协议设计

## 6.1 Orders 列表接口

建议新增或改造：

- `GET /api/v1/orders?limit=50&offset=0`
- `GET /api/v1/orders?liveSessionId=...&limit=20`
- `GET /api/v1/orders?accountId=...&status=FILLED&from=...&to=...`

建议支持：

- `view=summary`
- `includeMetadata=false`

其中：

- `summary` 用于列表展示
- `includeMetadata=false` 用于明确禁止回传重型 metadata

### 6.2 Orders 详情接口

建议保留或新增：

- `GET /api/v1/orders/:id`

只在查看单笔订单详情时返回完整 metadata。

### 6.3 实时订阅接口

中长期建议考虑：

- `GET /api/v1/stream/orders`
- `GET /api/v1/stream/live-sessions`
- `GET /api/v1/stream/positions`
- `GET /api/v1/stream/alerts`

可选基于：

- WebSocket
- SSE

如果只需要单向推送，SSE 复杂度通常低于 WS，可优先评估。

## 7. 需要其他 LLM 执行时遵守的边界

本专项建议拆成多个 PR，不要一次性混改。

建议拆分：

### PR 1

- 只改 `/api/v1/orders` 服务端分页
- 补后端测试

### PR 2

- 前端订单列表改为按页请求
- 停止 dashboard 全量拉 orders

### PR 3

- dashboard 轮询拆分
- `.env` 轮询配置化

### PR 4

- 其他全量接口专项治理

### PR 5

- WS / SSE 方案设计与 PoC

不要在同一个 PR 中顺手：

- 改 live 执行逻辑
- 改 recovery 状态机
- 改 dispatch 默认行为
- 改部署/CI

## 8. 测试与验证要求

### 8.1 后端

至少覆盖：

- `/api/v1/orders` 默认 limit 行为
- limit/offset/filter 组合查询
- limit 超上限保护
- 空结果与越界分页
- `view=summary` / `includeMetadata=false` 行为
- 查询顺序稳定性

### 8.2 前端

至少覆盖：

- 订单 tab 首次进入加载第一页
- 翻页会重新请求
- 修改 pageSize 会重新请求
- 不进入订单 tab 时，不会全量拉订单
- 轮询间隔走 `.env`

### 8.3 生产验证

建议上线后重点观察：

- `/api/v1/orders` 平均响应体积
- `/api/v1/orders` p95/p99 耗时
- `bktrader-app` 内存占用
- 同一浏览器打开时的请求频率
- 是否还出现 OOM / 卡死 / 手工重启

## 9. 风险提示

### 9.1 风险一：前端改了分页，但 monitor/account 派生逻辑仍依赖全量 orders

当前很多派生逻辑直接基于全局 `orders`：

- HeaderMetrics
- MonitorStage
- AccountStage
- DockContent
- derivation utilities

因此不能只把订单列表切成分页就结束，必须同时区分：

- “全局态真正需要的最小订单集”
- “订单面板展示需要的分页订单集”

建议后续明确两套数据：

- `ordersSummaryForDashboard`
- `ordersPage`

### 9.2 风险二：只做前端节流，不改后端协议

如果只把轮询从 5 秒改成 30 秒，但后端仍然全量查，问题只是缓解，不是解决。

### 9.3 风险三：直接上 WS，跳过分页和分层治理

如果不先收敛数据模型和接口边界，WS 只会把问题换一种运输方式继续放大。

## 10. 最终建议

建议按以下优先顺序执行：

1. 先把 `/api/v1/orders` 改成服务端分页和默认限流。
2. 前端订单列表改成按页请求，不再依赖 dashboard 全量 orders。
3. 将 `useDashboard()` 拆分为多组数据请求，并移除写死的 `5s`。
4. 所有轮询阈值改为 `.env` 可配置。
5. 系统性排查其他“全量读取 + 高频轮询”接口。
6. 最后再推进 WS / SSE 化，优先覆盖 orders/fills/positions/live sessions/alerts。

这次事件的根因判断，应统一表述为：

> 生产问题核心是 Orders 查询链路设计不合理：前端全局高频轮询、后端无分页全量返回、单条订单 metadata 过重，三者叠加导致 API 卡死并最终引发容器内存风险。

