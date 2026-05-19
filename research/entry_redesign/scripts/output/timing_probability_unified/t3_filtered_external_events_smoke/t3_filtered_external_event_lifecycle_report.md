# T3 Filtered External Event Lifecycle

Research-only strict lifecycle replay for filtered scored T3 event sources.

- Scored events: `/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/t3_probability_overlay_extended/t3_probability_overlay_scored_events.csv`
- Timeframe: `1h`
- Reentry fill policy: `strict_next_second_cross`
- External entry mode: `reentry_window`
- T3 exit overrides: `{"min_hold_seconds_before_sl": 3600.0}`
- Months: 2026-03
- Symbols: ETHUSDT, BTCUSDT

## Candidate Summary

| Candidate | Calendar Sum | Avg/Symbol-Month | Worst Silo | Neg Silos | Trades | T3 Net PnL | Events | Locks | Filters | Read |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `short_all` | 0.700000% | 0.350000% | 0.090000% | 0 | 9 | 1.053380% | 24 | 24 | `{"side": "short"}` | all short-side T3 events; sanity check against native short-only lifecycle |
| `short_speed_abs_ge_0p35` | 0.560000% | 0.280000% | -0.120000% | 1 | 3 | 0.680710% | 7 | 7 | `{"side": "short", "speed_abs_min": 0.35}` | short T3 events with absolute 300s speed >= 0.35 ATR |

## Read

- This isolates filtered T3 event sources; it is not mixed with native original_t2 or native t3_swing.
- `reentry_window` is promotion-comparable to the strict lifecycle floor; `next_second_*` modes are entry-redesign diagnostics.
- Tiny-sample buckets are useful only as direction finders before broader validation.
