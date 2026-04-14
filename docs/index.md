# bktrader 项目文档导航

本文档是本项目的所有核心说明文件的索引与导航系统。

## 1. AI 与 Harness 核心文档
这些文档用于规范 AI Coding Agent 以及 CI/CD 对代码操作的安全防线：
- [AGENTS.md](../AGENTS.md) - **入口**：全局禁区、常用验证命令集合。
- [agent-risk-model.md](agent-risk-model.md) - Agent 及人工改动风险分级表 (L0~L3)。
- [live-safety-invariants.md](live-safety-invariants.md) - 实盘关键不变量定性和边界约束。
- [test-matrix.md](test-matrix.md) - 各相关组件所需的最低自测/回归验证矩阵。
- [pr-checklist.md](pr-checklist.md) - PR 提交前的范围感知自查清单。
- [harness-engineering-部署方案.md](harness-engineering-部署方案.md) - 早期 Harness 设计思路。

## 2. 架构与工程设计
- [llm-project-index.md](llm-project-index.md) - **推荐阅读：深入解读代码目录结构的索引层**。
- [system-design.md](system-design.md) - 项目早期整体抽象架构。
- [20260403改进计划_plan.md](20260403改进计划_plan.md) - 记录了平台重构与组件拆解的核心思考。
- [部署与网络架构.md](部署与网络架构.md) - 包含有关容器/负载路由的信息。
- [cicd-maintenance.md](cicd-maintenance.md) - GitHub Actions 维保说明。

## 3. 金融与投研文档
- [STRATEGY_ANALYSIS.md](STRATEGY_ANALYSIS.md) - BK体系策略逻辑分析。
- [tick-data-spec.md](tick-data-spec.md) - 行情 tick 与 Bar 数据规范。
- [20260407-ma-filter-research.md](20260407-ma-filter-research.md) / [20260407-testnet-最小闭环进度.md](20260407-testnet-最小闭环进度.md) - 相关投研实验备忘。

> `Smoke Test`: 若需要部署回测，请手动执行 `scripts/testnet_live_session_smoke.sh`。对于本项目的自动化防御，依靠 `ci.yml` 中的风险拦截脚本。
