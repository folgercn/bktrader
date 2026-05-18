# Breakout Structure Walk-Forward Validation

Generated: 2026-05-18T03:28:58.801956+00:00

Scope: research-only, ETHUSDT 1h, current production shape `restrictive_0p5bps`, model-advance events only. Thresholds are calibrated on trailing train windows.

## Selected-Gate Aggregate

| gate                 | forward_months | trade_count | same_close_calendar_sum | same_close_worst_month | same_close_neg_months | adverse10_calendar_sum | adverse10_worst_month | adverse10_neg_months |
| -------------------- | -------------- | ----------- | ----------------------- | ---------------------- | --------------------- | ---------------------- | --------------------- | -------------------- |
| walkforward_selected | 6              | 48          | 0.073731                | -0.003876              | 1                     | 0.024575               | -0.010931             | 2                    |

## Candidate Forward Aggregate

| gate                      | forward_months | trade_count | same_close_calendar_sum | same_close_worst_month | same_close_neg_months | adverse10_calendar_sum | adverse10_worst_month | adverse10_neg_months |
| ------------------------- | -------------- | ----------- | ----------------------- | ---------------------- | --------------------- | ---------------------- | --------------------- | -------------------- |
| low_eff_q20               | 6              | 77          | 0.184845                | -0.009697              | 2                     | 0.070874               | -0.022803             | 2                    |
| low_eff_low_atr_q20_q40   | 6              | 37          | 0.100886                | 0.000899               | 0                     | 0.055565               | -0.010290             | 1                    |
| level_far_sma_gap_q60_q80 | 6              | 27          | 0.069399                | 0.000089               | 0                     | 0.040594               | -0.004825             | 2                    |
| low_rf_slope_up_q40_q60   | 6              | 49          | 0.084987                | -0.003876              | 2                     | 0.036427               | -0.010931             | 3                    |
| wick_late_q40             | 6              | 42          | 0.073298                | -0.007528              | 1                     | 0.001247               | -0.018258             | 4                    |
| wick_touch_ext_le_0       | 6              | 83          | 0.059290                | -0.018170              | 2                     | -0.061051              | -0.034178             | 3                    |
| baseline_model_advance    | 6              | 298         | 0.097436                | -0.065459              | 2                     | -0.306544              | -0.126334             | 5                    |

## Split Decisions

| forward_month | selected_gate             | selected_conditions                                                  | train_calendar_sum | train_worst_sm | train_trade_count | same_close_calendar_sum | adverse10_calendar_sum | trade_count |
| ------------- | ------------------------- | -------------------------------------------------------------------- | ------------------ | -------------- | ----------------- | ----------------------- | ---------------------- | ----------- |
| 2025-11       | level_far_sma_gap_q60_q80 | level_to_signal_open_atr >= 0.438969 & prev_sma5_gap_atr >= 0.344559 | 0.034170           | -0.004367      | 27                | 0.025332                | 0.017613               | 8           |
| 2025-12       | low_rf_slope_up_q40_q60   | rf_probability <= 0.545000 & prev_sma5_slope_atr >= 0.043242         | 0.061032           | -0.024846      | 46                | -0.003876               | -0.010931              | 7           |
| 2026-01       | low_rf_slope_up_q40_q60   | rf_probability <= 0.575000 & prev_sma5_slope_atr >= 0.042017         | 0.058279           | -0.024846      | 44                | 0.005836                | -0.001707              | 8           |
| 2026-02       | low_rf_slope_up_q40_q60   | rf_probability <= 0.600000 & prev_sma5_slope_atr >= 0.042017         | 0.083738           | -0.003694      | 45                | 0.019272                | 0.006525               | 12          |
| 2026-03       | low_rf_slope_up_q40_q60   | rf_probability <= 0.604000 & prev_sma5_slope_atr >= 0.042544         | 0.078278           | -0.003694      | 44                | 0.007576                | 0.000624               | 7           |
| 2026-04       | low_eff_q20               | eff_300s <= 0.750641                                                 | 0.123120           | -0.012709      | 52                | 0.019591                | 0.012451               | 6           |

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
  "train_months": 5,
  "min_train_trades": 20,
  "forward_months": [
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
  "runtime_seconds": 25.821682929992676
}
```
