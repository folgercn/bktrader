# Live Launch Template 隔离改造需求

> **目标**：修正“点击某个一键应用并启动模板后，runtime 继续同时订阅其它 symbol/timeframe”的问题。
>
> **推荐实施方式**：前后端一起改，以后端语义收敛为主，前端同步表达新语义与风险提示。
>
> **建议 PR 名称**：`fix/live-launch-template-isolation`

## 一、问题定义

当前 `AccountStage` 的“一键应用并启动”模板入口，用户直觉上会理解为：

- 点 `BTCUSDT 4h`，系统只启动这一套订阅
- 点 `ETHUSDT 4h`，系统只启动这一套订阅

但当前真实行为不是“独占切换”，而是“增量叠加”：

- 模板会把当前模板的 `signal / trigger / feature` 三条策略级 bindings 写入同一个策略
- 后端按 `account + strategy` 复用同一个 runtime session
- runtime 规划会吃到该策略下全部 ACTIVE bindings
- 因此之前点过 BTC，再点 ETH，runtime 最终会同时订阅 BTC + ETH

这会导致：

- 用户误以为“点一个模板只会跑一个模板”
- runtime 出现多 symbol / 多周期混合订阅
- UI 与运行时真实行为不一致
- 排查时容易误判为脏 runtime 或旧 session 残留

## 二、根因结论

根因不是前端一次点击触发多个策略，也不是废旧 runtime 残留，而是：

1. 所有 launch templates 当前共用同一个 `strategyId`
2. 模板执行时会把 bindings 追加/幂等写入该策略
3. 绑定逻辑只替换“同 key binding”，不会清理其它 symbol/timeframe
4. runtime session 复用维度是 `account + strategy`
5. runtime plan 会把该策略下全部匹配 bindings 纳入订阅集合

## 三、产品目标

本次改造后，系统应满足以下用户预期：

1. 从“一键应用并启动”入口点击某个模板后，runtime 只使用当前模板对应的一套 bindings。
2. 再点击另一个模板时，系统应显式切换到新的模板配置，而不是在旧模板之上继续叠加。
3. 用户在 UI 上能清楚知道当前 runtime 归属哪个模板、哪个 symbol、哪个 timeframe。
4. 问题要从后端语义上收敛，不能只靠前端入口“碰巧不触发”来规避。

## 四、推荐方案

### 推荐主方案：模板独占应用 + runtime 语义保持清晰

采用“模板独占应用”语义：

- 当用户通过模板入口启动某个模板时，后端应把该模板视为该 `account + strategy` 组合下的唯一活动模板输入集
- 在应用当前模板 bindings 前，清理该策略下不属于当前模板的旧 bindings
- 然后再启动或复用 runtime / live session

这样做的好处：

- 不需要立即重构整个 runtime 维度模型
- 语义最贴近当前 UI 的用户预期
- 改动面比“把 runtime 主键改成 account + strategy + symbol + timeframe”更小
- 能在不推翻现有启动链路的前提下，先消除 BTC/ETH 混订问题

### 为什么不是只改前端

只改前端只能约束当前这个按钮，不足以形成系统保证：

- 其它入口仍可写入多套策略 bindings
- 手工 API 调用仍可复现问题
- runtime 的真实复用和订阅规划语义没有变化

因此本次必须由后端定义“模板独占应用”的规则，前端只负责正确触发、展示和提示。

## 五、范围边界

### 本次要做

- 收敛“一键应用并启动”模板入口的后端语义
- 让点击某张模板卡后只留下当前模板对应的策略 bindings
- 让前端明确展示“这次启动会覆盖该策略下模板 bindings”
- 提供足够清晰的成功态、失败态和确认提示

### 本次不做

- 不改 `dispatchMode` 默认值
- 不引入 mainnet 行为变化
- 不重构核心 live 执行策略
- 不把整个 runtime/session 主键体系全面升级为 `account + strategy + symbol + timeframe`
- 不处理手工信号绑定页面的完整产品重设计

## 六、后端需求

### 1. 新增“模板独占应用”能力

后端需要提供一个受控的模板应用语义，保证模板入口不是“增量叠加”，而是“独占切换”。

建议满足的行为：

1. 根据模板内容计算当前模板应存在的 bindings 集合。
2. 在该 `strategyId` 下扫描现有策略 bindings。
3. 删除所有“不属于当前模板集合”的 ACTIVE bindings。
4. 对当前模板要求的三条 bindings 执行幂等 upsert。
5. 然后再进入 launch 流程。

注意：

- 清理范围必须谨慎限定为“模板管理的 bindings”
- 不能误删与模板无关、且明确由人工维护的非模板 bindings，除非团队确认模板入口就是该策略唯一管理入口
- 如果当前策略本来被设计为允许人工叠加 bindings，需要先把治理边界定清楚

### 2. 明确模板管理边界

后端需要决定并落地一种可审计的边界策略，推荐二选一：

1. 强约束模式
   模板入口管理的策略不得再混入其它 symbol/timeframe bindings。
2. 标记模式
   模板写入的 bindings 带上模板来源标识，只清理同来源 bindings，不碰人工 bindings。

若没有这个边界，后续仍容易出现“模板清理误删人工配置”或“人工配置重新污染模板 runtime”的问题。

### 3. launch 返回结果补充模板上下文

建议后端在 launch 结果或 runtime/session state 中补充足够的模板上下文，至少包括：

- 当前 symbol
- 当前 signal timeframe
- 当前模板 key 或等价标识
- 当前 runtime 依据哪组 bindings 启动

这样前端才能明确展示“当前运行的是哪一张模板”。

### 4. 幂等与安全要求

后端需要保证：

- 重复点击同一模板不会产生重复 bindings
- BTC 模板切换到 ETH 模板后，runtime plan 中不再出现 BTC bindings
- 失败时错误信息能区分：
  - 清理旧 bindings 失败
  - 写入新 bindings 失败
  - launch 失败

### 5. 后端验收标准

必须满足：

1. 先点 `BTCUSDT 4h`，再点 `ETHUSDT 4h` 后，runtime plan 中只剩 ETH 对应 bindings。
2. 当前运行中的 runtime `matchedBindings` / `subscriptions` 不再包含旧模板 symbol。
3. 重复点击同一模板不会制造重复 binding 脏数据。
4. `live session` 仍可按当前既有行为正常创建或复用。
5. 不得改变默认 `dispatchMode=manual-review`。

## 七、前端需求

### 1. 调整模板入口文案

前端需要把模板卡的语义表达清楚，不能继续让用户误解为“无副作用叠加”。

建议文案方向：

- `一键切换并启动`
- 或保留 `一键应用并启动`，但显式补一句：
  `会覆盖该策略当前模板绑定并切换到本模板`

### 2. 增加确认提示

当用户点击与当前运行模板不同的卡片时，前端应增加明确确认：

- 你正在切换模板
- 当前策略下旧模板 bindings 将被替换
- runtime 订阅将切换到新 symbol/timeframe

如果点击的是当前同一模板，可直接走幂等执行，不必重复确认。

### 3. 展示当前模板归属

前端需要在 `AccountStage` 或 runtime 面板中明确展示：

- 当前 runtime 使用的 symbol
- 当前 signal timeframe
- 当前模板 key / 模板名
- 当前策略 bindings 是否已与模板一致

避免用户只能从底层日志或 websocket 事件里反推。

### 4. 错误态与反馈

前端需要区分 3 类失败：

1. 清理旧模板失败
2. 写入新模板 bindings 失败
3. runtime / live session 启动失败

不能继续把所有失败都折叠成同一个泛化错误。

### 5. 前端验收标准

必须满足：

1. 用户能明确知道“点击模板会切换当前模板订阅”。
2. 切换模板后，页面展示的当前 runtime 信息与后端 runtime plan 一致。
3. 模板切换失败时，用户能知道失败发生在哪个阶段。
4. 不得误导用户认为 BTC 与 ETH 会并行保留，除非后端显式返回当前就是多模板模式。

## 八、联调建议

推荐联调顺序：

1. 先启动 `BTCUSDT 4h`
2. 确认 runtime plan 只包含 BTC 的三条 bindings
3. 再切换到 `ETHUSDT 4h`
4. 确认 runtime plan 已不再包含 BTC
5. 重复点击 `ETHUSDT 4h`
6. 确认没有新增重复 bindings

重点观察：

- `/api/v1/strategies/:id/signal-bindings`
- `/api/v1/signal-runtime/sessions`
- `/api/v1/signal-runtime/plan`
- 当前 live session state 中的 `symbol` / `signalTimeframe`

## 九、PR 拆分建议

建议拆成两个协作面，但归到同一个目标 PR：

### 后端负责

- 定义并实现模板独占应用语义
- 收敛 bindings 清理和幂等写入规则
- 保证 runtime plan 不再混入旧模板 symbol
- 提供足够的 runtime/template 上下文字段
- 补充单元测试和集成验证

### 前端负责

- 更新模板卡文案与确认交互
- 展示当前模板归属与切换结果
- 按后端新返回语义展示成功态与失败态
- 补充最小回归验证

## 十、PR 描述草稿

以下内容可直接作为 `.github/pull_request_template.md` 的初稿：

```md
## 目的
修正 live launch template 的语义偏差。此前从模板入口点击 BTC/ETH 不会独占切换当前模板，而是把新模板 bindings 追加到同一策略上，并由 account + strategy 级 runtime 统一复用，导致 runtime 同时订阅多个 symbol/timeframe。此次改动将模板入口收敛为“独占应用当前模板 bindings 并启动/复用 runtime”，使 UI 语义与系统真实行为一致。

## 本次改动风险定级 (参照 agent-risk-model.md)
- [ ] **L0** - 低风险 (无逻辑、纯样式、研究脚本、文档)
- [ ] **L1** - 中风险 (新增无害接口、辅助工具扩容)
- [x] **L2** - 高风险 (执行面板改动、核心数据流替换、CI/部署调整) -> **需w/f双重Review**
- [ ] **L3** - 绝对极高等级 (涉及 dispatchMode / 实盘 Live / Mock出界) -> **极度敏感预警**

## AI Agent 参与声明
- [ ] 纯人工手打改动
- [x] 这段属于由 LLM/Agent 生成的代码，但我已经确切通读并检查了

## 风险点 checklist
- [ ] `dispatchMode` 默认值有否变化？(变化则不可轻率上 main)
- [ ] 存在直接调用 `mainnet` 凭证或路由地址的硬编码？
- [ ] DB migration 是否具备向下兼容幂等性？
- [ ] 配置字段有没有无意被混改？

## 验证方式与测试证据
- [ ] 启动 BTCUSDT 模板后，runtime plan 仅包含 BTC bindings
- [ ] 切换到 ETHUSDT 模板后，runtime plan 不再包含 BTC bindings
- [ ] 重复点击同一模板不会产生重复 bindings
- [ ] 前端能明确展示当前模板归属、symbol、timeframe 和切换结果
```

## 十一、实施提醒

这是 live/runtime 相关改造，虽然不是直接改执行策略，但仍属于高风险协作任务。

实施时必须明确检查：

- 不要顺手改 `dispatchMode` 默认值
- 不要把 testnet / sandbox 语义打穿
- 不要扩大到 `internal/service/live*.go` 的无关重构
- 不要把“模板隔离”混成“实盘执行策略调整”

