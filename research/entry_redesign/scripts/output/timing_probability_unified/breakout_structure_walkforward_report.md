# Breakout Structure Walk-Forward Validation

Generated: 2026-05-18T03:27:10.633463+00:00

Scope: research-only, ETHUSDT 1h, current production shape `restrictive_0p5bps`, model-advance events only. Thresholds are calibrated on trailing train windows.

## Selected-Gate Aggregate

| gate                 | forward_months | trade_count | same_close_calendar_sum | same_close_worst_month | same_close_neg_months | adverse10_calendar_sum | adverse10_worst_month | adverse10_neg_months |
| -------------------- | -------------- | ----------- | ----------------------- | ---------------------- | --------------------- | ---------------------- | --------------------- | -------------------- |
| walkforward_selected | 7              | 75          | 0.161410                | -0.026318              | 2                     | 0.060353               | -0.037999             | 3                    |

## Candidate Forward Aggregate

| gate                      | forward_months | trade_count | same_close_calendar_sum | same_close_worst_month | same_close_neg_months | adverse10_calendar_sum | adverse10_worst_month | adverse10_neg_months |
| ------------------------- | -------------- | ----------- | ----------------------- | ---------------------- | --------------------- | ---------------------- | --------------------- | -------------------- |
| low_eff_low_atr_q20_q40   | 7              | 38          | 0.093862                | -0.010761              | 1                     | 0.048269               | -0.012840             | 2                    |
| level_far_sma_gap_q60_q80 | 7              | 30          | 0.074537                | 0.000089               | 0                     | 0.042541               | -0.004825             | 2                    |
| low_rf_slope_up_q40_q60   | 7              | 56          | 0.092361                | -0.003876              | 2                     | 0.037976               | -0.010931             | 3                    |
| low_eff_q20               | 7              | 80          | 0.145274                | -0.017985              | 3                     | 0.030022               | -0.024200             | 3                    |
| wick_late_q40             | 7              | 49          | 0.052499                | -0.020799              | 2                     | -0.027017              | -0.028264             | 5                    |
| wick_touch_ext_le_0       | 7              | 93          | 0.032973                | -0.026318              | 3                     | -0.099050              | -0.037999             | 4                    |
| baseline_model_advance    | 7              | 345         | 0.016877                | -0.080559              | 3                     | -0.443783              | -0.137240             | 6                    |

## Split Decisions

| forward_month | selected_gate           | selected_conditions                                          | train_calendar_sum | train_worst_sm | train_trade_count | same_close_calendar_sum | adverse10_calendar_sum | trade_count |
| ------------- | ----------------------- | ------------------------------------------------------------ | ------------------ | -------------- | ----------------- | ----------------------- | ---------------------- | ----------- |
| 2025-10       | wick_touch_ext_le_0     | touch_extension_atr <= 0.000000                              | 0.028631           | -0.008066      | 43                | -0.026318               | -0.037999              | 10          |
| 2025-11       | low_rf_slope_up_q40_q60 | rf_probability <= 0.541000 & prev_sma5_slope_atr >= 0.041076 | 0.004482           | -0.024680      | 38                | 0.056852                | 0.047792               | 10          |
| 2025-12       | low_rf_slope_up_q40_q60 | rf_probability <= 0.546000 & prev_sma5_slope_atr >= 0.045439 | 0.066145           | -0.024846      | 34                | -0.003876               | -0.010931              | 7           |
| 2026-01       | low_rf_slope_up_q40_q60 | rf_probability <= 0.595000 & prev_sma5_slope_atr >= 0.037258 | 0.074049           | -0.003694      | 37                | 0.006025                | -0.002499              | 9           |
| 2026-02       | low_rf_slope_up_q40_q60 | rf_probability <= 0.624000 & prev_sma5_slope_atr >= 0.042544 | 0.059205           | -0.003694      | 33                | 0.019272                | 0.006525               | 12          |
| 2026-03       | low_eff_q20             | eff_300s <= 0.768036                                         | 0.081612           | -0.005611      | 43                | 0.089865                | 0.045014               | 21          |
| 2026-04       | low_eff_q20             | eff_300s <= 0.738583                                         | 0.072300           | -0.012709      | 41                | 0.019591                | 0.012451               | 6           |

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
  "train_months": 4,
  "min_train_trades": 20,
  "forward_months": [
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
  "runtime_seconds": 26.906622171401978
}
```
