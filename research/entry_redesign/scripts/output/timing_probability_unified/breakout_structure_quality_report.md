# Breakout Structure Quality-Gate Sweep — ETH Pretouch Timing Lead

Generated: 2026-05-18T03:15:35.452119+00:00

Scope: research-only, ETHUSDT 1h, current production shape `restrictive_0p5bps`, frozen `data/pretouch_model.json` `20260515_v1`.
These gates are mined candidates from the broad live-like event pool. They are evidence for the next validation step, not live defaults.

## Summary

| variant              | quality_events | model_advance_events | d0_traded_events | same_close_calendar_sum | same_close_worst_sm | same_close_neg_sm | same_close_pre_2026_sum | same_close_2026_sum | adverse10_calendar_sum | adverse10_worst_sm | adverse10_neg_sm |
| -------------------- | -------------- | -------------------- | ---------------- | ----------------------- | ------------------- | ----------------- | ----------------------- | ------------------- | ---------------------- | ------------------ | ---------------- |
| baseline_touch_entry | 561            | 561                  | 561              | -0.319441               | -0.116655           | 7                 | -0.480174               | 0.160733            | -1.029829              | -0.181796          | 10               |
| low_eff_le_q20       | 113            | 113                  | 113              | 0.106604                | -0.027861           | 7                 | -0.045964               | 0.152569            | -0.044537              | -0.037510          | 7                |
| low_eff_low_atr_pct  | 49             | 49                   | 49               | 0.109478                | -0.010761           | 2                 | 0.004178                | 0.105300            | 0.053389               | -0.016782          | 4                |
| low_rf_slope_up      | 96             | 96                   | 96               | 0.122515                | -0.024846           | 3                 | 0.083132                | 0.039382            | 0.033666               | -0.034381          | 4                |
| level_far_sma_gap_up | 54             | 54                   | 54               | 0.103569                | -0.004367           | 3                 | 0.061567                | 0.042002            | 0.047704               | -0.009075          | 5                |
| wick_touch_ext_le_0  | 136            | 136                  | 136              | 0.061603                | -0.026318           | 4                 | 0.004145                | 0.057458            | -0.114873              | -0.037999          | 6                |
| wick_late            | 80             | 80                   | 80               | 0.049045                | -0.020799           | 5                 | -0.004597               | 0.053643            | -0.060588              | -0.028264          | 7                |

## Gate Conditions

| variant              | conditions                                                           | description                                                                    |
| -------------------- | -------------------------------------------------------------------- | ------------------------------------------------------------------------------ |
| baseline_touch_entry | none                                                                 | current production-shape D0 lens; no extra structure-quality gate              |
| low_eff_le_q20       | eff_300s <= 0.769915                                                 | pre-touch 300s efficiency in the lowest 20% of model-advance events            |
| low_eff_low_atr_pct  | eff_300s <= 0.769915 & signal_atr_percentile <= 0.291667             | low 300s efficiency plus low 24h ATR percentile                                |
| low_rf_slope_up      | rf_probability <= 0.580000 & prev_sma5_slope_atr >= 0.047875         | lower RF probability but closed-bar SMA5 slope already positive                |
| level_far_sma_gap_up | level_to_signal_open_atr >= 0.440024 & prev_sma5_gap_atr >= 0.348070 | breakout level far from signal open and previous close above SMA5 for the side |
| wick_touch_ext_le_0  | touch_extension_atr <= 0.000000                                      | first touch is wick-led; touch-second close has not extended beyond the level  |
| wick_late            | touch_extension_atr <= 0.000000 & pre_touch_seconds >= 598.000000    | wick-led first touch and not too early in the signal bar                       |

## Adverse Fill Matrix

| variant              | scenario                | calendar_sum_gate_on | worst_sm_gate_on | neg_sm_count | trade_count_gate_on |
| -------------------- | ----------------------- | -------------------- | ---------------- | ------------ | ------------------- |
| baseline_touch_entry | same_close_xslip0bps    | -0.319441            | -0.116655        | 7            | 561                 |
| baseline_touch_entry | next_close_xslip0bps    | -0.367571            | -0.115610        | 8            | 561                 |
| baseline_touch_entry | next_adverse_xslip0bps  | -0.442779            | -0.122389        | 9            | 561                 |
| baseline_touch_entry | next_adverse_xslip1bps  | -0.501484            | -0.128330        | 9            | 561                 |
| baseline_touch_entry | next_adverse_xslip3bps  | -0.618894            | -0.140211        | 9            | 561                 |
| baseline_touch_entry | next_adverse_xslip5bps  | -0.736304            | -0.152093        | 9            | 561                 |
| baseline_touch_entry | next_adverse_xslip7bps  | -0.853714            | -0.163974        | 9            | 561                 |
| baseline_touch_entry | next_adverse_xslip10bps | -1.029829            | -0.181796        | 10           | 561                 |
| low_eff_le_q20       | same_close_xslip0bps    | 0.106604             | -0.027861        | 7            | 113                 |
| low_eff_le_q20       | next_close_xslip0bps    | 0.090745             | -0.027604        | 7            | 113                 |
| low_eff_le_q20       | next_adverse_xslip0bps  | 0.069997             | -0.028757        | 7            | 113                 |
| low_eff_le_q20       | next_adverse_xslip1bps  | 0.058543             | -0.029300        | 7            | 113                 |
| low_eff_le_q20       | next_adverse_xslip3bps  | 0.035637             | -0.030385        | 7            | 113                 |
| low_eff_le_q20       | next_adverse_xslip5bps  | 0.012730             | -0.031469        | 7            | 113                 |
| low_eff_le_q20       | next_adverse_xslip7bps  | -0.010177            | -0.033339        | 7            | 113                 |
| low_eff_le_q20       | next_adverse_xslip10bps | -0.044537            | -0.037510        | 7            | 113                 |
| low_eff_low_atr_pct  | same_close_xslip0bps    | 0.109478             | -0.010761        | 2            | 49                  |
| low_eff_low_atr_pct  | next_close_xslip0bps    | 0.111483             | -0.010768        | 2            | 49                  |
| low_eff_low_atr_pct  | next_adverse_xslip0bps  | 0.104100             | -0.010967        | 3            | 49                  |
| low_eff_low_atr_pct  | next_adverse_xslip1bps  | 0.099029             | -0.011548        | 3            | 49                  |
| low_eff_low_atr_pct  | next_adverse_xslip3bps  | 0.088887             | -0.012711        | 3            | 49                  |
| low_eff_low_atr_pct  | next_adverse_xslip5bps  | 0.078745             | -0.013875        | 3            | 49                  |
| low_eff_low_atr_pct  | next_adverse_xslip7bps  | 0.068602             | -0.015038        | 4            | 49                  |
| low_eff_low_atr_pct  | next_adverse_xslip10bps | 0.053389             | -0.016782        | 4            | 49                  |
| low_rf_slope_up      | same_close_xslip0bps    | 0.122515             | -0.024846        | 3            | 96                  |
| low_rf_slope_up      | next_close_xslip0bps    | 0.122898             | -0.024984        | 2            | 96                  |
| low_rf_slope_up      | next_adverse_xslip0bps  | 0.111715             | -0.025597        | 3            | 96                  |
| low_rf_slope_up      | next_adverse_xslip1bps  | 0.103910             | -0.026476        | 3            | 96                  |
| low_rf_slope_up      | next_adverse_xslip3bps  | 0.088300             | -0.028232        | 3            | 96                  |
| low_rf_slope_up      | next_adverse_xslip5bps  | 0.072690             | -0.029989        | 3            | 96                  |
| low_rf_slope_up      | next_adverse_xslip7bps  | 0.057080             | -0.031746        | 3            | 96                  |
| low_rf_slope_up      | next_adverse_xslip10bps | 0.033666             | -0.034381        | 4            | 96                  |
| level_far_sma_gap_up | same_close_xslip0bps    | 0.103569             | -0.004367        | 3            | 54                  |
| level_far_sma_gap_up | next_close_xslip0bps    | 0.106089             | -0.004667        | 4            | 54                  |
| level_far_sma_gap_up | next_adverse_xslip0bps  | 0.096992             | -0.004667        | 4            | 54                  |
| level_far_sma_gap_up | next_adverse_xslip1bps  | 0.092063             | -0.005107        | 5            | 54                  |
| level_far_sma_gap_up | next_adverse_xslip3bps  | 0.082205             | -0.005989        | 5            | 54                  |
| level_far_sma_gap_up | next_adverse_xslip5bps  | 0.072348             | -0.006871        | 5            | 54                  |
| level_far_sma_gap_up | next_adverse_xslip7bps  | 0.062490             | -0.007752        | 5            | 54                  |
| level_far_sma_gap_up | next_adverse_xslip10bps | 0.047704             | -0.009075        | 5            | 54                  |
| wick_touch_ext_le_0  | same_close_xslip0bps    | 0.061603             | -0.026318        | 4            | 136                 |
| wick_touch_ext_le_0  | next_close_xslip0bps    | 0.049770             | -0.025326        | 4            | 136                 |
| wick_touch_ext_le_0  | next_adverse_xslip0bps  | 0.028762             | -0.026455        | 4            | 136                 |
| wick_touch_ext_le_0  | next_adverse_xslip1bps  | 0.014399             | -0.027610        | 4            | 136                 |
| wick_touch_ext_le_0  | next_adverse_xslip3bps  | -0.014328            | -0.029919        | 4            | 136                 |
| wick_touch_ext_le_0  | next_adverse_xslip5bps  | -0.043055            | -0.032227        | 5            | 136                 |
| wick_touch_ext_le_0  | next_adverse_xslip7bps  | -0.071782            | -0.034536        | 5            | 136                 |
| wick_touch_ext_le_0  | next_adverse_xslip10bps | -0.114873            | -0.037999        | 6            | 136                 |
| wick_late            | same_close_xslip0bps    | 0.049045             | -0.020799        | 5            | 80                  |
| wick_late            | next_close_xslip0bps    | 0.033836             | -0.019747        | 5            | 80                  |
| wick_late            | next_adverse_xslip0bps  | 0.020846             | -0.020872        | 5            | 80                  |
| wick_late            | next_adverse_xslip1bps  | 0.012703             | -0.021611        | 6            | 80                  |
| wick_late            | next_adverse_xslip3bps  | -0.003584            | -0.023090        | 6            | 80                  |
| wick_late            | next_adverse_xslip5bps  | -0.019871            | -0.024568        | 7            | 80                  |
| wick_late            | next_adverse_xslip7bps  | -0.036158            | -0.026046        | 7            | 80                  |
| wick_late            | next_adverse_xslip10bps | -0.060588            | -0.028264        | 7            | 80                  |

## Monthly PnL

| year_month | baseline_touch_entry | level_far_sma_gap_up | low_eff_le_q20 | low_eff_low_atr_pct | low_rf_slope_up | wick_late | wick_touch_ext_le_0 |
| ---------- | -------------------- | -------------------- | -------------- | ------------------- | --------------- | --------- | ------------------- |
| 2025-06    | -0.089237            | 0.040481             | -0.015379      | 0.004249            | 0.021693        | 0.013340  | 0.013328            |
| 2025-07    | -0.116655            | -0.003292            | -0.021934      | -0.010575           | -0.000358       | -0.003207 | -0.008066           |
| 2025-08    | -0.099919            | -0.003790            | -0.027861      | 0.000000            | -0.024846       | -0.013639 | 0.013836            |
| 2025-09    | -0.030508            | -0.004367            | -0.001303      | 0.010425            | 0.026571        | 0.005474  | 0.009532            |
| 2025-10    | -0.080559            | 0.005138             | -0.017985      | -0.010761           | 0.007050        | -0.020799 | -0.026318           |
| 2025-11    | 0.002162             | 0.025332             | 0.044110       | 0.009941            | 0.056716        | 0.015421  | 0.020003            |
| 2025-12    | -0.065459            | 0.002065             | -0.005611      | 0.000899            | -0.003694       | -0.001188 | -0.018170           |
| 2026-01    | -0.039055            | 0.003134             | -0.005531      | 0.007852            | 0.005836        | -0.007339 | -0.012940           |
| 2026-02    | 0.128830             | 0.023053             | 0.048644       | 0.030079            | 0.019272        | 0.037297  | 0.032695            |
| 2026-03    | 0.070376             | 0.015726             | 0.089865       | 0.038863            | 0.007576        | 0.021652  | 0.030335            |
| 2026-04    | 0.000582             | 0.000089             | 0.019591       | 0.028506            | 0.006699        | 0.002033  | 0.007369            |

## Interpretation

- Pure shape expansion and post-touch confirmation did not turn the broad live-like pool positive.
- Several structure-quality gates are positive even under next-second adverse 10bps stress, but the sample is small and thresholds were mined in-sample.
- The most interesting next step is walk-forward validation/retraining around these features, not a live default change.

## Diagnostics

```json
{
  "base_events_path": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_shape_expansion_events_restrictive_0p5bps.csv",
  "quality_events": 736,
  "model_advance_events": 561,
  "eval_start": "2025-06-01T00:00:00+00:00",
  "eval_end_exclusive": "2026-05-01T00:00:00+00:00",
  "base_share": 0.8,
  "exec_params": {
    "breakeven_at_r": 0.8,
    "cost_lock_bps": 10.0,
    "entry_fee": 0.0002,
    "exit_fee": 0.0004,
    "initial_stop_atr": 0.45,
    "max_hold_hours": 2.0,
    "min_stop_bps": 12.0,
    "slippage": 0.0002,
    "stop_buffer_atr": 0.05,
    "stop_cap_atr": 0.8,
    "trail_buffer_atr": 0.05,
    "trail_start_r": 1.5
  },
  "variants": {
    "baseline_touch_entry": {
      "conditions": [],
      "events": 561,
      "description": "current production-shape D0 lens; no extra structure-quality gate"
    },
    "low_eff_le_q20": {
      "conditions": [
        [
          "eff_300s",
          "<=",
          0.7699153778815291
        ]
      ],
      "events": 113,
      "description": "pre-touch 300s efficiency in the lowest 20% of model-advance events"
    },
    "low_eff_low_atr_pct": {
      "conditions": [
        [
          "eff_300s",
          "<=",
          0.7699153778815291
        ],
        [
          "signal_atr_percentile",
          "<=",
          0.2916666666666667
        ]
      ],
      "events": 49,
      "description": "low 300s efficiency plus low 24h ATR percentile"
    },
    "low_rf_slope_up": {
      "conditions": [
        [
          "rf_probability",
          "<=",
          0.58
        ],
        [
          "prev_sma5_slope_atr",
          ">=",
          0.0478745222904068
        ]
      ],
      "events": 96,
      "description": "lower RF probability but closed-bar SMA5 slope already positive"
    },
    "level_far_sma_gap_up": {
      "conditions": [
        [
          "level_to_signal_open_atr",
          ">=",
          0.4400237847225935
        ],
        [
          "prev_sma5_gap_atr",
          ">=",
          0.3480695487355221
        ]
      ],
      "events": 54,
      "description": "breakout level far from signal open and previous close above SMA5 for the side"
    },
    "wick_touch_ext_le_0": {
      "conditions": [
        [
          "touch_extension_atr",
          "<=",
          0.0
        ]
      ],
      "events": 136,
      "description": "first touch is wick-led; touch-second close has not extended beyond the level"
    },
    "wick_late": {
      "conditions": [
        [
          "touch_extension_atr",
          "<=",
          0.0
        ],
        [
          "pre_touch_seconds",
          ">=",
          598.0
        ]
      ],
      "events": 80,
      "description": "wick-led first touch and not too early in the signal bar"
    }
  },
  "runtime_seconds": 28.205073833465576
}
```
