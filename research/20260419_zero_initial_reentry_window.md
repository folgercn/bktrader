# 2026-04-19 Zero-Initial Reentry Window Research

Scope: research-only experiment on a separate branch. No `internal/service` live or replay code was changed.

## Question

Current canonical zero-initial semantics still creates a persistent zero-notional position after `Initial` breakout, then manages that synthetic position with `SL/PT` exits.

This experiment tests an alternative research-only semantic:

- `dir2_zero_initial=true`
- `Initial` breakout does **not** create a persistent position
- breakout only opens a reentry window for:
  - the current signal bar
  - the next signal bar
- if no reentry signal appears inside that window, state resets and the engine waits for a fresh breakout again

Implemented as `--zero-initial-mode reentry_window` in `research/ma_filter_backtest.py`.

## Important Caveat

The reentry trigger remains the existing `reclaim` rule:

- long: `high >= prev_low_1 + 0.1 * ATR`
- short: `low <= prev_high_1`

That means this experiment is **not** "breakout must wait until the next signal bar". It is:

- breakout opens a current+next-bar reentry window
- reentry can still happen inside the same signal bar if `reP` is already satisfied

So this experiment isolates one thing only:

- remove the long-lived zero-notional synthetic position
- keep reentry as the first real exposure

## Config

Shared config for the comparisons below:

- `dir2_zero_initial=true`
- `stop_mode=atr`
- `stop_loss_atr=0.05`
- `profit_protect_atr=1.0`
- `reentry_anchor_levels=wick`
- `reentry_trigger_mode=reclaim`
- `max_trades_per_bar=4`
- `reentry_size_schedule=[0.10, 0.05, 0.025]`

## BTC 1D Q1 2026

Data:

- `BTC_1min_Q1.csv`
- window: `2026-01-01` to `2026-03-31`
- filter: `SMA5 hard`
- early reversal gate: `0.06 ATR`

| Scenario | Final Balance | Return | Max DD | Trade Pairs | Entry Reasons |
|---|---:|---:|---:|---:|---|
| Baseline `position` | 101,656.40 | 1.66% | -0.33% | 52 | `Initial:10`, `PT-Reentry:18`, `SL-Reentry:25` |
| Variant `reentry_window` | 106,203.96 | 6.20% | -0.33% | 44 | `Zero-Initial-Reentry:10`, `PT-Reentry:18`, `SL-Reentry:17` |

Delta, `reentry_window - position`:

- Final balance: `+4,547.56`
- Return: `+4.55 pp`
- Max drawdown: `+0.00 pp` (no material change)
- Trade pairs: `-8`

## ETH 4H Q1 2026

Data:

- `ETH_1min_Q1.csv`
- derived signal file: `research/ETH_4H_Q1_signals.csv`
- window: `2026-01-01` to `2026-03-31`
- filter: `SMA20 hard`
- no early reversal gate

| Scenario | Final Balance | Return | Max DD | Trade Pairs | Entry Reasons |
|---|---:|---:|---:|---:|---|
| Baseline `position` | 101,447.67 | 1.45% | -1.77% | 392 | `Initial:50`, `PT-Reentry:83`, `SL-Reentry:260` |
| Variant `reentry_window` | 108,023.66 | 8.02% | -0.79% | 356 | `Zero-Initial-Reentry:50`, `PT-Reentry:83`, `SL-Reentry:224` |

Delta, `reentry_window - position`:

- Final balance: `+6,575.99`
- Return: `+6.58 pp`
- Max drawdown: `+0.98 pp` improvement
- Trade pairs: `-36`

## Initial Read

On both sampled runs, replacing the persistent zero-notional synthetic `Initial` position with a short-lived reentry window improved returns and reduced churn.

The most likely reason is:

- baseline zero-initial creates many long-lived synthetic positions
- those synthetic positions then consume `SL/PT` management cycles even though no real exposure exists yet
- the windowed variant delays real exposure until a reentry trigger actually appears, which removes a class of synthetic exit noise

## Next Step

If we want to treat this as a canonical candidate rather than a one-off idea, the next research step should be:

1. run the same comparison on longer BTC / ETH windows
2. decide whether same-bar reclaim should remain allowed for zero-initial windows
3. only then consider porting the semantic into Go replay/live together
