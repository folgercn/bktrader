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
- `GET /api/v1/strategy-engines` — 可用策略引擎列表
- `GET|POST /api/v1/accounts` — 账户管理
- `GET /api/v1/live-adapters` — 可用实盘执行适配器
- `POST /api/v1/live/accounts/{id}/binding` — 绑定实盘账户到交易所适配器
- `GET /api/v1/signal-sources` — 信号源目录（按环境分组）
- `GET /api/v1/signal-source-types` — 信号源类型说明
- `GET /api/v1/signal-runtime/adapters` — 可用信号 runtime adapter 列表
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
- `GET|POST /api/v1/paper/sessions` — 模拟交易会话
- `POST /api/v1/paper/sessions/{id}/start` — 启动模拟会话
- `POST /api/v1/paper/sessions/{id}/stop` — 停止模拟会话
- `POST /api/v1/paper/sessions/{id}/tick` — 手动推进模拟会话

创建模拟会话时支持可选 runtime overrides：
- `signalTimeframe`
- `executionDataSource`
- `symbol`
- `from` / `to`
- `strategyEngine`
- `tradingFeeBps`
- `fundingRateBps`
- `fundingIntervalHours`
- `GET /api/v1/chart/annotations` — 图表标注数据
- `GET /api/v1/chart/candles` — K 线数据

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

当前 `paper session` 已经开始接入 signal runtime：
- 创建 `executionDataSource=tick` 的 paper session 时，会自动生成并挂上 `signalRuntimeSessionId`
- 启动 paper session 时，会先把 linked signal runtime 拉起
- `PAPER + tick + linked runtime` 现在会在收到真实 tick 后，按节流频率推进一次策略 heartbeat
- 每次事件驱动评估都会把当前 linked runtime 的 `sourceStates` 快照写入 paper session state
- 事件驱动评估现在还会做 `source gate` 检查：
  - 必需 trigger / feature 源没有状态快照时，不推进策略
  - 快照超过新鲜度窗口时，不推进策略
  - 默认新鲜度：`trade_tick=15s`，`order_book=10s`
  - 通过 `source gate` 后，paper 现在会调用策略引擎的 `signal evaluation` 入口：
    - 策略引擎可以基于 `trigger + sourceStates` 决定本次是 `advance-plan` 还是 `wait`
    - 同时会产出更明确的决策状态，例如：
      - `entry-ready`
      - `exit-ready`
      - `waiting-time`
      - `waiting-price`
      - `waiting-inputs`
    - 同时也会产出更偏策略语义的 `signalKind`：
      - `initial-entry`
      - `initial-entry-watch`
      - `initial-entry-near`
      - `initial-entry-near-strong`
      - `initial-entry-near-weak`
      - `sl-reentry`
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
TRADE_TICK_FRESHNESS_SECONDS=15
ORDER_BOOK_FRESHNESS_SECONDS=10
SIGNAL_BAR_FRESHNESS_SECONDS=30
RUNTIME_QUIET_SECONDS=30
PAPER_START_READINESS_TIMEOUT_SECONDS=5
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
- PostgreSQL 持久化目前覆盖策略、账户、订单、持仓、回测记录和模拟交易会话。
- `cmd/db-migrate` 执行嵌入式 SQL 迁移，并在 `schema_migrations` 表中记录迁移历史。
- 提交到 `PAPER` 模式账户的订单会立即成交，生成 `fills` 记录并更新净 `positions`。
- `GET /api/v1/account-summaries` 返回模拟账户的权益、费用、已实现/未实现盈亏及敞口快照。
- 净值快照在创建模拟会话时和模拟订单成交时自动追加。
- 模拟交易会话支持启动、停止和手动推进；活跃会话从 `FINAL_1D_LEDGER_BEST_SL.csv` 回放策略交易账本。
- 模拟会话状态在 `paper_sessions.state` 中持久化策略执行计划进度，`planIndex` 可跨重启保持。
- `GET /api/v1/runtime-policy` 返回统一运行阈值，前端告警和 paper preflight 共享同一套 freshness / quiet / readiness timeout 配置。
- `POST /api/v1/runtime-policy` 支持热更新运行阈值，当前为进程级生效；控制台 `Signals` 页面已提供对应配置面板。
Parity checks:
```bash
python3 scripts/check_1d_1min_parity.py
python3 scripts/check_4h_1min_parity.py
python3 scripts/check_tick_strategy_regression.py
```

说明：

- `1min` 路径当前有 Python 研究版对齐脚本。
- `tick` 路径当前使用 Go 主策略回放的稳定回归窗口校验，不再拿旧的 `run_tick_full_scan_dual` 做 parity；那段研究函数属于另一套历史参数口径，不是当前平台默认策略。
