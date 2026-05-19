# T2 Lifecycle External Event Gate

Research-only strict lifecycle replay for probability/RF-selected external T2 events.

- Timeframe: `1h`
- Reentry fill policy: `strict_next_second_cross`
- External entry mode: `next_second_open`
- Months: 2025-06, 2025-07, 2025-08, 2025-09, 2025-10, 2025-11, 2025-12, 2026-01, 2026-02, 2026-03, 2026-04
- Symbols: ETHUSDT

## Candidate Summary

| Candidate | Calendar Sum | Avg/Symbol-Month | Worst Silo | Neg Silos | Trades | T3 Trades | External Trades | T3 Net PnL | External Net PnL | External Events | External Locks | Read |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `low_eff_rf_rank_median_external_touch_close_ext_ge_0p03atr_next_second_open_t3_60m` | 0.700000% | 0.063636% | -0.400000% | 5 | 64 | 53 | 11 | 3.350280% | -0.097630% | 10 | 10 | Inject low_eff_rf_rank_median_000 active events as explicit breakout locks; native original_t2 remains disabled; event source=touch_close_ext_ge_0p03atr; external entry mode=next_second_open. |

## Read

- This is the exact lifecycle bridge for the existing RF-selected active event file, not a new model fit.
- Native original_t2 is disabled, so any delta versus the T2-disabled floor comes from the external probability events.
- `reentry_window` preserves the original strict next-second cross lifecycle; `next_second_*` modes test post-touch direct entry without same-second fills.
- A good result must beat `t2_disabled_t3_60m` before it is worth promoting into broader risk audit.
