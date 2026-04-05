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

- register pluggable exchange and replay feeds
- support both strategy-level and account-level source bindings
- allow multiple sources per strategy and per account
- distinguish trigger sources from feature sources
- preserve full signal audit trail
- expose recent signals to UI and monitoring

Signal record fields:

- strategy version
- symbol
- side
- reason
- bar timestamp
- metadata including reentry index and stop mode

Source binding rules:

- strategy bindings answer "which inputs does this strategy require"
- account bindings answer "which market feeds does this account actually subscribe to"
- one strategy may bind multiple sources, for example:
  - `BINANCE trade tick` as `trigger`
  - `BINANCE order book` as `feature`
- one account may also bind multiple sources, for example:
  - `BINANCE trade tick` for local execution triggering
  - `OKX trade tick` or `OKX order book` for cross-market arbitrage observation
- trigger and feature sources must be modeled separately so future order-book features do not get mixed with execution trigger streams
- before paper/live starts, the platform should build a `signal runtime plan`:
  - match required strategy bindings against active account bindings
  - resolve which runtime adapter should drive each source
  - fail early when required trigger streams are missing
  - surface optional feature-stream gaps separately from blocking trigger gaps
- after planning, the platform should create a `signal runtime session`:
  - persist the resolved subscription set
  - track runtime adapter, transport, health and heartbeat
  - expose recent event summaries for operational visibility
  - maintain structured per-source state snapshots so downstream strategy evaluation can read stable trigger / feature state instead of raw last-message strings
  - serve as the control point for starting/stopping exchange market-data consumers
- runtime rollout order:
  - first connect public exchange trade-tick streams and verify heartbeats / recent events
  - then extend to order-book streams
  - only after stream stability is confirmed should strategy-trigger execution be switched onto the live runtime path

Current implementation status:

- Binance public market websocket:
  - trade tick: connected
  - order book: connected
  - session health / heartbeat / recent event summary: connected
- OKX websocket:
  - adapter and subscription plan reserved
  - full live message consumption pending

Paper/runtime integration status:

- paper sessions now link to signal runtime sessions when source bindings exist
- tick-based paper sessions require a ready signal runtime plan before start
- starting a linked paper session will start the associated market-data runtime first
- linked tick events now drive a throttled paper-session heartbeat
- each signal-driven paper evaluation now records the linked runtime `sourceStates` snapshot into paper session state
- before paper evaluation advances strategy execution, the platform now checks a `source gate`:
  - all required strategy bindings must have a source-state snapshot
  - required snapshots must be fresh enough for their stream type
  - default freshness windows are short and stream-specific so stale market-data does not silently drive execution
- after source gating passes, the platform now calls a strategy-engine-level `signal evaluation` hook:
  - the engine receives trigger summary + structured source-state snapshot
  - the engine decides whether this event should advance execution or wait
  - the engine also sees the next planned execution timestamp so paper runtime can respect event-time ordering instead of advancing on any incoming tick
  - this hook is the migration path from plan-driven paper execution to true real-time strategy decisions
- strategy triggering is still a minimal event-driven rollout:
  - real tick events update the linked paper session
  - the session is nudged forward by runtime events at a throttled cadence
  - full per-tick strategy evaluation replacement is still pending, but the decision entrypoint now lives in the strategy engine instead of the paper runner

### 4.2 Strategy Management

Responsibilities:

- manage strategy definitions
- version parameters
- register pluggable strategy engines
- bind strategy versions to accounts
- keep runtime config immutable per deployment

Runtime rules:

- strategy modules must be pluggable; the platform resolves a `StrategyEngine` by key instead of hard-coding one strategy into each workflow
- signal sources must also be pluggable; the platform resolves registered source definitions by key instead of hard-coding Binance-only or single-stream behavior
- the same strategy engine must be used across backtest, paper, and live modes
- live exchange connectivity must also be pluggable; the platform resolves a `LiveExecutionAdapter` by key per account binding
- the only allowed execution-semantic difference is slippage:
  - `BACKTEST`: simulated slippage may be injected
  - `PAPER` / `LIVE`: no extra synthetic slippage inside strategy execution; fills must come from canonical execution flow
- fees and funding:
  - `BACKTEST`: trading fee / funding are configurable parameters
  - `PAPER`: trading fee is configurable; funding should be configurable when paper holding lifecycle is promoted to the canonical engine path
- `LIVE`: trading fee / funding / rebates must come from exchange responses and reconciled ledgers
- `LIVE` account credentials should be referenced indirectly (`apiKeyRef`, `apiSecretRef`) rather than embedded in strategy configuration

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
- support execution-layer replay with selectable `tick` or `1min` execution sources

Backtest configuration rules:

- signal timeframe should reflect strategy decision bars, currently `4h` or `1d`
- execution data source should reflect fill simulation granularity, with `tick` and `1min` both available as selectable execution tests
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

Execution consistency rule:

- strategy decision logic and order-intent generation must stay identical across backtest, paper, and live
- execution adapters may differ by environment, but strategy code must not fork behavior except for backtest-only slippage simulation
- signal runtime wiring must also stay environment-consistent:
  - backtest resolves replay file loaders
  - paper/live resolves exchange market-data adapters
  - the strategy should see the same logical source roles (`trigger`, `feature`) even when the underlying transport differs
- cost accounting may differ only by data source:
  - backtest/paper use configured fee models
  - live uses exchange-reported fee/funding records

## 7. Data Consistency Rules

- every order must reference strategy version when strategy-generated
- every strategy version should snapshot its source bindings
- every account should persist its active source bindings
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
- pluggable `StrategyEngine` registry with built-in `bk-default`
- pluggable `LiveExecutionAdapter` registry with built-in `binance-futures`
- HTTP CRUD-style endpoints for strategies, accounts, orders, backtests, and paper sessions
- `GET /api/v1/strategy-engines` exposes registered strategy engines to the UI
- paper account orders are executed immediately into `fills` and net `positions`
- account summary snapshots expose start equity, fees, realized/unrealized PnL, and exposure
- account equity snapshots provide a time series for paper account net-equity charts
- paper sessions support background runners that prebuild canonical strategy execution plans and persist replay progress in session state
- paper session creation supports runtime overrides for timeframe, execution source, symbol, range, and cost semantics
- live account bindings persist to `accounts.metadata.liveBinding` with adapter key, credential refs, and execution-mode settings
- `LIVE` orders now route through the bound `LiveExecutionAdapter` and persist adapter acknowledgements in order metadata
- chart annotation endpoint
- candle feed endpoint suitable for TradingView integration scaffolding
- PostgreSQL persistence implemented for strategies, accounts, orders, positions, backtest runs, and paper sessions
- backend selection controlled by `STORE_BACKEND`
- embedded SQL migrations with `cmd/db-migrate`
- optional local auto-migration controlled by `AUTO_MIGRATE`

Current backtest focus:

- primary workflow is pluggable strategy replay on `tick` or `1min` data sources
- users choose the signal timeframe (`4h` / `1d`) separately from the execution source
- backtest runtime is now resolved through `StrategyEngine` plus `ExecutionSemantics`
- `BACKTEST` semantics allow simulated slippage; `PAPER/LIVE` semantics are intended to share canonical execution behavior

Current paper runner details:

- runtime source: registered `StrategyEngine`
- replay symbol: strategy-configured symbol, currently defaulting to `BTCUSDT`
- session state stores `planIndex`, runtime semantics, last executed event metadata, and completion marker
- `PAPER` uses the same canonical strategy runtime as backtests, with observed execution semantics and configurable fees/funding

### Phase 2: Live Trading

- exchange adapter
- live account sync
- risk rules
- alerting
- reconciliation jobs

Current live adapter details:

- `GET /api/v1/live-adapters` lists registered live adapters
- `POST /api/v1/live/accounts/{id}/binding` binds a `LIVE` account to an adapter using credential references
- `POST /api/v1/orders` for a bound `LIVE` account resolves the adapter and stores an `ACCEPTED` acknowledgement
- `POST /api/v1/orders/{id}/sync` asks the adapter for the latest exchange order state and materializes fills locally
- current `binance-futures` implementation is a mock submission adapter that returns exchange-style metadata without hitting the network
- current mock sync path can transition `ACCEPTED -> FILLED`, create `fills`, and update `positions`

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
