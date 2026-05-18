# Breakout Structure Model Retrain Validation

Generated: 2026-05-18T09:24:30.340373+00:00

Scope: research-only. Event pools are retrained under production template exit params (`trail_start_r=1.5`, `max_hold_hours=2.0`) and evaluated on full-window plus forward splits.

## Summary

| pool                                      | feature_set | total_events | train_events | test_events | forward_events | forward_avg_pool_size_weight | timing_depth | rf_test_auc | full_adverse10_calendar_sum | full_adverse10_worst_sm | full_adverse10_neg_sm | forward_adverse10_calendar_sum | forward_adverse10_worst_sm | forward_adverse10_neg_sm | forward_trade_count |
| ----------------------------------------- | ----------- | ------------ | ------------ | ----------- | -------------- | ---------------------------- | ------------ | ----------- | --------------------------- | ----------------------- | --------------------- | ------------------------------ | -------------------------- | ------------------------ | ------------------- |
| combo_wf5_low_eff_q20                     | production8 | 225          | 40           | 28          | 157            | 1.000000                     | 4            | 0.860000    | 0.272486                    | 0.009001                | 0                     | 0.423931                       | -0.019235                  | 2                        | 149                 |
| combo_wf3_low_eff_low_atr                 | production8 | 195          | 42           | 29          | 124            | 1.000000                     | 4            | 0.758333    | 0.283974                    | 0.000228                | 0                     | 0.408512                       | -0.002244                  | 1                        | 115                 |
| combo_wf5_low_eff_q20                     | structure13 | 225          | 40           | 28          | 157            | 1.000000                     | 4            | 0.826923    | 0.276461                    | 0.013236                | 0                     | 0.402052                       | -0.017682                  | 2                        | 149                 |
| combo_wf3_low_eff_low_atr                 | structure13 | 195          | 42           | 29          | 124            | 1.000000                     | 3            | 0.650000    | 0.274401                    | 0.004365                | 0                     | 0.389701                       | -0.000636                  | 1                        | 115                 |
| combo_wf3_low_eff_low_atr_ctx12h_up       | production8 | 168          | 41           | 28          | 99             | 1.000000                     | 4            | 0.812500    | 0.263563                    | -0.001117               | 1                     | 0.371748                       | 0.011073                   | 0                        | 91                  |
| combo_wf3_low_eff_low_atr_ctx4h_scaled025 | production8 | 195          | 42           | 29          | 124            | 0.854839                     | 4            | 0.758333    | 0.290612                    | 0.006866                | 0                     | 0.370266                       | 0.006097                   | 0                        | 115                 |
| combo_wf3_low_eff_low_atr_ctx4h_up        | production8 | 169          | 41           | 28          | 100            | 1.000000                     | 4            | 0.893333    | 0.291738                    | 0.009079                | 0                     | 0.357531                       | 0.006612                   | 0                        | 92                  |
| combo_wf3_low_eff_low_atr_ctx12h_up       | structure13 | 168          | 41           | 28          | 99             | 1.000000                     | 3            | 0.700000    | 0.253480                    | 0.002985                | 0                     | 0.352291                       | 0.005951                   | 0                        | 91                  |
| combo_wf3_low_eff_low_atr_ctx4h_scaled025 | structure13 | 195          | 42           | 29          | 124            | 0.854839                     | 3            | 0.650000    | 0.281073                    | 0.011036                | 0                     | 0.350971                       | 0.000933                   | 0                        | 115                 |
| combo_wf3_low_eff_low_atr_ctx4h_up        | structure13 | 169          | 41           | 28          | 100            | 1.000000                     | 3            | 0.826923    | 0.281941                    | 0.013284                | 0                     | 0.337952                       | 0.001467                   | 0                        | 92                  |
| canonical_only                            | structure13 | 154          | 40           | 28          | 86             | 1.000000                     | 4            | 0.826923    | 0.276461                    | 0.013236                | 0                     | 0.303815                       | -0.002459                  | 1                        | 78                  |
| canonical_only                            | production8 | 154          | 40           | 28          | 86             | 1.000000                     | 4            | 0.860000    | 0.272486                    | 0.009001                | 0                     | 0.299716                       | 0.002688                   | 0                        | 78                  |

## Interpretation

- `production8` is the Go live trainer feature contract.
- `structure13` adds ATR percentile, prior body, SMA gap/slope, and level-to-prev-close structure features.
- A pool only deserves promotion if forward adverse10 improves without relying on in-sample `static_*` gates.

## Diagnostics

```json
{
  "canonical_events_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/tick_flow_event_sources/20260514_pretouch_full_window/feature_filtered_seed_events/robust_quality/pretouch_small_pullback_rf_q50_speed300_ge_q10_touch30m_eff300le1.csv",
  "base_events_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_shape_expansion_events_restrictive_0p5bps.csv",
  "pool_sizes": {
    "canonical_only": {
      "events": 154,
      "overlap_removed_events": 0,
      "source_counts": {
        "canonical": 154
      }
    },
    "combo_wf3_low_eff_low_atr": {
      "events": 195,
      "overlap_removed_events": 1,
      "source_counts": {
        "canonical": 154,
        "wf3_low_eff_low_atr": 41
      }
    },
    "combo_wf3_low_eff_low_atr_ctx4h_up": {
      "events": 169,
      "overlap_removed_events": 1,
      "source_counts": {
        "canonical": 154,
        "wf3_low_eff_low_atr_ctx4h_up": 15
      }
    },
    "combo_wf3_low_eff_low_atr_ctx12h_up": {
      "events": 168,
      "overlap_removed_events": 1,
      "source_counts": {
        "canonical": 154,
        "wf3_low_eff_low_atr_ctx12h_up": 14
      }
    },
    "combo_wf3_low_eff_low_atr_ctx4h_scaled025": {
      "events": 195,
      "overlap_removed_events": 1,
      "source_counts": {
        "canonical": 154,
        "wf3_low_eff_low_atr": 41
      }
    },
    "combo_wf5_low_eff_q20": {
      "events": 225,
      "overlap_removed_events": 6,
      "source_counts": {
        "canonical": 154,
        "wf5_low_eff_q20": 71
      }
    }
  },
  "feature_sets": {
    "production8": [
      "roundtrip_cost_atr",
      "prev1_range_atr",
      "prev1_close_pos_side",
      "level_to_signal_open_atr",
      "touch_extension_atr",
      "speed_300s_atr",
      "eff_300s",
      "pre_touch_seconds"
    ],
    "structure13": [
      "roundtrip_cost_atr",
      "prev1_range_atr",
      "prev1_close_pos_side",
      "level_to_signal_open_atr",
      "touch_extension_atr",
      "speed_300s_atr",
      "eff_300s",
      "pre_touch_seconds",
      "signal_atr_percentile",
      "prev1_body_atr",
      "prev_sma5_gap_atr",
      "prev_sma5_slope_atr",
      "level_to_prev_close_atr"
    ]
  },
  "base_share": 0.8,
  "forward_start": "2025-11-01T00:00:00+00:00",
  "runtime_seconds": 215.42578887939453
}
```
