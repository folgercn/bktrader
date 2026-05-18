# Breakout Structure Context Sizing History Validation

Generated: 2026-05-18T09:24:47.014336+00:00

Scope: research-only. This validates context sizing on a supplied historical current-shape event source.

## Summary

| variant                                    | events | pass_events | fail_events | same_close_calendar_sum | same_close_worst_sm | same_close_neg_sm | adverse10_calendar_sum | adverse10_worst_sm | adverse10_neg_sm | adverse10_trade_count |
| ------------------------------------------ | ------ | ----------- | ----------- | ----------------------- | ------------------- | ----------------- | ---------------------- | ------------------ | ---------------- | --------------------- |
| low_eff_low_atr_ctx12h_up_fail_weight_0.00 | 5      | 5           | 12          | 0.008707                | -0.002423           | 1                 | 0.002595               | -0.003435          | 1                | 5                     |
| low_eff_low_atr_ctx12h_up_fail_weight_0.25 | 17     | 5           | 12          | 0.000380                | -0.003544           | 1                 | -0.009415              | -0.004335          | 3                | 17                    |
| low_eff_low_atr_ctx12h_up_fail_weight_0.50 | 17     | 5           | 12          | -0.007948               | -0.007088           | 2                 | -0.021426              | -0.014701          | 2                | 17                    |
| low_eff_low_atr_ctx4h_up_fail_weight_0.00  | 8      | 8           | 9           | -0.014439               | -0.012016           | 2                 | -0.025601              | -0.022166          | 2                | 8                     |
| low_eff_low_atr_ctx4h_up_fail_weight_0.25  | 17     | 8           | 9           | -0.016980               | -0.014152           | 2                 | -0.030562              | -0.025482          | 3                | 17                    |
| low_eff_low_atr_ctx12h_up_fail_weight_0.75 | 17     | 5           | 12          | -0.016276               | -0.012637           | 2                 | -0.033436              | -0.025066          | 2                | 17                    |
| low_eff_low_atr_ctx4h_up_fail_weight_0.50  | 17     | 8           | 9           | -0.019521               | -0.016287           | 2                 | -0.035524              | -0.028798          | 2                | 17                    |
| low_eff_low_atr_ctx4h_up_fail_weight_0.75  | 17     | 8           | 9           | -0.022062               | -0.018423           | 2                 | -0.040485              | -0.032115          | 2                | 17                    |
| low_eff_low_atr_q20_q40                    | 17     |             |             | -0.024603               | -0.020559           | 2                 | -0.045447              | -0.035431          | 2                | 17                    |
| low_eff_low_atr_ctx4h_up_fail_weight_1.00  | 17     | 8           | 9           | -0.024603               | -0.020559           | 2                 | -0.045447              | -0.035431          | 2                | 17                    |
| low_eff_low_atr_ctx12h_up_fail_weight_1.00 | 17     | 5           | 12          | -0.024603               | -0.020559           | 2                 | -0.045447              | -0.035431          | 2                | 17                    |
| baseline_model_advance                     | 149    |             |             | -0.202311               | -0.089283           | 3                 | -0.390524              | -0.160892          | 3                | 149                   |

## Interpretation

- Baseline rows use the supplied walk-forward gate rows and historical event source.
- Context-scaled rows keep all `low_eff_low_atr` events and scale events that fail the context gate.
- A robust candidate should remain positive outside the 2025-11..2026-04 forward window.

## Diagnostics

```json
{
  "symbol": "ETHUSDT",
  "events_csv": "research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_ethusdt_202503_202506_train1m_events.csv",
  "candidate_rows_csv": "research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_ethusdt_202503_202506_train1m_min5_candidates.csv",
  "bars_cache_dir": "research/probabilistic_v6_runs/2025_m03_m06_original_t2_delay60/bars_cache",
  "eval_start": "2025-03-01T00:00:00+00:00",
  "eval_end_exclusive": "2025-07-01T00:00:00+00:00",
  "base_gate": "low_eff_low_atr_q20_q40",
  "context_gates": [
    "low_eff_low_atr_ctx4h_up",
    "low_eff_low_atr_ctx12h_up"
  ],
  "fail_weights": [
    0.0,
    0.25,
    0.5,
    0.75,
    1.0
  ],
  "events": 207,
  "base_gate_events": 17,
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
  "runtime_seconds": 4.360809803009033
}
```
