# Canonical Lead + Breakout Structure Expansion Combo

Generated: 2026-05-18T09:04:47.893432+00:00

Scope: research-only. Canonical lead trades are preserved; expansion legs only add non-overlapping current-shape live-like events.

## Summary

| variant                       | candidate_source_events | overlap_removed_events | extra_events | extra_same_close_calendar_sum | combo_same_close_calendar_sum | combo_same_close_delta_vs_lead | extra_adverse10_calendar_sum | combo_adverse10_calendar_sum_exact | combo_adverse10_delta_vs_lead_exact | combo_adverse10_worst_sm_exact | combo_adverse10_neg_sm_exact | combo_same_close_worst_sm | combo_same_close_neg_sm |
| ----------------------------- | ----------------------- | ---------------------- | ------------ | ----------------------------- | ----------------------------- | ------------------------------ | ---------------------------- | ---------------------------------- | ----------------------------------- | ------------------------------ | ---------------------------- | ------------------------- | ----------------------- |
| wf5_low_eff_q20               | 77                      | 6                      | 71           | 0.160778                      | 0.462950                      | 0.160778                       | 0.055303                     | 0.285020                           | 0.055303                            | -0.019114                      | 2                            | -0.005611                 | 2                       |
| wf3_low_eff_low_atr           | 42                      | 1                      | 41           | 0.100281                      | 0.402453                      | 0.100281                       | 0.052882                     | 0.282598                           | 0.052882                            | -0.010290                      | 2                            | -0.003351                 | 1                       |
| wf3_low_eff_low_atr_ctx4h_up  | 16                      | 1                      | 15           | 0.062803                      | 0.364975                      | 0.062803                       | 0.046332                     | 0.276048                           | 0.046332                            | -0.001849                      | 1                            | 0.002202                  | 0                       |
| wf5_low_eff_low_atr           | 37                      | 2                      | 35           | 0.084921                      | 0.387093                      | 0.084921                       | 0.042604                     | 0.272321                           | 0.042604                            | -0.010290                      | 2                            | -0.003351                 | 1                       |
| wf3_low_eff_q20               | 90                      | 4                      | 86           | 0.163790                      | 0.465962                      | 0.163790                       | 0.042100                     | 0.271817                           | 0.042100                            | -0.019774                      | 3                            | -0.005446                 | 2                       |
| wf4_selected                  | 75                      | 1                      | 74           | 0.139666                      | 0.441838                      | 0.139666                       | 0.040381                     | 0.270097                           | 0.040381                            | -0.024041                      | 4                            | -0.003876                 | 3                       |
| wf3_low_eff_low_atr_ctx12h_up | 15                      | 1                      | 14           | 0.056999                      | 0.359171                      | 0.056999                       | 0.039195                     | 0.268912                           | 0.039195                            | -0.001586                      | 1                            | 0.000316                  | 0                       |
| static_low_eff_low_atr_pct    | 49                      | 1                      | 48           | 0.087734                      | 0.389906                      | 0.087734                       | 0.033417                     | 0.263133                           | 0.033417                            | -0.010290                      | 1                            | 0.000899                  | 0                       |
| wf3_low_rf_slope_up           | 65                      | 2                      | 63           | 0.093059                      | 0.395231                      | 0.093059                       | 0.033407                     | 0.263123                           | 0.033407                            | -0.013447                      | 5                            | -0.003694                 | 4                       |
| wf4_low_eff_q20               | 80                      | 4                      | 76           | 0.142510                      | 0.444683                      | 0.142510                       | 0.032236                     | 0.261952                           | 0.032236                            | -0.019114                      | 3                            | -0.005611                 | 2                       |
| static_level_far_sma_gap_up   | 54                      | 2                      | 52           | 0.081625                      | 0.383797                      | 0.081625                       | 0.028532                     | 0.258249                           | 0.028532                            | -0.004825                      | 3                            | 0.000089                  | 0                       |
| wf4_low_eff_low_atr           | 38                      | 1                      | 37           | 0.072118                      | 0.374290                      | 0.072118                       | 0.028297                     | 0.258013                           | 0.028297                            | -0.010290                      | 2                            | -0.003351                 | 1                       |
| wf3_selected                  | 85                      | 2                      | 83           | 0.135704                      | 0.437876                      | 0.135704                       | 0.026248                     | 0.255965                           | 0.026248                            | -0.024041                      | 4                            | -0.003694                 | 3                       |
| static_low_rf_slope_up        | 96                      | 5                      | 91           | 0.089162                      | 0.391335                      | 0.089162                       | 0.005969                     | 0.235686                           | 0.005969                            | -0.013447                      | 4                            | -0.003694                 | 3                       |
| wf5_selected                  | 48                      | 2                      | 46           | 0.043367                      | 0.345539                      | 0.043367                       | -0.002775                    | 0.226942                           | -0.002775                           | -0.013447                      | 4                            | -0.003876                 | 3                       |
| static_low_eff_le_q20         | 113                     | 3                      | 110          | 0.097693                      | 0.399865                      | 0.097693                       | -0.049368                    | 0.180349                           | -0.049368                           | -0.019114                      | 3                            | -0.005611                 | 2                       |

## Notes

- Overlap is removed by canonical `(signal_start, side)`, using the full canonical event source, not only traded rows.
- Same-close combo metrics are exact trade-ledger combinations.
- Adverse combo metrics are now rebuilt from one combined per-trade ledger: canonical lead adverse trades plus expansion adverse trades.
- Canonical lead metrics are replayed with the production template exit contract (`trail_start_r=1.5`, `max_hold_hours=2.0`); deltas versus current CSV artifacts are recorded in diagnostics.
- `static_*` candidates are in-sample gates; `wf*` candidates are train-calibrated walk-forward gates.

## Diagnostics

```json
{
  "canonical_events_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/tick_flow_event_sources/20260514_pretouch_full_window/feature_filtered_seed_events/robust_quality/pretouch_small_pullback_rf_q50_speed300_ge_q10_touch30m_eff300le1.csv",
  "canonical_overlap_keys": 154,
  "lead_trades_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/unified_trades.csv",
  "lead_gate_on_trades": 62,
  "lead_same_close_csv_artifact": {
    "calendar_sum": 0.3053329148127901,
    "worst_sm": 0.0235831891534755,
    "neg_sm": 0,
    "trade_count": 62
  },
  "lead_same_close": {
    "calendar_sum": 0.30217222209740113,
    "worst_sm": 0.02358318915347573,
    "neg_sm": 0,
    "trade_count": 62
  },
  "lead_adverse10_csv_artifact": {
    "calendar_sum": 0.2328809117887866,
    "worst_sm": 0.0139582068530584,
    "neg_sm": 0,
    "trade_count": 62
  },
  "lead_adverse10_exact": {
    "calendar_sum": 0.2297164769930056,
    "worst_sm": 0.013958206853058487,
    "neg_sm": 0,
    "trade_count": 62
  },
  "lead_same_close_exact_delta_vs_csv_artifact": -0.0031606927153889908,
  "lead_adverse10_exact_delta_vs_csv_artifact": -0.0031644347957809904,
  "lead_replayed_events": 68,
  "lead_delay_errors": 0,
  "lead_exec_params": {
    "initial_stop_atr": 0.45,
    "stop_buffer_atr": 0.05,
    "stop_cap_atr": 0.8,
    "min_stop_bps": 12.0,
    "breakeven_at_r": 0.8,
    "cost_lock_bps": 10.0,
    "trail_start_r": 1.5,
    "trail_buffer_atr": 0.05,
    "max_hold_hours": 2.0,
    "slippage": 0.0002,
    "entry_fee": 0.0002,
    "exit_fee": 0.0004
  },
  "base_events_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_shape_expansion_events_restrictive_0p5bps.csv",
  "base_model_advance_events": 561,
  "exec_params": {
    "initial_stop_atr": 0.45,
    "stop_buffer_atr": 0.05,
    "stop_cap_atr": 0.8,
    "min_stop_bps": 12.0,
    "breakeven_at_r": 0.8,
    "cost_lock_bps": 10.0,
    "trail_start_r": 1.5,
    "trail_buffer_atr": 0.05,
    "max_hold_hours": 2.0,
    "slippage": 0.0002,
    "entry_fee": 0.0002,
    "exit_fee": 0.0004
  },
  "runtime_seconds": 34.002750873565674
}
```
