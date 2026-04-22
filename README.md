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
- 运行模式：`dir2_zero_initial=true` + `zero_initial_mode=reentry_window`
- 初始仓位为零
- `max_trades_per_bar=2`（每根 K 线最大交易次数）
- 再入场风险仓位管理：首次 `20%`，后续 `10%` (`reentry_size_schedule=[0.20, 0.10]`)
- 止损模式：`atr`

## 🏗️ Harness Engineering & 协作准则

本项目引入了 **Harness Engineering** 体系，旨在通过自动化工具和严格的协作纪律确保交易系统的执行安全性：

- **[AGENTS.md](AGENTS.md)**: AI Agent 介入本项目的最高行动指南。所有参与开发的 Agent 必须首先阅读并严格遵守其中的风险约束、baseline 规范和协作纪律。
- **安全传感器 (Safety Sensors)**: 
    - `scripts/check_high_risk_defaults.sh`: 静态扫描代码库，防止高风险默认配置（如 `auto-dispatch`）被误提交。
    - `scripts/check_migration_safety.py`: 数据库迁移安全检测，防止非向后兼容的变更破坏生产数据。
    - `scripts/check_env_safety.sh`: 环境安全检查。
- **验证矩阵**: 详见 `docs/test-matrix.md`，定义了不同风险等级改动所需的验证深度。

## 🧠 知识图谱 (Knowledge Graph)

本项目通过 `graphify` 维护着一套自动更新的知识图谱：

- **目录**: `graphify-out/`
- **核心报告**: `graphify-out/GRAPH_REPORT.md` (包含 God Nodes 分析、社区结构和潜在架构瓶颈建议)
- **作用**: 帮助 LLM 和开发者快速理解系统核心组件（如 `Platform`, `Store`, `Adapter`）之间的依赖拓扑，避免在修改热路径代码时产生非预期的副作用。

## 快速开始

### 后端

```bash
cp configs/app.example.env .env
go run ./cmd/platform-api
```

默认使用 `STORE_BACKEND=memory`（内存存储）启动 API。服务启动时会优先读取 `APP_ENV_FILE` 指向的文件，其次读取当前工作目录下的 `.env`。

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
- `GET /api/v1/strategy-engines` — 可用策略引擎列表
- `GET|POST /api/v1/accounts` — 账户管理
- `GET /api/v1/live-adapters` — 可用实盘执行适配器
- `POST /api/v1/live/accounts/{id}/binding` — 绑定实盘账户到交易所适配器
- `GET /api/v1/signal-sources` — 信号源目录（按环境分组）
- `GET /api/v1/signal-source-types` — 信号源类型说明
- `GET /api/v1/signal-runtime/adapters` — 可用信号 runtime adapter 列表
- `GET /api/v1/alerts` — 统一运行告警列表
- `GET /api/v1/notifications?includeAcked=true|false` — 平台内通知中心
- `POST /api/v1/notifications/{id}/ack` — 确认一条通知
- `DELETE /api/v1/notifications/{id}/ack` — 取消确认一条通知
- `POST /api/v1/notifications/{id}/telegram` — 将单条通知发送到 Telegram
- `GET /api/v1/telegram/config` — Telegram 配置（脱敏）
- `POST /api/v1/telegram/config` — 更新 Telegram 配置
- `POST /api/v1/telegram/test` — 发送 Telegram 测试消息
- `GET /api/v1/signal-runtime/plan?accountId=...&strategyId=...` — 账户与策略的信号运行计划
- `GET|POST /api/v1/signal-runtime/sessions` — 信号运行时会话列表/创建
- `GET /api/v1/signal-runtime/sessions/{id}` — 单个信号运行时会话
- `POST /api/v1/signal-runtime/sessions/{id}/start` — 启动信号运行时会话
- `POST /api/v1/signal-runtime/sessions/{id}/stop` — 停止信号运行时会话
- `GET|POST /api/v1/strategies/{id}/signal-bindings` — 策略级多信号源绑定
- `GET|POST /api/v1/accounts/{id}/signal-bindings` — 账户级多信号源绑定
- `GET /api/v1/account-summaries` — 账户汇总（权益、PnL、费用）
- `GET /api/v1/account-equity-snapshots?accountId=...` — 账户净值快照
- `GET|POST /api/v1/orders` — 订单管理
- `GET /api/v1/fills` — 成交记录
- `GET /api/v1/positions` — 持仓查询
- `GET|POST /api/v1/backtests` — 回测管理
- `GET /api/v1/backtests/options` — 回测配置选项（信号周期、执行数据源、可发现数据文件、支持标的、CSV 字段规范）
- `GET|POST /api/v1/live/sessions` — 实盘策略会话
- `POST /api/v1/live/sessions/{id}/start` — 启动实盘策略会话
- `POST /api/v1/live/sessions/{id}/stop` — 停止实盘策略会话
- `POST /api/v1/live/sessions/{id}/dispatch` — 手动确认并派发当前实盘策略意图

- `GET /api/v1/chart/annotations` — 图表标注数据
- `GET /api/v1/chart/candles` — K 线数据

当前推荐的模拟交易主链路：

- 创建 `LIVE` 账户
- 绑定 `binance-futures`
- 设置 `sandbox=true`
- 在 `.env` 中提供 `BINANCE_TESTNET_API_KEY` / `BINANCE_TESTNET_API_SECRET`
- 通过 Live/Testnet 会话、订单同步和持仓恢复来跑完整模拟链路

信号源当前支持的建模方式：
- 策略可以同时绑定多个信号源，例如 `trade_tick(trigger) + order_book(feature)`
- 账户也可以同时绑定多个信号源，例如一个账户同时观察 `BINANCE trade tick` 和 `OKX order book`
- 策略绑定解决“策略依赖哪些输入”
- 账户绑定解决“这个账户实际接收哪些市场流”
- 这两层分离后，后续做双市场交易和跨市场套利时不需要改模型
- `signal-runtime plan` 会把策略需要的源和账户实际绑定的源做匹配，直接告诉你：
  - 哪些源已经 READY
  - 哪些 trigger/feature 还缺失
  - 这些源后面应由哪个 runtime adapter 驱动
- `signal-runtime session` 会把一组绑定转换成可启动的运行时骨架，当前会记录：
  - 订阅数和订阅 channel
  - runtime adapter
  - 健康状态
  - 最近心跳
  - 最近事件摘要
- `signal-runtime session` 现在也会维护结构化 `sourceStates`：
  - 按 `sourceKey + symbol + role` 聚合最近源状态
  - 为后续策略实时评估提供稳定的 trade tick / order book 快照
- 当前 `binance-market-ws` 已经接入真实公共 WebSocket：
  - 可以真实订阅 `trade_tick`
  - 可以在同一 session 里同时真实订阅 `trade_tick + order_book`
  - 会更新 `connectedAt / lastHeartbeatAt / lastEventSummary`
  - `order_book` 摘要会返回 `bestBid / bestAsk / bestBidQty / bestAskQty`
- `okx-market-ws` 目前仍先作为可启动骨架保留，下一步再补齐真实消息消费

当前推荐的运行链路已经统一到 live/testnet：
- 通过 `signal-runtime session` 拉起实时行情
- 通过 `live session` 进行策略评估、派单、同步与恢复
- 通过 Binance Futures testnet 完成模拟交易、订单同步与持仓恢复
- 前端 Monitor 主图优先使用 runtime `sourceStates.signal_bar` 展示 K 线；仅在 runtime bars 不足时按需请求 `/api/v1/chart/candles` 作为展示兜底，且最小 fallback 粒度为 `5m`
- 服务启动时会主动从 Binance 行情源预热 `1m / 4h / 1d` 市场缓存，并计算 `SMA5 / MA20 / ATR14`，live 链路不再依赖本地 CSV
- 已验证一条真实的 `4h -> live intent -> auto-dispatch -> Binance Futures testnet FILLED` 主链路
      - `sl-reentry-watch`
      - `sl-reentry-near`
      - `sl-reentry-near-strong`
      - `sl-reentry-near-weak`
      - `pt-reentry`
      - `pt-reentry-watch`
      - `pt-reentry-near`
      - `pt-reentry-near-strong`
      - `pt-reentry-near-weak`
      - `hold`
      - `hold-long`
      - `hold-short`
      - `protect-exit`
      - `protect-exit-watch`
      - `protect-exit-near`
      - `protect-exit-near-strong`
      - `protect-exit-near-weak`
      - `risk-exit`
      - `risk-exit-watch`
      - `risk-exit-near`
      - `ignore`
    - `signalKind` 现在已经开始结合当前 paper 持仓快照，而不只是看下一步计划角色
    - `protect-exit-watch` / `risk-exit-watch` 现在还会结合当前持仓相对市场价的盈亏方向
    - 当市场价距离下一步退出价足够近时，会进一步升级成 `protect-exit-near` / `risk-exit-near`
    - 入场侧也一样：当市场价距离下一步入场价足够近时，会升级成 `initial-entry-near` / `sl-reentry-near` / `pt-reentry-near`
    - 如果盘口方向也支持当前 near 状态，会进一步升级成 `*-near-strong`；保护性退出在盘口方向逆风时会显示 `protect-exit-near-weak`
    - `sl-reentry` 对盘口方向要求更严格：balanced book 也可能继续等待，避免止损后过早重入
    - 当前 `bk-default` 先实现了最小决策：非 trigger 事件、symbol 不匹配、缺少源状态时不会推进
    - 同时还会检查 `next planned event` 的事件时间，没走到下一步计划时间之前不会推进
    - 当前还会比较“当前市场价”和“下一笔计划价”的偏离：
      - `BUY` 优先看 `bestAsk`
      - `SELL/SHORT` 优先看 `bestBid`
      - 默认允许最大偏离 `50 bps`
      - 并且按交易方向判断当前价格是否仍然“可执行”
      - 不满足时会返回 `wait / price-not-actionable`
    - 如果 order book 可用，还会检查盘口质量：
      - 计算 `spread bps`
      - 计算 `bid/ask imbalance`
      - 生成简化的 `liquidityBias`
      - 当 spread 过宽时会返回 `wait / spread-too-wide`
      - 当盘口方向明显逆风时，会返回 `wait / bias-unfavorable`
- 当前这一步仍是最小事件驱动版本：先让实时 tick 参与推进调度，再逐步替换掉旧的计划式推进

当前 `live session` 也已经接入主交易链路：
- `live session` 绑定 `LIVE account + strategy`
- 启动前会检查 live adapter、signal runtime plan、runtime health 和 source freshness
- 启动后会随 linked runtime 的真实事件更新策略评估状态
- 当前默认 `dispatchMode=manual-review`
- 会在 session state 中记录：
  - `lastStrategyDecision`
  - `lastStrategyIntent`
  - `lastStrategyEvaluationSourceGate`
  - `timeline`
- 当前可以对 `lastStrategyIntent` 执行人工确认派单：
  - 只有当 live session 产出了 ready intent 时才允许 dispatch
  - dispatch 会复用现有 live preflight 和 live adapter 提交流程
- 这一步先把“实盘策略会话”和“实盘自动派单”拆开，方便先把 runtime/策略评估链路跑稳

实盘账户当前支持：
- `LIVE` 账户默认状态为 `PENDING_SETUP`
- 通过 `POST /api/v1/live/accounts/{id}/binding` 绑定 adapter 后会切到 `CONFIGURED`
- `LIVE` 账户 binding 写入 `accounts.metadata.liveBinding`
- 凭证只保存引用，例如 `credentialRefs.apiKeyRef` / `credentialRefs.apiSecretRef`
- 实盘手续费和资金费来源固定为交易所回报，不走平台静态配置

当前 `LIVE` 订单流骨架：
- `POST /api/v1/orders` 对 `LIVE` 账户会按 `accounts.metadata.liveBinding.adapterKey` 选择 adapter
- 当前内置 `binance-futures` mock submission，会返回 `ACCEPTED`
- 订单 metadata 会写入 `exchangeOrderId`、`acceptedAt`、`adapterSubmission`、`feeSource=exchange`、`fundingSource=exchange`
- `POST /api/v1/orders/{id}/sync` 会通过 adapter 把 `ACCEPTED` 订单同步成 `FILLED`
- sync 后会写入真实风格的 `fill`、更新 `position`，并把 `adapterSync`、`lastSyncAt` 回写到订单 metadata

### 前端控制台

```bash
cd web/console
npm install
export VITE_API_BASE=http://127.0.0.1:8080
npm run dev
```

前端开发环境默认通过 `VITE_API_BASE` 直连本地后端，例如：

- `VITE_API_BASE=http://127.0.0.1:8080`

生产环境推荐改为同域部署，不显式配置 `VITE_API_BASE`，让前端直接请求当前域名下的 `/api/...`。这样可以避免跨域，并通过反向代理统一前后端入口。

### 前端生产发布

当前推荐的前端生产部署方式不是 Docker，而是：

1. GitHub Actions 在 `web/console` 下执行 `npm ci` 和 `npm run build`
2. 产出静态文件 `web/console/dist`
3. 通过 `rsync` 直接覆盖远端 Nginx 静态目录，例如 `/var/www/bktrader`
4. 由 Nginx 提供前端页面，并把 `/api/` 和 `/healthz` 反代到后端

推荐的 Nginx 路由结构如下：

- `/`：前端静态文件目录，例如 `/var/www/bktrader`
- `/api/`：反代到后端 API
- `/healthz`：反代到后端健康检查

当前一套可工作的线上结构示例：

- 前端静态目录：`/var/www/bktrader`
- 前端站点：`https://trade.sunnywifi.cn:3088`
- 后端公网入口：由 FRP 暴露到远端 `127.0.0.1:3081`
- Nginx 反代：`/api/` -> `http://127.0.0.1:3081`

如果沿用仓库里的 `cd.yml`，前端发布链路是：

1. 在 self-hosted macOS runner 上构建 `web/console/dist`
2. 通过 `ssh root@1.95.71.247 'mkdir -p /var/www/bktrader'` 确保目录存在
3. 通过 `rsync -av --delete dist/ root@1.95.71.247:/var/www/bktrader/` 覆盖远端静态目录

这种模式适合当前项目，因为：

- 前端是 Vite 静态站点，没有必要再套一层容器
- 回滚和排障更直接，核心就是 `dist/` 目录内容
- Nginx 可以同时处理静态资源、TLS 和 `/api` 反代

## 回测执行数据源

平台将策略信号周期与执行层数据源分开管理，回测模块支持可选执行测试源：

- 信号周期：`4h`、`1d`
- 执行数据源：`tick`、`1min`

当前执行层测试支持的 CSV / archive 约定如下：

- `tick`
  - 文件名示例：`BTC_tick_Clean.csv`、`ETH_tick.csv`
  - 必需列：`timestamp`、`price`
  - 可选列：`quantity`、`side`
默认目录可通过环境变量配置：

```env
MINUTE_DATA_DIR=.
TICK_DATA_DIR=./dataset/archive
```

仓库内已经预留了标准 tick 目录和模板文件：

- [data/tick/.gitkeep](/Users/wuyaocheng/Downloads/bkTrader/data/tick/.gitkeep)
- [data/tick/BTC_tick.sample.template](/Users/wuyaocheng/Downloads/bkTrader/data/tick/BTC_tick.sample.template)
- [docs/tick-data-spec.md](/Users/wuyaocheng/Downloads/bkTrader/docs/tick-data-spec.md)

前端回测面板和 `GET /api/v1/backtests/options` 会展示：

- 当前目录下实际发现的数据文件
- 每种执行数据源支持的标的列表
- 缺失数据源时的可用性状态

回测创建时还支持可选时间窗口参数：

- `from`：RFC3339 起始时间
- `to`：RFC3339 结束时间

当前 `tick` runner 已接入按时间窗口挑选月分片和流式预览，不会默认把整个 archive 全量扫完。
当前平台的主回测入口是 `Strategy Replay`：

- 选择 `4h` 或 `1d` 作为信号周期
- 选择 `tick` 或 `1min` 作为执行数据源
- 由 Go 版策略引擎直接生成交易并回放

其中 `tick` 执行源现在已经是流式逐笔 `Strategy Replay`，会按信号窗口顺序消费 trade archive，而不是先把整段逐笔数据并入内存。

## 策略模块约束

- 策略引擎必须可插拔，平台通过 `StrategyEngine` registry 装载，不把单个策略硬编码在回测、模拟盘或实盘入口上。
- 回测、模拟交易、实盘交易必须共享同一套策略执行语义和订单意图生成逻辑。
- 只有回测允许显式注入模拟滑点；`paper/live` 默认使用 `observed` 执行语义，不在策略层额外叠加虚拟滑点。
- 回测和模拟盘的交易手续费、资金费参数可配置。
- 实盘的手续费、资金费、返佣等成本项必须以交易所返回为准，不在平台里做静态硬编码。
- 当前内置引擎键值为 `bk-default`，可通过策略参数中的 `strategyEngine` 绑定。

当前默认成本参数：

- `tradingFeeBps = 10`
- `fundingRateBps = 0`
- `fundingIntervalHours = 8`

`replayLedger=true` 仍然保留为可选内部审计能力，用于排查历史账本和执行层之间的差异，但它不是当前平台推荐的主回测入口。

当前仓库还提供了一个对齐脚本，用于校验 Go 策略回放和 Python 研究版在 `1d -> 1min` 场景下的一致性：

```bash
python3 scripts/check_1d_1min_parity.py
```

脚本会自动拉起本地 API，分别运行：

- Python 研究版策略回测
- Go `Strategy Replay`

然后比较：

- `return`
- `maxDrawdown`
- `tradePairs`
- `finalBalance`

推荐做法：

1. 把真实逐笔数据清洗成统一 CSV。
2. 按 `BTC_tick_Clean.csv` 这类格式命名。
3. 放到 `data/tick/`。
4. 设置 `TICK_DATA_DIR=./data/tick`。
5. 打开回测面板确认 `tick` 状态从 `missing` 变成 `available`。

你当前仓库里的真实逐笔数据已经位于：

- [dataset/archive](/Users/wuyaocheng/Downloads/bkTrader/dataset/archive)

平台现在同时支持两种 tick 组织方式：

- 扁平清洗文件：`BTC_tick_Clean.csv`
- Binance 月度 archive：`BTCUSDT-trades-2020-01/BTCUSDT-trades-2020-01.csv`

对于 `dataset/archive` 这类大体量逐笔数据，平台当前设计是：

- 先扫描目录生成轻量 manifest
- 回测时按月文件顺序流式读取
- 不把跨年的 tick 数据整段并入内存

## CI/CD

仓库已提供一套最小可用的 CI/CD 骨架：

- `.github/workflows/ci.yml`：后端 `go test/build`、前端 `npm run build`、Docker 构建校验
- `.github/workflows/cd.yml`：构建后端镜像并推送到 GHCR，在 self-hosted runner 上部署后端 Docker，同时构建并发布前端静态文件
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

后端部署节点至少安装：

- Docker
- Docker Compose Plugin

前端静态发布目标机至少需要：

- Nginx
- 可写的静态目录，例如 `/var/www/bktrader`
- GitHub Actions runner 到目标机的 SSH 可达性
- `rsync`

如果前端和 Nginx 在同一台机器，建议额外确认：

- 防火墙已放行站点监听端口，例如 `3088`
- 站点证书和 `server_name` 已配置
- `/api/` 和 `/healthz` 已反代到后端入口

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
TRADE_TICK_FRESHNESS_SECONDS=15
ORDER_BOOK_FRESHNESS_SECONDS=10
SIGNAL_BAR_FRESHNESS_SECONDS=30
RUNTIME_QUIET_SECONDS=30
PAPER_START_READINESS_TIMEOUT_SECONDS=5
BINANCE_TESTNET_API_KEY=your_testnet_api_key
BINANCE_TESTNET_API_SECRET=your_testnet_api_secret
BINANCE_FUTURES_KLINE_BASE_URL=https://fapi.binance.com
BINANCE_FUTURES_WS_URL=wss://fstream.binance.com/ws
OKX_PUBLIC_WS_URL=wss://ws.okx.com:8443/ws/v5/public
```

> 当前 `cd.yml` 默认推送镜像到 `ghcr.io/<owner>/bktrader:latest`，并在 `main` 分支 push 后触发部署。

## 备注

- 回测配置已区分“信号周期”和“执行数据源”两个维度。
- 当前平台标准支持的信号周期为 `4h / 1d`。
- 当前平台标准支持的执行数据源为 `tick / 1min`。
- `1min` 在这套策略里主要用于近似 tick 级执行，不应被误解为策略主交易周期。
- `tick` 模式当前要求本地存在 tick 数据文件；若缺失，回测会以 `FAILED` 状态返回明确错误信息。

- 现有的研究文件已整理至 `research/` 目录，避免干扰策略研究工作。
- 平台脚手架采用模块化设计，初期以可部署的单体架构启动，便于快速迭代，后续可按需拆分。
- Phase 1 支持内存存储和 PostgreSQL 两种存储后端，通过 `STORE_BACKEND` 环境变量切换。
- PostgreSQL 持久化目前覆盖策略、账户、订单、持仓、回测记录和 live 运行状态。
- `cmd/db-migrate` 执行嵌入式 SQL 迁移，并在 `schema_migrations` 表中记录迁移历史。
- `GET /api/v1/account-summaries` 返回模拟账户的权益、费用、已实现/未实现盈亏及敞口快照。
- 当前推荐的“模拟交易”已经切到 Binance Futures testnet，凭据默认从 `.env` 读取。
- live session 现在支持两种下单仓位模式：
  - `positionSizingMode=fixed_quantity`：使用 `defaultOrderQuantity`
  - `positionSizingMode=fixed_fraction`：使用 `defaultOrderFraction` 按账户可用余额/权益换算数量
- 固定比例模式算出的数量在实际提交到 Binance 前，仍会走交易所 `stepSize / minQty / minNotional` 归一化，避免小数位和最小名义价值不符合要求。
- 行情数据接入当前也已经配置化：
  - `BINANCE_FUTURES_KLINE_BASE_URL`：启动预热和图表历史 K 线读取地址
  - `BINANCE_FUTURES_WS_URL`：`binance-market-ws` signal runtime 的公共 WebSocket 地址
  - `OKX_PUBLIC_WS_URL`：`okx-market-ws` 的公共 WebSocket 地址
  - 未配置时会分别回退到 Binance Futures / OKX 官方公共地址
- 服务启动时会从行情源主动预热 `1m / 4h / 1d` bars，并计算 `SMA5 / MA20 / ATR14`，后续 `live / testnet / 实盘` 的策略评估直接复用这批缓存。
- live/testnet 当前默认建议使用 `defaultOrderQuantity=0.002` 跑 BTCUSDT smoke test，`0.001` 可能低于 testnet 最小名义价值限制。
- `scripts/testnet_live_session_smoke.sh` 现在会同时校验退出侧 execution profile 默认值：`PT exit => LIMIT / GTX / postOnly / reduceOnly`，`SL exit => MARKET / reduceOnly`。
- 给 `EXPECT_EXIT_PROFILE=pt-exit` 或 `EXPECT_EXIT_PROFILE=sl-exit` 后，脚本会轮询 live session 的 `lastExecutionProfile / lastExecutionDispatch`，用于等待真实 testnet 退出信号并校验最终执行策略是否按预期落地。
- `GET /api/v1/runtime-policy` 返回统一运行阈值，前端告警和 live/runtime preflight 共享同一套 freshness / quiet / readiness timeout 配置。
- `POST /api/v1/runtime-policy` 支持热更新运行阈值；当前会持久化到平台配置表，服务重启后仍然保留，控制台 `Signals` 页面已提供对应配置面板。
- `GET /api/v1/alerts` 统一聚合 `live / runtime` 两类运行告警，作为控制台告警面板和后续外部通知通道的统一源头。
- `GET /api/v1/notifications` 基于当前活跃告警生成平台内通知 Inbox；支持 `ack / unack`，当前确认状态已持久化。
- Telegram 通知当前只接这一条通道：
  - 可在控制台保存 `bot token / chat id / send levels`
  - 支持发送测试消息
  - 支持把单条平台通知手动转发到 Telegram
  - 平台后台每 `15s` 会自动扫描 active 通知，并只自动发送符合 `sendLevels` 的未发送通知
  - 当前建议先把 `sendLevels` 保持在 `critical`
- `LIVE` 下单前现在也会执行 runtime readiness preflight：要求账户对应策略的 signal runtime plan ready、runtime session 处于 `RUNNING/healthy`，并且必需 source states 不缺失且不过期。
Parity checks:
```bash
python3 scripts/check_1d_1min_parity.py
python3 scripts/check_4h_1min_parity.py
python3 scripts/check_tick_strategy_regression.py
```

说明：

- `1min` 路径当前有 Python 研究版对齐脚本。
- `tick` 路径当前使用 Go 主策略回放的稳定回归窗口校验，不再拿旧的 `run_tick_full_scan_dual` 做 parity；那段研究函数属于另一套历史参数口径，不是当前平台默认策略。
