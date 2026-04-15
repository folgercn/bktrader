# Frontend Coordination Tasks

## 说明

这份文件汇总当前需要前端协作跟进的 3 组后端改动：

- PR `#39`：health snapshot / runtime policy 扩展
- PR `#40`：live data collection telemetry 落库
- 当前分支：multi-timeframe signal bindings 修复

目标不是让前端一次性全做完，而是把“已经可做的 UI 对齐”和“还依赖后端补接口的事项”拆清楚。

## 建议处理顺序

1. 先处理 PR `#39`
2. 再处理 multi-timeframe signal bindings
3. 最后排 PR `#40`，其中一部分仍依赖后端补 HTTP 暴露

## 范围约束

仅限 `web/console`，不要顺手改后端。

优先会涉及这些位置：

- `web/console/src/types/domain.ts`
- `web/console/src/store/useUIStore.ts`
- `web/console/src/hooks/useDashboard.ts`
- `web/console/src/hooks/useTradingActions.ts`
- `web/console/src/pages/AccountStage.tsx`
- `web/console/src/pages/MonitorStage.tsx`
- 如有必要，再补 `web/console/src/utils/derivation.ts`

## Task A: PR #39 Health Snapshot And Runtime Policy

### 后端已提供

- `GET /api/v1/monitor/health`
- `GET /api/v1/runtime-policy`
- `POST /api/v1/runtime-policy`

`/api/v1/monitor/health` 返回的核心结构包括：

- `status`
- `alertCounts`
- `runtimePolicy`
- `liveAccounts`
- `runtimeSessions`
- `liveSessions`
- `paperSessions`

`runtimePolicy` 现在比前端当前类型更多 2 个字段：

- `strategyEvaluationQuietSeconds`
- `liveAccountSyncFreshnessSeconds`

并且后端已经支持显式传 `0` 作为 disable，而不是只能传正数。

### 当前前端缺口

我在当前前端代码里看到：

- 已经消费 `/api/v1/runtime-policy`
- 但 `RuntimePolicy` 类型仍缺少上面两个新增字段
- `RuntimePolicyForm` 也缺少这两个输入项
- 还没有消费 `/api/v1/monitor/health`

### 前端需要做什么

1. 给 `RuntimePolicy` 和 `RuntimePolicyForm` 补齐：
   - `strategyEvaluationQuietSeconds`
   - `liveAccountSyncFreshnessSeconds`
2. 运行时策略表单允许用户显式保存 `0`。
3. 接入 `/api/v1/monitor/health`，为监控页面提供平台级健康视图。
4. 至少把这些健康信息展示出来：
   - 平台总体状态 `status`
   - 告警聚合 `alertCounts`
   - live account 的 `syncStale` / `syncAgeSeconds`
   - runtime session 的 `health` / `quiet`
   - live session 的 `evaluationQuiet`
5. 现有 alerts 视图如果继续保留，建议和 `/api/v1/monitor/health` 的摘要信息形成互补，而不是重复堆同样字段。

### 验收标准

- 前端不会再丢失 `strategyEvaluationQuietSeconds` 和 `liveAccountSyncFreshnessSeconds`
- UI 能表达 `0 = disable`
- 页面能看出 live account 是否 sync stale
- 页面能看出 runtime quiet / live evaluation quiet

## Task B: Multi-Timeframe Signal Bindings

### 背景

后端已修复信号绑定身份键，`signal` 类绑定现在会把 `timeframe` 纳入匹配与去重逻辑。

这意味着以下场景现在在后端是合法且可区分的：

- 同一账户同时绑定 `BTCUSDT 1d` 与 `BTCUSDT 4h`
- 同一策略同时绑定 `BTCUSDT 1d` 与 `BTCUSDT 4h`
- runtime plan 对 `BTCUSDT 4h` 不会再误匹配到账户侧的 `BTCUSDT 1d`

另外，后端运行模型已经进一步收敛：

- runtime 订阅规划现在直接基于策略绑定生成
- 账户级 signal binding 不再参与 runtime plan / readiness / live launch
- 快速启动路径也已修正：同一账户下相同策略但不同 `symbol/timeframe` 的 live session 不会再被错误复用成同一个 session

也就是说，账户绑定 UI 现在属于待下线状态，而不是运行时必填项。

### 后端已提供

- `GET /api/v1/accounts/:id/signal-bindings`
- `GET /api/v1/strategies/:id/signal-bindings`
- `GET /api/v1/signal-runtime/plan`

策略绑定列表与 runtime plan 现在是主数据源。

账户绑定接口仍保留兼容，但后端运行链路已经不再依赖它。

策略绑定列表接口里，周期仍主要放在：

- `item.options?.timeframe`

账户绑定兼容接口里，周期如果还有历史数据，也仍然会在：

- `item.options?.timeframe`

runtime plan 里的 binding map 现在额外带出顶层字段：

- `timeframe`

### 当前前端缺口

当前控制台还有一块“账户信号源绑定”UI，这和后端新模型已经不一致。

同时，“当前信号绑定”列表没有展示 `timeframe`，因此：

- 用户无法区分同一 `sourceKey + role + symbol` 下的不同周期绑定
- 同 symbol 多周期数据会看起来像重复行
- runtime plan 的 `Missing` / `Matched` 摘要也容易误读
- 用户还会被误导，以为账户层必须重复绑定 signal source

### 前端需要做什么

1. 下线或明显标记“账户信号源绑定”区域为 deprecated，最终目标是移除这块 UI。
2. 保留并强化“策略信号源绑定”作为唯一有效的 signal source 配置入口。
3. 在“当前信号绑定”的展示上，优先展示策略级绑定；如果过渡期仍保留账户级历史数据，需明确标记为“兼容旧数据/不参与运行”。
4. 在 runtime plan 的 `Missing` / `Matched` 摘要里稳定展示 `timeframe`。
5. 绑定展示优先读取：
   - `item.timeframe`
   - 若没有，再回退 `item.options?.timeframe`
6. 对无周期的源，例如 `trade_tick` / `order_book`，展示 `--`。
7. 如类型定义过窄，允许给相关 binding/view model 增加可选的 `timeframe?: string`。

### 验收标准

- 用户不再被要求先绑账户信号源，才能理解或使用 live/runtime 配置
- 页面上能同时清楚区分 `BTCUSDT 1d` 和 `BTCUSDT 4h`
- 同 symbol 多周期绑定不会被 UI 合并或误读为重复数据
- runtime plan 缺失项能明确指出是哪个周期缺失
- 不修改现有表单提交流程语义，不引入新的默认行为

### 联调建议

建议用下面这组数据手测：

- 策略绑定：`BTCUSDT 1d`, `BTCUSDT 4h`
- 可选保留一些旧账户绑定历史数据，用来验证 UI 是否正确标记为兼容数据

预期：

- 策略绑定表应显示 2 条
- 即使账户绑定为空，runtime plan 也应可正常 ready
- runtime plan 应正确区分 `BTCUSDT 1d` 与 `BTCUSDT 4h`

## Task C: One-Click Live Launch Templates

### 背景

后端现在已经提供 4 个“Binance testnet 一键启动模板”，目的是把联调时最容易出错的几层配置直接固定下来：

- 账户绑定固定为 Binance Futures testnet REST
- 策略绑定固定为 `signal + trigger + feature` 三件套
- live session 固定为 `tick` 执行、默认执行策略
- 只有 `dispatchMode` 允许前端在模板区顶部单独选择
- symbol / timeframe 直接收敛成 4 个常用组合

这 4 个模板分别是：

- `binance-testnet-btc-4h`
- `binance-testnet-btc-1d`
- `binance-testnet-eth-4h`
- `binance-testnet-eth-1d`

### 后端已提供

- `GET /api/v1/live/launch-templates`

返回结构里，前端最需要关心这些字段：

- `key`
- `name`
- `description`
- `symbol`
- `signalTimeframe`
- `defaultDispatchMode`
- `dispatchModeOptions`
- `strategyId`
- `strategyName`
- `accountRequirements`
- `accountBinding`
- `strategySignalBindings`
- `launchPayload`
- `steps`
- `notes`

### 每个模板实际固定了什么

#### 1. 账户绑定

模板里的 `accountBinding` 已经固定为：

- `adapterKey = binance-futures`
- `sandbox = true`
- `executionMode = rest`
- `positionMode = ONE_WAY`
- `marginMode = CROSSED`
- `credentialRefs.apiKeyRef = BINANCE_TESTNET_API_KEY`
- `credentialRefs.apiSecretRef = BINANCE_TESTNET_API_SECRET`

这里最重要的是 `executionMode = rest`。

当前控制台普通 live binding 表单没有把这个字段显式暴露出来，而后端默认值偏向 `mock`。如果前端一键启动不显式按模板写入这组 `accountBinding`，用户很容易以为自己跑的是 Binance testnet，实际却还是 mock。

#### 2. 策略绑定

模板里的 `strategySignalBindings` 每次都固定包含 3 条：

1. `binance-kline` + `role=signal` + `symbol` + `timeframe`
2. `binance-trade-tick` + `role=trigger` + `symbol`
3. `binance-order-book` + `role=feature` + `symbol`

也就是：

- `signal` 看 Binance 原生 `4h/1d kline`
- `trigger` 用 Binance `trade tick`
- `feature` 用 Binance `order book`

这 3 个请求前端应该按“幂等 upsert”理解。因为后端绑定键已经按 `sourceKey + role + symbol + timeframe` 去重，多次点同一个模板不会无限重复堆积同一条绑定。

#### 3. live session 覆盖参数

模板里的 `launchPayload.liveSessionOverrides` 至少已经固定了这些字段：

- `symbol`
- `signalTimeframe`
- `executionDataSource = tick`
- `positionSizingMode = fixed_quantity`
- `defaultOrderQuantity`
- `executionStrategy = book-aware-v1`
- `executionEntryOrderType = MARKET`
- `executionEntryMaxSpreadBps = 8`
- `executionEntryWideSpreadMode = limit-maker`
- `executionEntryRestingTimeoutSeconds = 15`
- `executionEntryTimeoutFallbackOrderType = MARKET`
- `executionPTExitOrderType = LIMIT`
- `executionPTExitTimeInForce = GTX`
- `executionPTExitPostOnly = true`
- `executionPTExitWideSpreadMode = limit-maker`
- `executionPTExitTimeoutFallbackOrderType = MARKET`
- `executionSLExitOrderType = MARKET`
- `executionSLExitMaxSpreadBps = 999`
- `executionSLExitTimeoutFallbackOrderType = MARKET`
- `dispatchCooldownSeconds = 30`

注意：

- `dispatchMode` 不再写死在模板 payload 里
- 模板会单独返回：
  - `defaultDispatchMode = manual-review`
  - `dispatchModeOptions = [manual-review, auto-dispatch]`
- 前端需要把用户在模板区顶部选中的 `dispatchMode` 注入到最终提交的 `launchPayload.liveSessionOverrides.dispatchMode`

数量目前后端模板固定为：

- `BTCUSDT`：`0.002`
- `ETHUSDT`：`0.1`

这是为了尽量避免 Binance testnet 的最小名义价值限制，把联调一上来就卡在“下单量太小”这种无意义失败上。

### 前端一键启动应该怎么接

不要把这个功能理解成“只是在 UI 上帮用户填表”。更稳妥的接法是：

1. 先 `GET /api/v1/live/launch-templates`
2. 在 4 张模板卡片上方放一个全局 `dispatchMode` 选择器
3. 在 UI 上展示 4 张模板卡片或 4 个明确按钮
4. 用户先选一个 live account，再点模板
5. 点击后，前端按模板里的 `steps` 顺序串行执行

推荐执行顺序：

1. `POST /api/v1/live/accounts/:accountId/binding`
   - body 直接用模板里的 `accountBinding`
2. 对模板里的每一条 `strategySignalBindings`
   - 分别 `POST /api/v1/strategies/:strategyId/signal-bindings`
3. `POST /api/v1/live/accounts/:accountId/launch`
   - body 基于模板里的 `launchPayload`
   - 仅在发请求前额外注入：
     - `liveSessionOverrides.dispatchMode = 当前选择器选中的值`

这里建议前端不要自己重新拼大对象，而是：

- 直接复用模板响应里的 `accountBinding`
- 直接复用模板响应里的 `strategySignalBindings`
- 直接复用模板响应里的 `launchPayload`
- 只覆写一个字段：`launchPayload.liveSessionOverrides.dispatchMode`

这样最不容易在命名和默认值上跑偏。

### UI 建议

建议把这一块放在 live account 区域，作为“Quick Launch Templates”或“联调一键启动”区域，而不是塞进策略绑定表单里。

推荐 UI 结构：

1. 顶部一行 `dispatchMode` 选择器
   - `manual-review`
   - `auto-dispatch`
2. 下方 4 张模板卡片

每张卡片至少展示：

- 模板名，例如 `Binance Testnet BTCUSDT 4h`
- `symbol`
- `signal timeframe`
- `trigger source = binance-trade-tick`
- `feature source = binance-order-book`
- `strategyName`
- `defaultOrderQuantity`
- 当前顶部选择的 `dispatchMode`

建议再补两行提示：

- `这会先确保账户绑定和策略绑定，再启动 runtime/live session`
- `这是 testnet 联调专用模板；是否自动派单由上方 dispatchMode 控制`

### 前端状态与错误处理建议

一键启动按钮最好做成明显的 3 段状态：

- `Binding account...`
- `Applying strategy sources...`
- `Launching live flow...`

同时要把失败点明确告诉用户，不要统一报成一个泛化错误。

建议至少区分这 3 类失败：

1. 账户绑定失败
   - 常见原因：凭证 ref 缺失、账户不是 `LIVE`、交易所不匹配
2. 策略绑定失败
   - 常见原因：策略不存在、sourceKey 拼错、role 不合法
3. launch 失败
   - 常见原因：live account 未配置、runtime/source readiness 不满足

同时建议在按钮附近放一个很明确的风险文案，且只在选择了 `auto-dispatch` 时高亮：

- `仅限 Binance testnet 联调`
- `点击后若策略触发，将自动向 testnet 发送真实 REST 订单`

### 和当前模型的关系

这里有两个容易让前端协作者误解的点，要特别注意：

1. 模板里虽然仍然会调用“账户绑定”接口，但那是 live adapter 绑定，不是旧的“账户信号源绑定”。
2. 模板里真正驱动 runtime 的，是 `strategySignalBindings`，不是 `account signal bindings`。

也就是说：

- `POST /api/v1/live/accounts/:id/binding` 是交易账户接入配置
- `POST /api/v1/strategies/:id/signal-bindings` 才是 runtime 订阅来源配置

不要把这两者混成同一个“绑定”概念。

### 验收标准

- 前端能读到 4 个模板，并明确展示差异
- 用户只需要先选账户，再点一个模板，就能完成联调启动
- 前端实际执行的是模板里的固定 payload，而不是重新手工拼字段
- 同一个模板重复点击不会制造同一条策略绑定的重复脏数据
- 模板区顶部会有唯一可编辑项 `dispatchMode`
- 模板卡片或按钮会明确标注 `testnet only`
- 模板启动后能正确创建或复用：
  - `account + strategy` 级 runtime session
  - `account + strategy + symbol + timeframe` 级 live session
- UI 上能明确告诉用户当前启动的是 `Binance testnet REST`，不是 mock
- 当选择 `auto-dispatch` 时，UI 要明确告诉用户这是自动派单模式

### 联调建议

推荐联调顺序：

1. 先用 `binance-testnet-btc-4h`
2. 再测 `binance-testnet-btc-1d`
3. 确认同一账户下 `BTC 4h` 和 `BTC 1d` 可以并存
4. 再补测 `ETHUSDT` 两个模板

重点观察：

- 账户绑定后是否真的写入了 `executionMode = rest`
- 策略绑定列表里是否出现 `signal / trigger / feature` 三条
- runtime plan 是否直接来自策略绑定
- live session 是否按 `symbol + timeframe` 分开
- 顶部切到 `manual-review` 时，live session state 里的 `dispatchMode` 是否为 `manual-review`
- 顶部切到 `auto-dispatch` 时，live session state 里的 `dispatchMode` 是否为 `auto-dispatch`

## Task D: PR #40 Live Data Collection Telemetry

### 后端当前状态

PR `#40` 已经把这 3 类 live telemetry 落到存储层：

- `StrategyDecisionEvent`
- `OrderExecutionEvent`
- `PositionAccountSnapshot`

这些数据结构已经在后端 domain / store 层存在，并且 live 流程里会持续记录。

### 当前阻塞

我在当前 `internal/http` 里没有找到对应的 telemetry 列表接口，也就是说：

- 数据已经被后端记录
- 但前端目前还没有 REST API 可以直接拉这些记录

所以这部分需要拆成“前置后端暴露”与“前端消费展示”两步。

### 前端先做的准备

即使接口还没出，前端协作者也可以先设计数据视图和类型草案，建议目标包括：

1. live session 级别的策略决策时间线
2. order 级别的执行事件时间线
3. account / live session 级别的仓位与账户快照时间线

建议 UI 展示重点：

- 决策为什么被阻断：`sourceGateReady` / `missingCount` / `staleCount`
- 订单实际执行质量：`priceDriftBps` / `spreadBps` / `fallback` / `failed`
- 仓位与账户变化：`positionQuantity` / `availableBalance` / `netEquity`

### 依赖后端后续补的接口

这部分目前不是既成事实，只是推荐的后续暴露方向。前端同学在排期时请把它视为 dependency，而不是当前已上线 API。

推荐至少需要有：

- live session 维度的策略决策事件列表接口
- order 维度的执行事件列表接口
- account 或 live session 维度的 position/account snapshot 列表接口

### 前端落地建议

等接口具备后，优先在现有监控工作台里补 1 个 telemetry 区域，而不是额外再开一整页。建议先从只读时间线开始，不要一开始就做复杂筛选器。

### 验收标准

- 能从 UI 上回溯一次 live session 的“信号 -> 决策 -> 下单 -> 同步/成交 -> 仓位/账户快照”
- 能快速看出某次执行是 readiness 问题、fallback 问题，还是价格质量问题
- 不引入新的控制面动作，先以只读诊断为主
