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
5. [docs/pr-lessons-learned.md](docs/pr-lessons-learned.md): **从 155 个 PR 提炼的实战踩坑模式和 review 黄金规则**。在修改 `internal/service/` 下的代码前**必读**。

若任务是线上告警 / 生产日志 / `stale-source-states` / Binance REST 限流排查，必须优先阅读 [docs/production-log-troubleshooting.md](docs/production-log-troubleshooting.md)，其中记录了生产服务器 SSH 入口与日志目录。

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
在结束任何前端代码编辑前，**必须**运行以下指令进行静态类型校验（路径见 `docs/AGENT_PATHS.md`）。**注意：** 禁止在验证时滥用 `--skipLibCheck`，必须确保命令行标志与 `tsconfig.json`（如 `moduleResolution: Bundler`）保持同步，严禁忽略 IDE 中已出现的红线报错。
```bash
# 示例：使用本地 tsc 执行校验 (确保不跳过必要的库检查)
cd web/console
./node_modules/.bin/tsc --noEmit src/pages/AccountStage.tsx --jsx react-jsx --esModuleInterop --target esnext --moduleResolution Bundler --allowSyntheticDefaultImports

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
## 7. Review 黄金规则（从 155 个 PR 提炼）

以下 11 条规则来自项目实际 PR review，不是理论推导。**修改 `internal/` 下任何代码前必须过一遍**：

1. **成功/失败路径必须统一 accounting** — 每个出口都走统一 helper，不允许散落多处
2. **不允许"失败装成功"** — fallback 失败必须真实返错，不能静默吞掉
3. **同一个判定只能有一个入口** — alerts 和 snapshot 不能各自维护平行条件
4. **全链路一致性** — 配置从 API → 内存 → 持久化 → 判断函数必须同语义
5. **缓存态 ≠ 事实** — WS 状态、内存快照、推导结果不能直接当交易事实
6. **未对账不允许自动执行** — recovery / takeover 后必须等 REST 对账完成
7. **PR 不能静默扩大范围** — "加监控"不能顺手变成"改 live sync 行为"
8. **Legacy 数据迁移需要兼容性测试** — 隐式改身份键必须补回归测试
9. **热路径不能全表扫描** — live sync / reconcile 路径上的查询必须有索引
10. **自动 resume / dispatch 必须有显式前置条件** — 不能靠"看起来没问题就恢复"
11. **精度和容差只能有一个入口** — 订单数量、交易所 step/tick、notional 边界禁止散落 `1e-9` / `math.Abs(...)`；必须使用 `internal/service/precision_tolerance.go` 的统一 helper，并补边界测试

完整案例与 PR 出处详见 [docs/pr-lessons-learned.md](docs/pr-lessons-learned.md)。

## 8. 高频踩坑模式速查

以下是 review 中出现频率最高的 7 类问题。**改 `internal/` 代码前先自查**，详细案例见 [docs/pr-lessons-learned.md](docs/pr-lessons-learned.md)：

1. **状态一致性** — N 个路径写同一字段 → 收敛统一 helper；同一判定只能有一个入口
2. **零值与默认值** — Go 零值 ≠ "未提供"；fallback/merge 必须显式定义零值语义；float 写入前检查 NaN/Inf
3. **身份与生命周期** — 缓存 key 必须唯一标识当前仓位；空值回填如改身份键 = 语义迁移，必须补兼容性测试
4. **执行安全边界** — 自动 resume 不能靠排除法；reconcile 只信任本系统活跃订单；settlement 未完成不能抢先落账
5. **性能与限流** — 热路径禁止全表扫描；外部 API 必须统一 gating + 限流
6. **监控与告警** — 告警 ID 必须稳定（不含时间戳）；flap suppression 必须覆盖完整状态循环
7. **精度与容差** — 数量/价格/notional 比较必须走统一 helper；新增交易所规则时必须明确 absolute/relative tolerance 和 round/ceil 语义

## 9. AI Agent 协作纪律

> 以下规则针对 AI Agent 的已知行为盲区。**所有 Agent 在本项目中必须自我约束**。

1. **禁止"一次修完"** — 发现修改范围超出原始目标时，**主动拆分到新 issue + 新分支**
2. **零值语义是 Agent 盲区** — 涉及 fallback / merge / default 的逻辑，**必须请求人工 review**
3. **测试必须覆盖失败路径** — 补测试时，**至少包含一个 failure path**（adapter resolve 失败、NaN 输入等）
4. **修改 recovery 代码前必须读状态机** — 先读完 §10 的状态机定义和 §7 的 review 黄金规则
5. **高风险 PR 仍需人工主审** — L2/L3 级别的 PR，**AI review 只做辅助**，必须等待人工主审通过
6. **严禁"跳过"静态校验错误** — 禁止滥用 `--skipLibCheck` 或忽略 IDE 报错。若命令行通过但 IDE 报错，以 IDE 为准，必须对齐环境配置。

## 10. 运行时恢复 / 接管专项规则（仅在相关改动时强制生效）

以下规则**不是全项目通用基础规范**，而是 **runtime recovery / takeover / passive-close / reconcile 专项规则**。

### 10.1 什么时候必须严格遵守

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

### 10.2 三类状态必须区分清楚

修改前必须明确：

- **事实源**：交易所 REST 对账结果、已确认持仓、已确认订单/成交
- **缓存态**：session.State、livePositionState、recoveredPosition、virtualPosition、WS 临时状态
- **推导态**：planIndex、intent、proposal、strategy decision

❗ 禁止把缓存态或推导态直接当成交易事实。

### 10.3 未对账恢复态不得自动交易

恢复 / 接管 / WS 重连后的状态：

- ❌ 未对账 → 不允许 auto-dispatch
- ❌ 未对账 → 不允许自动被动平仓
- ❌ 未对账 → 不允许继续推进策略

允许的行为：

- 标记 stale / conflict / error
- 进入 close-only takeover
- 等待人工处理

### 10.4 恢复状态必须显式

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

### 10.5 WS ≠ REST（必须区分职责）

- WS：实时事件
- REST：权威校验

❗ 强制要求：

- 不允许 WS 直接作为最终事实
- WS 重连 → 必须触发 REST 校验
- 不允许“WS 连上就当恢复正常”

### 10.6 执行边界必须二次校验

在最终 submit 前必须再次检查：

- 订单类型（entry / exit / recovered close）
- reduceOnly 是否正确
- HEDGE / ONE_WAY payload 是否正确
- positionSide 是否匹配
- 当前状态是否允许执行

❗ 禁止只在 proposal 阶段校验。

### 10.7 禁止顺手扩大修改范围

- 一个 PR 只解决一个问题域
- 不允许顺手改状态机 / 执行链 / reconcile
- 不允许“顺手优化结构”导致语义变化

推荐：

- 一 issue 一分支
- 一 issue 一 PR
- merge 串行

### 10.8 必须补 recovery regression tests

至少覆盖：

- DB takeover
- exchange takeover
- mismatch 场景
- duplicate exit 防护
- partial fill + restart
- passive close payload

❗ 禁止只写 helper 测试。

### 10.9 AI / Codex 必须遵守

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

### 10.10 相关参考文档

必须结合：

- [docs/runtime-recovery-stabilization-summary.md](docs/runtime-recovery-stabilization-summary.md)
- [docs/runtime-recovery-extension-coding-rules.md](docs/runtime-recovery-extension-coding-rules.md)
