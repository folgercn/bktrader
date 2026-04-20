# 2026-04-20 ETH Q1 `reentry_window` Baseline, 1s Replay, `20%/10%`, `max_trades_per_bar=2`

Scope: research-only backtest work. No `internal/service` live or execution path was changed.

## Question

Rerun the ETH `Q1 2026` `1s`-bar replay after correcting the `reentry_window` real-order sizing logic.

Requested sizing semantics:

- first real order in a signal bar: `20%`
- second real order in the same signal bar: `10%`
- `max_trades_per_bar=2`

Requested signal timeframes:

- `4h`
- `1h`
- `30min`

## Research Logic Adjustment

The previous `reentry_window` implementation could still map the second real order in a bar back onto the first schedule bucket.

For this run, research logic was adjusted so that under:

- `dir2_zero_initial=true`
- `zero_initial_mode='reentry_window'`

the real-order count inside the bar is interpreted as:

- first real order -> `reentry_size_schedule[0]`
- second real order -> `reentry_size_schedule[1]`

This keeps the sizing aligned with the intended semantics instead of reusing the first bucket for both real orders.

## Shared Parameters

- Initial balance: `100000.0`
- Slippage: `0.0005`
- `dir2_zero_initial=true`
- `zero_initial_mode='reentry_window'`
- `stop_mode='atr'`
- `stop_loss_atr=0.05`
- `profit_protect_atr=1.0`
- `max_trades_per_bar=2`
- `reentry_size_schedule=[0.20, 0.10]`
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

Replay path:

- raw tick -> continuous `1s` bars
- `1s` bars -> `1min` bars
- `1min` bars -> signal bars
- execution replay on `1s` bars

## ETH Q1 2026 Results

| Timeframe | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | First Entry | Last Exit |
|---|---:|---:|---:|---:|---:|---:|---|---|
| `4h` | 151,400.17 | 51.40% | -0.13% | 180 | 94.44% | 21.61 | `2026-01-04 21:11:02+00:00` | `2026-03-31 14:34:50+00:00` |
| `1h` | 221,038.10 | 121.04% | -0.45% | 802 | 86.78% | 16.06 | `2026-01-01 22:55:36+00:00` | `2026-03-31 16:42:11+00:00` |
| `30min` | 263,418.74 | 163.42% | -0.90% | 1721 | 81.93% | 15.46 | `2026-01-01 11:04:30+00:00` | `2026-03-31 23:59:59+00:00` |

Signal-frame stats:

| Timeframe | Signal Rows | Signal Range | Valid MA20 Rows | Valid ATR Rows |
|---|---:|---|---:|---:|
| `4h` | 540 | `2026-01-01 00:00:00+00:00` to `2026-03-31 20:00:00+00:00` | 521 | 527 |
| `1h` | 2160 | `2026-01-01 00:00:00+00:00` to `2026-03-31 23:00:00+00:00` | 2141 | 2147 |
| `30min` | 4320 | `2026-01-01 00:00:00+00:00` to `2026-03-31 23:30:00+00:00` | 4301 | 4307 |

Entry / exit mix:

| Timeframe | Entry Reasons | Exit Reasons |
|---|---|---|
| `4h` | `SL-Reentry:106`, `Zero-Initial-Reentry:74` | `SL:180` |
| `1h` | `SL-Reentry:503`, `Zero-Initial-Reentry:299` | `SL:802` |
| `30min` | `SL-Reentry:1082`, `Zero-Initial-Reentry:638`, `PT-Reentry:1` | `SL:1719`, `PT:1`, `FinalMarkToMarket:1` |

## Read

This sizing change materially reduced trade count while improving trade quality:

- `4h` trades fell from `374` to `180`
- `1h` trades fell from `1447` to `802`
- `30min` trades fell from `3194` to `1721`

At the same time:

- `4h` return improved from `47.34%` to `51.40%`
- `1h` return improved from `86.39%` to `121.04%`
- `30min` return improved from `115.08%` to `163.42%`

The main interpretation is:

- limiting same-bar churn helped more than the larger first real order hurt
- forcing the second real order down to `10%` appears to reduce destructive same-bar overtrading
- the strategy benefits from taking fewer, more decisive same-bar entries

## Current Takeaway

On ETH `Q1 2026`, under the stricter `1s` replay path:

- `30min` remains the strongest return candidate
- `1h` also improves materially and remains attractive
- `4h` stays the cleanest risk profile and now also improves in return

This `20% / 10% / max=2` configuration is a stronger candidate than the previous `20%`-agnostic `10% / 5% / 2.5%` with `max_trades_per_bar=4` setting for the tested Q1 window.
