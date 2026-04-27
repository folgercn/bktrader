# bktrader 项目文档导航

本文档是本项目的所有核心说明文件的索引与导航系统。

## 1. AI 与 Harness 核心文档
这些文档用于规范 AI Coding Agent 以及 CI/CD 对代码操作的安全防线：
- [AGENTS.md](../AGENTS.md) - **入口**：全局禁区、常用验证命令集合。
- [agent-risk-model.md](agent-risk-model.md) - Agent 及人工改动风险分级表 (L0~L3)。
- [live-safety-invariants.md](live-safety-invariants.md) - 实盘关键不变量定性和边界约束。
- [test-matrix.md](test-matrix.md) - 各相关组件所需的最低自测/回归验证矩阵。
- [harness-engineering-部署方案.md](harness-engineering-部署方案.md) - Harness Engineering 建设方案、PR 实战踩坑模式、Review 黄金规则与 AI Agent 协作纪律。
- [pr-lessons-learned.md](pr-lessons-learned.md) - **从 155 个 PR 提炼的实战踩坑模式、review 黄金规则与 AI Agent 协作纪律**。

## 2. 架构与工程设计
- [llm-project-index.md](llm-project-index.md) - **推荐阅读：深入解读代码目录结构的索引层**。
- [system-design.md](system-design.md) - 项目早期整体抽象架构。
- [bktrader-ctl-install-deploy.md](bktrader-ctl-install-deploy.md) - **bktrader-ctl 安装与发布说明**。
- [bktrader-ctl-reference.md](bktrader-ctl-reference.md) - **bktrader-ctl 命令手册**。
- [dashboard-sse-architecture.md](dashboard-sse-architecture.md) - 基于 SSE 的实时仪表盘架构设计。
- [bento-ui-modernization-guidelines.md](bento-ui-modernization-guidelines.md) - Bento 风格 UI 现代化指南。
- [runtime-runner-decomposition-protocol.md](runtime-runner-decomposition-protocol.md) - `live-runner` / `signal-runtime-runner` 拆分协议基线。
- [production-log-troubleshooting.md](production-log-troubleshooting.md) - 生产服务器日志排障起手式。
- [部署与网络架构.md](部署与网络架构.md) - 包含有关容器/负载路由的信息。
- [cicd-maintenance.md](cicd-maintenance.md) - GitHub Actions 维保说明。
- [frontend-live-reconcile-collab.md](frontend-live-reconcile-collab.md) - Live 账户全量对账的前端协作文档与 API 接入约定。
- [frontend-live-launch-template-isolation-collab.md](frontend-live-launch-template-isolation-collab.md) - Live launch template 独占切换语义的前端协作文档。

## 3. 金融与投研文档
- [STRATEGY_ANALYSIS.md](STRATEGY_ANALYSIS.md) - BK体系策略逻辑 analysis。
- [tick-data-spec.md](tick-data-spec.md) - 行情 tick 与 Bar 数据规范。

> `Smoke Test`: 若需要部署回测，请手动执行 `scripts/testnet_live_session_smoke.sh`。对于本项目的自动化防御，依靠 `ci.yml` 中的风险拦截脚本。
