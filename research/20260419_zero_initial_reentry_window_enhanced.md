# 2026-04-19 Zero-Initial Reentry Window, Enhanced Baseline

Scope: research-only experiment on a separate branch. No `internal/service` live or replay code was changed.

## Question

Evaluate the same `zero_initial_mode=reentry_window` idea against the current enhanced ETH `4h` baseline, instead of the simplified `ma_filter_backtest.py` path.

This isolates one semantic change:

- `dir2_zero_initial=true`
- breakout does **not** create a long-lived zero-notional synthetic position
- breakout only opens a reentry window for:
  - the current signal bar
  - the next signal bar
- if no reentry appears inside that window, state resets and the engine waits for a fresh breakout again

Enhanced path used: `research/backTest.py:run_backtest_enhanced()`.

## Shared Setup

- Data: `/Users/wuyaocheng/Downloads/bkTrader/ETH_1min_Q1.csv`
- Window: `2026-01-01` to `2026-03-31`
- Signal timeframe: `4h`
- Initial balance: `100000.0`
- Slippage: `0.0005`
- `dir2_zero_initial=true`
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

## Important Caveat

This compare is apples-to-apples on the current codebase, data, and parameters.

It does **not** exactly reproduce the older archived note that reported `34.35%` for ETH `4h` Q1. On current `origin/main`-based code in this branch, the same enhanced baseline reproduces at `31.77%`.

That archive drift matters for historical comparison, but it does not invalidate this experiment because both scenarios below run on the same current engine.

## ETH 4H Q1 2026

| Scenario | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Entry Reasons | Exit Reasons |
|---|---:|---:|---:|---:|---:|---:|---|---|
| Baseline `position` | 131,774.05 | 31.77% | -0.12% | 299 | 80.60% | 16.67 | `Initial:68`, `PT-Reentry:3`, `SL-Reentry:367` | `PT:3`, `SL:432` |
| Variant `reentry_window` | 144,458.56 | 44.46% | -0.12% | 348 | 83.05% | 17.42 | `Zero-Initial-Reentry:65`, `PT-Reentry:3`, `SL-Reentry:356` | `PT:3`, `SL:421` |

Delta, `reentry_window - position`:

- Final balance: `+12,684.51`
- Return: `+12.68 pp`
- Max drawdown: `+0.00 pp` (no material change)
- Trades: `+49`
- Win rate: `+2.44 pp`
- Sharpe: `+0.75`

## BTC 1D Q1 2026

| Scenario | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Entry Reasons | Exit Reasons |
|---|---:|---:|---:|---:|---:|---:|---|---|
| Baseline `position` | 117,616.10 | 17.62% | -0.17% | 78 | 83.33% | 20.47 | `Initial:15`, `SL-Reentry:97` | `SL:112` |
| Variant `reentry_window` | 123,531.85 | 23.53% | -0.17% | 86 | 84.88% | 21.37 | `Zero-Initial-Reentry:15`, `SL-Reentry:90` | `SL:105` |

Delta, `reentry_window - position`:

- Final balance: `+5,915.75`
- Return: `+5.92 pp`
- Max drawdown: `+0.00 pp` (no material change)
- Trades: `+8`
- Win rate: `+1.55 pp`
- Sharpe: `+0.90`

## ETH 1D Q1 2026

| Scenario | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Entry Reasons | Exit Reasons |
|---|---:|---:|---:|---:|---:|---:|---|---|
| Baseline `position` | 121,632.54 | 21.63% | -0.04% | 54 | 98.15% | 26.24 | `Initial:16`, `SL-Reentry:67` | `SL:83` |
| Variant `reentry_window` | 131,336.73 | 31.34% | -0.04% | 63 | 98.41% | 27.89 | `Zero-Initial-Reentry:16`, `SL-Reentry:60` | `SL:76` |

Delta, `reentry_window - position`:

- Final balance: `+9,704.20`
- Return: `+9.70 pp`
- Max drawdown: `+0.00 pp` (no material change)
- Trades: `+9`
- Win rate: `+0.26 pp`
- Sharpe: `+1.65`

## Read

Across the three enhanced Q1 runs completed so far, replacing the persistent zero-notional `Initial` position with a current+next-bar reentry window improved returns without increasing drawdown.

The main behavioral shift is:

- baseline keeps a synthetic position alive and spends many bars managing its `SL/PT`
- `reentry_window` waits for the first actual reentry trigger before creating exposure
- the trade mix shifts from `Initial` entries into `Zero-Initial-Reentry` entries, while downstream `SL-Reentry` remains the dominant path

Current enhanced Q1 deltas:

- `ETH 4h`: `+12.68 pp`
- `BTC 1d`: `+5.92 pp`
- `ETH 1d`: `+9.70 pp`

## Next Step

If this semantic is a serious candidate for canonical strategy behavior, the next research step should be:

1. rerun the same enhanced compare on BTC `1d` and ETH `1d`
2. decide whether same-bar `reclaim` should remain allowed after a breakout opens the window
3. only then consider replay/live alignment work
