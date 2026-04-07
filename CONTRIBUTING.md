# bktrader 协作开发规范

## 目标

保持 `main` 分支稳定、可审查且始终可运行，同时支持核心交易开发和运维/部署工作的高效并行。

## 角色与职责边界

### `wuyaocheng`
主要负责核心代码、交易策略、业务逻辑、算法实现。

核心负责范围：
- `internal/service/**` （核心业务逻辑）
- `internal/domain/**`
- 策略相关代码
- 交易信号、决策、intent、dispatch 流程
- 核心 domain / engine / strategy 设计
- 功能测试、策略测试
- 和交易执行逻辑直接相关的代码

**总结**：凡是“系统为什么这样交易”的代码，主要归 `wuyaocheng` 负责。

### `folgercn`
主要负责运维、部署、CI/CD、环境、配置、安全、发布。

核心负责范围：
- GitHub Actions (`.github/**`)
- CI/CD 脚本
- Docker / Dockerfile / compose (`deployments/**` 或对应目录)
- 部署脚本 (`scripts/**`)
- 环境变量模板及运行时配置
- 运维与部署文档
- 监控、日志、发布流程
- 服务器对接、测试环境、生产环境配置

**总结**：凡是“代码怎么跑起来、怎么上线、怎么稳定运行”的内容，主要归 `folgercn` 负责。

## 共有文件（高度易冲突）

这些区域最容易产生冲突，任何人在修改前**必须先在群里打招呼**：
- `README.md`
- `.gitignore`
- `.dockerignore`
- 公共配置文件
- 公共接口定义和请求/响应模型数据结构
- `internal/service/live.go` 这种大文件
- 默认行为配置，比如 `dispatchMode`

## 分支规则

不需要繁重的 Git Flow，采用简单的策略：

### 固定分支
- `main`：稳定主线集成段，保持绝对可运行。只接受 PR 合并，**不直接推 (push)**。

### 个人工作分支
- `dev/core-*`：`wuyaocheng` 专属（如交易逻辑、策略开发等）
- `dev/ops-*`：`folgercn` 专属（如运维、CI/CD、部署脚本等）

### 其他类型分支
- `feature/*`：需要保留一段时间的大功能联调分支
- `codex/*`：Codex 自动或半自动生成的实验分支（依然需要人工审查评测）

**命名示例**：
- `dev/core-order-dispatch`
- `dev/ops-github-actions`
- `feature/runtime-integration-testnet`
- `codex/add-signal-tests`

## 日常协作流程

1. **每天开始前同步主线**：
   从最新 `main` 分支拉自己的代码
   ```sh
   git checkout main
   git pull origin main
   git checkout -b dev/core-xxx # 或 dev/ops-xxx
   ```
2. **开发时规则**：
   - 一个分支只做一件事
   - 一个 PR 只解决一个问题
   - 不要顺手大面积改格式
   - 不要把“逻辑修改 + 部署修改 + 文档修改”全混合在一个 PR 中
   - 不要两个人同时改同一个大文件的同一段
3. **提交前自查**：
   - 必须先看 `git diff --stat` 和 `git diff`
   - 如果使用了 Codex，更要严查，防止工具“顺手改了不该改的地方”
4. **重新同步 main**：
   发 PR 后若 `main` 变化，进行同步：
   - 推荐小分支用 `rebase`
   - 推荐公共联调分支用 `merge` 

## PR 审查与合并权属

- **wuyaocheng 的核心功能 PR** → `folgercn` review
- **folgercn 的运维/CI/CD PR** → `wuyaocheng` review
- **涉及默认行为等高风险修改 PR** → 必须双方都 review 后才能合进主线

### 高风险修改（必须单独开 PR 且需双人确认）
以下改动必须单独提 PR 明确说明，不得混在“联调 checkpoint” 或 其他混合 PR 中：
- `manual-review` 改成 `auto-dispatch` (自动派单默认开启)
- `mock` 切换为真实执行 (`real`)
- `testnet` 切换为 `mainnet` 
- 环境依赖改变（如 memory 切换成 Postgres）
- 会影响线上行为的默认值变化
- 生产环境变量含义变化
- 数据模型大幅更改
- 交易执行路径改变

## Codex 使用边界

为避免代码混乱，设定 AI 编码工具使用红线：

1. **只允许处理“小范围任务”**：
   - 修一个函数 / 补一处测试 / 改一个模块 / 写写文档 / 调整某步 CI
   - **禁止**：跨多个模块大重构、大包揽一次性改大量模块、悄悄修改默认交易行为。
2. **给指令时必须限定范围**：
   必须使用明确约束的话术。例如：“只允许修改 `internal/service/live.go` 和对应测试，不允许改部署和 GH Actions，不准改默认 `dispatchMode`”。
3. **输出不直接进主干**：
   - Codex 改代码先跑在 `codex/*` 分支
   - 由人工 Review 或调试好后再进入两人各自的 `dev/core-*` 或 `dev/ops-*`
   - 最后以标准 PR 进入 `main`

## 最简协作口诀

1. `main` 只接受 PR 合并，不直接推
2. `folgercn` 负责 ops / CI/CD / deploy / env / workflow
3. `wuyaocheng` 负责 core / strategy / runtime / trading logic
4. 公共大文件改动前先说一声
5. 默认行为变化必须单独 PR 
6. Codex 只在限定目录内修改
7. 合并前提必须看 `git diff` 
8. 高风险 PR 必须两人都看
9. checkpoint PR 可以合，但要明确写清“未完成点”
10. `main` 始终尽量保持可运行
