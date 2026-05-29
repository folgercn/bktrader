# Relaxed T3 deterministic stop-gate replay

- Relaxed scored events: `research/entry_redesign/scripts/output/timing_probability_unified/t3_probability_overlay_relaxed_prev3_dominates_20260529/t3_probability_overlay_scored_events.csv`
- RF/cost event scores: `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_rf_cost_sizing_relaxed_prev3_dominates_20260529/t3_overlay_rf_cost_event_scores.csv`
- Base trades: `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_rf_cost_sizing_relaxed_prev3_dominates_20260529/t3_overlay_rf_cost_base_trades.csv`
- Event source: `all_speed_abs_ge_0p35`
- Quantity variant: `wf_t3_rf_cost_quantity_0p20_0p40_shadow`
- Deterministic selected events: `24` / `146` active qband events
- Selected action: `{"min_hold_seconds_before_trailing_sl": 4740.0, "stop_loss_atr": 3.0}`

| Metric | Value |
|---|---:|
| Overlay PnL | `194.323156%` |
| Delta vs relaxed q020-q040 baseline | `102.962541pp` |
| Lead q020-q040 + overlay | `255.394073%` |
| Worst month | `-3.357137%` |
| Negative months | `1` |
| Max drawdown | `-7.827585%` |
| Filled trades | `415` |
| Active events | `146` |

## Monthly PnL

| Month | PnL |
|---|---:|
| `2025-06` | `34.966796%` |
| `2025-07` | `7.467700%` |
| `2025-08` | `4.877858%` |
| `2025-09` | `-3.357137%` |
| `2025-10` | `18.736567%` |
| `2025-11` | `6.864241%` |
| `2025-12` | `13.890354%` |
| `2026-01` | `4.987556%` |
| `2026-02` | `35.313462%` |
| `2026-03` | `61.523019%` |
| `2026-04` | `9.052740%` |
