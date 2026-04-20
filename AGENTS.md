# bktrader AI Agent 导航指南 (AGENTS)

> **致所有介入本项目的 AI 编码助手 / Agent：**
> 如果你是第一次进入本项目，这篇文档是你必须首先阅读的**最优先级**内容，它是 bktrader Harness Engineering 的入口。

## 1. 项目概述

**bktrader** 是一个基于 Go (后端) + React/Vite (前端) 的强一致性加密货币自动化交易系统。
核心重点不在于提供复杂的“智能推荐”，而是在于**严格约束执行安全性**（`live execution`, `order management`, `reconciliation`, `dispatch rules`）。

## 2. 核心记忆与工具 (Core Memory)

- **图谱** == **graphify** (本项目的知识图谱工具)
- **UI 规范** == **shadcn** (本项目的基础 UI 组件库与规范指南，见 [.skills/shadcn/SKILL.md](.skills/shadcn/SKILL.md))
- **Research Baseline**: 研究/回测语境下，当前长期 baseline 视为 `dir2_zero_initial=true` + `zero_initial_mode=reentry_window`，并固定使用 `reentry_size_schedule=[0.20, 0.10]`、`max_trades_per_bar=2`。也就是同一根 signal bar 内，第 1 次真实下单为 `20%`，第 2 次真实下单为 `10%`。除非人类明确要求复现历史对照组，否则不要再默认把 `position` 或旧的 `10%/5%/2.5%` 方案当作 baseline 反复判断。

### 环境路径规约 (Environment Paths)

由于部分 Shell 会话环境受限，请在涉及以下工具的操作中优先使用绝对路径，或参考 [docs/AGENT_PATHS.md](docs/AGENT_PATHS.md)：
- **GitHub CLI (gh)**: `/usr/local/bin/gh`
- **Git**: `/usr/local/bin/git`

### graphify 规则

This project has a graphify knowledge graph at `graphify-out/`.

- Before answering architecture or codebase questions, read `graphify-out/GRAPH_REPORT.md` for god nodes and community structure.
- If `graphify-out/wiki/index.md` exists, navigate it instead of reading raw files.
- Do not rebuild graphify after every code change in a session.
- Rebuild graphify immediately before `git push` by running `python3 -c "from graphify.watch import _rebuild_code; from pathlib import Path; _rebuild_code(Path('.'))"` if needed.
- **Automated Workflow**: A `pre-push` git hook is installed and should be the default path for rebuilding the graph before every `git push`.

## 3. 修改禁区与高风险目录

**不要在没有与人类明确沟通（显式 approval）的情况下擅自修改以下高风险区域**：

- 🚨 `internal/service/live*.go` (Live 逻辑是全系统的核心禁区，修改易造成实盘资金损失)
- 🚨 `internal/service/execution_strategy.go` (交易执行策略)
- 🚨 `deployments/`, `.github/workflows/` (涉及 CI/CD 与凭证)
- 🚨 默认的强约束参数配置 (例如系统中的 `dispatchMode` 必须默认为 `manual-review`，杜绝一切隐式 `auto-dispatch`)

详见 [agent-risk-model](docs/agent-risk-model.md) 了解完整的 L0 到 L3 风险定级。

## 4. 行动准则与强制阅读顺序

当你接受任务时，除了查阅本页面，请按下述顺序查阅：
1. [docs/index.md](docs/index.md): 项目级 Harness 与文档清单
2. [docs/llm-project-index.md](docs/llm-project-index.md): 详细的目录边界说明
3. [CONTRIBUTING.md](CONTRIBUTING.md): wuyaocheng 和 folgercn 两位主核心贡献者的协作纪律
4. `graphify-out/GRAPH_REPORT.md` (如有必要): 理解项目具体实现的依赖拓扑。

## 5. 常规验证手段

你在提交代码后必须做以下自查（若相关）：

**后端**：
```bash
gofmt -w .
go test ./...
go build ./cmd/platform-api
go build ./cmd/db-migrate
```

**前端**：
在结束任何前端代码编辑前，**必须**运行以下指令进行静态类型校验（路径见 `docs/AGENT_PATHS.md`）：
```bash
# 示例：使用本地 tsc 执行校验
cd web/console
./node_modules/.bin/tsc --noEmit src/pages/AccountStage.tsx --jsx react-jsx --esModuleInterop --skipLibCheck --target esnext --moduleResolution node --allowSyntheticDefaultImports

# 全量构建验证 (如必要)
npm run build
```

**Smoke Test (实盘会话可用性回归测试)**：
Smoke Test 主要为部署验证预留。如遇重构，请人工执行：
```bash
bash scripts/testnet_live_session_smoke.sh
```

## 6. PR 与提交约束

你协助提交的改动或是撰写的 PR 必须：
- 严格使用 `.github/pull_request_template.md`，并在描述中声明你的参与。
- 不能隐式破坏 `testnet` 到 `mainnet` 的沙盒设定。
## 7. 运行时恢复 / 接管专项规则（仅在相关改动时强制生效）

以下规则**不是全项目通用基础规范**，而是 **runtime recovery / takeover / passive-close / reconcile 专项规则**。

### 7.1 什么时候必须严格遵守

如果本次改动涉及以下任一内容，则必须严格遵守本节：

- `internal/service/live*.go`
- `internal/service/execution_strategy.go`
- 恢复 / 接管逻辑
- 被动平仓逻辑
- dispatch / 最终下单边界
- 交易所同步 / reconcile
- WebSocket 重连
- session / runtime state 语义

如果不涉及以上内容，则不需要套用本规则。

### 7.2 三类状态必须区分清楚

修改前必须明确：

- **事实源**：交易所 REST 对账结果、已确认持仓、已确认订单/成交
- **缓存态**：session.State、livePositionState、recoveredPosition、virtualPosition、WS 临时状态
- **推导态**：planIndex、intent、proposal、strategy decision

❗ 禁止把缓存态或推导态直接当成交易事实。

### 7.3 未对账恢复态不得自动交易

恢复 / 接管 / WS 重连后的状态：

- ❌ 未对账 → 不允许 auto-dispatch
- ❌ 未对账 → 不允许自动被动平仓
- ❌ 未对账 → 不允许继续推进策略

允许的行为：

- 标记 stale / conflict / error
- 进入 close-only takeover
- 等待人工处理

### 7.4 恢复状态必须显式

禁止用多个 flag “猜状态”。

必须明确：

- 状态名称
- 状态允许动作
- 状态禁止动作
- 状态转移条件

至少回答：

- 能不能开仓？
- 能不能平仓？
- 能不能 auto-dispatch？

### 7.5 WS ≠ REST（必须区分职责）

- WS：实时事件
- REST：权威校验

❗ 强制要求：

- 不允许 WS 直接作为最终事实
- WS 重连 → 必须触发 REST 校验
- 不允许“WS 连上就当恢复正常”

### 7.6 执行边界必须二次校验

在最终 submit 前必须再次检查：

- 订单类型（entry / exit / recovered close）
- reduceOnly 是否正确
- HEDGE / ONE_WAY payload 是否正确
- positionSide 是否匹配
- 当前状态是否允许执行

❗ 禁止只在 proposal 阶段校验。

### 7.7 禁止顺手扩大修改范围

- 一个 PR 只解决一个问题域
- 不允许顺手改状态机 / 执行链 / reconcile
- 不允许“顺手优化结构”导致语义变化

推荐：

- 一 issue 一分支
- 一 issue 一 PR
- merge 串行

### 7.8 必须补 recovery regression tests

至少覆盖：

- DB takeover
- exchange takeover
- mismatch 场景
- duplicate exit 防护
- partial fill + restart
- passive close payload

❗ 禁止只写 helper 测试。

### 7.9 AI / Codex 必须遵守

AI 修改本范围代码时必须输出：

- root cause
- 修改点
- 行为变化
- 新增测试

禁止：

- 一个 PR 改多个 issue
- 为了通过测试放宽校验
- 擅自扩大 scope
- 未说明事实源直接改逻辑

### 7.10 相关参考文档

必须结合：

- [docs/runtime-recovery-stabilization-summary.md](docs/runtime-recovery-stabilization-summary.md)
- [docs/runtime-recovery-extension-coding-rules.md](docs/runtime-recovery-extension-coding-rules.md)

