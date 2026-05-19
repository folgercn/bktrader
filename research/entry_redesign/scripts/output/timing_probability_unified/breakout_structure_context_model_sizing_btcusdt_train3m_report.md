# Breakout Structure Context Model Sizing

Generated: 2026-05-18T09:59:53.798527+00:00

Scope: research-only. This trains a trailing context-aware sizing overlay for `low_eff_low_atr` events.

## Aggregate

| variant            | forward_months | events | avg_scale | same_close_calendar_sum | same_close_worst_month | same_close_neg_months | adverse10_calendar_sum | adverse10_worst_month | adverse10_neg_months | adverse10_trade_count | adverse10_active_trade_count | trained_months |
| ------------------ | -------------- | ------ | --------- | ----------------------- | ---------------------- | --------------------- | ---------------------- | --------------------- | -------------------- | --------------------- | ---------------------------- | -------------- |
| rf_binary_000      | 8              | 38     | 0.250000  | 0.015818                | -0.003908              | 1                     | -0.008731              | -0.009684             | 1                    | 38                    | 11                           | 6              |
| ctx12h_scaled025   | 8              | 38     | 0.250000  | -0.001853               | -0.003136              | 4                     | -0.014867              | -0.005055             | 6                    | 38                    | 38                           | 6              |
| ctx4h_scaled025    | 8              | 38     | 0.250000  | -0.001853               | -0.003136              | 4                     | -0.014867              | -0.005055             | 6                    | 38                    | 38                           | 6              |
| rf_binary_025      | 8              | 38     | 0.437500  | 0.010011                | -0.003908              | 4                     | -0.021416              | -0.009684             | 6                    | 38                    | 38                           | 6              |
| rf_replace_p2      | 8              | 38     | 0.602986  | -0.002964               | -0.009247              | 4                     | -0.034631              | -0.012987             | 7                    | 38                    | 38                           | 6              |
| rf_prob_cap1       | 8              | 38     | 0.602986  | -0.000959               | -0.009574              | 4                     | -0.038415              | -0.014358             | 7                    | 38                    | 38                           | 6              |
| rf_prob_floor025   | 8              | 38     | 0.702239  | -0.002572               | -0.009540              | 4                     | -0.043678              | -0.015321             | 6                    | 38                    | 38                           | 6              |
| rf_rank_median_000 | 8              | 38     | 0.831845  | -0.003861               | -0.013574              | 3                     | -0.045705              | -0.018415             | 7                    | 38                    | 30                           | 6              |
| rf_rank_q60_000    | 8              | 38     | 0.831845  | -0.003861               | -0.013574              | 3                     | -0.045705              | -0.018415             | 7                    | 38                    | 30                           | 6              |
| rf_rank_q70_000    | 8              | 38     | 0.696429  | -0.008387               | -0.013574              | 4                     | -0.048551              | -0.018415             | 7                    | 38                    | 26                           | 6              |
| rf_rank_median_025 | 8              | 38     | 0.873884  | -0.004749               | -0.012540              | 4                     | -0.049146              | -0.018364             | 6                    | 38                    | 38                           | 6              |
| baseline_original  | 8              | 38     | 1.000000  | -0.007412               | -0.012545              | 4                     | -0.059469              | -0.020220             | 6                    | 38                    | 38                           | 6              |

## Monthly Rows

| forward_month | variant            | model_status | events | avg_scale | same_close_calendar_sum | adverse10_calendar_sum | adverse10_trade_count | adverse10_active_trade_count |
| ------------- | ------------------ | ------------ | ------ | --------- | ----------------------- | ---------------------- | --------------------- | ---------------------------- |
| 2025-09       | baseline_original  | trained      | 4      | 1.000000  | 0.000200                | -0.000800              | 4                     | 4                            |
| 2025-09       | ctx4h_scaled025    | trained      | 4      | 0.250000  | 0.000050                | -0.000200              | 4                     | 4                            |
| 2025-09       | ctx12h_scaled025   | trained      | 4      | 0.250000  | 0.000050                | -0.000200              | 4                     | 4                            |
| 2025-09       | rf_prob_cap1       | trained      | 4      | 0.374112  | 0.000063                | -0.000252              | 4                     | 4                            |
| 2025-09       | rf_prob_floor025   | trained      | 4      | 0.530584  | 0.000097                | -0.000389              | 4                     | 4                            |
| 2025-09       | rf_binary_025      | trained      | 4      | 0.250000  | 0.000050                | -0.000200              | 4                     | 4                            |
| 2025-09       | rf_binary_000      | trained      | 4      | 0.000000  | 0.000000                | 0.000000               | 4                     | 0                            |
| 2025-09       | rf_rank_median_025 | trained      | 4      | 1.000000  | 0.000200                | -0.000800              | 4                     | 4                            |
| 2025-09       | rf_rank_median_000 | trained      | 4      | 1.000000  | 0.000200                | -0.000800              | 4                     | 4                            |
| 2025-09       | rf_rank_q60_000    | trained      | 4      | 1.000000  | 0.000200                | -0.000800              | 4                     | 4                            |
| 2025-09       | rf_rank_q70_000    | trained      | 4      | 0.750000  | 0.000200                | -0.000800              | 4                     | 3                            |
| 2025-09       | rf_replace_p2      | trained      | 4      | 0.374112  | 0.000050                | -0.000201              | 4                     | 4                            |
| 2025-10       | baseline_original  | trained      | 3      | 1.000000  | -0.008626               | -0.011369              | 3                     | 3                            |
| 2025-10       | ctx4h_scaled025    | trained      | 3      | 0.250000  | -0.002157               | -0.002842              | 3                     | 3                            |
| 2025-10       | ctx12h_scaled025   | trained      | 3      | 0.250000  | -0.002157               | -0.002842              | 3                     | 3                            |
| 2025-10       | rf_prob_cap1       | trained      | 3      | 0.620513  | -0.005770               | -0.007486              | 3                     | 3                            |
| 2025-10       | rf_prob_floor025   | trained      | 3      | 0.715385  | -0.006484               | -0.008457              | 3                     | 3                            |
| 2025-10       | rf_binary_025      | trained      | 3      | 0.250000  | -0.002157               | -0.002842              | 3                     | 3                            |
| 2025-10       | rf_binary_000      | trained      | 3      | 0.000000  | 0.000000                | 0.000000               | 3                     | 0                            |
| 2025-10       | rf_rank_median_025 | trained      | 3      | 1.000000  | -0.008626               | -0.011369              | 3                     | 3                            |
| 2025-10       | rf_rank_median_000 | trained      | 3      | 1.000000  | -0.008626               | -0.011369              | 3                     | 3                            |
| 2025-10       | rf_rank_q60_000    | trained      | 3      | 1.000000  | -0.008626               | -0.011369              | 3                     | 3                            |
| 2025-10       | rf_rank_q70_000    | trained      | 3      | 0.666667  | -0.007745               | -0.009648              | 3                     | 2                            |
| 2025-10       | rf_replace_p2      | trained      | 3      | 0.620513  | -0.005173               | -0.006742              | 3                     | 3                            |
| 2025-11       | baseline_original  | single_class | 6      | 1.000000  | -0.003908               | -0.009684              | 6                     | 6                            |
| 2025-11       | ctx4h_scaled025    | single_class | 6      | 0.250000  | -0.000977               | -0.002421              | 6                     | 6                            |
| 2025-11       | ctx12h_scaled025   | single_class | 6      | 0.250000  | -0.000977               | -0.002421              | 6                     | 6                            |
| 2025-11       | rf_prob_cap1       | single_class | 6      | 1.000000  | -0.003908               | -0.009684              | 6                     | 6                            |
| 2025-11       | rf_prob_floor025   | single_class | 6      | 1.000000  | -0.003908               | -0.009684              | 6                     | 6                            |
| 2025-11       | rf_binary_025      | single_class | 6      | 1.000000  | -0.003908               | -0.009684              | 6                     | 6                            |
| 2025-11       | rf_binary_000      | single_class | 6      | 1.000000  | -0.003908               | -0.009684              | 6                     | 6                            |
| 2025-11       | rf_rank_median_025 | single_class | 6      | 1.000000  | -0.003908               | -0.009684              | 6                     | 6                            |
| 2025-11       | rf_rank_median_000 | single_class | 6      | 1.000000  | -0.003908               | -0.009684              | 6                     | 6                            |
| 2025-11       | rf_rank_q60_000    | single_class | 6      | 1.000000  | -0.003908               | -0.009684              | 6                     | 6                            |
| 2025-11       | rf_rank_q70_000    | single_class | 6      | 1.000000  | -0.003908               | -0.009684              | 6                     | 6                            |
| 2025-11       | rf_replace_p2      | single_class | 6      | 1.000000  | -0.004189               | -0.009204              | 6                     | 6                            |
| 2025-12       | baseline_original  | trained      | 3      | 1.000000  | 0.000716                | -0.003797              | 3                     | 3                            |
| 2025-12       | ctx4h_scaled025    | trained      | 3      | 0.250000  | 0.000179                | -0.000949              | 3                     | 3                            |
| 2025-12       | ctx12h_scaled025   | trained      | 3      | 0.250000  | 0.000179                | -0.000949              | 3                     | 3                            |
| 2025-12       | rf_prob_cap1       | trained      | 3      | 0.541976  | 0.000382                | -0.002088              | 3                     | 3                            |
| 2025-12       | rf_prob_floor025   | trained      | 3      | 0.656482  | 0.000465                | -0.002515              | 3                     | 3                            |
| 2025-12       | rf_binary_025      | trained      | 3      | 0.250000  | 0.000179                | -0.000949              | 3                     | 3                            |
| 2025-12       | rf_binary_000      | trained      | 3      | 0.000000  | 0.000000                | 0.000000               | 3                     | 0                            |
| 2025-12       | rf_rank_median_025 | trained      | 3      | 1.000000  | 0.000716                | -0.003797              | 3                     | 3                            |
| 2025-12       | rf_rank_median_000 | trained      | 3      | 1.000000  | 0.000716                | -0.003797              | 3                     | 3                            |
| 2025-12       | rf_rank_q60_000    | trained      | 3      | 1.000000  | 0.000716                | -0.003797              | 3                     | 3                            |
| 2025-12       | rf_rank_q70_000    | trained      | 3      | 1.000000  | 0.000716                | -0.003797              | 3                     | 3                            |
| 2025-12       | rf_replace_p2      | trained      | 3      | 0.541976  | 0.000260                | -0.001495              | 3                     | 3                            |
| 2026-01       | baseline_original  | trained      | 6      | 1.000000  | -0.012545               | -0.020220              | 6                     | 6                            |
| 2026-01       | ctx4h_scaled025    | trained      | 6      | 0.250000  | -0.003136               | -0.005055              | 6                     | 6                            |
| 2026-01       | ctx12h_scaled025   | trained      | 6      | 0.250000  | -0.003136               | -0.005055              | 6                     | 6                            |
| 2026-01       | rf_prob_cap1       | trained      | 6      | 0.339774  | -0.002546               | -0.005103              | 6                     | 6                            |
| 2026-01       | rf_prob_floor025   | trained      | 6      | 0.504831  | -0.005046               | -0.008882              | 6                     | 6                            |
| 2026-01       | rf_binary_025      | trained      | 6      | 0.250000  | -0.003136               | -0.005055              | 6                     | 6                            |
| 2026-01       | rf_binary_000      | trained      | 6      | 0.000000  | 0.000000                | 0.000000               | 6                     | 0                            |
| 2026-01       | rf_rank_median_025 | trained      | 6      | 0.500000  | -0.002883               | -0.006621              | 6                     | 6                            |
| 2026-01       | rf_rank_median_000 | trained      | 6      | 0.333333  | 0.000338                | -0.002089              | 6                     | 2                            |
| 2026-01       | rf_rank_q60_000    | trained      | 6      | 0.333333  | 0.000338                | -0.002089              | 6                     | 2                            |
| 2026-01       | rf_rank_q70_000    | trained      | 6      | 0.333333  | 0.000338                | -0.002089              | 6                     | 2                            |
| 2026-01       | rf_replace_p2      | trained      | 6      | 0.339774  | -0.001446               | -0.003544              | 6                     | 6                            |
| 2026-02       | baseline_original  | trained      | 7      | 1.000000  | -0.009437               | -0.018210              | 7                     | 7                            |
| 2026-02       | ctx4h_scaled025    | trained      | 7      | 0.250000  | -0.002359               | -0.004553              | 7                     | 7                            |
| 2026-02       | ctx12h_scaled025   | trained      | 7      | 0.250000  | -0.002359               | -0.004553              | 7                     | 7                            |
| 2026-02       | rf_prob_cap1       | trained      | 7      | 0.569425  | -0.009574               | -0.014358              | 7                     | 7                            |
| 2026-02       | rf_prob_floor025   | trained      | 7      | 0.677069  | -0.009540               | -0.015321              | 7                     | 7                            |
| 2026-02       | rf_binary_025      | trained      | 7      | 0.250000  | -0.002359               | -0.004553              | 7                     | 7                            |
| 2026-02       | rf_binary_000      | trained      | 7      | 0.000000  | 0.000000                | 0.000000               | 7                     | 0                            |
| 2026-02       | rf_rank_median_025 | trained      | 7      | 0.678571  | -0.012540               | -0.018364              | 7                     | 7                            |
| 2026-02       | rf_rank_median_000 | trained      | 7      | 0.571429  | -0.013574               | -0.018415              | 7                     | 4                            |
| 2026-02       | rf_rank_q60_000    | trained      | 7      | 0.571429  | -0.013574               | -0.018415              | 7                     | 4                            |
| 2026-02       | rf_rank_q70_000    | trained      | 7      | 0.571429  | -0.013574               | -0.018415              | 7                     | 4                            |
| 2026-02       | rf_replace_p2      | trained      | 7      | 0.569425  | -0.009247               | -0.012987              | 7                     | 7                            |
| 2026-03       | baseline_original  | single_class | 5      | 1.000000  | 0.019726                | 0.000953               | 5                     | 5                            |
| 2026-03       | ctx4h_scaled025    | single_class | 5      | 0.250000  | 0.004932                | 0.000238               | 5                     | 5                            |
| 2026-03       | ctx12h_scaled025   | single_class | 5      | 0.250000  | 0.004932                | 0.000238               | 5                     | 5                            |
| 2026-03       | rf_prob_cap1       | single_class | 5      | 1.000000  | 0.019726                | 0.000953               | 5                     | 5                            |
| 2026-03       | rf_prob_floor025   | single_class | 5      | 1.000000  | 0.019726                | 0.000953               | 5                     | 5                            |
| 2026-03       | rf_binary_025      | single_class | 5      | 1.000000  | 0.019726                | 0.000953               | 5                     | 5                            |
| 2026-03       | rf_binary_000      | single_class | 5      | 1.000000  | 0.019726                | 0.000953               | 5                     | 5                            |
| 2026-03       | rf_rank_median_025 | single_class | 5      | 1.000000  | 0.019726                | 0.000953               | 5                     | 5                            |
| 2026-03       | rf_rank_median_000 | single_class | 5      | 1.000000  | 0.019726                | 0.000953               | 5                     | 5                            |
| 2026-03       | rf_rank_q60_000    | single_class | 5      | 1.000000  | 0.019726                | 0.000953               | 5                     | 5                            |
| 2026-03       | rf_rank_q70_000    | single_class | 5      | 1.000000  | 0.019726                | 0.000953               | 5                     | 5                            |
| 2026-03       | rf_replace_p2      | single_class | 5      | 1.000000  | 0.016333                | 0.000072               | 5                     | 5                            |
| 2026-04       | baseline_original  | trained      | 4      | 1.000000  | 0.006463                | 0.003659               | 4                     | 4                            |
| 2026-04       | ctx4h_scaled025    | trained      | 4      | 0.250000  | 0.001616                | 0.000915               | 4                     | 4                            |
| 2026-04       | ctx12h_scaled025   | trained      | 4      | 0.250000  | 0.001616                | 0.000915               | 4                     | 4                            |
| 2026-04       | rf_prob_cap1       | trained      | 4      | 0.378083  | 0.000668                | -0.000397              | 4                     | 4                            |
| 2026-04       | rf_prob_floor025   | trained      | 4      | 0.533563  | 0.002117                | 0.000617               | 4                     | 4                            |
| 2026-04       | rf_binary_025      | trained      | 4      | 0.250000  | 0.001616                | 0.000915               | 4                     | 4                            |
| 2026-04       | rf_binary_000      | trained      | 4      | 0.000000  | 0.000000                | 0.000000               | 4                     | 0                            |
| 2026-04       | rf_rank_median_025 | trained      | 4      | 0.812500  | 0.002566                | 0.000537               | 4                     | 4                            |
| 2026-04       | rf_rank_median_000 | trained      | 4      | 0.750000  | 0.001268                | -0.000504              | 4                     | 3                            |
| 2026-04       | rf_rank_q60_000    | trained      | 4      | 0.750000  | 0.001268                | -0.000504              | 4                     | 3                            |
| 2026-04       | rf_rank_q70_000    | trained      | 4      | 0.250000  | -0.004140               | -0.005072              | 4                     | 1                            |
| 2026-04       | rf_replace_p2      | trained      | 4      | 0.378083  | 0.000448                | -0.000530              | 4                     | 4                            |

## Interpretation

- `baseline_original` keeps the event source's original RF sizing.
- `ctx*_scaled025` are fixed 4h/12h context overlays for comparison.
- `rf_*` variants train only on prior-window adverse10 labels, then size the next month.
- Promotion requires improvement on ETH late without breaking ETH early or BTC late.

## Diagnostics

```json
{
  "symbol": "BTCUSDT",
  "events_csv": "research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_btcusdt_train3m_events.csv",
  "candidate_rows_csv": "research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_btcusdt_train3m_min5_candidates.csv",
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
      "train_events_after_base_gate": 11,
      "forward_events_after_base_gate": 4,
      "train_events": 11,
      "model_status": "trained",
      "label_events": 11,
      "positive_labels": 1,
      "negative_labels": 10,
      "train_auc": 1.0,
      "train_prob_median": 0.1141807498057498,
      "train_prob_q40": 0.10324586524586525,
      "train_prob_q60": 0.11848611111111113,
      "train_prob_q70": 0.1315007215007215,
      "forward_prob_mean": 0.18705606199356195,
      "forward_prob_median": 0.1535474456099456,
      "feature_importance_top5": [
        [
          "speed_300s_atr",
          0.16393442622950816
        ],
        [
          "ctx4h_range_atr",
          0.15573770491803277
        ],
        [
          "prev1_range_atr",
          0.1311475409836065
        ],
        [
          "level_to_signal_open_atr",
          0.1311475409836065
        ],
        [
          "ctx12h_range_atr",
          0.10655737704918032
        ]
      ]
    },
    {
      "forward_month": "2025-10",
      "train_start": "2025-07-01T00:00:00+00:00",
      "train_events_after_base_gate": 8,
      "forward_events_after_base_gate": 3,
      "train_events": 8,
      "model_status": "trained",
      "label_events": 8,
      "positive_labels": 1,
      "negative_labels": 7,
      "train_auc": 1.0,
      "train_prob_median": 0.17798115079365082,
      "train_prob_q40": 0.15378571428571433,
      "train_prob_q60": 0.21338492063492065,
      "train_prob_q70": 0.267141865079365,
      "forward_prob_mean": 0.31025661375661373,
      "forward_prob_median": 0.2961289682539682,
      "feature_importance_top5": [
        [
          "prev1_close_pos_side",
          0.13178294573643412
        ],
        [
          "ctx12h_range_atr",
          0.12403100775193798
        ],
        [
          "speed_300s_atr",
          0.10852713178294573
        ],
        [
          "ctx12h_side_return_atr",
          0.10077519379844961
        ],
        [
          "eff_300s",
          0.07751937984496124
        ]
      ]
    },
    {
      "forward_month": "2025-11",
      "train_start": "2025-08-01T00:00:00+00:00",
      "train_events_after_base_gate": 11,
      "forward_events_after_base_gate": 6,
      "train_events": 11,
      "model_status": "single_class",
      "label_events": 11,
      "positive_labels": 0,
      "negative_labels": 11
    },
    {
      "forward_month": "2025-12",
      "train_start": "2025-09-01T00:00:00+00:00",
      "train_events_after_base_gate": 12,
      "forward_events_after_base_gate": 3,
      "train_events": 12,
      "model_status": "trained",
      "label_events": 12,
      "positive_labels": 1,
      "negative_labels": 11,
      "train_auc": 1.0,
      "train_prob_median": 0.0873676878676879,
      "train_prob_q40": 0.08054817404817408,
      "train_prob_q60": 0.10226435786435789,
      "train_prob_q70": 0.14724061077811082,
      "forward_prob_mean": 0.2709880674880673,
      "forward_prob_median": 0.27970988733488716,
      "feature_importance_top5": [
        [
          "ctx12h_side_return_atr",
          0.1450381679389313
        ],
        [
          "ctx12h_range_atr",
          0.13740458015267176
        ],
        [
          "prev1_range_atr",
          0.10687022900763359
        ],
        [
          "pre_touch_seconds",
          0.10687022900763359
        ],
        [
          "rf_probability",
          0.08396946564885495
        ]
      ]
    },
    {
      "forward_month": "2026-01",
      "train_start": "2025-10-01T00:00:00+00:00",
      "train_events_after_base_gate": 14,
      "forward_events_after_base_gate": 6,
      "train_events": 14,
      "model_status": "trained",
      "label_events": 14,
      "positive_labels": 2,
      "negative_labels": 12,
      "train_auc": 1.0,
      "train_prob_median": 0.13078191808191808,
      "train_prob_q40": 0.1202309657009657,
      "train_prob_q60": 0.13975959824489237,
      "train_prob_q70": 0.17613868912133615,
      "forward_prob_mean": 0.1698872255018354,
      "forward_prob_median": 0.11579320857637673,
      "feature_importance_top5": [
        [
          "ctx12h_side_return_atr",
          0.23512425685905247
        ],
        [
          "touch_extension_atr",
          0.18181122079427162
        ],
        [
          "prev1_close_pos_side",
          0.08543386946824173
        ],
        [
          "level_to_signal_open_atr",
          0.05749605795763618
        ],
        [
          "pre_touch_seconds",
          0.052909619682730294
        ]
      ]
    },
    {
      "forward_month": "2026-02",
      "train_start": "2025-11-01T00:00:00+00:00",
      "train_events_after_base_gate": 12,
      "forward_events_after_base_gate": 7,
      "train_events": 12,
      "model_status": "trained",
      "label_events": 12,
      "positive_labels": 2,
      "negative_labels": 10,
      "train_auc": 1.0,
      "train_prob_median": 0.16305287767787768,
      "train_prob_q40": 0.14619421411921416,
      "train_prob_q60": 0.21261487956487946,
      "train_prob_q70": 0.2786565920190918,
      "forward_prob_mean": 0.28471254340897184,
      "forward_prob_median": 0.33491350316350293,
      "feature_importance_top5": [
        [
          "ctx12h_side_return_atr",
          0.21156291815632472
        ],
        [
          "prev1_close_pos_side",
          0.16810112963959117
        ],
        [
          "level_to_signal_open_atr",
          0.07987885130742273
        ],
        [
          "touch_extension_atr",
          0.0750639988826802
        ],
        [
          "ctx12h_range_atr",
          0.06734748767715802
        ]
      ]
    },
    {
      "forward_month": "2026-03",
      "train_start": "2025-12-01T00:00:00+00:00",
      "train_events_after_base_gate": 10,
      "forward_events_after_base_gate": 5,
      "train_events": 10,
      "model_status": "single_class",
      "label_events": 10,
      "positive_labels": 0,
      "negative_labels": 10
    },
    {
      "forward_month": "2026-04",
      "train_start": "2026-01-01T00:00:00+00:00",
      "train_events_after_base_gate": 11,
      "forward_events_after_base_gate": 4,
      "train_events": 11,
      "model_status": "trained",
      "label_events": 11,
      "positive_labels": 1,
      "negative_labels": 10,
      "train_auc": 1.0,
      "train_prob_median": 0.1483611111111111,
      "train_prob_q40": 0.13650835275835274,
      "train_prob_q60": 0.14971037296037293,
      "train_prob_q70": 0.19853535353535348,
      "forward_prob_mean": 0.1890417152292152,
      "forward_prob_median": 0.17426641414141414,
      "feature_importance_top5": [
        [
          "pre_touch_seconds",
          0.17389770723104062
        ],
        [
          "prev1_close_pos_side",
          0.14074074074074075
        ],
        [
          "touch_extension_atr",
          0.1259259259259259
        ],
        [
          "level_to_signal_open_atr",
          0.09907407407407406
        ],
        [
          "speed_300s_atr",
          0.09882874327318769
        ]
      ]
    }
  ],
  "runtime_seconds": 26.048260927200317
}
```
