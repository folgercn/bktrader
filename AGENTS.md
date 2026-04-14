# bktrader AI Agent 导航指南 (AGENTS)

> **致所有介入本项目的 AI 编码助手 / Agent：**
> 如果你是第一次进入本项目，这篇文档是你必须首先阅读的**最优先级**内容，它是 bktrader Harness Engineering 的入口。

## 1. 项目概述

**bktrader** 是一个基于 Go (后端) + React/Vite (前端) 的强一致性加密货币自动化交易系统。
核心重点不在于提供复杂的“智能推荐”，而是在于**严格约束执行安全性**（`live execution`, `order management`, `reconciliation`, `dispatch rules`）。

## 2. 核心记忆与工具 (Core Memory)

- **图谱** == **graphify** (本项目的知识图谱工具)

### graphify 规则

This project has a graphify knowledge graph at `graphify-out/`.

- Before answering architecture or codebase questions, read `graphify-out/GRAPH_REPORT.md` for god nodes and community structure.
- If `graphify-out/wiki/index.md` exists, navigate it instead of reading raw files.
- After modifying code files in this session, use the repo helper to locate a `graphify`-capable interpreter and rebuild the graph:
  ```bash
  GRAPHIFY_PYTHON="$(bash scripts/find_graphify_python.sh)"
  "$GRAPHIFY_PYTHON" -c "from graphify.watch import _rebuild_code; from pathlib import Path; _rebuild_code(Path('.'))"
  ```
- **Automated Workflow**: 仓库内已提供版本化 `pre-push` hook：`.githooks/pre-push`。首次 clone 后请执行 `bash scripts/install_git_hooks.sh`，这样每次 `git push` 前会自动重建 graphify 图谱并执行范围感知自查。

### 面向 Gemini 的技能组 (Skills)

本项目现已对接部分 `.agents/skills`。遇到相关场景时，请务必先查阅 Skill：
- **前端 UI 修改**：必须先参考 `Frontend-Design-System-Skill`。
- **React 动效控制**：必须先参考 `React-Polanyi-Interaction-Skill`。

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
```bash
cd web/console && npm ci && npm run build
```

**Smoke Test (实盘会话可用性回归测试)**：
Smoke Test 主要为部署验证预留。如遇重构，请人工执行：
```bash
bash scripts/testnet_live_session_smoke.sh
```

**范围感知自查**：
```bash
bash scripts/run_changed_scope_checks.sh --staged
```

## 6. PR 与提交约束

你协助提交的改动或是撰写的 PR 必须：
- 严格使用 `.github/pull_request_template.md`，并在描述中声明你的参与。
- 对照 `docs/pr-checklist.md` 补齐验证证据和剩余风险说明。
- 不能隐式破坏 `testnet` 到 `mainnet` 的沙盒设定。
- 不能破坏 `mock` 模式或制造未授权的 `real` 数据分发。
