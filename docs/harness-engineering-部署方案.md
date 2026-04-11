# bktrader Harness Engineering 部署方案

## 1. 文档目的

本文档用于为 `bktrader` 项目规划一套可落地的 Harness Engineering 建设方案。

这里的 Harness Engineering，不是单纯“接一个 AI 编码工具”，而是为 AI Coding Agent 配置一套可审计、可验证、可回滚、可持续演进的工作环境。重点是让模型在本项目中：

- 更快理解代码与业务边界
- 更少犯高风险错误
- 改完后能自己完成大部分验证
- 在 PR 阶段暴露问题，而不是在 testnet / live 阶段才发现
- 将人的经验沉淀为仓库内可复用规则，而不是停留在聊天和口头约定中

---

## 2. 为什么 bktrader 适合优先建设 Harness Engineering

相对历史包袱较重的多栈存量系统，`bktrader` 更适合作为 Harness Engineering 的首个完整试点，原因如下：

- 技术栈相对集中：后端以 Go 为主，前端以 React + Vite + TypeScript 为主，基础设施也较简单。
- 目录边界较清晰：`internal/`、`web/console/`、`db/migrations/`、`deployments/`、`docs/` 已具备较好的模块分层。
- 已有初步基础设施：仓库已有 CI、CD、AI PR Review、LLM 项目索引、协作规范。
- 业务风险高但边界明确：`live dispatch`、`position`、`fill`、`reconciliation`、`testnet/mainnet` 切换等风险点明确，适合转化为机器可执行的规则。
- 团队协作模式已成型：`CONTRIBUTING.md` 中已有角色边界和 Codex 使用边界，非常适合作为 harness 规则的前置制度层。

换句话说，`bktrader` 不是最简单的项目，但它已经具备“把工程经验固化进仓库”的土壤。

---

## 3. 建设目标

本项目建设 Harness Engineering 的目标，不是追求“AI 自动写所有代码”，而是建立以下能力：

### 3.1 上下文可达

让 Agent 在不依赖口头解释的前提下，能快速知道：

- 这个项目做什么
- 哪些模块能改，哪些不能碰
- 哪些行为属于高风险默认值
- 一次改动应如何自测
- 一次 PR 应如何审查

### 3.2 风险前移

把以下高风险问题尽量前移到本地和 PR 阶段发现：

- 默认 `dispatchMode` 被错误修改
- `testnet` 与 `mainnet` 路径混淆
- live order、fill、position、equity snapshot 状态不一致
- live session / signal runtime / recovery 逻辑不幂等
- migration 不可重复执行或对热路径造成性能风险
- 前端误触发高风险执行动作
- 部署脚本、Secrets、GHCR、SSH、rsync 等链路变成交互式或泄漏敏感信息

### 3.3 自动验证闭环

让 Agent 对一次变更能尽可能自主完成：

- 读取相关设计文档
- 识别影响范围
- 运行针对性的测试与构建
- 输出结构化验证结果
- 在 PR 阶段生成基于项目风险模型的 review 评论

### 3.4 经验沉淀

将团队对“什么是危险改动、什么是可接受改动、什么必须双人确认”的经验，沉淀为：

- 文档
- 脚本
- CI 规则
- PR 模板
- Agent 使用规范

---

## 4. Harness Engineering 要建设的能力清单

以下能力建议按优先级分批建设。

### 4.1 项目入口与 Agent 导航层

目标：

- 让任何 Agent 第一次进入项目时，有固定入口和固定阅读顺序。

建议建设内容：

- 新增根目录 `AGENTS.md`
- 保留并强化 `docs/llm-project-index.md`
- 在 `AGENTS.md` 中定义：
  - 项目目标
  - 高风险目录
  - 修改禁区
  - 推荐阅读顺序
  - 常用验证命令
  - PR 前自查要求

预期效果：

- 降低 Agent 上来就“全仓库乱扫”的概率
- 降低误改 `live` 默认行为、部署逻辑和公共配置的风险

### 4.2 风险分级与改动边界层

目标：

- 将现有协作规范中的“人类经验”转成 Agent 可识别的改动边界。

建议建设内容：

- 在 `AGENTS.md` 或独立文档中定义改动分级：
  - `L0` 低风险：文档、测试、前端样式、小范围工具脚本
  - `L1` 中风险：非默认行为的功能补充、回测与研究脚本、普通 API 扩展
  - `L2` 高风险：`live.go`、`live_execution.go`、`dispatchMode`、adapter 切换、migration
  - `L3` 极高风险：`testnet -> mainnet`、自动派单默认开启、真实执行链路切换
- 为每级定义：
  - 是否允许 Agent 直接改
  - 是否必须补测试
  - 是否必须人工双审
  - 是否必须单独 PR

预期效果：

- 让 Agent 不仅知道“能不能改”，还知道“改到什么程度必须停下来”

### 4.3 仓库内知识库层

目标：

- 让核心业务规则、运行规则、部署规则都能在仓库内查到，而不是靠聊天记录。

建议建设内容：

- 对现有 `docs/` 做结构强化，补齐下列文档：
  - `docs/agent-risk-model.md`
  - `docs/live-safety-invariants.md`
  - `docs/test-matrix.md`
  - `docs/pr-checklist.md`
- 对 `README.md` 中较长的运行说明进行拆分索引，避免单一入口过长。

建议固化的关键知识：

- live session 生命周期
- signal runtime readiness / freshness 判定规则
- order / fill / position / equity 一致性约束
- `manual-review` 与 `auto-dispatch` 的区别
- `mock` / `testnet` / `real` 的严格边界
- migration 的编写与回滚要求

预期效果：

- 让 Agent 做修改时能引用项目真实约束，而不是靠语言模型“猜”

### 4.4 自动验证与传感器层

目标：

- 让 Agent 改完不是只说“应该可以”，而是拿出机器验证结果。

建议建设内容：

- 统一定义最小验证矩阵：
  - Go 格式检查：`gofmt`
  - Go 测试：`go test ./...`
  - 后端构建：`go build ./cmd/platform-api`、`go build ./cmd/db-migrate`
  - 前端构建：`npm ci && npm run build`
  - Docker 构建校验
- 增加专项检查脚本：
  - 检查高风险默认值是否被修改
  - 检查 `dispatchMode` 默认值
  - 检查 `mainnet`、`real`、`auto-dispatch` 等敏感字符串是否进入不该进入的配置
  - 检查 migration 文件命名、幂等性约束和基础 SQL 风险
  - 检查 research 脚本是否对大 tick 数据做无界加载

预期效果：

- 把“风控红线”从 review 经验变成 CI 传感器

### 4.5 Agent 审查层

目标：

- 让 AI Review 不是泛泛提建议，而是基于本项目风险模型做有效审查。

当前已有基础：

- `.github/workflows/ai-review.yml`
- `scripts/ai_review_pr.py`
- `scripts/ai_review_prompt.md`

建议增强方向：

- 引入文件级风险标签
- 按目录切换不同审查提示词
- 对 `internal/service/live*.go`、`db/migrations/*`、`deployments/*`、`.github/workflows/*` 提高审查严格度
- 对高风险改动强制要求测试证据或 summary 中明确说明未覆盖风险

预期效果：

- 让 AI Review 更接近“项目专属 reviewer”，而不是通用 lint 评论机

### 4.6 运行时可观测层

目标：

- 让 Agent 能读取足够的运行信号，辅助判断改动是否真的安全。

建议建设内容：

- 统一后端关键日志格式，至少覆盖：
  - live session 启停
  - signal runtime 健康状态变化
  - dispatch 尝试与结果
  - order sync / fill reconcile / position recover
  - notification dispatch
- 为关键链路输出结构化事件字段
- 为 smoke test 保留机器可解析输出

预期效果：

- 让 Agent 不只看编译通过，还能通过日志和状态判断系统行为

### 4.7 部署与环境隔离层

目标：

- 让 Agent 和 CI/CD 能明确区分开发环境、testnet 环境、生产环境。

建议建设内容：

- 统一 env 文件层次：
  - `.env.local`
  - `.env.testnet`
  - `.env.production`
  - `configs/*.example.env`
- 对敏感环境变量分层命名：
  - testnet credentials
  - mainnet credentials
  - deploy secrets
  - Telegram / GHCR / SSH
- 在 CI/CD 中增加环境前置检查：
  - 当前部署目标是否 testnet
  - 当前 adapter 是否 real
  - 当前 dispatchMode 是否允许自动派单

预期效果：

- 降低“配置没看清就发到真实环境”的概率

---

## 5. 这套 Harness 要达到的实际结果

如果部署完成，理想状态下，`bktrader` 的日常开发会达到以下结果：

### 5.1 对普通功能迭代

- Agent 能读懂项目入口、结构、规则
- Agent 能在限定目录内做小步修改
- Agent 能跑完基本验证并输出结果
- PR 中 AI Review 能指出与交易安全相关的问题

### 5.2 对高风险 live 逻辑

- Agent 不会轻易跨越默认行为边界
- 一旦改动碰到 `dispatchMode`、adapter、mainnet/testnet、live recovery 等区域，系统会自动提示高风险
- PR 会要求人工双审和测试证据

### 5.3 对团队协作

- `wuyaocheng` 的核心交易判断和 `folgercn` 的 ops/deploy 边界更容易被 Agent 遵守
- review 精力更多放在真正高风险问题，而不是基础解释
- 新加入的 Agent 或新协作者进入项目的学习成本更低

---

## 6. 推荐部署方案

建议分三期部署，不建议一次性把所有规则铺满。

### 第一期：建立最小可用 Harness

目标：

- 先让 Agent 有入口、有边界、有最基础验证。

建议交付：

- 新增 `AGENTS.md`
- 补一份项目级 Harness 文档索引
- 统一 README 到 docs 的跳转关系
- 固化基础验证命令
- 保持现有 AI Review 工作流稳定运行

建议优先目录：

- `docs/`
- `.github/workflows/`
- `scripts/`
- `README.md`

完成标志：

- 新 Agent 进入仓库后，能在 5 到 10 分钟内建立项目全局认知
- 小范围 PR 可按固定流程完成“阅读 -> 修改 -> 验证 -> 审查”

### 第二期：增加高风险传感器

目标：

- 把项目最危险的改动模式机械化检测出来。

建议交付：

- 新增风险检查脚本
- 为高风险目录增加专项 CI
- 将 `dispatchMode`、`testnet/mainnet`、`real/mock`、migration 风险纳入校验
- 强化 AI Review 提示词和输出规则

建议优先目录：

- `internal/service/`
- `db/migrations/`
- `.github/workflows/`
- `deployments/`

完成标志：

- 高风险改动在 PR 阶段就能被标红或要求额外确认

### 第三期：接入运行时反馈闭环

目标：

- 让 Agent 不只审代码，还能理解运行状态与行为正确性。

建议交付：

- 关键 live 链路结构化日志
- smoke test 输出标准化
- testnet 最小闭环验证脚本固化
- 文档中明确“何时允许扩大 Agent 修改范围”

完成标志：

- Agent 能结合构建结果、测试结果、日志与 smoke 输出给出更可靠结论

---

## 7. 具体部署位置建议

建议在现有项目目录中按如下方式部署：

### 7.1 新增文档

- `AGENTS.md`
  - 项目总入口
  - Agent 工作规则
  - 阅读顺序
  - 禁区
  - 验证命令
- `docs/harness-engineering-部署方案.md`
  - 即本文档
- `docs/agent-risk-model.md`
  - 风险分级与高风险目录说明
- `docs/live-safety-invariants.md`
  - live 关键不变量
- `docs/test-matrix.md`
  - 不同目录改动对应的测试矩阵

### 7.2 新增或增强脚本

- `scripts/check_high_risk_defaults.sh`
- `scripts/check_env_safety.sh`
- `scripts/check_migration_safety.py`
- `scripts/check_large_data_guard.py`
- `scripts/run_changed_scope_checks.sh`

### 7.3 增强 CI/CD

- 在 `.github/workflows/ci.yml` 中加入高风险检查步骤
- 在 `.github/workflows/ai-review.yml` 中加入按目录分层审查策略
- 保持 `cd.yml` 的部署步骤非交互、可重复、可审计

---

## 8. 部署时的原则

### 8.1 先约束，再放权

不要一开始就让 Agent 深度参与 `live dispatch` 主链路。
应先从：

- 文档
- 测试
- CI/CD
- 前端非高风险区域
- 回测和研究脚本

这些区域开始，等验证链完善后，再逐步扩大范围。

### 8.2 规则必须能执行

如果某条规则只能靠人解释，就不算真正进入 harness。
优先把规则做成：

- 检查脚本
- CI 任务
- PR 模板
- Agent 指令

### 8.3 高风险默认值必须单独看守

以下项目应被视为一级红线：

- `dispatchMode`
- `auto-dispatch`
- `testnet -> mainnet`
- `mock -> real`
- live adapter 默认选择
- 真实凭证引用路径
- order / fill / position / reconcile 的一致性逻辑

### 8.4 人工 review 仍然保留最终裁决权

Harness Engineering 的目标是减少无效人工消耗，不是移除人工判断。
对于真实交易和默认行为变更，仍应坚持：

- 单独 PR
- 双人 review
- 测试证据
- 环境确认

---

## 9. 推荐落地顺序

建议按下面顺序执行：

1. 先补 `AGENTS.md`
2. 再补风险模型和测试矩阵文档
3. 再补高风险检查脚本
4. 再把脚本接进 CI
5. 最后再扩大 Agent 可修改范围

这是比较稳的顺序，因为它先解决“让 Agent 读懂和守规则”，再解决“让 Agent 自动化跑更深的链路”。

---

## 10. 成功标准

当以下指标成立时，可以认为 `bktrader` 的 Harness Engineering 进入可用状态：

- 新 Agent 进入仓库后能按固定入口完成上下文建立
- 普通 PR 能稳定走通：阅读、修改、构建、测试、AI Review
- 高风险改动会被自动识别并要求更高审查等级
- CI/CD 能对环境、部署、敏感默认值提供基础防呆
- 团队对“哪些改动可以让 Agent 做，哪些必须人工主导”形成稳定共识

---

## 11. 结论

`bktrader` 非常适合作为 Harness Engineering 的首个完整试点项目。

它的优势不是“完全没有风险”，恰恰相反，是它的高风险点已经非常明确，因此更值得把经验工程化。只要按“入口清晰、边界明确、验证前移、规则机械化、环境隔离”的路线推进，这个项目可以较快建立一套真正可用的 Agent 工程体系。

建议本项目将 Harness Engineering 定位为：

- 一套项目级工程治理能力
- 一套 AI Coding Agent 的安全工作系统
- 一套把团队经验沉淀为仓库资产的机制

而不是一套“让 AI 自由发挥”的工具接入方案。
