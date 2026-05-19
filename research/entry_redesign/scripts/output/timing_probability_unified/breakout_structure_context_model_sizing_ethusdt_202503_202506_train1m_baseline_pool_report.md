# Breakout Structure Context Model Sizing

Generated: 2026-05-18T10:01:11.478018+00:00

Scope: research-only. This trains a trailing context-aware sizing overlay for `low_eff_low_atr` events.

## Aggregate

| variant            | forward_months | events | avg_scale | same_close_calendar_sum | same_close_worst_month | same_close_neg_months | adverse10_calendar_sum | adverse10_worst_month | adverse10_neg_months | adverse10_trade_count | adverse10_active_trade_count | trained_months |
| ------------------ | -------------- | ------ | --------- | ----------------------- | ---------------------- | --------------------- | ---------------------- | --------------------- | -------------------- | --------------------- | ---------------------------- | -------------- |
| rf_rank_q70_000    | 3              | 149    | 0.226731  | 0.014721                | -0.014613              | 2                     | -0.031129              | -0.018658             | 3                    | 149                   | 37                           | 3              |
| rf_binary_000      | 3              | 149    | 0.073114  | -0.035320               | -0.020433              | 2                     | -0.045536              | -0.026200             | 2                    | 149                   | 10                           | 3              |
| rf_binary_025      | 3              | 149    | 0.304835  | -0.077068               | -0.037646              | 3                     | -0.131783              | -0.059873             | 3                    | 149                   | 149                          | 3              |
| ctx12h_scaled025   | 3              | 149    | 0.549067  | -0.058373               | -0.045455              | 2                     | -0.163704              | -0.082450             | 3                    | 149                   | 149                          | 3              |
| rf_rank_q60_000    | 3              | 149    | 0.476772  | -0.089782               | -0.036327              | 3                     | -0.180458              | -0.068092             | 3                    | 149                   | 75                           | 3              |
| rf_replace_p2      | 3              | 149    | 0.660766  | -0.103551               | -0.049042              | 3                     | -0.194417              | -0.090374             | 3                    | 149                   | 149                          | 3              |
| rf_rank_median_000 | 3              | 149    | 0.770154  | -0.087642               | -0.050559              | 3                     | -0.228169              | -0.084739             | 3                    | 149                   | 114                          | 3              |
| rf_prob_cap1       | 3              | 149    | 0.649937  | -0.121823               | -0.059987              | 3                     | -0.236203              | -0.108719             | 3                    | 149                   | 149                          | 3              |
| ctx4h_scaled025    | 3              | 149    | 0.543027  | -0.149480               | -0.074236              | 3                     | -0.252396              | -0.109823             | 3                    | 149                   | 149                          | 3              |
| rf_rank_median_025 | 3              | 149    | 0.827615  | -0.116309               | -0.060004              | 3                     | -0.268758              | -0.094279             | 3                    | 149                   | 149                          | 3              |
| rf_prob_floor025   | 3              | 149    | 0.737453  | -0.141945               | -0.067075              | 3                     | -0.274783              | -0.121762             | 3                    | 149                   | 149                          | 3              |
| baseline_original  | 3              | 149    | 1.000000  | -0.202311               | -0.089283              | 3                     | -0.390524              | -0.160892             | 3                    | 149                   | 149                          | 3              |

## Monthly Rows

| forward_month | variant            | model_status | events | avg_scale | same_close_calendar_sum | adverse10_calendar_sum | adverse10_trade_count | adverse10_active_trade_count |
| ------------- | ------------------ | ------------ | ------ | --------- | ----------------------- | ---------------------- | --------------------- | ---------------------------- |
| 2025-04       | baseline_original  | trained      | 34     | 1.000000  | -0.088338               | -0.129498              | 34                    | 34                           |
| 2025-04       | ctx4h_scaled025    | trained      | 34     | 0.580882  | -0.053433               | -0.077562              | 34                    | 34                           |
| 2025-04       | ctx12h_scaled025   | trained      | 34     | 0.558824  | -0.019978               | -0.042766              | 34                    | 34                           |
| 2025-04       | rf_prob_cap1       | trained      | 34     | 0.781966  | -0.059987               | -0.091486              | 34                    | 34                           |
| 2025-04       | rf_prob_floor025   | trained      | 34     | 0.836474  | -0.067075               | -0.100989              | 34                    | 34                           |
| 2025-04       | rf_binary_025      | trained      | 34     | 0.338235  | -0.033250               | -0.046876              | 34                    | 34                           |
| 2025-04       | rf_binary_000      | trained      | 34     | 0.117647  | -0.014887               | -0.019336              | 34                    | 4                            |
| 2025-04       | rf_rank_median_025 | trained      | 34     | 0.845588  | -0.060004               | -0.094279              | 34                    | 34                           |
| 2025-04       | rf_rank_median_000 | trained      | 34     | 0.794118  | -0.050559               | -0.082540              | 34                    | 27                           |
| 2025-04       | rf_rank_q60_000    | trained      | 34     | 0.294118  | -0.035316               | -0.046226              | 34                    | 10                           |
| 2025-04       | rf_rank_q70_000    | trained      | 34     | 0.058824  | -0.005014               | -0.007223              | 34                    | 2                            |
| 2025-04       | rf_replace_p2      | trained      | 34     | 0.795034  | -0.049042               | -0.072569              | 34                    | 34                           |
| 2025-05       | baseline_original  | trained      | 56     | 1.000000  | -0.024691               | -0.100134              | 56                    | 56                           |
| 2025-05       | ctx4h_scaled025    | trained      | 56     | 0.531250  | -0.021811               | -0.065011              | 56                    | 56                           |
| 2025-05       | ctx12h_scaled025   | trained      | 56     | 0.571429  | 0.007060                | -0.038488              | 56                    | 56                           |
| 2025-05       | rf_prob_cap1       | trained      | 56     | 0.427389  | -0.005181               | -0.035998              | 56                    | 56                           |
| 2025-05       | rf_prob_floor025   | trained      | 56     | 0.570542  | -0.010059               | -0.052032              | 56                    | 56                           |
| 2025-05       | rf_binary_025      | trained      | 56     | 0.250000  | -0.006173               | -0.025033              | 56                    | 56                           |
| 2025-05       | rf_binary_000      | trained      | 56     | 0.000000  | 0.000000                | 0.000000               | 56                    | 0                            |
| 2025-05       | rf_rank_median_025 | trained      | 56     | 0.866071  | -0.025126               | -0.088588              | 56                    | 56                           |
| 2025-05       | rf_rank_median_000 | trained      | 56     | 0.821429  | -0.025272               | -0.084739              | 56                    | 46                           |
| 2025-05       | rf_rank_q60_000    | trained      | 56     | 0.678571  | -0.018140               | -0.068092              | 56                    | 38                           |
| 2025-05       | rf_rank_q70_000    | trained      | 56     | 0.553571  | 0.034347                | -0.005249              | 56                    | 31                           |
| 2025-05       | rf_replace_p2      | trained      | 56     | 0.427389  | -0.007850               | -0.031474              | 56                    | 56                           |
| 2025-06       | baseline_original  | trained      | 59     | 1.000000  | -0.089283               | -0.160892              | 59                    | 59                           |
| 2025-06       | ctx4h_scaled025    | trained      | 59     | 0.516949  | -0.074236               | -0.109823              | 59                    | 59                           |
| 2025-06       | ctx12h_scaled025   | trained      | 59     | 0.516949  | -0.045455               | -0.082450              | 59                    | 59                           |
| 2025-06       | rf_prob_cap1       | trained      | 59     | 0.740457  | -0.056655               | -0.108719              | 59                    | 59                           |
| 2025-06       | rf_prob_floor025   | trained      | 59     | 0.805343  | -0.064812               | -0.121762              | 59                    | 59                           |
| 2025-06       | rf_binary_025      | trained      | 59     | 0.326271  | -0.037646               | -0.059873              | 59                    | 59                           |
| 2025-06       | rf_binary_000      | trained      | 59     | 0.101695  | -0.020433               | -0.026200              | 59                    | 6                            |
| 2025-06       | rf_rank_median_025 | trained      | 59     | 0.771186  | -0.031179               | -0.085891              | 59                    | 59                           |
| 2025-06       | rf_rank_median_000 | trained      | 59     | 0.694915  | -0.011811               | -0.060890              | 59                    | 41                           |
| 2025-06       | rf_rank_q60_000    | trained      | 59     | 0.457627  | -0.036327               | -0.066140              | 59                    | 27                           |
| 2025-06       | rf_rank_q70_000    | trained      | 59     | 0.067797  | -0.014613               | -0.018658              | 59                    | 4                            |
| 2025-06       | rf_replace_p2      | trained      | 59     | 0.759875  | -0.046659               | -0.090374              | 59                    | 59                           |

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
  "min_train_events": 20,
  "base_gate": "baseline_model_advance",
  "context_gates": {
    "ctx4h_scaled025": "ctx4h_side_up_q60",
    "ctx12h_scaled025": "ctx12h_side_up_q60"
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
      "train_events_after_base_gate": 58,
      "forward_events_after_base_gate": 34,
      "train_events": 58,
      "model_status": "trained",
      "label_events": 58,
      "positive_labels": 20,
      "negative_labels": 38,
      "train_auc": 0.9960526315789473,
      "train_prob_median": 0.33384427380352494,
      "train_prob_q40": 0.3155045550572986,
      "train_prob_q60": 0.4495483862893896,
      "train_prob_q70": 0.5589506659732176,
      "forward_prob_mean": 0.39751691823765317,
      "forward_prob_median": 0.4071280033818407,
      "feature_importance_top5": [
        [
          "ctx12h_side_return_atr",
          0.13482674877898793
        ],
        [
          "ctx12h_range_atr",
          0.13424038897302354
        ],
        [
          "prev1_close_pos_side",
          0.09465146738220712
        ],
        [
          "ctx4h_range_atr",
          0.0865760524655466
        ],
        [
          "touch_extension_atr",
          0.07686187772722143
        ]
      ]
    },
    {
      "forward_month": "2025-05",
      "train_start": "2025-04-01T00:00:00+00:00",
      "train_events_after_base_gate": 34,
      "forward_events_after_base_gate": 56,
      "train_events": 34,
      "model_status": "trained",
      "label_events": 34,
      "positive_labels": 6,
      "negative_labels": 28,
      "train_auc": 1.0,
      "train_prob_median": 0.10735594743369369,
      "train_prob_q40": 0.09826515038677444,
      "train_prob_q60": 0.15483021329195695,
      "train_prob_q70": 0.1942399571946742,
      "forward_prob_mean": 0.21369470947730537,
      "forward_prob_median": 0.20845125599941772,
      "feature_importance_top5": [
        [
          "eff_300s",
          0.1389189130234329
        ],
        [
          "ctx4h_side_return_atr",
          0.12595008049174475
        ],
        [
          "speed_300s_atr",
          0.11458239731469026
        ],
        [
          "level_to_signal_open_atr",
          0.09842263601779552
        ],
        [
          "touch_extension_atr",
          0.07353621305399838
        ]
      ]
    },
    {
      "forward_month": "2025-06",
      "train_start": "2025-05-01T00:00:00+00:00",
      "train_events_after_base_gate": 56,
      "forward_events_after_base_gate": 59,
      "train_events": 56,
      "model_status": "trained",
      "label_events": 56,
      "positive_labels": 18,
      "negative_labels": 38,
      "train_auc": 0.9985380116959065,
      "train_prob_median": 0.3241680468044209,
      "train_prob_q40": 0.2752707692366257,
      "train_prob_q60": 0.37353136339733894,
      "train_prob_q70": 0.5473946902029236,
      "forward_prob_mean": 0.37993765189307854,
      "forward_prob_median": 0.3682711708591277,
      "feature_importance_top5": [
        [
          "prev1_range_atr",
          0.11409237138913884
        ],
        [
          "level_to_signal_open_atr",
          0.10324961209475493
        ],
        [
          "ctx12h_side_return_atr",
          0.08974055644590376
        ],
        [
          "ctx4h_range_atr",
          0.08346961706222414
        ],
        [
          "ctx12h_range_atr",
          0.08320599216604457
        ]
      ]
    }
  ],
  "runtime_seconds": 15.980764150619507
}
```
