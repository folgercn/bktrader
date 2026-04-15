# ETH Tick Replay Research

## Scope

This branch focuses on two closely related research tasks:

1. Make `research/backTest.py` support a safer monthly tick replay path that stays aligned with the current enhanced 1-minute backtest logic.
2. Download ETHUSDT 2023 monthly tick archives and compare `tick replay` vs `1min OHLC replay` under the same strategy configuration.

The intent is to answer a practical question:

- Does the edge still exist once we replay a more realistic intraminute path?

## What Changed

### 1. Tick replay logic in `research/backTest.py`

`run_tick_full_scan_dual(...)` no longer relies on the older raw per-tick scan assumptions.

The updated flow is:

1. Load raw monthly tick CSV in chunks.
2. Aggregate ticks into minute summaries:
   - `open`
   - `high`
   - `low`
   - `close`
   - `first_ts`
   - `last_ts`
   - `high_ts`
   - `low_ts`
3. Reconstruct a minute-level event stream that preserves intraminute ordering:
   - `open`
   - then `high/low` in actual observed order
   - then `close`
4. Reuse the current enhanced strategy state machine on top of that event stream.

This makes tick replay:

- much closer to the current `1min` backtest semantics
- more realistic than plain `1min OHLC`
- still fast enough to run month-by-month on full 2023 data

The parser now supports both archive formats we observed:

- BTC monthly files without a header
- ETH monthly files with header:
  - `id,price,qty,quote_qty,time,is_buyer_maker`

### 2. ETH data scripts

The ETH research helpers were cleaned up so the dataset pipeline can run reliably:

- `research/ethusdt_1min.py`
- `research/parquet_to_csv_ETHUSDT.py`

Notable fixes:

- funding schema compatibility
- open-interest API defensive handling
- parquet fallback via `pyarrow` when pandas parquet support is missing

## Test Data

### ETH 1-minute clean data

- Source file:
  - `ETH_1min_Clean.csv`

### ETH 2023 monthly tick archives

Downloaded under:

- `dataset/archive/ETHUSDT-trades-2023-01/...`
- ...
- `dataset/archive/ETHUSDT-trades-2023-12/...`

Total downloaded size is about `46.3 GB`.

## Strategy Configuration Used

All `ETH 2023 tick vs 1min` comparisons in this branch used the same canonical enhanced configuration:

- `dir2_zero_initial=True`
- `fixed_slippage=0.0005`
- `stop_loss_atr=0.05`
- `stop_mode='atr'`
- `max_trades_per_bar=4`
- `reentry_size_schedule=[0.10, 0.05, 0.025]`
- `trailing_stop_atr=0.3`
- `delayed_trailing_activation=0.5`
- `long_reentry_atr=0.1`
- `short_reentry_atr=0.0`

## Backtests Run

### A. BTC 2023 full-year control run

Purpose:

- validate that the new tick replay path stays close to the current enhanced `1min` engine

Result:

- `enhanced_1m`: `+69.41%`
- `tick_replay`: `+67.77%`

Interpretation:

- the updated tick replay is aligned with the current strategy logic
- `1min OHLC` is still mildly optimistic, but not catastrophically divergent

### B. ETH 2023 full-year tick vs 1min comparison

Output files:

- `tmp_eth_2023_full_tick_vs_1m_overall.csv`
- `tmp_eth_2023_full_tick_vs_1m_monthly.csv`
- `tmp_eth_2023_full_1m_ledger.csv`
- `tmp_eth_2023_full_tick_ledger.csv`

Overall result:

- `enhanced_1m`
  - return: `+79.54%`
  - trades: `273`
  - maxDD: `-0.0888%`
  - final balance: `179,540.45`

- `tick_replay`
  - return: `+65.97%`
  - trades: `245`
  - maxDD: `-0.0830%`
  - final balance: `165,965.89`

Monthly tick replay result:

- `2023-01`: `+5.30%`
- `2023-02`: `+9.44%`
- `2023-03`: `+5.38%`
- `2023-04`: `+4.41%`
- `2023-05`: `+5.85%`
- `2023-06`: `+3.90%`
- `2023-07`: `+1.49%`
- `2023-08`: `+3.78%`
- `2023-09`: `+1.48%`
- `2023-10`: `+4.31%`
- `2023-11`: `+6.29%`
- `2023-12`: `+0.43%`

## Main Findings

### 1. The edge still exists under tick replay

ETH 2023 remains clearly profitable after moving from `1min OHLC` replay to the more realistic tick-derived replay.

That is the most important result from this branch.

### 2. `1min OHLC` is still optimistic

The same strategy on the same year drops from:

- `+79.54%` in enhanced `1min`
- to `+65.97%` in tick replay

So the minute-bar version overstates annual performance by roughly `13.58` percentage points in this configuration.

### 3. The difference mainly comes from intraminute path realism

The new tick replay preserves whether price reached:

- `high` before `low`
- or `low` before `high`

inside a minute.

That matters for:

- breakout entry timing
- same-minute whipsaw exits
- quick `SL -> reentry` chains
- trailing/protection updates

### 4. ETH still looks stronger than BTC, but less extreme after tick replay

The older `1min-only` ETH studies looked extremely strong.

This branch suggests:

- ETH still has edge
- but part of the apparent strength was amplified by `1min OHLC` assumptions

That makes the tick-based number more credible for further parameter research.

## Practical Conclusion

For research quality, ETH should no longer be judged only by `1min OHLC` replay.

The safer default interpretation is:

- use `1min` replay for fast iteration
- use tick replay to validate whether a promising edge survives intraminute ordering

## Validation Performed

- `python3 -m py_compile research/backTest.py`
- `python3 -m py_compile research/ethusdt_1min.py research/parquet_to_csv_ETHUSDT.py`
- ETH monthly tick replay smoke on `2023-01`
- ETH full-year `2023 tick vs 1min` run
- BTC 2023 full-year control comparison

## Notes

- Attempted graph refresh per repo instructions:
  - `python3 -c "from graphify.watch import _rebuild_code; from pathlib import Path; _rebuild_code(Path('.'))"`
- Current local environment does not have the `graphify` Python module installed, so refresh failed with:
  - `ModuleNotFoundError: No module named 'graphify'`
