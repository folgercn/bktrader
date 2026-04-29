# bktrader 项目文档导航

本文档是项目所有核心文档的分级索引。**按优先级分三层，LLM 首次进入只需读 🔴 必读层**。

---

## 🔴 必读（首次进入项目）

| 文档 | 说明 |
|------|------|
| [AGENTS.md](../AGENTS.md) | **入口**：全局禁区、验证命令、Review 黄金规则、Agent 协作纪律 |
| [llm-project-index.md](llm-project-index.md) | 代码目录结构与模块边界（先理解代码再查文档） |
| [CONTRIBUTING.md](../CONTRIBUTING.md) | 角色分工（wuyaocheng / folgercn）、分支策略、PR 审查权属 |

---

## 🟡 按需读取（改代码前查）

| 文档 | 适用场景 |
|------|----------|
| [pr-lessons-learned.md](pr-lessons-learned.md) | 修改 `internal/` 前必读：155+ PR 的实战踩坑案例与 Review 规则 |
| [agent-risk-model.md](agent-risk-model.md) | 判断改动风险等级 (L0~L3) |
| [test-matrix.md](test-matrix.md) | 确定改动需要哪些验证项 |
| [live-safety-invariants.md](live-safety-invariants.md) | 实盘关键不变量与边界约束 |
| [runtime-recovery-extension-coding-rules.md](runtime-recovery-extension-coding-rules.md) | 恢复/接管/被动平仓专项编码规则 |
| [runtime-recovery-stabilization-summary.md](runtime-recovery-stabilization-summary.md) | 运行时恢复稳定性改造历史总结 |
| [production-log-troubleshooting.md](production-log-troubleshooting.md) | 线上告警/stale-source/429限流排障起手式 |
| [AGENT_PATHS.md](AGENT_PATHS.md) | 工具链绝对路径导览（本地路径以 `AGENTS.local.md` 为准） |

---

## 🟢 参考文档（需要时查阅）

### 架构与工程设计
- [fill-reconciliation-engine.md](fill-reconciliation-engine.md) — 成交一致性引擎：real/synthetic/remainder fill、`fill_source`、事务化 settlement、交易所成交归一化
- [dashboard-sse-architecture.md](dashboard-sse-architecture.md) — SSE 实时仪表盘架构
- [bento-ui-modernization-guidelines.md](bento-ui-modernization-guidelines.md) — Bento 风格 UI 现代化指南
- [runtime-runner-decomposition-protocol.md](runtime-runner-decomposition-protocol.md) — `live-runner` / `signal-runtime-runner` 拆分协议
- [runtime-supervisor.md](runtime-supervisor.md) — Runtime Supervisor / Service Supervisor 分层规划

### CLI 工具
- [bktrader-ctl-reference.md](bktrader-ctl-reference.md) — bktrader-ctl 命令手册
- [bktrader-ctl-install-deploy.md](bktrader-ctl-install-deploy.md) — bktrader-ctl 安装与发布说明

### 部署与运维
- [部署与网络架构.md](部署与网络架构.md) — 容器/负载路由
- [cicd-maintenance.md](cicd-maintenance.md) — GitHub Actions 维保
- [cd-service-routing.md](cd-service-routing.md) — CD 服务路由

### 投研
- [量化交易与CTA扩展说明.md](量化交易与CTA扩展说明.md) — 量化交易扩展说明
- [30m-enhanced-filter-research.md](30m-enhanced-filter-research.md) — BTC/ETH 30m enhanced 过滤器、模板参数与 Q1 2026 回测结论

---

> **归档文档**在 [docs/归档/](归档/) 目录下，包含历史规划和已完成的实施方案。
>
> **Smoke Test**: 若需要部署验证，请手动执行 `scripts/testnet_live_session_smoke.sh`。
