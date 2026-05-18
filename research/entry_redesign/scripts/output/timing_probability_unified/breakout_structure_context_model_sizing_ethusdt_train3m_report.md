# Breakout Structure Context Model Sizing

Generated: 2026-05-18T09:59:18.835703+00:00

Scope: research-only. This trains a trailing context-aware sizing overlay for `low_eff_low_atr` events.

## Aggregate

| variant            | forward_months | events | avg_scale | same_close_calendar_sum | same_close_worst_month | same_close_neg_months | adverse10_calendar_sum | adverse10_worst_month | adverse10_neg_months | adverse10_trade_count | adverse10_active_trade_count | trained_months |
| ------------------ | -------------- | ------ | --------- | ----------------------- | ---------------------- | --------------------- | ---------------------- | --------------------- | -------------------- | --------------------- | ---------------------------- | -------------- |
| rf_rank_median_000 | 8              | 42     | 0.637004  | 0.106086                | -0.001607              | 1                     | 0.073127               | -0.004736             | 2                    | 42                    | 28                           | 8              |
| rf_rank_median_025 | 8              | 42     | 0.727753  | 0.110071                | -0.002690              | 1                     | 0.073059               | -0.006124             | 3                    | 42                    | 42                           | 8              |
| baseline_original  | 8              | 42     | 1.000000  | 0.122025                | -0.010761              | 1                     | 0.072854               | -0.012840             | 2                    | 42                    | 42                           | 8              |
| ctx4h_scaled025    | 8              | 42     | 0.561533  | 0.093917                | -0.002690              | 1                     | 0.067941               | -0.003210             | 2                    | 42                    | 42                           | 8              |
| rf_prob_floor025   | 8              | 42     | 0.749297  | 0.103375                | -0.007343              | 1                     | 0.066093               | -0.008752             | 3                    | 42                    | 42                           | 8              |
| rf_prob_cap1       | 8              | 42     | 0.665730  | 0.097158                | -0.006204              | 1                     | 0.063840               | -0.007390             | 3                    | 42                    | 42                           | 8              |
| ctx12h_scaled025   | 8              | 42     | 0.497247  | 0.089564                | -0.007884              | 1                     | 0.062589               | -0.009336             | 2                    | 42                    | 42                           | 8              |
| rf_replace_p2      | 8              | 42     | 0.704602  | 0.088297                | -0.004594              | 2                     | 0.061179               | -0.007175             | 3                    | 42                    | 42                           | 8              |
| rf_rank_q60_000    | 8              | 42     | 0.475397  | 0.083419                | -0.001607              | 1                     | 0.057883               | -0.004736             | 2                    | 42                    | 21                           | 8              |
| rf_binary_025      | 8              | 42     | 0.372247  | 0.077290                | -0.002858              | 2                     | 0.057678               | -0.006372             | 2                    | 42                    | 42                           | 8              |
| rf_binary_000      | 8              | 42     | 0.162996  | 0.062378                | -0.004110              | 1                     | 0.052619               | -0.005066             | 1                    | 42                    | 8                            | 8              |
| rf_rank_q70_000    | 8              | 42     | 0.201389  | 0.015277                | -0.004110              | 2                     | 0.010347               | -0.005066             | 2                    | 42                    | 5                            | 8              |

## Monthly Rows

| forward_month | variant            | model_status | events | avg_scale | same_close_calendar_sum | adverse10_calendar_sum | adverse10_trade_count | adverse10_active_trade_count |
| ------------- | ------------------ | ------------ | ------ | --------- | ----------------------- | ---------------------- | --------------------- | ---------------------------- |
| 2025-09       | baseline_original  | trained      | 1      | 1.000000  | 0.010425                | 0.009583               | 1                     | 1                            |
| 2025-09       | ctx4h_scaled025    | trained      | 1      | 1.000000  | 0.010425                | 0.009583               | 1                     | 1                            |
| 2025-09       | ctx12h_scaled025   | trained      | 1      | 0.250000  | 0.002606                | 0.002396               | 1                     | 1                            |
| 2025-09       | rf_prob_cap1       | trained      | 1      | 0.637515  | 0.006646                | 0.006109               | 1                     | 1                            |
| 2025-09       | rf_prob_floor025   | trained      | 1      | 0.728136  | 0.007591                | 0.006977               | 1                     | 1                            |
| 2025-09       | rf_binary_025      | trained      | 1      | 0.250000  | 0.002606                | 0.002396               | 1                     | 1                            |
| 2025-09       | rf_binary_000      | trained      | 1      | 0.000000  | 0.000000                | 0.000000               | 1                     | 0                            |
| 2025-09       | rf_rank_median_025 | trained      | 1      | 1.000000  | 0.010425                | 0.009583               | 1                     | 1                            |
| 2025-09       | rf_rank_median_000 | trained      | 1      | 1.000000  | 0.010425                | 0.009583               | 1                     | 1                            |
| 2025-09       | rf_rank_q60_000    | trained      | 1      | 1.000000  | 0.010425                | 0.009583               | 1                     | 1                            |
| 2025-09       | rf_rank_q70_000    | trained      | 1      | 1.000000  | 0.010425                | 0.009583               | 1                     | 1                            |
| 2025-09       | rf_replace_p2      | trained      | 1      | 0.637515  | 0.006329                | 0.005818               | 1                     | 1                            |
| 2025-10       | baseline_original  | trained      | 2      | 1.000000  | -0.010761               | -0.012840              | 2                     | 2                            |
| 2025-10       | ctx4h_scaled025    | trained      | 2      | 0.250000  | -0.002690               | -0.003210              | 2                     | 2                            |
| 2025-10       | ctx12h_scaled025   | trained      | 2      | 0.625000  | -0.007884               | -0.009336              | 2                     | 2                            |
| 2025-10       | rf_prob_cap1       | trained      | 2      | 0.556572  | -0.006204               | -0.007390              | 2                     | 2                            |
| 2025-10       | rf_prob_floor025   | trained      | 2      | 0.667429  | -0.007343               | -0.008752              | 2                     | 2                            |
| 2025-10       | rf_binary_025      | trained      | 2      | 0.250000  | -0.002690               | -0.003210              | 2                     | 2                            |
| 2025-10       | rf_binary_000      | trained      | 2      | 0.000000  | 0.000000                | 0.000000               | 2                     | 0                            |
| 2025-10       | rf_rank_median_025 | trained      | 2      | 0.250000  | -0.002690               | -0.003210              | 2                     | 2                            |
| 2025-10       | rf_rank_median_000 | trained      | 2      | 0.000000  | 0.000000                | 0.000000               | 2                     | 0                            |
| 2025-10       | rf_rank_q60_000    | trained      | 2      | 0.000000  | 0.000000                | 0.000000               | 2                     | 0                            |
| 2025-10       | rf_rank_q70_000    | trained      | 2      | 0.000000  | 0.000000                | 0.000000               | 2                     | 0                            |
| 2025-10       | rf_replace_p2      | trained      | 2      | 0.556572  | -0.004594               | -0.005488              | 2                     | 2                            |
| 2025-11       | baseline_original  | trained      | 7      | 1.000000  | 0.016961                | 0.008626               | 7                     | 7                            |
| 2025-11       | ctx4h_scaled025    | trained      | 7      | 0.571429  | 0.005892                | 0.000770               | 7                     | 7                            |
| 2025-11       | ctx12h_scaled025   | trained      | 7      | 0.357143  | 0.004477                | 0.000967               | 7                     | 7                            |
| 2025-11       | rf_prob_cap1       | trained      | 7      | 0.755056  | 0.017697                | 0.011595               | 7                     | 7                            |
| 2025-11       | rf_prob_floor025   | trained      | 7      | 0.816292  | 0.017513                | 0.010853               | 7                     | 7                            |
| 2025-11       | rf_binary_025      | trained      | 7      | 0.357143  | 0.009505                | 0.006694               | 7                     | 7                            |
| 2025-11       | rf_binary_000      | trained      | 7      | 0.142857  | 0.007019                | 0.006050               | 7                     | 1                            |
| 2025-11       | rf_rank_median_025 | trained      | 7      | 0.892857  | 0.021147                | 0.013836               | 7                     | 7                            |
| 2025-11       | rf_rank_median_000 | trained      | 7      | 0.857143  | 0.022542                | 0.015572               | 7                     | 6                            |
| 2025-11       | rf_rank_q60_000    | trained      | 7      | 0.714286  | 0.016762                | 0.010682               | 7                     | 5                            |
| 2025-11       | rf_rank_q70_000    | trained      | 7      | 0.000000  | 0.000000                | 0.000000               | 7                     | 0                            |
| 2025-11       | rf_replace_p2      | trained      | 7      | 0.791830  | 0.018732                | 0.013682               | 7                     | 7                            |
| 2025-12       | baseline_original  | trained      | 9      | 1.000000  | 0.000899                | -0.010290              | 9                     | 9                            |
| 2025-12       | ctx4h_scaled025    | trained      | 9      | 0.583333  | 0.005011                | -0.001379              | 9                     | 9                            |
| 2025-12       | ctx12h_scaled025   | trained      | 9      | 0.583333  | 0.005011                | -0.001379              | 9                     | 9                            |
| 2025-12       | rf_prob_cap1       | trained      | 9      | 0.743490  | 0.001424                | -0.006871              | 9                     | 9                            |
| 2025-12       | rf_prob_floor025   | trained      | 9      | 0.807618  | 0.001293                | -0.007725              | 9                     | 9                            |
| 2025-12       | rf_binary_025      | trained      | 9      | 0.333333  | -0.002858               | -0.006372              | 9                     | 9                            |
| 2025-12       | rf_binary_000      | trained      | 9      | 0.111111  | -0.004110               | -0.005066              | 9                     | 1                            |
| 2025-12       | rf_rank_median_025 | trained      | 9      | 0.916667  | 0.004314                | -0.006124              | 9                     | 9                            |
| 2025-12       | rf_rank_median_000 | trained      | 9      | 0.888889  | 0.005453                | -0.004736              | 9                     | 8                            |
| 2025-12       | rf_rank_q60_000    | trained      | 9      | 0.888889  | 0.005453                | -0.004736              | 9                     | 8                            |
| 2025-12       | rf_rank_q70_000    | trained      | 9      | 0.111111  | -0.004110               | -0.005066              | 9                     | 1                            |
| 2025-12       | rf_replace_p2      | trained      | 9      | 0.778456  | -0.000762               | -0.007175              | 9                     | 9                            |
| 2026-01       | baseline_original  | trained      | 5      | 1.000000  | 0.008021                | 0.003008               | 5                     | 5                            |
| 2026-01       | ctx4h_scaled025    | trained      | 5      | 0.550000  | 0.005683                | 0.003197               | 5                     | 5                            |
| 2026-01       | ctx12h_scaled025   | trained      | 5      | 0.550000  | 0.009226                | 0.005803               | 5                     | 5                            |
| 2026-01       | rf_prob_cap1       | trained      | 5      | 0.429645  | 0.001001                | -0.001110              | 5                     | 5                            |
| 2026-01       | rf_prob_floor025   | trained      | 5      | 0.572234  | 0.002756                | -0.000081              | 5                     | 5                            |
| 2026-01       | rf_binary_025      | trained      | 5      | 0.250000  | 0.002005                | 0.000752               | 5                     | 5                            |
| 2026-01       | rf_binary_000      | trained      | 5      | 0.000000  | 0.000000                | 0.000000               | 5                     | 0                            |
| 2026-01       | rf_rank_median_025 | trained      | 5      | 0.700000  | 0.000800                | -0.002044              | 5                     | 5                            |
| 2026-01       | rf_rank_median_000 | trained      | 5      | 0.600000  | -0.001607               | -0.003727              | 5                     | 3                            |
| 2026-01       | rf_rank_q60_000    | trained      | 5      | 0.400000  | -0.001607               | -0.003727              | 5                     | 2                            |
| 2026-01       | rf_rank_q70_000    | trained      | 5      | 0.400000  | -0.001607               | -0.003727              | 5                     | 2                            |
| 2026-01       | rf_replace_p2      | trained      | 5      | 0.429645  | 0.000132                | -0.001617              | 5                     | 5                            |
| 2026-02       | baseline_original  | trained      | 4      | 1.000000  | 0.018394                | 0.012771               | 4                     | 4                            |
| 2026-02       | ctx4h_scaled025    | trained      | 4      | 0.625000  | 0.027031                | 0.023352               | 4                     | 4                            |
| 2026-02       | ctx12h_scaled025   | trained      | 4      | 0.437500  | 0.020907                | 0.018172               | 4                     | 4                            |
| 2026-02       | rf_prob_cap1       | trained      | 4      | 0.760326  | 0.021127                | 0.016767               | 4                     | 4                            |
| 2026-02       | rf_prob_floor025   | trained      | 4      | 0.820244  | 0.020444                | 0.015768               | 4                     | 4                            |
| 2026-02       | rf_binary_025      | trained      | 4      | 0.437500  | 0.020907                | 0.018172               | 4                     | 4                            |
| 2026-02       | rf_binary_000      | trained      | 4      | 0.250000  | 0.021745                | 0.019972               | 4                     | 1                            |
| 2026-02       | rf_rank_median_025 | trained      | 4      | 0.812500  | 0.027158                | 0.022705               | 4                     | 4                            |
| 2026-02       | rf_rank_median_000 | trained      | 4      | 0.750000  | 0.030079                | 0.026016               | 4                     | 3                            |
| 2026-02       | rf_rank_q60_000    | trained      | 4      | 0.250000  | 0.021745                | 0.019972               | 4                     | 1                            |
| 2026-02       | rf_rank_q70_000    | trained      | 4      | 0.000000  | 0.000000                | 0.000000               | 4                     | 0                            |
| 2026-02       | rf_replace_p2      | trained      | 4      | 0.760387  | 0.022139                | 0.018546               | 4                     | 4                            |
| 2026-03       | baseline_original  | trained      | 10     | 1.000000  | 0.049582                | 0.038316               | 10                    | 10                           |
| 2026-03       | ctx4h_scaled025    | trained      | 10     | 0.475000  | 0.030482                | 0.025381               | 10                    | 10                           |
| 2026-03       | ctx12h_scaled025   | trained      | 10     | 0.550000  | 0.036985                | 0.030778               | 10                    | 10                           |
| 2026-03       | rf_prob_cap1       | trained      | 10     | 0.572754  | 0.030702                | 0.024342               | 10                    | 10                           |
| 2026-03       | rf_prob_floor025   | trained      | 10     | 0.679565  | 0.035422                | 0.027835               | 10                    | 10                           |
| 2026-03       | rf_binary_025      | trained      | 10     | 0.475000  | 0.030482                | 0.025381               | 10                    | 10                           |
| 2026-03       | rf_binary_000      | trained      | 10     | 0.300000  | 0.024115                | 0.021070               | 10                    | 3                            |
| 2026-03       | rf_rank_median_025 | trained      | 10     | 0.625000  | 0.031585                | 0.024448               | 10                    | 10                           |
| 2026-03       | rf_rank_median_000 | trained      | 10     | 0.500000  | 0.025586                | 0.019826               | 10                    | 5                            |
| 2026-03       | rf_rank_q60_000    | trained      | 10     | 0.300000  | 0.024115                | 0.021070               | 10                    | 3                            |
| 2026-03       | rf_rank_q70_000    | trained      | 10     | 0.100000  | 0.010569                | 0.009558               | 10                    | 1                            |
| 2026-03       | rf_replace_p2      | trained      | 10     | 0.646052  | 0.026114                | 0.020622               | 10                    | 10                           |
| 2026-04       | baseline_original  | trained      | 4      | 1.000000  | 0.028506                | 0.023680               | 4                     | 4                            |
| 2026-04       | ctx4h_scaled025    | trained      | 4      | 0.437500  | 0.012083                | 0.010247               | 4                     | 4                            |
| 2026-04       | ctx12h_scaled025   | trained      | 4      | 0.625000  | 0.018237                | 0.015187               | 4                     | 4                            |
| 2026-04       | rf_prob_cap1       | trained      | 4      | 0.870481  | 0.024764                | 0.020398               | 4                     | 4                            |
| 2026-04       | rf_prob_floor025   | trained      | 4      | 0.902861  | 0.025699                | 0.021218               | 4                     | 4                            |
| 2026-04       | rf_binary_025      | trained      | 4      | 0.625000  | 0.017333                | 0.013865               | 4                     | 4                            |
| 2026-04       | rf_binary_000      | trained      | 4      | 0.500000  | 0.013609                | 0.010593               | 4                     | 2                            |
| 2026-04       | rf_rank_median_025 | trained      | 4      | 0.625000  | 0.017333                | 0.013865               | 4                     | 4                            |
| 2026-04       | rf_rank_median_000 | trained      | 4      | 0.500000  | 0.013609                | 0.010593               | 4                     | 2                            |
| 2026-04       | rf_rank_q60_000    | trained      | 4      | 0.250000  | 0.006527                | 0.005039               | 4                     | 1                            |
| 2026-04       | rf_rank_q70_000    | trained      | 4      | 0.000000  | 0.000000                | 0.000000               | 4                     | 0                            |
| 2026-04       | rf_replace_p2      | trained      | 4      | 1.036361  | 0.020207                | 0.016790               | 4                     | 4                            |

## Interpretation

- `baseline_original` keeps the event source's original RF sizing.
- `ctx*_scaled025` are fixed 4h/12h context overlays for comparison.
- `rf_*` variants train only on prior-window adverse10 labels, then size the next month.
- Promotion requires improvement on ETH late without breaking ETH early or BTC late.

## Diagnostics

```json
{
  "symbol": "ETHUSDT",
  "events_csv": "research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_ethusdt_train3m_events.csv",
  "candidate_rows_csv": "research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_ethusdt_train3m_min5_candidates.csv",
  "bars_cache_dir": "/Users/wuyaocheng/Downloads/bkTrader/research/probabilistic_v6_runs/walkforward_delay60_original_t2_feature60_valbest/bars_cache",
  "eval_start": "2025-06-01T00:00:00+00:00",
  "eval_end_exclusive": "2026-05-01T00:00:00+00:00",
  "train_months": 3,
  "min_train_events": 8,
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
      "forward_month": "2025-09",
      "train_start": "2025-06-01T00:00:00+00:00",
      "train_events_after_base_gate": 14,
      "forward_events_after_base_gate": 1,
      "train_events": 14,
      "model_status": "trained",
      "label_events": 14,
      "positive_labels": 3,
      "negative_labels": 11,
      "train_auc": 1.0,
      "train_prob_median": 0.21791325991065058,
      "train_prob_q40": 0.20206301338298416,
      "train_prob_q60": 0.23719980785854067,
      "train_prob_q70": 0.3131088911989886,
      "forward_prob_mean": 0.31875744935190425,
      "forward_prob_median": 0.31875744935190425,
      "feature_importance_top5": [
        [
          "pre_touch_seconds",
          0.1717288175299227
        ],
        [
          "ctx12h_range_atr",
          0.12267540241762537
        ],
        [
          "speed_300s_atr",
          0.08786435182842252
        ],
        [
          "prev1_close_pos_side",
          0.0768061244831678
        ],
        [
          "ctx4h_side_return_atr",
          0.07459836837400764
        ]
      ]
    },
    {
      "forward_month": "2025-10",
      "train_start": "2025-07-01T00:00:00+00:00",
      "train_events_after_base_gate": 11,
      "forward_events_after_base_gate": 2,
      "train_events": 11,
      "model_status": "trained",
      "label_events": 11,
      "positive_labels": 3,
      "negative_labels": 8,
      "train_auc": 1.0,
      "train_prob_median": 0.3218757679703901,
      "train_prob_q40": 0.28171037530020554,
      "train_prob_q60": 0.3227830156685419,
      "train_prob_q70": 0.3359338758841481,
      "forward_prob_mean": 0.27828583468860235,
      "forward_prob_median": 0.27828583468860235,
      "feature_importance_top5": [
        [
          "prev1_close_pos_side",
          0.13276122139969515
        ],
        [
          "pre_touch_seconds",
          0.1150794392730777
        ],
        [
          "touch_extension_atr",
          0.0893064051228216
        ],
        [
          "eff_300s",
          0.08569893851450529
        ],
        [
          "signal_atr_percentile",
          0.06938925972842146
        ]
      ]
    },
    {
      "forward_month": "2025-11",
      "train_start": "2025-08-01T00:00:00+00:00",
      "train_events_after_base_gate": 11,
      "forward_events_after_base_gate": 7,
      "train_events": 11,
      "model_status": "trained",
      "label_events": 11,
      "positive_labels": 4,
      "negative_labels": 7,
      "train_auc": 1.0,
      "train_prob_median": 0.21020694956075936,
      "train_prob_q40": 0.16133303917178157,
      "train_prob_q60": 0.32237561832465467,
      "train_prob_q70": 0.67696004438458,
      "forward_prob_mean": 0.3959152103627961,
      "forward_prob_median": 0.42751270332694497,
      "feature_importance_top5": [
        [
          "prev1_close_pos_side",
          0.16790671605486418
        ],
        [
          "pre_touch_seconds",
          0.15831788239195646
        ],
        [
          "level_to_signal_open_atr",
          0.1094692873312402
        ],
        [
          "ctx12h_range_atr",
          0.08836254156119476
        ],
        [
          "ctx4h_side_return_atr",
          0.07552371559105567
        ]
      ]
    },
    {
      "forward_month": "2025-12",
      "train_start": "2025-09-01T00:00:00+00:00",
      "train_events_after_base_gate": 11,
      "forward_events_after_base_gate": 9,
      "train_events": 11,
      "model_status": "trained",
      "label_events": 11,
      "positive_labels": 4,
      "negative_labels": 7,
      "train_auc": 1.0,
      "train_prob_median": 0.28753533354597194,
      "train_prob_q40": 0.2320335306275282,
      "train_prob_q60": 0.2947376995553465,
      "train_prob_q70": 0.5646597067615136,
      "forward_prob_mean": 0.38922821366575855,
      "forward_prob_median": 0.36775733532300464,
      "feature_importance_top5": [
        [
          "pre_touch_seconds",
          0.21146726970174917
        ],
        [
          "level_to_signal_open_atr",
          0.16289304495515594
        ],
        [
          "prev1_close_pos_side",
          0.11231023118200692
        ],
        [
          "touch_extension_atr",
          0.06791661275862658
        ],
        [
          "ctx4h_side_return_atr",
          0.05997249404578173
        ]
      ]
    },
    {
      "forward_month": "2026-01",
      "train_start": "2025-10-01T00:00:00+00:00",
      "train_events_after_base_gate": 16,
      "forward_events_after_base_gate": 5,
      "train_events": 16,
      "model_status": "trained",
      "label_events": 16,
      "positive_labels": 4,
      "negative_labels": 12,
      "train_auc": 1.0,
      "train_prob_median": 0.17268747821047983,
      "train_prob_q40": 0.1588330234017656,
      "train_prob_q60": 0.27552541101956196,
      "train_prob_q70": 0.3075152554372151,
      "forward_prob_mean": 0.2148226847075601,
      "forward_prob_median": 0.17384605886794155,
      "feature_importance_top5": [
        [
          "touch_extension_atr",
          0.17959204767629508
        ],
        [
          "pre_touch_seconds",
          0.12112600666942931
        ],
        [
          "eff_300s",
          0.09967405853409385
        ],
        [
          "ctx4h_side_return_atr",
          0.09832659337671222
        ],
        [
          "prev1_close_pos_side",
          0.0824328520552231
        ]
      ]
    },
    {
      "forward_month": "2026-02",
      "train_start": "2025-11-01T00:00:00+00:00",
      "train_events_after_base_gate": 19,
      "forward_events_after_base_gate": 4,
      "train_events": 19,
      "model_status": "trained",
      "label_events": 19,
      "positive_labels": 7,
      "negative_labels": 12,
      "train_auc": 1.0,
      "train_prob_median": 0.32769841285325535,
      "train_prob_q40": 0.25448842348798284,
      "train_prob_q60": 0.3907792935130062,
      "train_prob_q70": 0.6320780435983503,
      "forward_prob_mean": 0.3801935486719564,
      "forward_prob_median": 0.36296211942686474,
      "feature_importance_top5": [
        [
          "prev1_close_pos_side",
          0.14507983964398427
        ],
        [
          "ctx12h_range_atr",
          0.11769233161292159
        ],
        [
          "ctx4h_side_return_atr",
          0.0906207280691981
        ],
        [
          "pre_touch_seconds",
          0.08289528279097941
        ],
        [
          "ctx12h_side_return_atr",
          0.08242051774828786
        ]
      ]
    },
    {
      "forward_month": "2026-03",
      "train_start": "2025-12-01T00:00:00+00:00",
      "train_events_after_base_gate": 17,
      "forward_events_after_base_gate": 10,
      "train_events": 17,
      "model_status": "trained",
      "label_events": 17,
      "positive_labels": 6,
      "negative_labels": 11,
      "train_auc": 1.0,
      "train_prob_median": 0.20811422040667624,
      "train_prob_q40": 0.16436456739262081,
      "train_prob_q60": 0.43258903741915844,
      "train_prob_q70": 0.6452369039124671,
      "forward_prob_mean": 0.3230260449742791,
      "forward_prob_median": 0.2211875258439291,
      "feature_importance_top5": [
        [
          "ctx4h_side_return_atr",
          0.21933496381166329
        ],
        [
          "ctx12h_side_return_atr",
          0.16156467293507398
        ],
        [
          "prev1_close_pos_side",
          0.08470988710939548
        ],
        [
          "ctx12h_range_atr",
          0.08377490089144006
        ],
        [
          "pre_touch_seconds",
          0.07203510425631482
        ]
      ]
    },
    {
      "forward_month": "2026-04",
      "train_start": "2026-01-01T00:00:00+00:00",
      "train_events_after_base_gate": 15,
      "forward_events_after_base_gate": 4,
      "train_events": 15,
      "model_status": "trained",
      "label_events": 15,
      "positive_labels": 8,
      "negative_labels": 7,
      "train_auc": 1.0,
      "train_prob_median": 0.5178845455637503,
      "train_prob_q40": 0.3931165701839826,
      "train_prob_q60": 0.7176715544106601,
      "train_prob_q70": 0.7467329274639117,
      "forward_prob_mean": 0.5181804882132675,
      "forward_prob_median": 0.5002934402060987,
      "feature_importance_top5": [
        [
          "rf_probability",
          0.15940607259312067
        ],
        [
          "sizing_multiplier",
          0.12734876734440873
        ],
        [
          "eff_300s",
          0.10110102577468265
        ],
        [
          "ctx12h_side_return_atr",
          0.09197834218672021
        ],
        [
          "ctx4h_range_atr",
          0.07792365059452773
        ]
      ]
    }
  ],
  "runtime_seconds": 28.06800103187561
}
```
