# 2026-04-20 ETH Q1 `reentry_window` Baseline, 1s Bar Replay

Scope: research-only backtest work. No `internal/service` live or execution path was changed.

## Question

Run ETH `Q1 2026` backtests with the latest research baseline, but use a finer execution approximation than the previous minute-order-aware replay:

- raw tick -> continuous `1s` bars
- `1s` bars -> `1min` bars
- `1min` bars -> signal timeframe bars
- execution replay on `1s` bars

Requested signal timeframes:

- `4h`
- `1h`
- `30min`

## Latest Baseline

Per project memory in `AGENTS.md`, the latest research baseline is:

- `dir2_zero_initial=true`
- `zero_initial_mode='reentry_window'`

Shared enhanced parameters:

- Initial balance: `100000.0`
- Slippage: `0.0005`
- `stop_mode='atr'`
- `stop_loss_atr=0.05`
- `profit_protect_atr=1.0`
- `max_trades_per_bar=4`
- `reentry_size_schedule=[0.10, 0.05, 0.025]`
- `trailing_stop_atr=0.3`
- `delayed_trailing_activation=0.5`
- `long_reentry_atr=0.1`
- `short_reentry_atr=0.0`
- `reentry_anchor_levels='wick'`
- `reentry_trigger_mode='reclaim'`

## Data And Construction Path

Raw tick source:

- `dataset/archive/ETHUSDT-trades-2026-01/ETHUSDT-trades-2026-01.csv`
- `dataset/archive/ETHUSDT-trades-2026-02/ETHUSDT-trades-2026-02.zip`
- `dataset/archive/ETHUSDT-trades-2026-03/ETHUSDT-trades-2026-03.zip`

Window:

- `2026-01-01 00:00:00+00:00` to `2026-03-31 23:59:59+00:00`

Construction summary:

- raw tick rows processed: `748,747,244`
- continuous `1s` rows built: `7,776,000`
- derived `1min` rows built: `129,600`

Important note:

- this path is stricter than plain `1min OHLC` replay because execution sees second-level price movement
- it is still an approximation, not a full raw-tick execution simulator, because multiple ticks inside the same second are compressed into one `1s OHLC` bar

## ETH Q1 2026 Results

| Timeframe | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | First Entry | Last Exit |
|---|---:|---:|---:|---:|---:|---:|---|---|
| `4h` | 147,342.50 | 47.34% | -0.13% | 374 | 85.56% | 18.73 | `2026-01-04 21:11:02+00:00` | `2026-03-31 14:49:09+00:00` |
| `1h` | 186,386.65 | 86.39% | -0.42% | 1447 | 76.30% | 14.12 | `2026-01-01 22:55:36+00:00` | `2026-03-31 16:46:07+00:00` |
| `30min` | 215,080.61 | 115.08% | -0.73% | 3194 | 72.67% | 13.02 | `2026-01-01 11:04:30+00:00` | `2026-03-31 23:59:59+00:00` |

Signal-frame stats:

| Timeframe | Signal Rows | Signal Range | Valid MA20 Rows | Valid ATR Rows |
|---|---:|---|---:|---:|
| `4h` | 540 | `2026-01-01 00:00:00+00:00` to `2026-03-31 20:00:00+00:00` | 521 | 527 |
| `1h` | 2160 | `2026-01-01 00:00:00+00:00` to `2026-03-31 23:00:00+00:00` | 2141 | 2147 |
| `30min` | 4320 | `2026-01-01 00:00:00+00:00` to `2026-03-31 23:30:00+00:00` | 4301 | 4307 |

Entry / exit mix:

| Timeframe | Entry Reasons | Exit Reasons |
|---|---|---|
| `4h` | `SL-Reentry:385`, `Zero-Initial-Reentry:65`, `PT-Reentry:2` | `SL:373`, `PT:1` |
| `1h` | `SL-Reentry:1430`, `Zero-Initial-Reentry:285`, `PT-Reentry:1` | `SL:1446`, `PT:1` |
| `30min` | `SL-Reentry:3248`, `Zero-Initial-Reentry:602`, `PT-Reentry:2` | `SL:3191`, `PT:2`, `FinalMarkToMarket:1` |

## Read

Under this `1s` execution approximation:

- `30min` remains the highest-return intraday candidate in this batch
- `1h` stays strong and materially profitable
- `4h` gives the cleanest risk profile, with the shallowest drawdown and highest win rate

Compared with the earlier minute-order-aware replay, the broad ranking does not flip:

- `30min` still leads on return
- `1h` remains a valid candidate
- the strategy is still not showing evidence that coarser execution modeling was flattering only one side of the comparison

## Current Takeaway

For ETH `Q1 2026`, with the current enhanced `reentry_window` baseline:

- keep `30min`, `1h`, and `4h` as valid candidates
- if prioritizing raw return, `30min` is still best in this batch
- if prioritizing risk cleanliness, `4h` looks strongest
- this `1s` path is more trustworthy than a plain `1min OHLC` replay, but it still does not fully replace a true raw-tick execution simulator
