# T3 Overlay RF/Cost Sizing

Research-only audit for applying event-level RF/cost sizing to the current ETH T3 overlay.

- Scored events: `research/entry_redesign/scripts/output/timing_probability_unified/t3_probability_overlay_extended/t3_probability_overlay_scored_events.csv`
- Overlay trades input: `cached base trades in output dir`
- Months: 2025-06, 2025-07, 2025-08, 2025-09, 2025-10, 2025-11, 2025-12, 2026-01, 2026-02, 2026-03, 2026-04
- Cost threshold ATR: `0.1`
- Baseline lead adverse10: `22.971648%`

## Variant Summary

| Variant | Live-compatible | Overlay PnL | Delta vs fixed | Lead adverse10 + overlay | Worst Month | Neg Months | DD | Events | Avg Mult | Avg Qty | Max Qty | Read |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `wf_t3_rf_cost_quantity_0p20_0p40_shadow` | false | 45.639102% | 34.236470pp | 68.610750% | -3.088880% | 3 | -4.602568% | 71 | 3.758289 | 0.300663 | 0.361649 | Shadow risk-on candidate: T3-specific walk-forward RF maps event probability directly into an absolute 0.20-0.40 ETH T3 overlay quantity band. |
| `wf_t3_rf_cost_linear_floor0p75_max1p25_shadow` | false | 12.974138% | 1.571506pp | 35.945786% | -0.814090% | 3 | -1.237590% | 71 | 0.996841 | 0.079747 | 0.100000 | Shadow candidate: T3-specific walk-forward RF maps the current 0.08 ETH overlay into a 0.75x-1.25x band, or 0.06-0.10 ETH before exchange precision. |
| `wf_t3_rf_cost_linear_max1p25_research` | false | 12.849059% | 1.446427pp | 35.820707% | -0.805866% | 3 | -1.237590% | 71 | 0.983027 | 0.078642 | 0.100000 | Research-only aggressive check that can exceed current overlay 2.0x by 25%. |
| `frozen_lead_rf_cost_floor0p75_max1p25_shadow` | false | 11.975949% | 0.573317pp | 34.947597% | -0.916271% | 3 | -1.233874% | 71 | 1.076620 | 0.086130 | 0.100000 | Shadow candidate using the already-loaded frozen lead RF model on T3 event features; maps the current 0.08 ETH overlay into a 0.75x-1.25x band. |
| `fixed_overlay_2p0` | true | 11.402632% | 0.000000pp | 34.374280% | -0.832619% | 3 | -1.286147% | 71 | 1.000000 | 0.080000 | 0.080000 | Current fixed T3 overlay 2.0x sizing. |
| `wf_t3_rf_cost_linear_max1` | true | 11.120513% | -0.282119pp | 34.092161% | -0.766809% | 3 | -1.155012% | 71 | 0.914531 | 0.073162 | 0.080000 | T3-specific walk-forward RF probability mapped linearly and capped at current overlay 2.0x. |
| `frozen_lead_rf_cost_max1` | true | 10.649990% | -0.752642pp | 33.621638% | -0.792913% | 3 | -1.120942% | 71 | 0.939014 | 0.075121 | 0.080000 | Frozen lead RF/cost score, capped so it can only reduce current 2.0x overlay size. |
| `frozen_lead_rf_cost_floor0p25_max1` | true | 10.649990% | -0.752642pp | 33.621638% | -0.792913% | 3 | -1.120942% | 71 | 0.939014 | 0.075121 | 0.080000 | Frozen lead RF/cost score with a 0.25 floor to avoid binary-style overfiltering. |
| `wf_t3_rf_cost_rank_max1` | true | 9.329228% | -2.073404pp | 32.300876% | -0.434839% | 2 | -0.906976% | 52 | 0.584507 | 0.046761 | 0.080000 | T3-specific walk-forward RF rank buckets: bottom 40% off, middle half, top full size. |

## Verdict

- No live-compatible RF/cost variant beat fixed overlay 2.0x; best was `wf_t3_rf_cost_linear_max1` at 11.120513% (-0.282119pp vs fixed).
- Best research-only variant is `wf_t3_rf_cost_quantity_0p20_0p40_shadow` at 45.639102% (34.236470pp vs fixed), but it can exceed the current 2.0x overlay cap and therefore needs a separate shadow risk decision.

## Read

- `fixed_overlay_2p0` is the current T3 overlay sizing reference.
- `live-compatible=true` rows never exceed the current fixed 2.0x overlay notional; they can only downweight events.
- `shadow` rows intentionally exceed the current fixed overlay notional; the promoted risk-on row maps RF/cost quality into a 0.20-0.40 ETH testnet-shadow quantity band.
- The T3-WF-RF quantity-band row is walk-forward evidence. Live/testnet wiring uses a separate accumulated-history T3 model artifact and must be monitored as shadow telemetry before mainnet consideration.
- A promoted variant should beat fixed overlay PnL without making worst-month or drawdown materially worse.
