# Breakout Structure Context Model Sizing

Generated: 2026-05-18T10:00:53.418322+00:00

Scope: research-only. This trains a trailing context-aware sizing overlay for `low_eff_low_atr` events.

## Aggregate

| variant            | forward_months | events | avg_scale | same_close_calendar_sum | same_close_worst_month | same_close_neg_months | adverse10_calendar_sum | adverse10_worst_month | adverse10_neg_months | adverse10_trade_count | adverse10_active_trade_count | trained_months |
| ------------------ | -------------- | ------ | --------- | ----------------------- | ---------------------- | --------------------- | ---------------------- | --------------------- | -------------------- | --------------------- | ---------------------------- | -------------- |
| rf_binary_000      | 8              | 395    | 0.155065  | 0.156504                | -0.028279              | 1                     | 0.079500               | -0.042275             | 3                    | 395                   | 60                           | 8              |
| rf_rank_q70_000    | 8              | 395    | 0.260119  | 0.148023                | -0.026200              | 3                     | 0.020325               | -0.044589             | 4                    | 395                   | 103                          | 8              |
| rf_rank_q60_000    | 8              | 395    | 0.428663  | 0.190844                | -0.032536              | 1                     | -0.020238              | -0.061856             | 4                    | 395                   | 170                          | 8              |
| rf_binary_025      | 8              | 395    | 0.366299  | 0.113970                | -0.037574              | 3                     | -0.073111              | -0.063290             | 4                    | 395                   | 395                          | 8              |
| rf_rank_median_000 | 8              | 395    | 0.569192  | 0.095858                | -0.063482              | 3                     | -0.181713              | -0.101646             | 6                    | 395                   | 225                          | 8              |
| ctx4h_scaled025    | 8              | 395    | 0.529564  | 0.042847                | -0.043117              | 4                     | -0.235017              | -0.067374             | 6                    | 395                   | 395                          | 8              |
| ctx12h_scaled025   | 8              | 395    | 0.544274  | 0.031853                | -0.066101              | 4                     | -0.250543              | -0.096396             | 6                    | 395                   | 395                          | 8              |
| rf_replace_p2      | 8              | 395    | 0.804567  | 0.049920                | -0.046937              | 4                     | -0.259233              | -0.084432             | 6                    | 395                   | 395                          | 8              |
| rf_rank_median_025 | 8              | 395    | 0.676894  | 0.068486                | -0.063976              | 3                     | -0.269021              | -0.107818             | 6                    | 395                   | 395                          | 8              |
| rf_prob_cap1       | 8              | 395    | 0.786467  | 0.049581                | -0.055535              | 4                     | -0.351907              | -0.103644             | 7                    | 395                   | 395                          | 8              |
| rf_prob_floor025   | 8              | 395    | 0.839850  | 0.033778                | -0.060716              | 4                     | -0.396666              | -0.109317             | 7                    | 395                   | 395                          | 8              |
| baseline_original  | 8              | 395    | 1.000000  | -0.013631               | -0.080559              | 4                     | -0.530945              | -0.137240             | 7                    | 395                   | 395                          | 8              |

## Monthly Rows

| forward_month | variant            | model_status | events | avg_scale | same_close_calendar_sum | adverse10_calendar_sum | adverse10_trade_count | adverse10_active_trade_count |
| ------------- | ------------------ | ------------ | ------ | --------- | ----------------------- | ---------------------- | --------------------- | ---------------------------- |
| 2025-09       | baseline_original  | trained      | 50     | 1.000000  | -0.030508               | -0.087162              | 50                    | 50                           |
| 2025-09       | ctx4h_scaled025    | trained      | 50     | 0.490000  | 0.002886                | -0.024142              | 50                    | 50                           |
| 2025-09       | ctx12h_scaled025   | trained      | 50     | 0.520000  | 0.005197                | -0.023210              | 50                    | 50                           |
| 2025-09       | rf_prob_cap1       | trained      | 50     | 0.772864  | -0.017604               | -0.060422              | 50                    | 50                           |
| 2025-09       | rf_prob_floor025   | trained      | 50     | 0.829648  | -0.020830               | -0.067107              | 50                    | 50                           |
| 2025-09       | rf_binary_025      | trained      | 50     | 0.310000  | 0.000700                | -0.016271              | 50                    | 50                           |
| 2025-09       | rf_binary_000      | trained      | 50     | 0.080000  | 0.011102                | 0.007359               | 50                    | 4                            |
| 2025-09       | rf_rank_median_025 | trained      | 50     | 0.745000  | -0.015157               | -0.055245              | 50                    | 50                           |
| 2025-09       | rf_rank_median_000 | trained      | 50     | 0.660000  | -0.010040               | -0.044606              | 50                    | 33                           |
| 2025-09       | rf_rank_q60_000    | trained      | 50     | 0.520000  | 0.003051                | -0.023352              | 50                    | 26                           |
| 2025-09       | rf_rank_q70_000    | trained      | 50     | 0.320000  | -0.001092               | -0.017712              | 50                    | 16                           |
| 2025-09       | rf_replace_p2      | trained      | 50     | 0.779614  | -0.006826               | -0.041340              | 50                    | 50                           |
| 2025-10       | baseline_original  | trained      | 47     | 1.000000  | -0.080559               | -0.137240              | 47                    | 47                           |
| 2025-10       | ctx4h_scaled025    | trained      | 47     | 0.409574  | -0.043117               | -0.067374              | 47                    | 47                           |
| 2025-10       | ctx12h_scaled025   | trained      | 47     | 0.505319  | -0.066101               | -0.096396              | 47                    | 47                           |
| 2025-10       | rf_prob_cap1       | trained      | 47     | 0.748255  | -0.054102               | -0.095557              | 47                    | 47                           |
| 2025-10       | rf_prob_floor025   | trained      | 47     | 0.811191  | -0.060716               | -0.105978              | 47                    | 47                           |
| 2025-10       | rf_binary_025      | trained      | 47     | 0.265957  | -0.020014               | -0.034812              | 47                    | 47                           |
| 2025-10       | rf_binary_000      | trained      | 47     | 0.021277  | 0.000168                | -0.000670              | 47                    | 1                            |
| 2025-10       | rf_rank_median_025 | trained      | 47     | 0.680851  | -0.035874               | -0.072362              | 47                    | 47                           |
| 2025-10       | rf_rank_median_000 | trained      | 47     | 0.574468  | -0.020979               | -0.050736              | 47                    | 27                           |
| 2025-10       | rf_rank_q60_000    | trained      | 47     | 0.404255  | 0.002036                | -0.017817              | 47                    | 19                           |
| 2025-10       | rf_rank_q70_000    | trained      | 47     | 0.191489  | -0.002187               | -0.010869              | 47                    | 9                            |
| 2025-10       | rf_replace_p2      | trained      | 47     | 0.752052  | -0.038251               | -0.069882              | 47                    | 47                           |
| 2025-11       | baseline_original  | trained      | 58     | 1.000000  | 0.002162                | -0.067044              | 58                    | 58                           |
| 2025-11       | ctx4h_scaled025    | trained      | 58     | 0.560345  | -0.015402               | -0.054577              | 58                    | 58                           |
| 2025-11       | ctx12h_scaled025   | trained      | 58     | 0.495690  | -0.006565               | -0.042002              | 58                    | 58                           |
| 2025-11       | rf_prob_cap1       | trained      | 58     | 0.763389  | 0.020906                | -0.030452              | 58                    | 58                           |
| 2025-11       | rf_prob_floor025   | trained      | 58     | 0.822541  | 0.016220                | -0.039600              | 58                    | 58                           |
| 2025-11       | rf_binary_025      | trained      | 58     | 0.314655  | 0.024300                | 0.003803               | 58                    | 58                           |
| 2025-11       | rf_binary_000      | trained      | 58     | 0.086207  | 0.031679                | 0.027419               | 58                    | 5                            |
| 2025-11       | rf_rank_median_025 | trained      | 58     | 0.663793  | 0.021232                | -0.022149              | 58                    | 58                           |
| 2025-11       | rf_rank_median_000 | trained      | 58     | 0.551724  | 0.027589                | -0.007184              | 58                    | 32                           |
| 2025-11       | rf_rank_q60_000    | trained      | 58     | 0.448276  | 0.041334                | 0.013499               | 58                    | 26                           |
| 2025-11       | rf_rank_q70_000    | trained      | 58     | 0.189655  | 0.026954                | 0.015502               | 58                    | 11                           |
| 2025-11       | rf_replace_p2      | trained      | 58     | 0.773144  | 0.033131                | -0.007073              | 58                    | 58                           |
| 2025-12       | baseline_original  | trained      | 49     | 1.000000  | -0.065459               | -0.126334              | 49                    | 49                           |
| 2025-12       | ctx4h_scaled025    | trained      | 49     | 0.586735  | -0.025538               | -0.061377              | 49                    | 49                           |
| 2025-12       | ctx12h_scaled025   | trained      | 49     | 0.602041  | -0.025940               | -0.061757              | 49                    | 49                           |
| 2025-12       | rf_prob_cap1       | trained      | 49     | 0.799150  | -0.055535               | -0.103644              | 49                    | 49                           |
| 2025-12       | rf_prob_floor025   | trained      | 49     | 0.849363  | -0.058016               | -0.109317              | 49                    | 49                           |
| 2025-12       | rf_binary_025      | trained      | 49     | 0.433673  | -0.037574               | -0.063290              | 49                    | 49                           |
| 2025-12       | rf_binary_000      | trained      | 49     | 0.244898  | -0.028279               | -0.042275              | 49                    | 12                           |
| 2025-12       | rf_rank_median_025 | trained      | 49     | 0.739796  | -0.063976               | -0.107818              | 49                    | 49                           |
| 2025-12       | rf_rank_median_000 | trained      | 49     | 0.653061  | -0.063482               | -0.101646              | 49                    | 32                           |
| 2025-12       | rf_rank_q60_000    | trained      | 49     | 0.489796  | -0.032536               | -0.061856              | 49                    | 24                           |
| 2025-12       | rf_rank_q70_000    | trained      | 49     | 0.326531  | -0.026200               | -0.044589              | 49                    | 16                           |
| 2025-12       | rf_replace_p2      | trained      | 49     | 0.830742  | -0.046937               | -0.084432              | 49                    | 49                           |
| 2026-01       | baseline_original  | trained      | 59     | 1.000000  | -0.039055               | -0.114596              | 59                    | 59                           |
| 2026-01       | ctx4h_scaled025    | trained      | 59     | 0.593220  | 0.000633                | -0.044731              | 59                    | 59                           |
| 2026-01       | ctx12h_scaled025   | trained      | 59     | 0.656780  | -0.028541               | -0.078612              | 59                    | 59                           |
| 2026-01       | rf_prob_cap1       | trained      | 59     | 0.783769  | -0.020405               | -0.079457              | 59                    | 59                           |
| 2026-01       | rf_prob_floor025   | trained      | 59     | 0.837827  | -0.025067               | -0.088242              | 59                    | 59                           |
| 2026-01       | rf_binary_025      | trained      | 59     | 0.351695  | -0.003563               | -0.030603              | 59                    | 59                           |
| 2026-01       | rf_binary_000      | trained      | 59     | 0.135593  | 0.008268                | -0.002606              | 59                    | 8                            |
| 2026-01       | rf_rank_median_025 | trained      | 59     | 0.618644  | 0.001522                | -0.044041              | 59                    | 59                           |
| 2026-01       | rf_rank_median_000 | trained      | 59     | 0.491525  | 0.015048                | -0.020522              | 59                    | 29                           |
| 2026-01       | rf_rank_q60_000    | trained      | 59     | 0.338983  | 0.002982                | -0.022152              | 59                    | 20                           |
| 2026-01       | rf_rank_q70_000    | trained      | 59     | 0.254237  | 0.016004                | -0.003569              | 59                    | 15                           |
| 2026-01       | rf_replace_p2      | trained      | 59     | 0.794247  | -0.017404               | -0.060749              | 59                    | 59                           |
| 2026-02       | baseline_original  | trained      | 45     | 1.000000  | 0.128830                | 0.060452               | 45                    | 45                           |
| 2026-02       | ctx4h_scaled025    | trained      | 45     | 0.450000  | 0.087295                | 0.052270               | 45                    | 45                           |
| 2026-02       | ctx12h_scaled025   | trained      | 45     | 0.516667  | 0.104174                | 0.065306               | 45                    | 45                           |
| 2026-02       | rf_prob_cap1       | trained      | 45     | 0.848961  | 0.113193                | 0.054566               | 45                    | 45                           |
| 2026-02       | rf_prob_floor025   | trained      | 45     | 0.886721  | 0.117102                | 0.056037               | 45                    | 45                           |
| 2026-02       | rf_binary_025      | trained      | 45     | 0.416667  | 0.064324                | 0.035214               | 45                    | 45                           |
| 2026-02       | rf_binary_000      | trained      | 45     | 0.222222  | 0.042822                | 0.026802               | 45                    | 10                           |
| 2026-02       | rf_rank_median_025 | trained      | 45     | 0.800000  | 0.101901                | 0.046193               | 45                    | 45                           |
| 2026-02       | rf_rank_median_000 | trained      | 45     | 0.733333  | 0.092925                | 0.041439               | 45                    | 33                           |
| 2026-02       | rf_rank_q60_000    | trained      | 45     | 0.622222  | 0.097508                | 0.051702               | 45                    | 28                           |
| 2026-02       | rf_rank_q70_000    | trained      | 45     | 0.400000  | 0.051686                | 0.023812               | 45                    | 18                           |
| 2026-02       | rf_replace_p2      | trained      | 45     | 0.871586  | 0.067773                | 0.021432               | 45                    | 45                           |
| 2026-03       | baseline_original  | trained      | 48     | 1.000000  | 0.070376                | -0.012268              | 48                    | 48                           |
| 2026-03       | ctx4h_scaled025    | trained      | 48     | 0.531250  | 0.049018                | 0.008767               | 48                    | 48                           |
| 2026-03       | ctx12h_scaled025   | trained      | 48     | 0.500000  | 0.048493                | 0.012669               | 48                    | 48                           |
| 2026-03       | rf_prob_cap1       | trained      | 48     | 0.777028  | 0.061289                | -0.000123              | 48                    | 48                           |
| 2026-03       | rf_prob_floor025   | trained      | 48     | 0.832771  | 0.063561                | -0.003160              | 48                    | 48                           |
| 2026-03       | rf_binary_025      | trained      | 48     | 0.453125  | 0.064519                | 0.029775               | 48                    | 48                           |
| 2026-03       | rf_binary_000      | trained      | 48     | 0.270833  | 0.062567                | 0.043790               | 48                    | 13                           |
| 2026-03       | rf_rank_median_025 | trained      | 48     | 0.609375  | 0.053485                | 0.008580               | 48                    | 48                           |
| 2026-03       | rf_rank_median_000 | trained      | 48     | 0.479167  | 0.047855                | 0.015530               | 48                    | 23                           |
| 2026-03       | rf_rank_q60_000    | trained      | 48     | 0.375000  | 0.045130                | 0.020301               | 48                    | 18                           |
| 2026-03       | rf_rank_q70_000    | trained      | 48     | 0.270833  | 0.062567                | 0.043790               | 48                    | 13                           |
| 2026-03       | rf_replace_p2      | trained      | 48     | 0.812786  | 0.047114                | 0.001049               | 48                    | 48                           |
| 2026-04       | baseline_original  | trained      | 39     | 1.000000  | 0.000582                | -0.046753              | 39                    | 39                           |
| 2026-04       | ctx4h_scaled025    | trained      | 39     | 0.615385  | -0.012927               | -0.043852              | 39                    | 39                           |
| 2026-04       | ctx12h_scaled025   | trained      | 39     | 0.557692  | 0.001137                | -0.026540              | 39                    | 39                           |
| 2026-04       | rf_prob_cap1       | trained      | 39     | 0.798320  | 0.001839                | -0.036816              | 39                    | 39                           |
| 2026-04       | rf_prob_floor025   | trained      | 39     | 0.848740  | 0.001525                | -0.039300              | 39                    | 39                           |
| 2026-04       | rf_binary_025      | trained      | 39     | 0.384615  | 0.021278                | 0.003073               | 39                    | 39                           |
| 2026-04       | rf_binary_000      | trained      | 39     | 0.179487  | 0.028176                | 0.019682               | 39                    | 7                            |
| 2026-04       | rf_rank_median_025 | trained      | 39     | 0.557692  | 0.005352                | -0.022180              | 39                    | 39                           |
| 2026-04       | rf_rank_median_000 | trained      | 39     | 0.410256  | 0.006942                | -0.013989              | 39                    | 16                           |
| 2026-04       | rf_rank_q60_000    | trained      | 39     | 0.230769  | 0.031339                | 0.019435               | 39                    | 9                            |
| 2026-04       | rf_rank_q70_000    | trained      | 39     | 0.128205  | 0.020290                | 0.013960               | 39                    | 5                            |
| 2026-04       | rf_replace_p2      | trained      | 39     | 0.822360  | 0.011319                | -0.018238              | 39                    | 39                           |

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
      "train_events_after_base_gate": 166,
      "forward_events_after_base_gate": 50,
      "train_events": 166,
      "model_status": "trained",
      "label_events": 166,
      "positive_labels": 33,
      "negative_labels": 133,
      "train_auc": 0.987468671679198,
      "train_prob_median": 0.3549745913643235,
      "train_prob_q40": 0.31686180976898276,
      "train_prob_q60": 0.3892363803754544,
      "train_prob_q70": 0.42502072621803855,
      "forward_prob_mean": 0.38980719151065524,
      "forward_prob_median": 0.40068079810847235,
      "feature_importance_top5": [
        [
          "touch_extension_atr",
          0.1216675834563573
        ],
        [
          "eff_300s",
          0.11171526088082101
        ],
        [
          "ctx12h_range_atr",
          0.09816303830775888
        ],
        [
          "level_to_signal_open_atr",
          0.08753130715976985
        ],
        [
          "prev1_range_atr",
          0.08432056715113939
        ]
      ]
    },
    {
      "forward_month": "2025-10",
      "train_start": "2025-07-01T00:00:00+00:00",
      "train_events_after_base_gate": 157,
      "forward_events_after_base_gate": 47,
      "train_events": 157,
      "model_status": "trained",
      "label_events": 157,
      "positive_labels": 32,
      "negative_labels": 125,
      "train_auc": 0.99,
      "train_prob_median": 0.36977934874194285,
      "train_prob_q40": 0.34285929846464097,
      "train_prob_q60": 0.39208177083280604,
      "train_prob_q70": 0.42486354580367525,
      "forward_prob_mean": 0.37602583758050306,
      "forward_prob_median": 0.3882065712049094,
      "feature_importance_top5": [
        [
          "touch_extension_atr",
          0.13973396106765565
        ],
        [
          "prev1_range_atr",
          0.10052955057097986
        ],
        [
          "ctx12h_range_atr",
          0.0989264266098446
        ],
        [
          "ctx4h_range_atr",
          0.08390916314505387
        ],
        [
          "level_to_signal_open_atr",
          0.0828591826631836
        ]
      ]
    },
    {
      "forward_month": "2025-11",
      "train_start": "2025-08-01T00:00:00+00:00",
      "train_events_after_base_gate": 146,
      "forward_events_after_base_gate": 58,
      "train_events": 146,
      "model_status": "trained",
      "label_events": 146,
      "positive_labels": 33,
      "negative_labels": 113,
      "train_auc": 0.9753285063019577,
      "train_prob_median": 0.3716111017335635,
      "train_prob_q40": 0.3222521063720761,
      "train_prob_q60": 0.4088187050173868,
      "train_prob_q70": 0.45651041537496584,
      "forward_prob_mean": 0.3865721565083336,
      "forward_prob_median": 0.39140576184420006,
      "feature_importance_top5": [
        [
          "ctx12h_range_atr",
          0.16587649587727507
        ],
        [
          "touch_extension_atr",
          0.08932792475706006
        ],
        [
          "level_to_signal_open_atr",
          0.08898878832013503
        ],
        [
          "prev1_close_pos_side",
          0.07522089834875398
        ],
        [
          "speed_300s_atr",
          0.060370088966052234
        ]
      ]
    },
    {
      "forward_month": "2025-12",
      "train_start": "2025-09-01T00:00:00+00:00",
      "train_events_after_base_gate": 155,
      "forward_events_after_base_gate": 49,
      "train_events": 155,
      "model_status": "trained",
      "label_events": 155,
      "positive_labels": 42,
      "negative_labels": 113,
      "train_auc": 0.9576485461441213,
      "train_prob_median": 0.3696777453685357,
      "train_prob_q40": 0.35269994887578576,
      "train_prob_q60": 0.40983729319632806,
      "train_prob_q70": 0.4529983997095004,
      "forward_prob_mean": 0.4153708103435178,
      "forward_prob_median": 0.39270391015459616,
      "feature_importance_top5": [
        [
          "level_to_signal_open_atr",
          0.14385774294194206
        ],
        [
          "ctx12h_range_atr",
          0.13094558134704287
        ],
        [
          "eff_300s",
          0.10380820616859823
        ],
        [
          "pre_touch_seconds",
          0.09064269918945428
        ],
        [
          "prev1_close_pos_side",
          0.07749338665846264
        ]
      ]
    },
    {
      "forward_month": "2026-01",
      "train_start": "2025-10-01T00:00:00+00:00",
      "train_events_after_base_gate": 154,
      "forward_events_after_base_gate": 59,
      "train_events": 154,
      "model_status": "trained",
      "label_events": 154,
      "positive_labels": 37,
      "negative_labels": 117,
      "train_auc": 0.9803649803649803,
      "train_prob_median": 0.41885884063176293,
      "train_prob_q40": 0.37320094967887424,
      "train_prob_q60": 0.4452259929866601,
      "train_prob_q70": 0.4758962269298823,
      "forward_prob_mean": 0.3971236462269577,
      "forward_prob_median": 0.410371523348026,
      "feature_importance_top5": [
        [
          "ctx4h_range_atr",
          0.13998235123254982
        ],
        [
          "ctx4h_side_return_atr",
          0.13895545078636273
        ],
        [
          "level_to_signal_open_atr",
          0.11030973655778593
        ],
        [
          "touch_extension_atr",
          0.08105401986889278
        ],
        [
          "eff_300s",
          0.08051787116342403
        ]
      ]
    },
    {
      "forward_month": "2026-02",
      "train_start": "2025-11-01T00:00:00+00:00",
      "train_events_after_base_gate": 166,
      "forward_events_after_base_gate": 45,
      "train_events": 166,
      "model_status": "trained",
      "label_events": 166,
      "positive_labels": 42,
      "negative_labels": 124,
      "train_auc": 0.961021505376344,
      "train_prob_median": 0.3789049333371838,
      "train_prob_q40": 0.35129276690621386,
      "train_prob_q60": 0.4155623135338397,
      "train_prob_q70": 0.46613261327588695,
      "forward_prob_mean": 0.4357931451710618,
      "forward_prob_median": 0.4300703459000305,
      "feature_importance_top5": [
        [
          "ctx4h_range_atr",
          0.15687119493504734
        ],
        [
          "pre_touch_seconds",
          0.12004823430949203
        ],
        [
          "touch_extension_atr",
          0.0998246022736633
        ],
        [
          "eff_300s",
          0.08264020662063025
        ],
        [
          "ctx4h_side_return_atr",
          0.07968476266641768
        ]
      ]
    },
    {
      "forward_month": "2026-03",
      "train_start": "2025-12-01T00:00:00+00:00",
      "train_events_after_base_gate": 153,
      "forward_events_after_base_gate": 48,
      "train_events": 153,
      "model_status": "trained",
      "label_events": 153,
      "positive_labels": 42,
      "negative_labels": 111,
      "train_auc": 0.9703989703989704,
      "train_prob_median": 0.39186467318716306,
      "train_prob_q40": 0.3642581346347415,
      "train_prob_q60": 0.4566166978953326,
      "train_prob_q70": 0.4932056159650013,
      "forward_prob_mean": 0.40639317174758266,
      "forward_prob_median": 0.389944698853921,
      "feature_importance_top5": [
        [
          "pre_touch_seconds",
          0.11219650050476504
        ],
        [
          "eff_300s",
          0.10850886458982442
        ],
        [
          "ctx4h_range_atr",
          0.10708388447061545
        ],
        [
          "ctx12h_side_return_atr",
          0.09796593011361154
        ],
        [
          "ctx12h_range_atr",
          0.07114803652035319
        ]
      ]
    },
    {
      "forward_month": "2026-04",
      "train_start": "2026-01-01T00:00:00+00:00",
      "train_events_after_base_gate": 152,
      "forward_events_after_base_gate": 39,
      "train_events": 152,
      "model_status": "trained",
      "label_events": 152,
      "positive_labels": 49,
      "negative_labels": 103,
      "train_auc": 0.9552209233207846,
      "train_prob_median": 0.44203619738425837,
      "train_prob_q40": 0.41209757992223167,
      "train_prob_q60": 0.4790391682732027,
      "train_prob_q70": 0.5175316160102627,
      "forward_prob_mean": 0.41118013322194075,
      "forward_prob_median": 0.4040745988087647,
      "feature_importance_top5": [
        [
          "eff_300s",
          0.15962298639372638
        ],
        [
          "ctx4h_side_return_atr",
          0.09715210601161747
        ],
        [
          "pre_touch_seconds",
          0.09096793880448653
        ],
        [
          "ctx12h_range_atr",
          0.08308661751804733
        ],
        [
          "ctx4h_range_atr",
          0.0803857831956875
        ]
      ]
    }
  ],
  "runtime_seconds": 57.56108808517456
}
```
