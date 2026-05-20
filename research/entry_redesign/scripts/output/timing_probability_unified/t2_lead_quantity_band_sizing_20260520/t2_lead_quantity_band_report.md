# T2 Lead Quantity-Band Sizing

Research-only audit for applying the testnet-shadow `0.20..0.40 ETH` lead quantity band to the current ETH pretouch lead.

## Inputs

- Lead adverse10 trades: `research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_lead_combo_lead_adverse10_trades.csv`
- Lead same-close trades: `research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_lead_combo_lead_same_close_trades.csv`
- T3 RF/cost summary: `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_rf_cost_sizing_20260520/t3_overlay_rf_cost_sizing_summary.json`
- T3 weighted trades: `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_rf_cost_sizing_20260520/t3_overlay_rf_cost_weighted_trades.csv`

## Formula

- `base_order_quantity=0.1`
- `base_share=0.8` and `max_production_multiplier=2.0`
- `production_quantity = base_order_quantity * position_size`
- `quality_score = clip(production_quantity / (base_order_quantity * base_share * max_production_multiplier), 0, 1)`
- `submitted_quantity = 0.2 + quality_score * (0.4 - 0.2)`, capped by `max_submitted_quantity=0.4`
- PnL is linearly rescaled from the existing adverse10 ledger by `submitted_quantity / production_quantity`.

## Summary

| Variant | Calendar Sum | Delta vs Base | Delta vs Legacy 1.5x | Worst Month | Neg Months | DD | Trades | Avg Qty | Max Qty | Avg Mult |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `base_lead_adverse10` | 22.971648% | 0.000000pp | - | 1.395821% | 0 | - | 62 | - | - | - |
| `legacy_lead_1p5_adverse10` | 34.457472% | 11.485824pp | 0.000000pp | - | - | - | 62 | - | - | 1.500000 |
| `lead_quantity_0p20_0p40_adverse10` | 61.070916% | 38.099268pp | 26.613444pp | 3.468905% | 0 | -2.641863% | 62 | 0.340113 | 0.400000 | 3.256265 |
| `lead_q020_q040_plus_t3_q020_q040` | 106.710018% | 38.099268pp vs base lead + T3 | - | -1.464655% | 1 | -4.682954% | - | - | - | - |

## Monthly

| Month | Base Lead | Lead Qty Band | T3 Qty Band | Bundle |
|---|---:|---:|---:|---:|
| 2025-06 | 4.493650% | 13.282835% | -1.630646% | 11.652189% |
| 2025-07 | 4.640439% | 11.363170% | 3.780934% | 15.144104% |
| 2025-08 | 7.341680% | 19.330424% | 3.656698% | 22.987122% |
| 2025-09 | 5.100058% | 13.625582% | -3.088880% | 10.536702% |
| 2025-10 | 1.395821% | 3.468905% | 3.372130% | 6.841035% |
| 2025-11 | 0.000000% | 0.000000% | 2.134829% | 2.134829% |
| 2025-12 | 0.000000% | 0.000000% | 5.941631% | 5.941631% |
| 2026-01 | 0.000000% | 0.000000% | -1.464655% | -1.464655% |
| 2026-02 | 0.000000% | 0.000000% | 24.359527% | 24.359527% |
| 2026-03 | 0.000000% | 0.000000% | 4.207432% | 4.207432% |
| 2026-04 | 0.000000% | 0.000000% | 4.370102% | 4.370102% |

## Read

- The T2 quantity-band result is a formal linear-notional backtest view of the testnet-shadow sizing contract, not a mainnet promotion result.
- The base event set, timing decisions, exit contract, and adverse10 fill ledger are unchanged.
- `legacy_lead_1p5_adverse10` is retained only as a continuity reference for the previous shadow sizing.
- The bundle row adds T2 quantity-band lead PnL and T3 RF/cost quantity-band overlay PnL month-by-month. It does not yet model additional slippage or exchange depth degradation from larger submitted quantity.
