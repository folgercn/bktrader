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
go run ./cmd/platform-api
```

### Frontend scaffold

```bash
cd web/console
npm install
npm run dev
```

## Notes

- Existing research files were kept in place to avoid disrupting strategy work.
- The platform scaffold is intentionally modular but starts as a deployable monolith so it can move fast early and split later.

