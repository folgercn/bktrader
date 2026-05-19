# Breakout Structure Context Model Sizing

Generated: 2026-05-18T10:02:07.771580+00:00

Scope: research-only. This trains a trailing context-aware sizing overlay for `low_eff_low_atr` events.

## Aggregate

| variant            | forward_months | events | avg_scale | same_close_calendar_sum | same_close_worst_month | same_close_neg_months | adverse10_calendar_sum | adverse10_worst_month | adverse10_neg_months | adverse10_trade_count | adverse10_active_trade_count | trained_months |
| ------------------ | -------------- | ------ | --------- | ----------------------- | ---------------------- | --------------------- | ---------------------- | --------------------- | -------------------- | --------------------- | ---------------------------- | -------------- |
| rf_binary_000      | 8              | 385    | 0.121824  | -0.031869               | -0.014297              | 6                     | -0.078599              | -0.022265             | 8                    | 385                   | 46                           | 8              |
| ctx12h_scaled025   | 8              | 385    | 0.250000  | -0.046430               | -0.015295              | 7                     | -0.155750              | -0.032872             | 8                    | 385                   | 385                          | 8              |
| ctx4h_scaled025    | 8              | 385    | 0.250000  | -0.046430               | -0.015295              | 7                     | -0.155750              | -0.032872             | 8                    | 385                   | 385                          | 8              |
| rf_binary_025      | 8              | 385    | 0.341368  | -0.070332               | -0.017319              | 7                     | -0.214699              | -0.041439             | 8                    | 385                   | 385                          | 8              |
| rf_rank_q70_000    | 8              | 385    | 0.329896  | -0.085006               | -0.035148              | 7                     | -0.215687              | -0.062058             | 7                    | 385                   | 126                          | 8              |
| rf_rank_q60_000    | 8              | 385    | 0.498814  | -0.092205               | -0.038609              | 6                     | -0.300060              | -0.066958             | 7                    | 385                   | 190                          | 8              |
| rf_replace_p2      | 8              | 385    | 0.710084  | -0.089133               | -0.025465              | 8                     | -0.328291              | -0.059033             | 8                    | 385                   | 385                          | 8              |
| rf_rank_median_000 | 8              | 385    | 0.617175  | -0.114149               | -0.040254              | 5                     | -0.378643              | -0.077715             | 8                    | 385                   | 236                          | 8              |
| rf_prob_cap1       | 8              | 385    | 0.699332  | -0.119707               | -0.039575              | 6                     | -0.424002              | -0.085055             | 8                    | 385                   | 385                          | 8              |
| rf_rank_median_025 | 8              | 385    | 0.712881  | -0.132043               | -0.041888              | 6                     | -0.439732              | -0.089772             | 8                    | 385                   | 385                          | 8              |
| rf_prob_floor025   | 8              | 385    | 0.774499  | -0.136211               | -0.044976              | 6                     | -0.473751              | -0.096663             | 8                    | 385                   | 385                          | 8              |
| baseline_original  | 8              | 385    | 1.000000  | -0.185722               | -0.061181              | 7                     | -0.622999              | -0.131488             | 8                    | 385                   | 385                          | 8              |

## Monthly Rows

| forward_month | variant            | model_status | events | avg_scale | same_close_calendar_sum | adverse10_calendar_sum | adverse10_trade_count | adverse10_active_trade_count |
| ------------- | ------------------ | ------------ | ------ | --------- | ----------------------- | ---------------------- | --------------------- | ---------------------------- |
| 2025-09       | baseline_original  | trained      | 50     | 1.000000  | -0.043832               | -0.085054              | 50                    | 50                           |
| 2025-09       | ctx4h_scaled025    | trained      | 50     | 0.250000  | -0.010958               | -0.021263              | 50                    | 50                           |
| 2025-09       | ctx12h_scaled025   | trained      | 50     | 0.250000  | -0.010958               | -0.021263              | 50                    | 50                           |
| 2025-09       | rf_prob_cap1       | trained      | 50     | 0.584491  | -0.028216               | -0.051940              | 50                    | 50                           |
| 2025-09       | rf_prob_floor025   | trained      | 50     | 0.688368  | -0.032120               | -0.060219              | 50                    | 50                           |
| 2025-09       | rf_binary_025      | trained      | 50     | 0.280000  | -0.011427               | -0.022728              | 50                    | 50                           |
| 2025-09       | rf_binary_000      | trained      | 50     | 0.040000  | -0.000625               | -0.001953              | 50                    | 2                            |
| 2025-09       | rf_rank_median_025 | trained      | 50     | 0.805000  | -0.041148               | -0.074272              | 50                    | 50                           |
| 2025-09       | rf_rank_median_000 | trained      | 50     | 0.740000  | -0.040254               | -0.070678              | 50                    | 37                           |
| 2025-09       | rf_rank_q60_000    | trained      | 50     | 0.660000  | -0.038609               | -0.063825              | 50                    | 33                           |
| 2025-09       | rf_rank_q70_000    | trained      | 50     | 0.400000  | -0.021586               | -0.037584              | 50                    | 20                           |
| 2025-09       | rf_replace_p2      | trained      | 50     | 0.587641  | -0.023559               | -0.042807              | 50                    | 50                           |
| 2025-10       | baseline_original  | trained      | 43     | 1.000000  | -0.012654               | -0.057929              | 43                    | 43                           |
| 2025-10       | ctx4h_scaled025    | trained      | 43     | 0.250000  | -0.003164               | -0.014482              | 43                    | 43                           |
| 2025-10       | ctx12h_scaled025   | trained      | 43     | 0.250000  | -0.003164               | -0.014482              | 43                    | 43                           |
| 2025-10       | rf_prob_cap1       | trained      | 43     | 0.561270  | -0.006478               | -0.031390              | 43                    | 43                           |
| 2025-10       | rf_prob_floor025   | trained      | 43     | 0.670952  | -0.008022               | -0.038025              | 43                    | 43                           |
| 2025-10       | rf_binary_025      | trained      | 43     | 0.284884  | -0.004142               | -0.017112              | 43                    | 43                           |
| 2025-10       | rf_binary_000      | trained      | 43     | 0.046512  | -0.001304               | -0.003507              | 43                    | 2                            |
| 2025-10       | rf_rank_median_025 | trained      | 43     | 0.790698  | -0.013211               | -0.048459              | 43                    | 43                           |
| 2025-10       | rf_rank_median_000 | trained      | 43     | 0.720930  | -0.013396               | -0.045303              | 43                    | 31                           |
| 2025-10       | rf_rank_q60_000    | trained      | 43     | 0.604651  | -0.001457               | -0.027834              | 43                    | 26                           |
| 2025-10       | rf_rank_q70_000    | trained      | 43     | 0.488372  | -0.008789               | -0.029308              | 43                    | 21                           |
| 2025-10       | rf_replace_p2      | trained      | 43     | 0.563410  | -0.007783               | -0.028115              | 43                    | 43                           |
| 2025-11       | baseline_original  | trained      | 62     | 1.000000  | -0.061181               | -0.131488              | 62                    | 62                           |
| 2025-11       | ctx4h_scaled025    | trained      | 62     | 0.250000  | -0.015295               | -0.032872              | 62                    | 62                           |
| 2025-11       | ctx12h_scaled025   | trained      | 62     | 0.250000  | -0.015295               | -0.032872              | 62                    | 62                           |
| 2025-11       | rf_prob_cap1       | trained      | 62     | 0.643073  | -0.039575               | -0.085055              | 62                    | 62                           |
| 2025-11       | rf_prob_floor025   | trained      | 62     | 0.732305  | -0.044976               | -0.096663              | 62                    | 62                           |
| 2025-11       | rf_binary_025      | trained      | 62     | 0.358871  | -0.016456               | -0.041439              | 62                    | 62                           |
| 2025-11       | rf_binary_000      | trained      | 62     | 0.145161  | -0.001548               | -0.011423              | 62                    | 9                            |
| 2025-11       | rf_rank_median_025 | trained      | 62     | 0.685484  | -0.041888               | -0.089772              | 62                    | 62                           |
| 2025-11       | rf_rank_median_000 | trained      | 62     | 0.580645  | -0.035457               | -0.075867              | 62                    | 36                           |
| 2025-11       | rf_rank_q60_000    | trained      | 62     | 0.451613  | -0.034566               | -0.066958              | 62                    | 28                           |
| 2025-11       | rf_rank_q70_000    | trained      | 62     | 0.370968  | -0.035148               | -0.062058              | 62                    | 23                           |
| 2025-11       | rf_replace_p2      | trained      | 62     | 0.663166  | -0.019393               | -0.055642              | 62                    | 62                           |
| 2025-12       | baseline_original  | trained      | 40     | 1.000000  | -0.020065               | -0.063655              | 40                    | 40                           |
| 2025-12       | ctx4h_scaled025    | trained      | 40     | 0.250000  | -0.005016               | -0.015914              | 40                    | 40                           |
| 2025-12       | ctx12h_scaled025   | trained      | 40     | 0.250000  | -0.005016               | -0.015914              | 40                    | 40                           |
| 2025-12       | rf_prob_cap1       | trained      | 40     | 0.738384  | -0.008958               | -0.039859              | 40                    | 40                           |
| 2025-12       | rf_prob_floor025   | trained      | 40     | 0.803788  | -0.011735               | -0.045808              | 40                    | 40                           |
| 2025-12       | rf_binary_025      | trained      | 40     | 0.400000  | -0.004732               | -0.021189              | 40                    | 40                           |
| 2025-12       | rf_binary_000      | trained      | 40     | 0.200000  | 0.000378                | -0.007033              | 40                    | 8                            |
| 2025-12       | rf_rank_median_025 | trained      | 40     | 0.606250  | -0.004860               | -0.029804              | 40                    | 40                           |
| 2025-12       | rf_rank_median_000 | trained      | 40     | 0.475000  | 0.000208                | -0.018521              | 40                    | 19                           |
| 2025-12       | rf_rank_q60_000    | trained      | 40     | 0.375000  | 0.012966                | 0.000122               | 40                    | 15                           |
| 2025-12       | rf_rank_q70_000    | trained      | 40     | 0.250000  | 0.009257                | 0.000853               | 40                    | 10                           |
| 2025-12       | rf_replace_p2      | trained      | 40     | 0.758046  | -0.000387               | -0.024985              | 40                    | 40                           |
| 2026-01       | baseline_original  | trained      | 41     | 1.000000  | -0.025361               | -0.069635              | 41                    | 41                           |
| 2026-01       | ctx4h_scaled025    | trained      | 41     | 0.250000  | -0.006340               | -0.017409              | 41                    | 41                           |
| 2026-01       | ctx12h_scaled025   | trained      | 41     | 0.250000  | -0.006340               | -0.017409              | 41                    | 41                           |
| 2026-01       | rf_prob_cap1       | trained      | 41     | 0.819634  | -0.019233               | -0.054329              | 41                    | 41                           |
| 2026-01       | rf_prob_floor025   | trained      | 41     | 0.864726  | -0.020765               | -0.058156              | 41                    | 41                           |
| 2026-01       | rf_binary_025      | trained      | 41     | 0.414634  | -0.009695               | -0.025778              | 41                    | 41                           |
| 2026-01       | rf_binary_000      | trained      | 41     | 0.219512  | -0.004473               | -0.011159              | 41                    | 9                            |
| 2026-01       | rf_rank_median_025 | trained      | 41     | 0.743902  | -0.016402               | -0.047121              | 41                    | 41                           |
| 2026-01       | rf_rank_median_000 | trained      | 41     | 0.658537  | -0.013416               | -0.039617              | 41                    | 27                           |
| 2026-01       | rf_rank_q60_000    | trained      | 41     | 0.536585  | -0.006841               | -0.026537              | 41                    | 22                           |
| 2026-01       | rf_rank_q70_000    | trained      | 41     | 0.292683  | -0.008098               | -0.017979              | 41                    | 12                           |
| 2026-01       | rf_replace_p2      | trained      | 41     | 0.830791  | -0.010896               | -0.038568              | 41                    | 41                           |
| 2026-02       | baseline_original  | trained      | 43     | 1.000000  | -0.026386               | -0.082561              | 43                    | 43                           |
| 2026-02       | ctx4h_scaled025    | trained      | 43     | 0.250000  | -0.006597               | -0.020640              | 43                    | 43                           |
| 2026-02       | ctx12h_scaled025   | trained      | 43     | 0.250000  | -0.006597               | -0.020640              | 43                    | 43                           |
| 2026-02       | rf_prob_cap1       | trained      | 43     | 0.811846  | -0.021873               | -0.067657              | 43                    | 43                           |
| 2026-02       | rf_prob_floor025   | trained      | 43     | 0.858884  | -0.023001               | -0.071383              | 43                    | 43                           |
| 2026-02       | rf_binary_025      | trained      | 43     | 0.337209  | -0.017319               | -0.037339              | 43                    | 43                           |
| 2026-02       | rf_binary_000      | trained      | 43     | 0.116279  | -0.014297               | -0.022265              | 43                    | 5                            |
| 2026-02       | rf_rank_median_025 | trained      | 43     | 0.790698  | -0.034708               | -0.078927              | 43                    | 43                           |
| 2026-02       | rf_rank_median_000 | trained      | 43     | 0.720930  | -0.037482               | -0.077715              | 43                    | 31                           |
| 2026-02       | rf_rank_q60_000    | trained      | 43     | 0.627907  | -0.023153               | -0.058337              | 43                    | 27                           |
| 2026-02       | rf_rank_q70_000    | trained      | 43     | 0.418605  | -0.008649               | -0.033317              | 43                    | 18                           |
| 2026-02       | rf_replace_p2      | trained      | 43     | 0.817207  | -0.025465               | -0.059033              | 43                    | 43                           |
| 2026-03       | baseline_original  | trained      | 51     | 1.000000  | -0.001439               | -0.080415              | 51                    | 51                           |
| 2026-03       | ctx4h_scaled025    | trained      | 51     | 0.250000  | -0.000360               | -0.020104              | 51                    | 51                           |
| 2026-03       | ctx12h_scaled025   | trained      | 51     | 0.250000  | -0.000360               | -0.020104              | 51                    | 51                           |
| 2026-03       | rf_prob_cap1       | trained      | 51     | 0.702701  | 0.002110                | -0.053737              | 51                    | 51                           |
| 2026-03       | rf_prob_floor025   | trained      | 51     | 0.777025  | 0.001223                | -0.060406              | 51                    | 51                           |
| 2026-03       | rf_binary_025      | trained      | 51     | 0.323529  | -0.008522               | -0.032061              | 51                    | 51                           |
| 2026-03       | rf_binary_000      | trained      | 51     | 0.098039  | -0.010883               | -0.015943              | 51                    | 5                            |
| 2026-03       | rf_rank_median_025 | trained      | 51     | 0.676471  | 0.016891                | -0.039528              | 51                    | 51                           |
| 2026-03       | rf_rank_median_000 | trained      | 51     | 0.568627  | 0.023000                | -0.025899              | 51                    | 29                           |
| 2026-03       | rf_rank_q60_000    | trained      | 51     | 0.352941  | 0.002579                | -0.030249              | 51                    | 18                           |
| 2026-03       | rf_rank_q70_000    | trained      | 51     | 0.254902  | -0.010309               | -0.024320              | 51                    | 13                           |
| 2026-03       | rf_replace_p2      | trained      | 51     | 0.711046  | -0.001066               | -0.044712              | 51                    | 51                           |
| 2026-04       | baseline_original  | trained      | 55     | 1.000000  | 0.005196                | -0.052263              | 55                    | 55                           |
| 2026-04       | ctx4h_scaled025    | trained      | 55     | 0.250000  | 0.001299                | -0.013066              | 55                    | 55                           |
| 2026-04       | ctx12h_scaled025   | trained      | 55     | 0.250000  | 0.001299                | -0.013066              | 55                    | 55                           |
| 2026-04       | rf_prob_cap1       | trained      | 55     | 0.733257  | 0.002516                | -0.040035              | 55                    | 55                           |
| 2026-04       | rf_prob_floor025   | trained      | 55     | 0.799943  | 0.003186                | -0.043092              | 55                    | 55                           |
| 2026-04       | rf_binary_025      | trained      | 55     | 0.331818  | 0.001961                | -0.017053              | 55                    | 55                           |
| 2026-04       | rf_binary_000      | trained      | 55     | 0.109091  | 0.000882                | -0.005316              | 55                    | 6                            |
| 2026-04       | rf_rank_median_025 | trained      | 55     | 0.604545  | 0.003285                | -0.031849              | 55                    | 55                           |
| 2026-04       | rf_rank_median_000 | trained      | 55     | 0.472727  | 0.002648                | -0.025044              | 55                    | 26                           |
| 2026-04       | rf_rank_q60_000    | trained      | 55     | 0.381818  | -0.003124               | -0.026443              | 55                    | 21                           |
| 2026-04       | rf_rank_q70_000    | trained      | 55     | 0.163636  | -0.001685               | -0.011975              | 55                    | 9                            |
| 2026-04       | rf_replace_p2      | trained      | 55     | 0.749367  | -0.000585               | -0.034430              | 55                    | 55                           |

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
  "min_train_events": 30,
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
      "forward_month": "2025-09",
      "train_start": "2025-06-01T00:00:00+00:00",
      "train_events_after_base_gate": 112,
      "forward_events_after_base_gate": 50,
      "train_events": 112,
      "model_status": "trained",
      "label_events": 112,
      "positive_labels": 14,
      "negative_labels": 98,
      "train_auc": 0.9948979591836734,
      "train_prob_median": 0.21210865344133162,
      "train_prob_q40": 0.18069116724950232,
      "train_prob_q60": 0.24817996069910842,
      "train_prob_q70": 0.30982704136531947,
      "forward_prob_mean": 0.2938204632783834,
      "forward_prob_median": 0.29452342887347216,
      "feature_importance_top5": [
        [
          "touch_extension_atr",
          0.15210493232122035
        ],
        [
          "signal_atr_percentile",
          0.1051410878007417
        ],
        [
          "prev1_close_pos_side",
          0.10265469987898201
        ],
        [
          "ctx12h_range_atr",
          0.10082383020106371
        ],
        [
          "ctx4h_range_atr",
          0.07334049465884754
        ]
      ]
    },
    {
      "forward_month": "2025-10",
      "train_start": "2025-07-01T00:00:00+00:00",
      "train_events_after_base_gate": 124,
      "forward_events_after_base_gate": 43,
      "train_events": 124,
      "model_status": "trained",
      "label_events": 124,
      "positive_labels": 14,
      "negative_labels": 110,
      "train_auc": 0.9961038961038962,
      "train_prob_median": 0.22193461140733667,
      "train_prob_q40": 0.18236472030530074,
      "train_prob_q60": 0.250980467804088,
      "train_prob_q70": 0.27901484633798085,
      "forward_prob_mean": 0.2817047930743472,
      "forward_prob_median": 0.27568555370352066,
      "feature_importance_top5": [
        [
          "eff_300s",
          0.13790621775050887
        ],
        [
          "prev1_range_atr",
          0.11000027347090062
        ],
        [
          "prev1_close_pos_side",
          0.10465503106412337
        ],
        [
          "level_to_signal_open_atr",
          0.09373217640022329
        ],
        [
          "pre_touch_seconds",
          0.08892970251732209
        ]
      ]
    },
    {
      "forward_month": "2025-11",
      "train_start": "2025-08-01T00:00:00+00:00",
      "train_events_after_base_gate": 127,
      "forward_events_after_base_gate": 62,
      "train_events": 127,
      "model_status": "trained",
      "label_events": 127,
      "positive_labels": 20,
      "negative_labels": 107,
      "train_auc": 1.0,
      "train_prob_median": 0.27329760122041097,
      "train_prob_q40": 0.22718333135940674,
      "train_prob_q60": 0.3190550828904465,
      "train_prob_q70": 0.3653443520236142,
      "forward_prob_mean": 0.3315830935467921,
      "forward_prob_median": 0.30254118836327004,
      "feature_importance_top5": [
        [
          "eff_300s",
          0.13662646637671008
        ],
        [
          "signal_atr_percentile",
          0.12875189374116883
        ],
        [
          "level_to_signal_open_atr",
          0.11239002463447172
        ],
        [
          "ctx12h_range_atr",
          0.07670610913274803
        ],
        [
          "prev1_close_pos_side",
          0.0743230259910367
        ]
      ]
    },
    {
      "forward_month": "2025-12",
      "train_start": "2025-09-01T00:00:00+00:00",
      "train_events_after_base_gate": 155,
      "forward_events_after_base_gate": 40,
      "train_events": 155,
      "model_status": "trained",
      "label_events": 155,
      "positive_labels": 36,
      "negative_labels": 119,
      "train_auc": 0.9883286647992531,
      "train_prob_median": 0.3650289902829232,
      "train_prob_q40": 0.3372282273782608,
      "train_prob_q60": 0.40867535086229617,
      "train_prob_q70": 0.4578405618550142,
      "forward_prob_mean": 0.3790228162372019,
      "forward_prob_median": 0.3492921038719201,
      "feature_importance_top5": [
        [
          "ctx4h_range_atr",
          0.11934013662608249
        ],
        [
          "eff_300s",
          0.10640417502684452
        ],
        [
          "signal_atr_percentile",
          0.10186948467610225
        ],
        [
          "level_to_signal_open_atr",
          0.08744969036293876
        ],
        [
          "prev1_range_atr",
          0.07357455158052181
        ]
      ]
    },
    {
      "forward_month": "2026-01",
      "train_start": "2025-10-01T00:00:00+00:00",
      "train_events_after_base_gate": 145,
      "forward_events_after_base_gate": 41,
      "train_events": 145,
      "model_status": "trained",
      "label_events": 145,
      "positive_labels": 39,
      "negative_labels": 106,
      "train_auc": 0.9871794871794872,
      "train_prob_median": 0.37968058436316005,
      "train_prob_q40": 0.36038850663136196,
      "train_prob_q60": 0.4168600014561053,
      "train_prob_q70": 0.46664068751076193,
      "forward_prob_mean": 0.41539532658246214,
      "forward_prob_median": 0.4296809365924994,
      "feature_importance_top5": [
        [
          "ctx4h_range_atr",
          0.11403565633242728
        ],
        [
          "touch_extension_atr",
          0.08979067110336218
        ],
        [
          "level_to_signal_open_atr",
          0.08975456218976022
        ],
        [
          "ctx12h_side_return_atr",
          0.08796579332998869
        ],
        [
          "ctx12h_range_atr",
          0.08339422636043654
        ]
      ]
    },
    {
      "forward_month": "2026-02",
      "train_start": "2025-11-01T00:00:00+00:00",
      "train_events_after_base_gate": 143,
      "forward_events_after_base_gate": 43,
      "train_events": 143,
      "model_status": "trained",
      "label_events": 143,
      "positive_labels": 35,
      "negative_labels": 108,
      "train_auc": 0.9923280423280423,
      "train_prob_median": 0.36574755486991994,
      "train_prob_q40": 0.3428335104874649,
      "train_prob_q60": 0.3850492408038716,
      "train_prob_q70": 0.42417793532295667,
      "forward_prob_mean": 0.40860358166556227,
      "forward_prob_median": 0.40920523092727895,
      "feature_importance_top5": [
        [
          "ctx12h_range_atr",
          0.11502236912673247
        ],
        [
          "level_to_signal_open_atr",
          0.1130896024060803
        ],
        [
          "ctx4h_side_return_atr",
          0.10241360525561366
        ],
        [
          "pre_touch_seconds",
          0.08817458930095629
        ],
        [
          "speed_300s_atr",
          0.08246245095167769
        ]
      ]
    },
    {
      "forward_month": "2026-03",
      "train_start": "2025-12-01T00:00:00+00:00",
      "train_events_after_base_gate": 124,
      "forward_events_after_base_gate": 51,
      "train_events": 124,
      "model_status": "trained",
      "label_events": 124,
      "positive_labels": 29,
      "negative_labels": 95,
      "train_auc": 0.9894736842105263,
      "train_prob_median": 0.3341817474504665,
      "train_prob_q40": 0.2994522396380429,
      "train_prob_q60": 0.3864288232415482,
      "train_prob_q70": 0.43739894779303656,
      "forward_prob_mean": 0.35552321572997847,
      "forward_prob_median": 0.3578365255668243,
      "feature_importance_top5": [
        [
          "ctx4h_side_return_atr",
          0.1365005725842299
        ],
        [
          "speed_300s_atr",
          0.11749080133438594
        ],
        [
          "ctx12h_range_atr",
          0.10561505579679245
        ],
        [
          "prev1_close_pos_side",
          0.08036058109534089
        ],
        [
          "sizing_multiplier",
          0.06697943742471993
        ]
      ]
    },
    {
      "forward_month": "2026-04",
      "train_start": "2026-01-01T00:00:00+00:00",
      "train_events_after_base_gate": 135,
      "forward_events_after_base_gate": 55,
      "train_events": 135,
      "model_status": "trained",
      "label_events": 135,
      "positive_labels": 32,
      "negative_labels": 103,
      "train_auc": 0.9808859223300971,
      "train_prob_median": 0.3631668183939899,
      "train_prob_q40": 0.30942077310250493,
      "train_prob_q60": 0.40287156081870307,
      "train_prob_q70": 0.4874775562670623,
      "forward_prob_mean": 0.37468340556423313,
      "forward_prob_median": 0.35997531733616095,
      "feature_importance_top5": [
        [
          "ctx12h_range_atr",
          0.1315905560261169
        ],
        [
          "ctx4h_range_atr",
          0.1278555511272347
        ],
        [
          "touch_extension_atr",
          0.12355090946119676
        ],
        [
          "ctx12h_side_return_atr",
          0.07968754087060698
        ],
        [
          "pre_touch_seconds",
          0.07830028737141631
        ]
      ]
    }
  ],
  "runtime_seconds": 54.50771617889404
}
```
