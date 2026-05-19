# Breakout Structure Context Sizing Sensitivity

Generated: 2026-05-18T09:12:23.540898+00:00

Scope: research-only. This tests context-quality position scaling for `wf3_low_eff_low_atr`; no live defaults are changed.

## Summary

| context_candidate             | fail_weight | extra_events | pass_extra_events | fail_extra_events | combo_adverse10_calendar_sum | combo_adverse10_delta_vs_lead | combo_adverse10_worst_sm | combo_adverse10_neg_sm | combo_adverse10_trade_count |
| ----------------------------- | ----------- | ------------ | ----------------- | ----------------- | ---------------------------- | ----------------------------- | ------------------------ | ---------------------- | --------------------------- |
| wf3_low_eff_low_atr_ctx4h_up  | 0.250000    | 41           | 15                | 26                | 0.277686                     | 0.047969                      | -0.001379                | 1                      | 103                         |
| wf3_low_eff_low_atr_ctx12h_up | 0.000000    | 14           | 14                | 27                | 0.268912                     | 0.039195                      | -0.001586                | 1                      | 76                          |
| wf3_low_eff_low_atr_ctx12h_up | 0.250000    | 41           | 14                | 27                | 0.272333                     | 0.042617                      | -0.001800                | 2                      | 103                         |
| wf3_low_eff_low_atr_ctx4h_up  | 0.000000    | 15           | 15                | 26                | 0.276048                     | 0.046332                      | -0.001849                | 1                      | 77                          |
| wf3_low_eff_low_atr_ctx4h_up  | 0.500000    | 41           | 15                | 26                | 0.279323                     | 0.049607                      | -0.004349                | 2                      | 103                         |
| wf3_low_eff_low_atr_ctx12h_up | 0.500000    | 41           | 14                | 27                | 0.275755                     | 0.046039                      | -0.004349                | 2                      | 103                         |
| wf3_low_eff_low_atr_ctx4h_up  | 0.750000    | 41           | 15                | 26                | 0.280961                     | 0.051244                      | -0.007319                | 2                      | 103                         |
| wf3_low_eff_low_atr_ctx12h_up | 0.750000    | 41           | 14                | 27                | 0.279177                     | 0.049460                      | -0.007319                | 2                      | 103                         |
| wf3_low_eff_low_atr_ctx4h_up  | 1.000000    | 41           | 15                | 26                | 0.282598                     | 0.052882                      | -0.010290                | 2                      | 103                         |
| wf3_low_eff_low_atr_ctx12h_up | 1.000000    | 41           | 14                | 27                | 0.282598                     | 0.052882                      | -0.010290                | 2                      | 103                         |

## Interpretation

- `fail_weight=0` is the hard context overlay.
- `fail_weight=1` is the bare `wf3_low_eff_low_atr` expansion.
- Useful sizing control should keep most of the bare `wf3` return while improving worst month and negative-month count.

## Diagnostics

```json
{
  "base_candidate": "wf3_low_eff_low_atr",
  "context_candidates": [
    "wf3_low_eff_low_atr_ctx4h_up",
    "wf3_low_eff_low_atr_ctx12h_up"
  ],
  "fail_weights": [
    0.0,
    0.25,
    0.5,
    0.75,
    1.0
  ],
  "base_events_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_shape_expansion_events_restrictive_0p5bps.csv",
  "wf3_source_events": 42,
  "wf3_extra_events": 41,
  "lead_same_close": {
    "calendar_sum": 0.30217222209740113,
    "worst_sm": 0.02358318915347573,
    "neg_sm": 0,
    "trade_count": 62
  },
  "lead_adverse10": {
    "calendar_sum": 0.2297164769930056,
    "worst_sm": 0.013958206853058487,
    "neg_sm": 0,
    "trade_count": 62
  },
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
  "runtime_seconds": 28.377127170562744,
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
  }
}
```
