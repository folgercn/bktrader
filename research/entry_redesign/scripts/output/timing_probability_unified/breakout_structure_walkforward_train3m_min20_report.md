# Breakout Structure Walk-Forward Validation

Generated: 2026-05-18T07:05:13.415667+00:00

Scope: research-only, ETHUSDT 1h, current production shape `restrictive_0p5bps`, model-advance events only. Thresholds are calibrated on trailing train windows.

## Selected-Gate Aggregate

| gate                 | forward_months | trade_count | same_close_calendar_sum | same_close_worst_month | same_close_neg_months | adverse10_calendar_sum | adverse10_worst_month | adverse10_neg_months |
| -------------------- | -------------- | ----------- | ----------------------- | ---------------------- | --------------------- | ---------------------- | --------------------- | -------------------- |
| walkforward_selected | 8              | 85          | 0.171124                | -0.026318              | 2                     | 0.059066               | -0.037999             | 4                    |

## Candidate Forward Aggregate

| gate                      | forward_months | trade_count | same_close_calendar_sum | same_close_worst_month | same_close_neg_months | adverse10_calendar_sum | adverse10_worst_month | adverse10_neg_months |
| ------------------------- | -------------- | ----------- | ----------------------- | ---------------------- | --------------------- | ---------------------- | --------------------- | -------------------- |
| low_eff_low_atr_q20_q40   | 8              | 42          | 0.122025                | -0.010761              | 1                     | 0.072854               | -0.012840             | 2                    |
| low_rf_slope_up_q40_q60   | 8              | 65          | 0.123423                | -0.003694              | 2                     | 0.060757               | -0.011795             | 3                    |
| low_eff_q20               | 8              | 90          | 0.166553                | -0.012039              | 4                     | 0.039887               | -0.022803             | 4                    |
| level_far_sma_gap_q60_q80 | 8              | 36          | 0.067909                | -0.007146              | 3                     | 0.028111               | -0.012187             | 4                    |
| wick_late_q40             | 8              | 55          | 0.058162                | -0.020799              | 2                     | -0.027778              | -0.028264             | 5                    |
| wick_touch_ext_le_0       | 8              | 102         | 0.042505                | -0.026318              | 3                     | -0.099473              | -0.037999             | 5                    |
| baseline_model_advance    | 8              | 395         | -0.013631               | -0.080559              | 4                     | -0.530945              | -0.137240             | 7                    |

## Split Decisions

| forward_month | selected_gate           | selected_conditions                                              | train_calendar_sum | train_worst_sm | train_trade_count | same_close_calendar_sum | adverse10_calendar_sum | trade_count |
| ------------- | ----------------------- | ---------------------------------------------------------------- | ------------------ | -------------- | ----------------- | ----------------------- | ---------------------- | ----------- |
| 2025-09       | wick_touch_ext_le_0     | touch_extension_atr <= 0                                         | 0.019099           | -0.008066      | 34                | 0.009532                | -0.000423              | 9           |
| 2025-10       | wick_touch_ext_le_0     | touch_extension_atr <= 0                                         | 0.015302           | -0.008066      | 32                | -0.026318               | -0.037999              | 10          |
| 2025-11       | low_rf_slope_up_q40_q60 | rf_probability <= 0.545 & prev_sma5_slope_atr >= 0.0419732163363 | 0.009429           | -0.024846      | 25                | 0.056852                | 0.047792               | 10          |
| 2025-12       | low_rf_slope_up_q40_q60 | rf_probability <= 0.595 & prev_sma5_slope_atr >= 0.042061565159  | 0.084840           | 0.007050       | 27                | -0.003694               | -0.011795              | 8           |
| 2026-01       | low_rf_slope_up_q40_q60 | rf_probability <= 0.607 & prev_sma5_slope_atr >= 0.0359739705949 | 0.049281           | -0.003694      | 25                | 0.006025                | -0.002499              | 9           |
| 2026-02       | low_rf_slope_up_q40_q60 | rf_probability <= 0.62 & prev_sma5_slope_atr >= 0.0479099655659  | 0.052155           | -0.003694      | 28                | 0.019272                | 0.006525               | 12          |
| 2026-03       | low_eff_q20             | eff_300s <= 0.771440028148                                       | 0.037502           | -0.005611      | 31                | 0.089865                | 0.045014               | 21          |
| 2026-04       | low_eff_q20             | eff_300s <= 0.741917639949                                       | 0.085009           | -0.008907      | 31                | 0.019591                | 0.012451               | 6           |

## Interpretation

- `Candidate Forward Aggregate` applies each gate family with train-calibrated thresholds to every forward month.
- `Selected-Gate Aggregate` simulates a realistic selector: choose the best positive train gate, then trade only the next month.
- This is stricter than the prior in-sample quality-gate sweep; a gate that survives here is worth model/retrain work, not immediate live promotion.

## Diagnostics

```json
{
  "base_events_path": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_shape_expansion_events_restrictive_0p5bps.csv",
  "base_trades_path": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_quality_trades_baseline_touch_entry.csv",
  "events": 561,
  "train_months": 3,
  "min_train_trades": 20,
  "forward_months": [
    "2025-09",
    "2025-10",
    "2025-11",
    "2025-12",
    "2026-01",
    "2026-02",
    "2026-03",
    "2026-04"
  ],
  "base_share": 0.8,
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
  "runtime_seconds": 24.736456871032715
}
```
