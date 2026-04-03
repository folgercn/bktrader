# bkTrader 交易平台

本仓库包含两部分内容：

- 历史策略研究与回测资产（BTCUSDT）
- 全新的 Go 实盘交易平台脚手架，支持信号驱动执行、监控、模拟盘交易和回测

## 新平台目录结构

- `cmd/platform-api`：Go API 入口
- `internal`：平台应用代码
- `web/console`：前端控制台脚手架
- `docs`：架构和系统设计文档
- `configs`：示例配置文件
- `deployments`：本地基础设施引导配置
- `db/migrations`：数据库迁移脚本
- `research`：历史策略研究和数据处理脚本

## 当前策略默认参数

平台围绕当前首选策略配置进行设计：

- 信号时间框架：`1D` 信号，`1m` 执行
- 初始仓位为零
- `max_trades_per_bar=3`（每根 K 线最大交易次数）
- 再入场风险仓位管理：首次 `10%`，后续 `20%`
- 止损模式：`atr`

## 快速开始

### 后端

```bash
cp configs/app.example.env .env
go run ./cmd/platform-api
```

默认使用 `STORE_BACKEND=memory`（内存存储）启动 API。

如需使用 PostgreSQL 持久化存储：

```bash
docker compose -f deployments/docker-compose.dev.yml up -d
go run ./cmd/db-migrate
export STORE_BACKEND=postgres
export POSTGRES_DSN=postgres://postgres:postgres@localhost:5432/bktrader?sslmode=disable
go run ./cmd/platform-api
```

在本地开发中启用自动数据库迁移：

```bash
export STORE_BACKEND=postgres
export AUTO_MIGRATE=true
go run ./cmd/platform-api
```

当前 MVP 可用接口：

- `GET /healthz` — 健康检查
- `GET /api/v1/overview` — 系统概览
- `GET|POST /api/v1/strategies` — 策略管理
- `GET|POST /api/v1/accounts` — 账户管理
- `GET /api/v1/account-summaries` — 账户汇总（权益、PnL、费用）
- `GET /api/v1/account-equity-snapshots?accountId=...` — 账户净值快照
- `GET|POST /api/v1/orders` — 订单管理
- `GET /api/v1/fills` — 成交记录
- `GET /api/v1/positions` — 持仓查询
- `GET|POST /api/v1/backtests` — 回测管理
- `GET /api/v1/backtests/options` — 回测配置选项（信号周期、执行数据源）
- `GET|POST /api/v1/paper/sessions` — 模拟交易会话
- `POST /api/v1/paper/sessions/{id}/start` — 启动模拟会话
- `POST /api/v1/paper/sessions/{id}/stop` — 停止模拟会话
- `POST /api/v1/paper/sessions/{id}/tick` — 手动推进模拟会话
- `GET /api/v1/signal-sources` — 信号源列表
- `GET /api/v1/chart/annotations` — 图表标注数据
- `GET /api/v1/chart/candles` — K 线数据

### 前端控制台

```bash
cd web/console
npm install
export VITE_API_BASE=http://127.0.0.1:8080
npm run dev
```

## CI/CD

仓库已提供一套最小可用的 Docker 化 CI/CD 骨架：

- `.github/workflows/ci.yml`：后端 `go test/build`、前端 `npm run build`、Docker 构建校验
- `.github/workflows/cd.yml`：构建镜像并推送到 GHCR，然后通过 SSH 到目标主机执行部署脚本
- `Dockerfile`：多阶段构建，产出 `platform-api` 运行镜像
- `deployments/docker-compose.prod.yml`：生产环境 Compose 编排
- `scripts/deploy.sh`：远程部署脚本

### 需要配置的 GitHub Secrets

如果使用 self-hosted runner + GHCR 拉取镜像，至少需要添加：

- `GHCR_USERNAME`：有权读取 GHCR 包的 GitHub 用户名
- `GHCR_READ_TOKEN`：用于读取 GHCR 包的 Personal Access Token（至少包含 `read:packages`）

> 如果你把 `ghcr.io/wuyaocheng/bktrader` 这个包设为 Public，也可以不再依赖读取凭证。

### Macmini 部署建议

如果目标部署机就是 Macmini，本仓库推荐这样设置：

- `DEPLOY_HOST=<Macmini 可被 GitHub Actions 访问到的地址>`
- `DEPLOY_USER=fujun`
- `DEPLOY_PATH=/Users/fujun/services/bktrader`
- `APP_ENV_FILE=/Users/fujun/services/bktrader/.env`

> 注意：如果 GitHub Actions 需要连回你家里的 Macmini，Macmini 必须能被公网、FRP、Tailscale Funnel/Serve 或其他稳定入口访问；仅本地局域网 IP 无法被 GitHub 云端 runner 直接访问。

### 推荐的部署机准备

目标机器至少安装：

- Docker
- Docker Compose Plugin

并准备好 `.env` 文件，例如：

```env
APP_NAME=bkTrader-platform
APP_ENV=production
HTTP_ADDR=:8080
STORE_BACKEND=postgres
AUTO_MIGRATE=true
POSTGRES_DSN=postgres://postgres:postgres@postgres:5432/bktrader?sslmode=disable
REDIS_ADDR=redis:6379
NATS_URL=nats://nats:4222
PAPER_TICK_INTERVAL=15
```

> 当前 `cd.yml` 默认推送镜像到 `ghcr.io/<owner>/bktrader:latest`，并在 `main` 分支 push 后触发部署。

## 备注

- 回测配置已区分“信号周期”和“执行数据源”两个维度。
- 当前平台标准支持的信号周期为 `4h / 1d`。
- 当前平台标准支持的执行数据源为 `tick / 1min`。
- `1min` 在这套策略里主要用于近似 tick 级执行，不应被误解为策略主交易周期。

- 现有的研究文件已整理至 `research/` 目录，避免干扰策略研究工作。
- 平台脚手架采用模块化设计，初期以可部署的单体架构启动，便于快速迭代，后续可按需拆分。
- Phase 1 支持内存存储和 PostgreSQL 两种存储后端，通过 `STORE_BACKEND` 环境变量切换。
- PostgreSQL 持久化目前覆盖策略、账户、订单、持仓、回测记录和模拟交易会话。
- `cmd/db-migrate` 执行嵌入式 SQL 迁移，并在 `schema_migrations` 表中记录迁移历史。
- 提交到 `PAPER` 模式账户的订单会立即成交，生成 `fills` 记录并更新净 `positions`。
- `GET /api/v1/account-summaries` 返回模拟账户的权益、费用、已实现/未实现盈亏及敞口快照。
- 净值快照在创建模拟会话时和模拟订单成交时自动追加。
- 模拟交易会话支持启动、停止和手动推进；活跃会话从 `FINAL_1D_LEDGER_BEST_SL.csv` 回放策略交易账本。
- 模拟会话状态在 `paper_sessions.state` 中持久化回放进度,`ledgerIndex` 可跨重启保持。
