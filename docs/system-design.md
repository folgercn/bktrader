# bkTrader Live Trading Platform Design

## 1. Goal

Build a production-oriented trading platform around the current BTCUSDT strategy line:

- signal timeframe: `1D`
- execution timeframe: `1m`
- zero initial position
- reentry-only risk allocation
- live trading + paper trading + backtesting
- TradingView used only for market display and trade annotations

The platform should support both research-to-production consistency and day-2 operational needs.

## 2. Product Scope

### In scope

- signal source management
- strategy version management
- account management for live and paper environments
- order and fill management
- real-time monitoring
- backtest orchestration and result storage
- paper trading
- chart datafeed and annotations for TradingView

### Out of scope for first release

- chart-embedded order placement
- multi-tenant SaaS billing
- high-frequency co-location execution

## 3. Architecture

The platform starts as a modular Go monolith and can split later.

```text
Frontend Console
  |- Strategy UI
  |- Orders / Positions UI
  |- Monitoring UI
  |- Backtest UI
  |- TradingView chart panel

Platform API
  |- Auth / Session
  |- Signal module
  |- Strategy module
  |- Account module
  |- Order module
  |- Backtest module
  |- Paper trading module
  |- Chart feed module

Core Infrastructure
  |- PostgreSQL
  |- Redis
  |- NATS
  |- Object storage (future)
  |- Metrics / Logs / Alerts
```

## 4. Core Domains

### 4.1 Signal Sources

Responsibilities:

- register internal strategy feeds and future external feeds
- deduplicate signals
- preserve full signal audit trail
- expose recent signals to UI and monitoring

Signal record fields:

- strategy version
- symbol
- side
- reason
- bar timestamp
- metadata including reentry index and stop mode

### 4.2 Strategy Management

Responsibilities:

- manage strategy definitions
- version parameters
- bind strategy versions to accounts
- keep runtime config immutable per deployment

Key parameters to snapshot:

- signal timeframe
- execution timeframe
- max trades per bar
- reentry size schedule
- stop mode
- stop loss ATR
- profit protect ATR

### 4.3 Account Management

Responsibilities:

- live exchange accounts
- paper accounts
- balances
- positions
- API credential metadata

### 4.4 Order Management

Responsibilities:

- order submission
- cancel / replace flows
- exchange status normalization
- fill reconciliation
- event sourcing for order state transitions

### 4.5 Real-time Monitoring

Responsibilities:

- strategy health
- signal heartbeat
- live orders and fills
- position and pnl tracking
- alerting on execution or sync failures

### 4.6 Backtests

Responsibilities:

- run parameterized backtests
- keep result snapshots reproducible
- store metrics and trade-level outputs
- expose chart annotations for order entry / exit markers
- separate signal timeframe from execution data source
- support execution-layer replay with `tick` as the primary backtest path

Backtest configuration rules:

- signal timeframe should reflect strategy decision bars, currently `4h` or `1d`
- execution data source should reflect fill simulation granularity, with `tick` as the default and primary backtest source
- `1min` is an execution proxy, not the strategy timeframe itself
- if `tick` data is unavailable, the backtest runner should fail loudly with a dataset error instead of silently falling back
- backtest options should expose discovered dataset files, supported symbols, and CSV schema so the UI can stop invalid runs before submission
- backtest runs should accept optional `from/to` windows so large tick archives can be replayed in bounded time slices

### 4.7 Paper Trading

Responsibilities:

- reuse real strategy and order model
- simulate exchange fills
- compare expected vs executable behavior before live rollout

## 5. TradingView Integration

TradingView is used only as:

- kline visualization
- indicator overlay
- order marker rendering
- backtest/live/paper annotation surface

Needed APIs:

- historical candles
- realtime candle updates
- chart annotations
- optional strategy markers and risk markers

No order placement is performed from the chart.

## 6. Execution Flow

1. market data arrives
2. strategy engine derives signal
3. signal is persisted
4. risk checks validate action
5. order intent is created
6. execution adapter routes order
7. order / fill updates are persisted
8. positions and account equity are updated
9. chart annotations and monitoring views are refreshed

## 7. Data Consistency Rules

- every order must reference strategy version when strategy-generated
- every fill must reference an order
- position state must be derivable from fills
- chart annotations should be generated from canonical order/fill events
- live, paper, and backtest trades should share the same annotation schema

## 8. Suggested Services for Later Split

- `marketdata-service`
- `signal-service`
- `strategy-engine-service`
- `risk-service`
- `execution-service`
- `account-service`
- `order-service`
- `backtest-service`
- `papertrade-service`
- `chartfeed-service`

## 9. Milestones

### Phase 1: MVP

- Go API
- strategy registry
- paper account model
- order and fill model
- backtest run records
- TradingView datafeed endpoints
- chart annotation endpoints

Current implementation status:

- pluggable repository layer with `memory` and `postgres` backends
- HTTP CRUD-style endpoints for strategies, accounts, orders, backtests, and paper sessions
- paper account orders are executed immediately into `fills` and net `positions`
- account summary snapshots expose start equity, fees, realized/unrealized PnL, and exposure
- account equity snapshots provide a time series for paper account net-equity charts
- paper sessions support background runners that replay the current strategy ledger and persist replay progress in session state
- chart annotation endpoint
- candle feed endpoint suitable for TradingView integration scaffolding
- PostgreSQL persistence implemented for strategies, accounts, orders, positions, backtest runs, and paper sessions
- backend selection controlled by `STORE_BACKEND`
- embedded SQL migrations with `cmd/db-migrate`
- optional local auto-migration controlled by `AUTO_MIGRATE`

Current paper runner details:

- replay source: `FINAL_1D_LEDGER_BEST_SL.csv`
- replay symbol: `BTCUSDT`
- session state stores `ledgerIndex`, last replay event metadata, and completion marker
- `notional=0` ledger rows are skipped so zero-initial bootstrap events do not create fake paper orders

### Phase 2: Live Trading

- exchange adapter
- live account sync
- risk rules
- alerting
- reconciliation jobs

### Phase 3: Production Hardening

- event bus
- retry orchestration
- fine-grained audit
- multi-account strategy rollout
- runbooks and dashboards

## 10. Initial API Surface

- `GET /healthz`
- `GET /api/v1/overview`
- `GET /api/v1/signal-sources`
- `GET /api/v1/strategies`
- `GET /api/v1/accounts`
- `GET /api/v1/orders`
- `GET /api/v1/backtests`
- `GET /api/v1/paper/sessions`
- `GET /api/v1/chart/annotations`

## 11. Repository Layout

```text
cmd/platform-api
internal/app
internal/config
internal/domain
internal/http
configs
db/migrations
deployments
docs
web/console
```

## 12. Next Build Steps

1. add persistent repositories for strategies, accounts, and orders
2. add Binance futures adapter
3. add paper matching engine
4. add TradingView-compatible candle datafeed endpoints
5. add authentication and user model
6. connect frontend console to platform API
