# 前端协作：Live Launch Template 隔离

> **上下文**：后端已把 live launch template 的语义从“增量叠加 bindings”改成“独占切换当前模板 bindings + 刷新 runtime 订阅”。
>
> **本轮约束**：当前 PR / 当前分支**不改前端页面**，只提供后端能力和前端协作文档，前端后续单独跟进。

## 目标

前端后续需要把模板区的用户心智和后端真实行为对齐：

- 点 `BTCUSDT 5m`，应理解为“切换到 BTCUSDT 5m 模板”
- 再点 `ETHUSDT 4h`，应理解为“覆盖旧模板并切到 ETHUSDT 4h”
- 不再把模板理解成“在原有策略 bindings 上继续叠加”

## 本轮后端已完成

### 1. 模板语义收敛

后端现在在 `LaunchLiveFlow` 内部处理模板切换，不再依赖前端逐条写策略绑定：

- 若 `launchPayload.strategySignalBindings` 存在，后端会把它视为本次模板的**唯一 bindings 集合**
- 启动前会替换当前策略下旧的模板 bindings
- 若同一 `account + strategy` 已有 runtime，会先刷新 runtime plan / subscriptions
- 若同一 `account + strategy` 下存在其它正在运行、但不属于本次模板 scope 的 live session，会先停掉它们

### 2. 模板返回结构变化

`GET /api/v1/live/launch-templates`

当前模板对象仍会返回：

- `accountBinding`
- `strategySignalBindings`
- `launchPayload`
- `steps`

但语义上有两个重要变化：

1. `steps` 现在只有 2 步：
   - `POST /api/v1/live/accounts/:accountId/binding`
   - `POST /api/v1/live/accounts/:accountId/launch`
2. `launchPayload` 内部已经包含：
   - `strategySignalBindings`
   - `launchTemplateKey`
   - `launchTemplateName`

也就是说，前端后续**不应该再自己循环调用**

- `POST /api/v1/strategies/:strategyId/signal-bindings`

而是应直接把模板返回的 `launchPayload` 原样提交给 launch 接口，仅按既有约定覆写允许变更的字段。

### 3. launch 返回结果补充

`POST /api/v1/live/accounts/:accountId/launch`

现在返回的 `LiveLaunchResult` 除原有字段外，还会额外包含：

- `templateApplied`
- `templateBindingCount`
- `runtimePlanRefreshed`
- `stoppedLiveSessions`

可用于前端展示“本次是否发生模板切换 / 是否刷新了 runtime / 是否停掉了旧 session”。

### 4. runtime / live session state 新增模板上下文

launch 成功后，后端会把模板上下文写进 runtime session 与 live session 的 state：

- `launchTemplateKey`
- `launchTemplateName`
- `launchTemplateSymbol`
- `launchTemplateTimeframe`
- `launchTemplateAppliedAt`

前端后续如果要展示“当前运行的是哪张模板”，直接读这些字段即可，不必自己从 bindings 倒推。

## 前端后续建议接法

### 1. 模板按钮文案

建议不要继续使用容易让人误解成“无副作用叠加”的文案。

建议方向：

- `一键切换并启动`
- 或保留 `一键应用并启动`，但必须补充说明：
  - `会覆盖该策略当前模板绑定`

### 2. 点击行为

前端后续点击模板时，建议流程收敛为：

1. 用户先选择 `live account`
2. 点击模板卡片
3. 前端串行执行模板返回的 `steps`
4. 对 `launch` 这一步：
   - 直接复用模板里的 `launchPayload`
   - 如仍需注入 `dispatchMode`，只覆写 `launchPayload.liveSessionOverrides.dispatchMode`
   - 不要自己重新拼 `strategySignalBindings`
   - 不要再单独调用策略绑定接口

### 3. 确认提示

建议后续前端在用户点击与当前模板不同的卡片时，给出明确确认：

- 当前策略模板绑定将被覆盖
- runtime 订阅将切换到新的 `symbol + timeframe`
- 其它不属于当前模板 scope 的运行中 live session 可能会被停掉

如果点击的是当前同一模板，可直接执行，不必重复确认。

### 4. 当前模板展示

建议前端在 `AccountStage` 或 runtime 详情里明确展示：

- 当前模板名：`launchTemplateName`
- 当前模板 key：`launchTemplateKey`
- 当前 symbol：`launchTemplateSymbol`
- 当前 timeframe：`launchTemplateTimeframe`

这样用户就不需要再从 `strategySignalBindings` 或 runtime channel 里反推。

### 5. 成功态反馈

建议 toast 或状态反馈包含这几类信息：

- 模板已切换成功
- runtime 订阅已刷新
- 若 `stoppedLiveSessions > 0`，明确提示停掉了多少个旧 live session

示例：

`模板切换完成：已刷新 runtime 订阅，并停掉 1 个旧模板会话`

### 6. 错误态拆分

建议至少区分以下几类失败：

- 账户 binding 失败
- 模板 bindings 替换失败
- runtime plan 刷新失败
- launch / live session 启动失败
- 因存在活动持仓或订单而拒绝模板切换

最后一类尤其要单独提示，因为这不是普通网络错误，而是后端安全阻断。

## 前端暂时不要做的事

本轮不建议前端自行实现或假设以下行为：

- 不要继续逐条 `POST /api/v1/strategies/:id/signal-bindings`
- 不要假设模板可以并行叠加多个 symbol / timeframe
- 不要通过本地状态推断“当前模板”而忽略后端返回的 `launchTemplate*` 字段
- 不要把这次模板隔离改造成普通 live session 表单行为

## 验证建议

### 最小联调路径

1. 选择一个 LIVE account
2. 点击 `BTCUSDT 5m`
3. 确认：
   - launch 成功
   - runtime plan 只包含 BTCUSDT 5m 对应的 signal / trigger / feature
4. 再点击 `ETHUSDT 4h`
5. 再确认：
   - runtime plan 已不再包含 BTCUSDT
   - runtime / live session state 中的 `launchTemplate*` 字段已切到 ETHUSDT 4h

### 建议观察接口

- `GET /api/v1/live/launch-templates`
- `POST /api/v1/live/accounts/:accountId/launch`
- `GET /api/v1/signal-runtime/plan?accountId=...&strategyId=...`
- `GET /api/v1/signal-runtime/sessions`
- `GET /api/v1/live/sessions`

## 给前端同学的一句提醒

这轮以后，模板入口不再是“把固定三条 binding 追加进去”，而是“把模板当成一个受控切换动作”。

前端最重要的事情不是自己管理 bindings，而是：

- 正确展示切换语义
- 正确串行执行模板 steps
- 正确消费后端返回的模板上下文和切换结果
