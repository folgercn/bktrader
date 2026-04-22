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
- [20260403改进计划_plan.md](20260403改进计划_plan.md) - 记录了平台重构与组件拆解的核心思考。
- [20260418-live-plan-exhausted-troubleshooting.md](20260418-live-plan-exhausted-troubleshooting.md) - `live session` 长时间不下单且持续 `plan-exhausted` 时的定位手册。
- [20260420-runtime-recovery-source-of-truth-map.md](20260420-runtime-recovery-source-of-truth-map.md) - Issue #84 的恢复事实源梳理、函数级 mapping、时序图与风险点清单。
- [20260420-live-reconcile-manual-close-state-closure-bug-spec.md](20260420-live-reconcile-manual-close-state-closure-bug-spec.md) - 手动平仓、成交回写、reconcile gate 与 session 刷新闭环缺失问题说明与修复范围。
- [20260422-binance-rest-rate-limit-and-live-sync-storm-plan.md](20260422-binance-rest-rate-limit-and-live-sync-storm-plan.md) - Binance REST 统一限流、`SyncLiveAccount` 风暴治理、stale source 观测与前端 K 线降载专项记录。
- [production-log-troubleshooting.md](production-log-troubleshooting.md) - 生产服务器日志 SSH 入口、日志目录、stale/source gate、Binance REST 限流与前端 K 线请求排查起手式。
- [部署与网络架构.md](部署与网络架构.md) - 包含有关容器/负载路由的信息。
- [cicd-maintenance.md](cicd-maintenance.md) - GitHub Actions 维保说明。
- [frontend-live-reconcile-collab.md](frontend-live-reconcile-collab.md) - Live 账户全量对账的前端协作文档与 API 接入约定。
- [frontend-live-launch-template-isolation-collab.md](frontend-live-launch-template-isolation-collab.md) - Live launch template 独占切换语义的前端协作文档。

## 3. 金融与投研文档
- [STRATEGY_ANALYSIS.md](STRATEGY_ANALYSIS.md) - BK体系策略逻辑分析。
- [tick-data-spec.md](tick-data-spec.md) - 行情 tick 与 Bar 数据规范。
- [20260407-ma-filter-research.md](20260407-ma-filter-research.md) / [20260407-testnet-最小闭环进度.md](20260407-testnet-最小闭环进度.md) - 相关投研实验备忘。

> `Smoke Test`: 若需要部署回测，请手动执行 `scripts/testnet_live_session_smoke.sh`。对于本项目的自动化防御，依靠 `ci.yml` 中的风险拦截脚本。
