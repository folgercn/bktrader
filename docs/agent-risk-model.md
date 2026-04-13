# 改动风险分级 (Agent Risk Model)

本表用于定义不同类型代码改动的风险定级。无论是人类开发者还是 AI Agent，都必须遵守以下风险防御模型。

## 级别 L0: 低风险
- **定义**：主要是文档、测试用例补充、前端样式细节调整、独立的研究脚本。
- **代表性区域**：
    - `docs/` (文档)
    - `research/` (不处理真实订单的投研脚本)
    - `.agents/`
    - `web/console/src/` 的表面纯样式 (如 css，或不涉及 websocket 强交互的只读组件)
- **要求**：
    - AI 可以自主完成大量任务。
    - PR 时无需特别声明，不阻拦合并（但需保证格式化通过）。

## 级别 L1: 中风险
- **定义**：后端周边逻辑补充、非关键业务 API 增加、对旧有逻辑做隔离兼容型优化。
- **代表性区域**：
    - `internal/http/` (新增非破坏性接口)
    - `internal/store/` (一般增删改查实现，非 migration)
    - 验证或 CI 的**辅助**脚本 (不含部署脚本)
- **要求**：
    - 必须带有对应的测试覆盖证据 (Unit Test 或明确的手动截图)。
    - Code review 时重点审查是否有意外报错处理漏洞。

## 级别 L2: 高风险
- **定义**：修改执行控制面板 (如 `live.go`、部署策略相关)、前端底层交互、重构数据库表、涉及到 `mainnet` 的潜在引用更改。
- **代表性区域**：
    - `internal/service/execution_strategy.go`
    - `db/migrations/` (必须保证幂等，无强锁表等操作，更不能有不可控的全量 `DROP`)
    - `deployments/` 或 `scripts/deploy.sh`
    - 前段订单触发器和弹窗动作表单，以及 WebSocket store 同步逻辑 (`useTradingStore.ts`)
- **要求**：
    - AI 不得在未获 approval 状态下批量改动。
    - **必须在 PR 模板中勾选“是”**（是否影响默认行为/部署）。
    - 必须人工双审 (wuyaocheng / folgercn)。

## 级别 L3: 极高风险 / 绝对禁区
- **定义**：涉及系统的最高级别不变量，这些属性一旦变更有造成真实本金亏损的不可逆风险。
- **代表性区域**：
    - **修改出厂默认值**：将 `dispatchMode` 的默认值从 `manual-review` 硬编码改为 `auto-dispatch`。
    - **核心运行时**：`internal/service/live*.go`，涉及到 `live order`, `fill`, `position`, `reconciliation` 不一致。
    - **环境强绑定**：代码中静态越过 `testnet` 沙盒连接 `real/mainnet`。
- **要求**：
    - **任何来自 AI 的建议均应被人类仔细推敲**。强烈建议人工手打，不使用代码补全面包屑。
    - 不管改一行还是一百行，都视为系统性的最高防御等级事件。
