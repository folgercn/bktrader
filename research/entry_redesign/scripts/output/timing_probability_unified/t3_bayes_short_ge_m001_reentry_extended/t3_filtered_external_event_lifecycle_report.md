# T3 Filtered External Event Lifecycle

Research-only strict lifecycle replay for filtered scored T3 event sources.

- Scored events: `research/entry_redesign/scripts/output/timing_probability_unified/t3_bayesian_event_filter_extended/selected_events_bayes_ge_m0p010.csv`
- Timeframe: `1h`
- Reentry fill policy: `strict_next_second_cross`
- External entry mode: `reentry_window`
- T3 exit overrides: `{"min_hold_seconds_before_sl": 3600.0}`
- Months: 2025-06, 2025-07, 2025-08, 2025-09, 2025-10, 2025-11, 2025-12, 2026-01, 2026-02, 2026-03, 2026-04
- Symbols: ETHUSDT, BTCUSDT

## Candidate Summary

| Candidate | Calendar Sum | Avg/Symbol-Month | Worst Silo | Neg Silos | Trades | T3 Net PnL | Events | Locks | Filters | Read |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `short_all` | 0.660000% | 0.030000% | -0.060000% | 4 | 20 | 1.458350% | 123 | 121 | `{"side": "short"}` | all short-side T3 events; sanity check against native short-only lifecycle |

## Read

- This isolates filtered T3 event sources; it is not mixed with native original_t2 or native t3_swing.
- `reentry_window` is promotion-comparable to the strict lifecycle floor; `next_second_*` modes are entry-redesign diagnostics.
- Tiny-sample buckets are useful only as direction finders before broader validation.
