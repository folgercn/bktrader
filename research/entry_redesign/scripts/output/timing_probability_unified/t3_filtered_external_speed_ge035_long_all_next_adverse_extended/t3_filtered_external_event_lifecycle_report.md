# T3 Filtered External Event Lifecycle

Research-only strict lifecycle replay for filtered scored T3 event sources.

- Scored events: `/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/t3_probability_overlay_extended/t3_probability_overlay_scored_events.csv`
- Timeframe: `1h`
- Reentry fill policy: `strict_next_second_cross`
- External entry mode: `next_second_adverse`
- T3 exit overrides: `{"min_hold_seconds_before_sl": 3600.0}`
- Months: 2025-06, 2025-07, 2025-08, 2025-09, 2025-10, 2025-11, 2025-12, 2026-01, 2026-02, 2026-03, 2026-04
- Symbols: ETHUSDT, BTCUSDT

## Candidate Summary

| Candidate | Calendar Sum | Avg/Symbol-Month | Worst Silo | Neg Silos | Trades | T3 Net PnL | Events | Locks | Filters | Read |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `long_speed_abs_ge_0p35` | -0.160000% | -0.007273% | -1.470000% | 13 | 219 | 8.574220% | 67 | 67 | `{"side": "long", "speed_abs_min": 0.35}` | long T3 events with absolute 300s speed >= 0.35 ATR; direct-entry diagnostic |
| `all_speed_abs_ge_0p35` | 2.040000% | 0.092727% | -1.720000% | 13 | 294 | 13.795070% | 130 | 127 | `{"speed_abs_min": 0.35}` | both sides with absolute 300s speed >= 0.35 ATR; direct-entry diagnostic |

## Read

- This isolates filtered T3 event sources; it is not mixed with native original_t2 or native t3_swing.
- `reentry_window` is promotion-comparable to the strict lifecycle floor; `next_second_*` modes are entry-redesign diagnostics.
- Tiny-sample buckets are useful only as direction finders before broader validation.
