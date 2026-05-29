# T3 Overlay RF/Cost Sizing

Research-only audit for applying event-level RF/cost sizing to the current ETH T3 overlay.

- Scored events: `research/entry_redesign/scripts/output/timing_probability_unified/t3_probability_overlay_relaxed_prev3_dominates_20260529/t3_probability_overlay_scored_events.csv`
- Overlay trades input: `replayed in this run`
- Months: 2025-06, 2025-07, 2025-08, 2025-09, 2025-10, 2025-11, 2025-12, 2026-01, 2026-02, 2026-03, 2026-04
- Cost threshold ATR: `0.1`
- Baseline lead adverse10: `61.070917%`

## Variant Summary

| Variant | Live-compatible | Overlay PnL | Delta vs fixed | Lead adverse10 + overlay | Worst Month | Neg Months | DD | Events | Avg Mult | Avg Qty | Max Qty | Read |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `wf_t3_rf_cost_quantity_0p20_0p40_shadow` | false | 91.360615% | 67.962620pp | 152.431532% | -4.498330% | 2 | -6.528768% | 146 | 3.834933 | 0.306795 | 0.355749 | Shadow risk-on candidate: T3-specific walk-forward RF maps event probability directly into an absolute 0.20-0.40 ETH T3 overlay quantity band. |
| `wf_t3_rf_cost_linear_floor0p75_max1p25_shadow` | false | 25.078474% | 1.680479pp | 86.149391% | -1.265166% | 2 | -1.855868% | 146 | 1.042511 | 0.083401 | 0.100000 | Shadow candidate: T3-specific walk-forward RF maps the current 0.08 ETH overlay into a 0.75x-1.25x band, or 0.06-0.10 ETH before exchange precision. |
| `wf_t3_rf_cost_linear_max1p25_research` | false | 24.835573% | 1.437578pp | 85.906490% | -1.263639% | 2 | -1.854340% | 146 | 1.035918 | 0.082873 | 0.100000 | Research-only aggressive check that can exceed current overlay 2.0x by 25%. |
| `frozen_lead_rf_cost_floor0p75_max1p25_shadow` | false | 24.527519% | 1.129524pp | 85.598436% | -1.219816% | 2 | -1.888060% | 146 | 1.000000 | 0.080000 | 0.100000 | Shadow candidate using the already-loaded frozen lead RF model on T3 event features; maps the current 0.08 ETH overlay into a 0.75x-1.25x band. |
| `fixed_overlay_2p0` | true | 23.397995% | 0.000000pp | 84.468912% | -1.116005% | 2 | -1.635043% | 146 | 1.000000 | 0.080000 | 0.080000 | Current fixed T3 overlay 2.0x sizing. |
| `wf_t3_rf_cost_linear_max1` | true | 22.100710% | -1.297285pp | 83.171627% | -1.082972% | 2 | -1.602010% | 146 | 0.933798 | 0.074704 | 0.080000 | T3-specific walk-forward RF probability mapped linearly and capped at current overlay 2.0x. |
| `wf_t3_rf_cost_rank_max1` | true | 16.405516% | -6.992479pp | 77.476433% | -0.648516% | 3 | -1.065961% | 91 | 0.520548 | 0.041644 | 0.080000 | T3-specific walk-forward RF rank buckets: bottom 40% off, middle half, top full size. |
| `frozen_lead_rf_cost_floor0p25_max1` | true | 16.318031% | -7.079964pp | 77.388948% | -0.853220% | 3 | -1.401427% | 146 | 0.625000 | 0.050000 | 0.080000 | Frozen lead RF/cost score with a 0.25 floor to avoid binary-style overfiltering. |
| `frozen_lead_rf_cost_max1` | true | 14.673817% | -8.724178pp | 75.744734% | -0.901754% | 3 | -1.351744% | 146 | 0.536575 | 0.042926 | 0.080000 | Frozen lead RF/cost score, capped so it can only reduce current 2.0x overlay size. |

## Verdict

- No live-compatible RF/cost variant beat fixed overlay 2.0x; best was `wf_t3_rf_cost_linear_max1` at 22.100710% (-1.297285pp vs fixed).
- Best research-only variant is `wf_t3_rf_cost_quantity_0p20_0p40_shadow` at 91.360615% (67.962620pp vs fixed), but it can exceed the current 2.0x overlay cap and therefore needs a separate shadow risk decision.

## Read

- `fixed_overlay_2p0` is the current T3 overlay sizing reference.
- `live-compatible=true` rows never exceed the current fixed 2.0x overlay notional; they can only downweight events.
- `shadow` rows intentionally exceed the current fixed overlay notional; the promoted risk-on row maps RF/cost quality into a 0.20-0.40 ETH testnet-shadow quantity band.
- The T3-WF-RF quantity-band row is walk-forward evidence. Live/testnet wiring uses a separate accumulated-history T3 model artifact and must be monitored as shadow telemetry before mainnet consideration.
- A promoted variant should beat fixed overlay PnL without making worst-month or drawdown materially worse.
