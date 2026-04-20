# 2026-04-20 ETH Q1 Intraday `reentry_window` Baseline, Tick Replay

Scope: research-only backtest work. No `internal/service` live or execution path was changed.

## Question

Run the latest research baseline on ETH `2026 Q1` for intraday signal timeframes:

- `1h`
- `30min`
- `5min`

Method constraint for this run:

- large-period signal bars must come from tick-derived `1min` data, not from exchange-provided higher-timeframe klines
- minute-level price occurrence order must be considered during replay, so the execution path should not rely on plain `1min OHLC` alone

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

## Data And Replay Path

Signal-bar source:

- `ETH_1min_Q1.csv`
- This file was previously generated directly from raw ETH Q1 tick data. See [20260416_breakout_reentry_experiments.md](./20260416_breakout_reentry_experiments.md).

Raw tick replay source:

- `dataset/archive/ETHUSDT-trades-2026-01/ETHUSDT-trades-2026-01.csv`
- `dataset/archive/ETHUSDT-trades-2026-02/ETHUSDT-trades-2026-02.zip`
- `dataset/archive/ETHUSDT-trades-2026-03/ETHUSDT-trades-2026-03.zip`

Replay window:

- `2026-01-01 00:00:00+00:00` to `2026-03-31 23:59:00+00:00`

Important execution detail:

- signal bars were aggregated from the raw-tick-derived `1min` file
- replay used raw tick event streams built with intra-minute ordering preserved via `high_ts` / `low_ts`
- combined Q1 tick event stream count: `488,115`
- combined event range: `2026-01-01 00:00:01.703+00:00` to `2026-03-31 23:58:59.356+00:00`

This means the run is materially stricter than a plain `1min OHLC` backtest, because the engine sees the minute-internal path ordering instead of only the final `open/high/low/close`.

## ETH Q1 2026 Results

| Timeframe | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | First Entry | Last Exit |
|---|---:|---:|---:|---:|---:|---:|---|---|
| `1h` | 193,219.18 | 93.22% | -0.51% | 1497 | 76.15% | 13.71 | `2026-01-01 22:55:37.017+00:00` | `2026-03-31 16:50:26.695+00:00` |
| `30min` | 223,607.35 | 123.61% | -0.77% | 3384 | 70.86% | 11.96 | `2026-01-01 11:04:53.005+00:00` | `2026-03-31 23:29:59.720+00:00` |
| `5min` | 39,458.59 | -60.54% | -60.54% | 22,755 | 37.57% | 6.45 | `2026-01-01 01:41:57.986+00:00` | `2026-03-31 23:54:59.689+00:00` |

Signal-frame stats:

| Timeframe | Signal Rows | Signal Range | Valid MA20 Rows | Valid ATR Rows |
|---|---:|---|---:|---:|
| `1h` | 2160 | `2026-01-01 00:00:00+00:00` to `2026-03-31 23:00:00+00:00` | 2141 | 2147 |
| `30min` | 4320 | `2026-01-01 00:00:00+00:00` to `2026-03-31 23:30:00+00:00` | 4301 | 4307 |
| `5min` | 25920 | `2026-01-01 00:00:00+00:00` to `2026-03-31 23:55:00+00:00` | 25901 | 25907 |

Entry / exit mix:

| Timeframe | Entry Reasons | Exit Reasons |
|---|---|---|
| `1h` | `SL-Reentry:1546`, `Zero-Initial-Reentry:279` | `SL:1497` |
| `30min` | `SL-Reentry:3706`, `Zero-Initial-Reentry:575`, `PT-Reentry:3` | `SL:3380`, `PT:3`, `FinalMarkToMarket:1` |
| `5min` | `SL-Reentry:31311`, `Zero-Initial-Reentry:2624`, `PT-Reentry:16` | `SL:22740`, `PT:14`, `FinalMarkToMarket:1` |

## Read

Under the stricter tick-replay path:

- `30min` is the strongest intraday result in this Q1 batch
- `1h` remains strong, but is weaker than `30min`
- `5min` breaks badly once minute-internal price order is respected

The `5min` failure is the most important outcome here. On a coarse `1min OHLC` path, very small signal bars can still look attractive because the engine does not know whether the minute hit favorable or unfavorable prices first. Once the replay sees the tick-ordered path, that optimism disappears and the strategy overtrades into a deep drawdown.

## Current Takeaway

For ETH `Q1 2026`, with the current enhanced `reentry_window` baseline and order-aware tick replay:

- keep `30min` and `1h` as viable intraday research candidates
- treat `5min` as a rejected baseline candidate for now
- prefer any future low-timeframe comparison to use this tick-replay path, not a plain `1min OHLC` shortcut
