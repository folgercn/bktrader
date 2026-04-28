# bktrader AI Agent 导航指南 (AGENTS)

> **致所有介入本项目的 AI 编码助手 / Agent：**
> 如果你是第一次进入本项目，这篇文档是你必须首先阅读的**最优先级**内容，它是 bktrader Harness Engineering 的入口。

## 1. 项目概述

**bktrader** 是一个基于 Go (后端) + React/Vite (前端) 的强一致性加密货币自动化交易系统。
核心重点不在于提供复杂的"智能推荐"，而是在于**严格约束执行安全性**（`live execution`, `order management`, `reconciliation`, `dispatch rules`）。

## 2. 核心记忆与工具 (Core Memory)

- **图谱** == **graphify** (本项目的知识图谱工具，资产位于 `graphify-out/`)
- **UI 规范** == **shadcn** (本项目的基础 UI 组件库与规范指南，见 [.skills/shadcn/SKILL.md](.skills/shadcn/SKILL.md))
- **Research Baseline**: 研究/回测语境下，当前长期 baseline 视为 `dir2_zero_initial=true` + `zero_initial_mode=reentry_window`，并固定使用 `reentry_size_schedule=[0.20, 0.10]`、`max_trades_per_bar=2`。也就是同一根 signal bar 内，第 1 次真实下单为 `20%`，第 2 次真实下单为 `10%`。除非人类明确要求复现历史对照组，否则不要再默认把 `position` 或旧的 `10%/5%/2.5%` 方案当作 baseline 反复判断。
- **环境路径**: 工具绝对路径见 `AGENTS.local.md`（本地私有）。若不存在，参考 [docs/AGENT_PATHS.md](docs/AGENT_PATHS.md)。

### graphify 规则

- Before answering architecture or codebase questions, read `graphify-out/GRAPH_REPORT.md` for god nodes and community structure.
- If `graphify-out/wiki/index.md` exists, navigate it instead of reading raw files.
- Do not rebuild graphify after every code change in a session.
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
1. [docs/llm-project-index.md](docs/llm-project-index.md): 代码目录结构与模块边界
2. [docs/index.md](docs/index.md): 分级文档导航（必读 / 按需 / 参考）
3. [CONTRIBUTING.md](CONTRIBUTING.md): wuyaocheng 和 folgercn 两位主核心贡献者的协作纪律
4. `graphify-out/GRAPH_REPORT.md` (如有必要): 理解项目具体实现的依赖拓扑。

若任务是线上告警 / 生产日志 / `stale-source-states` / Binance REST 限流排查，必须优先阅读 [docs/production-log-troubleshooting.md](docs/production-log-troubleshooting.md)。

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
在结束任何前端代码编辑前，**必须**运行以下指令进行静态类型校验。**注意：** 禁止在验证时滥用 `--skipLibCheck`，必须确保命令行标志与 `tsconfig.json`（如 `moduleResolution: Bundler`）保持同步，严禁忽略 IDE 中已出现的红线报错。
```bash
cd web/console
./node_modules/.bin/tsc --noEmit src/pages/AccountStage.tsx --jsx react-jsx --esModuleInterop --target esnext --moduleResolution Bundler --allowSyntheticDefaultImports

# 全量构建验证 (如必要)
npm run build
```

**Smoke Test**：部署验证预留。如遇重构，请人工执行 `bash scripts/testnet_live_session_smoke.sh`。

## 6. PR 与提交约束

你协助提交的改动或是撰写的 PR 必须：
- 严格使用 `.github/pull_request_template.md`，并在描述中声明你的参与。
- 不能隐式破坏 `testnet` 到 `mainnet` 的沙盒设定。

## 7. Review 黄金规则（从 155+ PR 提炼）

**修改 `internal/` 下任何代码前必须过一遍**。完整案例与 PR 出处详见 [docs/pr-lessons-learned.md](docs/pr-lessons-learned.md)。

1. 成功/失败路径必须统一 accounting
2. 不允许"失败装成功"
3. 同一个判定只能有一个入口
4. 全链路一致性（API → 内存 → 持久化 → 判断函数）
5. 缓存态 ≠ 事实
6. 未对账不允许自动执行
7. PR 不能静默扩大范围
8. Legacy 数据迁移需要兼容性测试
9. 热路径不能全表扫描
10. 自动 resume / dispatch 必须有显式前置条件
11. 精度和容差只能有一个入口 — 必须使用 `internal/service/precision_tolerance.go`

## 8. 高频踩坑模式速查

**改 `internal/` 代码前先自查**，详细案例见 [docs/pr-lessons-learned.md](docs/pr-lessons-learned.md)：

| 类别 | 核心要点 |
|------|----------|
| 状态一致性 | N 个路径写同一字段 → 收敛统一 helper |
| 零值与默认值 | Go 零值 ≠ "未提供"；float 写入前检查 NaN/Inf |
| 身份与生命周期 | 缓存 key 必须唯一标识当前仓位 |
| 执行安全边界 | 自动 resume 不能靠排除法；reconcile 只信任本系统活跃订单 |
| 性能与限流 | 热路径禁止全表扫描；外部 API 统一 gating |
| 监控与告警 | 告警 ID 必须稳定；flap suppression 覆盖完整状态循环 |
| 精度与容差 | 数量/价格/notional 比较走统一 helper |

## 9. AI Agent 协作纪律

> 以下规则针对 AI Agent 的已知行为盲区。**所有 Agent 在本项目中必须自我约束**。

1. **禁止"一次修完"** — 发现修改范围超出原始目标时，**主动拆分到新 issue + 新分支**
2. **零值语义是 Agent 盲区** — 涉及 fallback / merge / default 的逻辑，**必须请求人工 review**
3. **测试必须覆盖失败路径** — 补测试时，**至少包含一个 failure path**（adapter resolve 失败、NaN 输入等）
4. **修改 recovery 代码前必须读状态机** — 先读完 §10 的触发条件和 [docs/runtime-recovery-extension-coding-rules.md](docs/runtime-recovery-extension-coding-rules.md)
5. **高风险 PR 仍需人工主审** — L2/L3 级别的 PR，**AI review 只做辅助**，必须等待人工主审通过
6. **严禁"跳过"静态校验错误** — 禁止滥用 `--skipLibCheck` 或忽略 IDE 报错。若命令行通过但 IDE 报错，以 IDE 为准，必须对齐环境配置。

## 10. 运行时恢复 / 接管专项规则

> 以下为精简速查。完整规则详见 [docs/runtime-recovery-extension-coding-rules.md](docs/runtime-recovery-extension-coding-rules.md)，历史背景见 [docs/runtime-recovery-stabilization-summary.md](docs/runtime-recovery-stabilization-summary.md)。

### 何时生效

如果本次改动涉及以下任一内容，则必须严格遵守：
`internal/service/live*.go`、`execution_strategy.go`、恢复/接管逻辑、被动平仓、dispatch/最终下单、交易所同步/reconcile、WebSocket 重连、session/runtime state 语义。

### 核心禁令

| 禁令 | 说明 |
|------|------|
| 缓存态 ≠ 事实 | 事实源 = 交易所 REST 对账；缓存态 = session.State / WS 临时状态；推导态 = intent / proposal |
| 未对账不得自动交易 | 恢复/接管/WS 重连后：❌ auto-dispatch、❌ 被动平仓、❌ 推进策略 |
| WS ≠ REST | WS 提供实时事件，REST 提供权威校验。WS 重连必须触发 REST 对账 |
| 执行边界二次校验 | 最终 submit 前必须再次检查订单类型、reduceOnly、positionSide |
| 禁止顺手扩大范围 | 一 PR 一问题域，一 issue 一分支 |
| 恢复状态必须显式 | 禁止用多个 flag 猜状态，必须用显式状态机 |
| 必须补 recovery 回归测试 | 至少覆盖：DB/exchange takeover、mismatch、duplicate exit、partial fill + restart |
