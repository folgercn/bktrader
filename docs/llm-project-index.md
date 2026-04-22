# BKTrader LLM 快速引导与文件索引

> **提示给后续的 LLM Agent：** 
> 当你介入本项目时，请**务必按顺序**阅读以下文件：
> 1. **[AGENTS.md](file:///Users/fujun/node/bktrader/AGENTS.md)**: **最高优先级规范**，包含风险约束、Baseline 参数与协作纪律。
> 2. **本文件 (docs/llm-project-index.md)**: 建立系统架构和文件目录的全局上下文。
> 3. **[docs/index.md](file:///Users/fujun/node/bktrader/docs/index.md)**: 详细的项目级 Harness 与文档清单。

## 1. 项目定位
**BKTrader** 是一个加密货币自动化交易与回测平台。
它包含一个用 Go 编写的高性能后端引擎（负责行情接入、信号处理、实盘执行、回测模拟），以及一个用 React 编写的现代化前端控制台（负责监控、策略管理和人工干预）。

## 2. 核心目录映射

### ⚙️ 后端 (Go)
后端采用经典的领域驱动设计（DDD）分层架构：
- **`cmd/`**: 服务的启动入口。
  - `platform-api/main.go`: API 服务主进程。
  - `db-migrate/main.go`: 数据库迁移工具。
- **`internal/`**: 业务核心代码。
  - `domain/`: 定义核心实体模型 (Models) 和错误类型。
  - `service/`: 业务逻辑层（涵盖实盘 Live、回测 Backtest、信号 Signal、引擎适配器等）。
  - `http/`: REST API 路由控制和 Handlers。
  - `store/`: 持久化层（主要使用 PostgreSQL）。
- **`db/migrations/`**: PostgreSQL 数据库的建表和变更 SQL 脚本。
- **`data/tick/`**: 存放 Tick 级行情数据的示例或规范。
- **`graphify-out/`**: **知识图谱资产**，包含 `GRAPH_REPORT.md`，用于理解系统核心组件的依赖拓扑。

### 🖥️ 前端 (React + Vite + TypeScript)
前端位于 **`web/console/`**，采用 Zustand 进行状态管理，设计风格偏向暗黑玻璃拟态。
- **`src/components/`**: 基础 UI 原子组件和业务图表 (Charts)。
- **`src/layouts/`**: 全局布局容器 (如 `WorkbenchLayout`)。
- **`src/modals/`**: 复杂的业务弹窗 (登录、创建实盘、绑定适配器等)。
- **`src/pages/`**: 主舞台页面 (MonitorStage 监控、StrategyStage 策略管理、AccountStage 账户主控)。
- **`src/panels/`**: 侧边栏配置面板和底部数据面板 (订单、告警、持仓等)。
- **`src/store/`**: 全局状态中心 (`useTradingStore.ts` 负责业务数据, `useUIStore.ts` 负责交互状态)。
- **`src/utils/`**: 工具集合 (`api.ts` 网络请求, `derivation.ts` 复杂状态派生, `format.ts` 格式化)。
- **`src/types/domain.ts`**: 前端使用的所有 TypeScript 类型定义（需与后端 Domain 保持对齐）。

### 🔬 投研与运维 (Python & Shell)
- **`research/`**: Python 投研脚本，用于数据预处理、本地回测和策略验证。
- **`scripts/`**: 运维和测试脚本。
  - **安全传感器 (Safety Sensors)**: `check_high_risk_defaults.sh` (防误操作), `check_migration_safety.py` (迁移安全), `check_env_safety.sh` (环境检查)。
  - **回归测试**: `testnet_live_session_smoke.sh` 等。
- **`deployments/`**: Docker Compose 编排文件（本地开发与生产环境）。

### 📚 文档与智能体资产
- **`docs/`**: 人类可读的系统设计文档。
  - **`test-matrix.md`**: 定义各风险等级改动所需的验证深度。
  - **`pr-lessons-learned.md`**: **必读！** 从 155 个 PR 提炼的实战踩坑模式与 Review 黄金规则。
  - **`agent-risk-model.md`**: L0 到 L3 的风险定级说明。
  - **`production-log-troubleshooting.md`**: 生产环境排障与 SSH/日志路径说明。
- **`docs/AGENT_PATHS.md`**: **重要！** 工具链绝对路径导览。
- **`.skills/`**: 注入给 LLM 的特定技能库（如前端规范、shadcn 使用规范）。

## 3. 最近的重大重构记录
- **前端模块化拆解**: 曾将 5000 行的 `main.tsx` 逻辑抽离成了几十个职责单一的 React 组件。
- **Harness Engineering 体系部署**: 引入了 `AGENTS.md` 强约束、自动化安全传感器以及基于风险定级的验证矩阵（`test-matrix.md`），将安全边界从文档口头约定转为机械化拦截。
