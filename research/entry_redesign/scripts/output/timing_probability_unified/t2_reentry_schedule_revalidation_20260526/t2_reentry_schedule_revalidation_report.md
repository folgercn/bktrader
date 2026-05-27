# T2 Reentry Schedule Revalidation

Research-only audit. Canonical ETH pretouch lead events are replayed with lead-side
`reentry_window` lifecycle semantics and `reentry_size_schedule=[0.20, 0.10]`.

- Input lead events: `/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_lead_exact_exposure/lead_exact_adverse10_exposure_windows.csv`
- External shape: `canonical_lead_t2_reentry`
- Native original_t2 and native t3_swing locks are disabled to isolate canonical lead events.
- This is a lifecycle reentry schedule check, not a live runtime change.

## Summary

| Variant | Calendar Sum | Delta vs current qband lead | Delta vs base adverse10 | Worst month | Negative months | Events | Locks | Trades |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `canonical_lead_t2_reentry_schedule` | -0.122020% | -61.192937pp | -23.093668pp | -0.052380% | 3 | 62 | 62 | 10 |

Reference:

- Current `lead_quantity_0p20_0p40_adverse10`: `61.070917%`.
- Base `base_lead_adverse10_exact`: `22.971648%`.

## Month Detail

| month   | external_net_pnl_pct | events | locks | trades | max_dd_pct | entry_reasons                                |
| ------- | -------------------- | ------ | ----- | ------ | ---------- | -------------------------------------------- |
| 2025-06 | -0.021590            | 13     | 13    | 2      | -0.060000  | {"SL-Reentry": 1, "Zero-Initial-Reentry": 1} |
| 2025-07 | 0.000000             | 13     | 13    | 0      | 0.000000   | {}                                           |
| 2025-08 | 0.000000             | 13     | 13    | 0      | 0.000000   | {}                                           |
| 2025-09 | -0.048050            | 14     | 14    | 4      | -0.150000  | {"SL-Reentry": 2, "Zero-Initial-Reentry": 2} |
| 2025-10 | -0.052380            | 9      | 9     | 4      | -0.150000  | {"SL-Reentry": 2, "Zero-Initial-Reentry": 2} |

## Read

- If this lifecycle schedule is below the current qband lead, do not restore lead-side reentry as the research lead.
- The current qband lead remains a selected-delay adverse10 ledger with absolute `0.20..0.40 ETH` sizing.
- T3 overlay lifecycle reentries should remain a separate overlay research surface.
