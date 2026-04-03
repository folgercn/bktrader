# bkTrader Platform

This repository now contains two parts:

- legacy strategy research and backtesting assets for BTCUSDT
- a new live trading platform scaffold in Go for signal-driven execution, monitoring, paper trading, and backtesting

## New platform layout

- `cmd/platform-api`: Go API entrypoint
- `internal`: platform application code
- `web/console`: frontend console scaffold
- `docs`: architecture and system design
- `configs`: example config files
- `deployments`: local infra bootstrap
- `db/migrations`: initial database schema

## Current strategy defaults

The platform is being designed around the current preferred strategy profile:

- timeframe: `1D` signal, `1m` execution
- zero initial position
- `max_trades_per_bar=3`
- reentry risk sizing: `10%` then `20%`
- stop mode: `atr`

## Quick start

### Backend

```bash
cp configs/app.example.env .env
go run ./cmd/platform-api
```

By default the API starts with `STORE_BACKEND=memory`.

To run with PostgreSQL persistence:

```bash
docker compose -f deployments/docker-compose.dev.yml up -d
go run ./cmd/db-migrate
export STORE_BACKEND=postgres
export POSTGRES_DSN=postgres://postgres:postgres@localhost:5432/bktrader?sslmode=disable
go run ./cmd/platform-api
```

To let the API apply migrations automatically in local development:

```bash
export STORE_BACKEND=postgres
export AUTO_MIGRATE=true
go run ./cmd/platform-api
```

Available MVP endpoints:

- `GET /healthz`
- `GET /api/v1/overview`
- `GET|POST /api/v1/strategies`
- `GET|POST /api/v1/accounts`
- `GET /api/v1/account-summaries`
- `GET /api/v1/account-equity-snapshots?accountId=...`
- `GET|POST /api/v1/orders`
- `GET /api/v1/fills`
- `GET /api/v1/positions`
- `GET|POST /api/v1/backtests`
- `GET|POST /api/v1/paper/sessions`
- `POST /api/v1/paper/sessions/{id}/start`
- `POST /api/v1/paper/sessions/{id}/stop`
- `POST /api/v1/paper/sessions/{id}/tick`
- `GET /api/v1/signal-sources`
- `GET /api/v1/chart/annotations`
- `GET /api/v1/chart/candles`

### Frontend scaffold

```bash
cd web/console
npm install
export VITE_API_BASE=http://127.0.0.1:8080
npm run dev
```

## Notes

- Existing research files were kept in place to avoid disrupting strategy work.
- The platform scaffold is intentionally modular but starts as a deployable monolith so it can move fast early and split later.
- Phase 1 supports both in-memory and PostgreSQL repository backends selected with `STORE_BACKEND`.
- PostgreSQL persistence currently covers strategies, accounts, orders, positions, backtest runs, and paper sessions.
- `cmd/db-migrate` applies embedded SQL migrations and records them in `schema_migrations`.
- Orders submitted to `PAPER` accounts are filled immediately, create `fills`, and update net `positions`.
- `GET /api/v1/account-summaries` returns paper-account equity, fees, realized/unrealized PnL, and exposure snapshots.
- Equity snapshots are appended when a paper session is created and when a paper order is filled.
- Paper sessions can now be started, stopped, or manually ticked; active sessions replay the project trade ledger from `FINAL_1D_LEDGER_BEST_SL.csv`.
- Paper session state persists replay progress in `paper_sessions.state`, so `ledgerIndex` survives restarts.
