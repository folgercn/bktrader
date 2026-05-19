# Breakout Structure Context Model Sizing

Generated: 2026-05-18T09:59:25.935020+00:00

Scope: research-only. This trains a trailing context-aware sizing overlay for `low_eff_low_atr` events.

## Aggregate

| variant            | forward_months | events | avg_scale | same_close_calendar_sum | same_close_worst_month | same_close_neg_months | adverse10_calendar_sum | adverse10_worst_month | adverse10_neg_months | adverse10_trade_count | adverse10_active_trade_count | trained_months |
| ------------------ | -------------- | ------ | --------- | ----------------------- | ---------------------- | --------------------- | ---------------------- | --------------------- | -------------------- | --------------------- | ---------------------------- | -------------- |
| ctx12h_scaled025   | 3              | 17     | 0.416667  | 0.000380                | -0.003544              | 1                     | -0.009415              | -0.004335             | 3                    | 17                    | 17                           | 2              |
| rf_replace_p2      | 3              | 17     | 0.844918  | -0.013800               | -0.008994              | 2                     | -0.028568              | -0.020712             | 2                    | 17                    | 17                           | 2              |
| ctx4h_scaled025    | 3              | 17     | 0.479167  | -0.016980               | -0.014152              | 2                     | -0.030562              | -0.025482             | 3                    | 17                    | 17                           | 2              |
| rf_rank_q60_000    | 3              | 17     | 0.611111  | -0.024993               | -0.020559              | 2                     | -0.041924              | -0.035431             | 2                    | 17                    | 14                           | 2              |
| rf_binary_000      | 3              | 17     | 0.500000  | -0.028150               | -0.020559              | 2                     | -0.044241              | -0.035431             | 2                    | 17                    | 13                           | 2              |
| rf_rank_q70_000    | 3              | 17     | 0.500000  | -0.028150               | -0.020559              | 2                     | -0.044241              | -0.035431             | 2                    | 17                    | 13                           | 2              |
| rf_binary_025      | 3              | 17     | 0.625000  | -0.027264               | -0.020559              | 2                     | -0.044543              | -0.035431             | 2                    | 17                    | 17                           | 2              |
| baseline_original  | 3              | 17     | 1.000000  | -0.024603               | -0.020559              | 2                     | -0.045447              | -0.035431             | 2                    | 17                    | 17                           | 2              |
| rf_rank_median_000 | 3              | 17     | 1.000000  | -0.024603               | -0.020559              | 2                     | -0.045447              | -0.035431             | 2                    | 17                    | 17                           | 2              |
| rf_rank_median_025 | 3              | 17     | 1.000000  | -0.024603               | -0.020559              | 2                     | -0.045447              | -0.035431             | 2                    | 17                    | 17                           | 2              |
| rf_prob_floor025   | 3              | 17     | 0.879456  | -0.027487               | -0.020559              | 2                     | -0.047136              | -0.035431             | 2                    | 17                    | 17                           | 2              |
| rf_prob_cap1       | 3              | 17     | 0.839275  | -0.028449               | -0.020559              | 2                     | -0.047698              | -0.035431             | 2                    | 17                    | 17                           | 2              |

## Monthly Rows

| forward_month | variant            | model_status  | events | avg_scale | same_close_calendar_sum | adverse10_calendar_sum | adverse10_trade_count | adverse10_active_trade_count |
| ------------- | ------------------ | ------------- | ------ | --------- | ----------------------- | ---------------------- | --------------------- | ---------------------------- |
| 2025-04       | baseline_original  | trained       | 2      | 1.000000  | -0.014176               | -0.016900              | 2                     | 2                            |
| 2025-04       | ctx4h_scaled025    | trained       | 2      | 0.250000  | -0.003544               | -0.004225              | 2                     | 2                            |
| 2025-04       | ctx12h_scaled025   | trained       | 2      | 0.250000  | -0.003544               | -0.004225              | 2                     | 2                            |
| 2025-04       | rf_prob_cap1       | trained       | 2      | 0.954619  | -0.013579               | -0.016166              | 2                     | 2                            |
| 2025-04       | rf_prob_floor025   | trained       | 2      | 0.965964  | -0.013728               | -0.016349              | 2                     | 2                            |
| 2025-04       | rf_binary_025      | trained       | 2      | 0.625000  | -0.009238               | -0.010833              | 2                     | 2                            |
| 2025-04       | rf_binary_000      | trained       | 2      | 0.500000  | -0.007592               | -0.008810              | 2                     | 1                            |
| 2025-04       | rf_rank_median_025 | trained       | 2      | 1.000000  | -0.014176               | -0.016900              | 2                     | 2                            |
| 2025-04       | rf_rank_median_000 | trained       | 2      | 1.000000  | -0.014176               | -0.016900              | 2                     | 2                            |
| 2025-04       | rf_rank_q60_000    | trained       | 2      | 0.500000  | -0.007592               | -0.008810              | 2                     | 1                            |
| 2025-04       | rf_rank_q70_000    | trained       | 2      | 0.500000  | -0.007592               | -0.008810              | 2                     | 1                            |
| 2025-04       | rf_replace_p2      | trained       | 2      | 0.971548  | -0.008835               | -0.010478              | 2                     | 2                            |
| 2025-05       | baseline_original  | too_few_train | 12     | 1.000000  | -0.020559               | -0.035431              | 12                    | 12                           |
| 2025-05       | ctx4h_scaled025    | too_few_train | 12     | 0.687500  | -0.014152               | -0.025482              | 12                    | 12                           |
| 2025-05       | ctx12h_scaled025   | too_few_train | 12     | 0.500000  | 0.003208                | -0.004335              | 12                    | 12                           |
| 2025-05       | rf_prob_cap1       | too_few_train | 12     | 1.000000  | -0.020559               | -0.035431              | 12                    | 12                           |
| 2025-05       | rf_prob_floor025   | too_few_train | 12     | 1.000000  | -0.020559               | -0.035431              | 12                    | 12                           |
| 2025-05       | rf_binary_025      | too_few_train | 12     | 1.000000  | -0.020559               | -0.035431              | 12                    | 12                           |
| 2025-05       | rf_binary_000      | too_few_train | 12     | 1.000000  | -0.020559               | -0.035431              | 12                    | 12                           |
| 2025-05       | rf_rank_median_025 | too_few_train | 12     | 1.000000  | -0.020559               | -0.035431              | 12                    | 12                           |
| 2025-05       | rf_rank_median_000 | too_few_train | 12     | 1.000000  | -0.020559               | -0.035431              | 12                    | 12                           |
| 2025-05       | rf_rank_q60_000    | too_few_train | 12     | 1.000000  | -0.020559               | -0.035431              | 12                    | 12                           |
| 2025-05       | rf_rank_q70_000    | too_few_train | 12     | 1.000000  | -0.020559               | -0.035431              | 12                    | 12                           |
| 2025-05       | rf_replace_p2      | too_few_train | 12     | 1.000000  | -0.008994               | -0.020712              | 12                    | 12                           |
| 2025-06       | baseline_original  | trained       | 3      | 1.000000  | 0.010132                | 0.006884               | 3                     | 3                            |
| 2025-06       | ctx4h_scaled025    | trained       | 3      | 0.500000  | 0.000716                | -0.000855              | 3                     | 3                            |
| 2025-06       | ctx12h_scaled025   | trained       | 3      | 0.500000  | 0.000716                | -0.000855              | 3                     | 3                            |
| 2025-06       | rf_prob_cap1       | trained       | 3      | 0.563206  | 0.005689                | 0.003898               | 3                     | 3                            |
| 2025-06       | rf_prob_floor025   | trained       | 3      | 0.672405  | 0.006799                | 0.004645               | 3                     | 3                            |
| 2025-06       | rf_binary_025      | trained       | 3      | 0.250000  | 0.002533                | 0.001721               | 3                     | 3                            |
| 2025-06       | rf_binary_000      | trained       | 3      | 0.000000  | 0.000000                | 0.000000               | 3                     | 0                            |
| 2025-06       | rf_rank_median_025 | trained       | 3      | 1.000000  | 0.010132                | 0.006884               | 3                     | 3                            |
| 2025-06       | rf_rank_median_000 | trained       | 3      | 1.000000  | 0.010132                | 0.006884               | 3                     | 3                            |
| 2025-06       | rf_rank_q60_000    | trained       | 3      | 0.333333  | 0.003157                | 0.002317               | 3                     | 1                            |
| 2025-06       | rf_rank_q70_000    | trained       | 3      | 0.000000  | 0.000000                | 0.000000               | 3                     | 0                            |
| 2025-06       | rf_replace_p2      | trained       | 3      | 0.563206  | 0.004028                | 0.002622               | 3                     | 3                            |

## Interpretation

- `baseline_original` keeps the event source's original RF sizing.
- `ctx*_scaled025` are fixed 4h/12h context overlays for comparison.
- `rf_*` variants train only on prior-window adverse10 labels, then size the next month.
- Promotion requires improvement on ETH late without breaking ETH early or BTC late.

## Diagnostics

```json
{
  "symbol": "ETHUSDT",
  "events_csv": "research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_ethusdt_202503_202506_train1m_events.csv",
  "candidate_rows_csv": "research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_ethusdt_202503_202506_train1m_min5_candidates.csv",
  "bars_cache_dir": "research/probabilistic_v6_runs/2025_m03_m06_original_t2_delay60/bars_cache",
  "eval_start": "2025-03-01T00:00:00+00:00",
  "eval_end_exclusive": "2025-07-01T00:00:00+00:00",
  "train_months": 1,
  "min_train_events": 4,
  "base_gate": "low_eff_low_atr_q20_q40",
  "context_gates": {
    "ctx4h_scaled025": "low_eff_low_atr_ctx4h_up",
    "ctx12h_scaled025": "low_eff_low_atr_ctx12h_up"
  },
  "model_variants": [
    "rf_prob_cap1",
    "rf_prob_floor025",
    "rf_binary_025",
    "rf_binary_000",
    "rf_rank_median_025",
    "rf_rank_median_000",
    "rf_rank_q60_000",
    "rf_rank_q70_000",
    "rf_replace_p2"
  ],
  "feature_columns": [
    "rf_probability",
    "sizing_multiplier",
    "signal_atr_percentile",
    "roundtrip_cost_atr",
    "prev1_range_atr",
    "prev1_close_pos_side",
    "level_to_signal_open_atr",
    "touch_extension_atr",
    "speed_300s_atr",
    "eff_300s",
    "pre_touch_seconds",
    "ctx4h_side_return_atr",
    "ctx12h_side_return_atr",
    "ctx4h_range_atr",
    "ctx12h_range_atr",
    "side_is_long"
  ],
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
  "splits": [
    {
      "forward_month": "2025-04",
      "train_start": "2025-03-01T00:00:00+00:00",
      "train_events_after_base_gate": 5,
      "forward_events_after_base_gate": 2,
      "train_events": 5,
      "model_status": "trained",
      "label_events": 5,
      "positive_labels": 2,
      "negative_labels": 3,
      "train_auc": 1.0,
      "train_prob_median": 0.39921428571428563,
      "train_prob_q40": 0.36663333333333326,
      "train_prob_q60": 0.45799999999999985,
      "train_prob_q70": 0.5167857142857141,
      "forward_prob_mean": 0.48577380952380933,
      "forward_prob_median": 0.48577380952380933,
      "feature_importance_top5": [
        [
          "ctx4h_range_atr",
          0.16853932584269662
        ],
        [
          "prev1_close_pos_side",
          0.15730337078651685
        ],
        [
          "pre_touch_seconds",
          0.0898876404494382
        ],
        [
          "prev1_range_atr",
          0.07865168539325842
        ],
        [
          "speed_300s_atr",
          0.07865168539325842
        ]
      ]
    },
    {
      "forward_month": "2025-05",
      "train_start": "2025-04-01T00:00:00+00:00",
      "train_events_after_base_gate": 3,
      "forward_events_after_base_gate": 12,
      "train_events": 3,
      "model_status": "too_few_train"
    },
    {
      "forward_month": "2025-06",
      "train_start": "2025-05-01T00:00:00+00:00",
      "train_events_after_base_gate": 7,
      "forward_events_after_base_gate": 3,
      "train_events": 7,
      "model_status": "trained",
      "label_events": 7,
      "positive_labels": 2,
      "negative_labels": 5,
      "train_auc": 1.0,
      "train_prob_median": 0.23679761904761903,
      "train_prob_q40": 0.21999047619047618,
      "train_prob_q60": 0.256647619047619,
      "train_prob_q70": 0.3689214285714282,
      "forward_prob_mean": 0.2816031746031745,
      "forward_prob_median": 0.25540476190476186,
      "feature_importance_top5": [
        [
          "prev1_range_atr",
          0.13559322033898305
        ],
        [
          "ctx12h_side_return_atr",
          0.11622276029055689
        ],
        [
          "ctx4h_side_return_atr",
          0.10734463276836158
        ],
        [
          "prev1_close_pos_side",
          0.096045197740113
        ],
        [
          "pre_touch_seconds",
          0.0847457627118644
        ]
      ]
    }
  ],
  "runtime_seconds": 4.964913368225098
}
```
