# Breakout Structure Cross-Asset Gate Search — ETHUSDT

Generated: 2026-05-18T08:57:56.647508+00:00

Scope: research-only. All thresholds are trailing-window quantiles; forward rows are out-of-sample by month.

## Candidate Aggregate

| gate                           | forward_months | forward_events | trade_count | same_close_calendar_sum | same_close_worst_month | same_close_neg_months | adverse10_calendar_sum | adverse10_worst_month | adverse10_neg_months |
| ------------------------------ | -------------- | -------------- | ----------- | ----------------------- | ---------------------- | --------------------- | ---------------------- | --------------------- | -------------------- |
| low_eff_low_atr_ctx12h_up      | 3              | 5              | 5           | 0.008707                | -0.002423              | 1                     | 0.002595               | -0.003435             | 1                    |
| low_eff_ctx12h_side_up_q20_q60 | 3              | 15             | 15          | 0.008467                | -0.016398              | 1                     | -0.015983              | -0.022142             | 1                    |
| low_eff_low_atr_ctx4h_up       | 3              | 8              | 8           | -0.014439               | -0.012016              | 2                     | -0.025601              | -0.022166             | 2                    |
| low_eff_high_speed_q20_q60     | 3              | 13             | 13          | -0.011837               | -0.011837              | 1                     | -0.028700              | -0.028700             | 1                    |
| low_atr_side_gap_q40_q60       | 3              | 21             | 21          | -0.011530               | -0.007055              | 2                     | -0.037640              | -0.015707             | 3                    |
| high_speed_q80                 | 3              | 47             | 47          | 0.022303                | -0.002266              | 1                     | -0.038259              | -0.036231             | 2                    |
| low_eff_low_atr_q20_q40        | 3              | 17             | 17          | -0.024603               | -0.020559              | 2                     | -0.045447              | -0.035431             | 2                    |
| low_eff_low_atr_q30_q50        | 3              | 26             | 26          | -0.021295               | -0.014176              | 3                     | -0.051409              | -0.021847             | 3                    |
| low_eff_side_slope_q20_q60     | 3              | 15             | 15          | -0.029591               | -0.020044              | 3                     | -0.052629              | -0.025257             | 3                    |
| level_far_q60                  | 3              | 58             | 58          | 0.004087                | -0.022188              | 1                     | -0.071340              | -0.036547             | 3                    |
| low_atr_q40                    | 3              | 62             | 62          | -0.011993               | -0.010622              | 2                     | -0.081980              | -0.044093             | 3                    |
| wick_touch_ext_le_0            | 3              | 30             | 30          | -0.052001               | -0.051133              | 2                     | -0.083182              | -0.064473             | 2                    |
| low_eff_ctx4h_side_up_q20_q60  | 3              | 17             | 17          | -0.056319               | -0.022467              | 3                     | -0.083238              | -0.037549             | 3                    |
| ctx12h_side_up_q60             | 3              | 59             | 59          | -0.010394               | -0.030845              | 1                     | -0.088097              | -0.056303             | 3                    |
| low_eff_q20                    | 3              | 36             | 36          | -0.050724               | -0.034202              | 3                     | -0.099580              | -0.040848             | 3                    |
| low_rf_q40                     | 3              | 63             | 63          | -0.041430               | -0.020635              | 3                     | -0.107885              | -0.043006             | 3                    |
| low_eff_q30                    | 3              | 45             | 45          | -0.061481               | -0.033948              | 3                     | -0.119642              | -0.042216             | 3                    |
| high_speed_q60                 | 3              | 70             | 70          | -0.036796               | -0.025633              | 2                     | -0.122711              | -0.069399             | 3                    |
| high_rf_q60                    | 3              | 57             | 57          | -0.114978               | -0.054620              | 3                     | -0.201487              | -0.086643             | 3                    |
| side_sma_gap_up_q60            | 3              | 60             | 60          | -0.124987               | -0.053348              | 3                     | -0.204209              | -0.080250             | 3                    |
| ctx4h_side_up_q60              | 3              | 57             | 57          | -0.131869               | -0.069221              | 3                     | -0.206354              | -0.092800             | 3                    |
| side_sma_slope_up_q60          | 3              | 62             | 62          | -0.131980               | -0.065966              | 3                     | -0.211107              | -0.092861             | 3                    |
| level_near_q40                 | 3              | 59             | 59          | -0.194069               | -0.084865              | 3                     | -0.268231              | -0.110220             | 3                    |
| late_touch_q40                 | 3              | 89             | 89          | -0.165336               | -0.076815              | 3                     | -0.277945              | -0.118166             | 3                    |
| baseline_model_advance         | 3              | 149            | 149         | -0.202311               | -0.089283              | 3                     | -0.390524              | -0.160892             | 3                    |

## Selected Aggregate

| gate                 | forward_months | forward_events | trade_count | same_close_calendar_sum | same_close_worst_month | same_close_neg_months | adverse10_calendar_sum | adverse10_worst_month | adverse10_neg_months |
| -------------------- | -------------- | -------------- | ----------- | ----------------------- | ---------------------- | --------------------- | ---------------------- | --------------------- | -------------------- |
| walkforward_selected | 3              | 47             | 47          | -0.012600               | -0.014197              | 2                     | -0.073152              | -0.036231             | 3                    |

## Split Decisions

| forward_month | selected_gate       | selected_conditions              | train_adverse10_calendar_sum | train_trade_count | same_close_calendar_sum | adverse10_calendar_sum | trade_count |
| ------------- | ------------------- | -------------------------------- | ---------------------------- | ----------------- | ----------------------- | ---------------------- | ----------- |
| 2025-04       | wick_touch_ext_le_0 | touch_extension_atr <= 0         | 0.026055                     | 14                | -0.014197               | -0.020368              | 6           |
| 2025-05       | high_speed_q80      | speed_300s_atr >= 0.343872750719 | 0.008399                     | 7                 | -0.002266               | -0.036231              | 26          |
| 2025-06       | high_speed_q80      | speed_300s_atr >= 0.42188911396  | 0.011762                     | 12                | 0.003863                | -0.016553              | 15          |

## Interpretation

- Candidate aggregate applies every gate family to every forward month.
- Selected aggregate chooses the best positive train gate each month; if no eligible gate is positive it falls back to baseline.
- Promotion needs positive adverse10, acceptable worst month, and non-trivial trade count without relying on the selected fallback.

## Diagnostics

```json
{
  "symbol": "ETHUSDT",
  "events_csv": "research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_ethusdt_202503_202506_train1m_events.csv",
  "bars_cache_dir": "research/probabilistic_v6_runs/2025_m03_m06_original_t2_delay60/bars_cache",
  "eval_start": "2025-03-01T00:00:00+00:00",
  "eval_end_exclusive": "2025-07-01T00:00:00+00:00",
  "train_months": 1,
  "min_train_trades": 5,
  "events": 207,
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
  "gate_specs": [
    {
      "name": "baseline_model_advance",
      "description": "no extra gate",
      "conditions": []
    },
    {
      "name": "low_eff_q20",
      "description": "lowest 20% 300s efficiency",
      "conditions": [
        {
          "column": "eff_300s",
          "op": "<=",
          "quantile": 0.2,
          "value": null
        }
      ]
    },
    {
      "name": "low_eff_q30",
      "description": "lowest 30% 300s efficiency",
      "conditions": [
        {
          "column": "eff_300s",
          "op": "<=",
          "quantile": 0.3,
          "value": null
        }
      ]
    },
    {
      "name": "low_atr_q40",
      "description": "lowest 40% 24h ATR percentile",
      "conditions": [
        {
          "column": "signal_atr_percentile",
          "op": "<=",
          "quantile": 0.4,
          "value": null
        }
      ]
    },
    {
      "name": "low_eff_low_atr_q20_q40",
      "description": "lowest 20% efficiency plus lowest 40% ATR percentile",
      "conditions": [
        {
          "column": "eff_300s",
          "op": "<=",
          "quantile": 0.2,
          "value": null
        },
        {
          "column": "signal_atr_percentile",
          "op": "<=",
          "quantile": 0.4,
          "value": null
        }
      ]
    },
    {
      "name": "low_eff_low_atr_q30_q50",
      "description": "lowest 30% efficiency plus lowest 50% ATR percentile",
      "conditions": [
        {
          "column": "eff_300s",
          "op": "<=",
          "quantile": 0.3,
          "value": null
        },
        {
          "column": "signal_atr_percentile",
          "op": "<=",
          "quantile": 0.5,
          "value": null
        }
      ]
    },
    {
      "name": "high_speed_q60",
      "description": "top 40% side-normalized 300s speed",
      "conditions": [
        {
          "column": "speed_300s_atr",
          "op": ">=",
          "quantile": 0.6,
          "value": null
        }
      ]
    },
    {
      "name": "high_speed_q80",
      "description": "top 20% side-normalized 300s speed",
      "conditions": [
        {
          "column": "speed_300s_atr",
          "op": ">=",
          "quantile": 0.8,
          "value": null
        }
      ]
    },
    {
      "name": "low_eff_high_speed_q20_q60",
      "description": "low efficiency plus high side-normalized speed",
      "conditions": [
        {
          "column": "eff_300s",
          "op": "<=",
          "quantile": 0.2,
          "value": null
        },
        {
          "column": "speed_300s_atr",
          "op": ">=",
          "quantile": 0.6,
          "value": null
        }
      ]
    },
    {
      "name": "low_rf_q40",
      "description": "lower RF probability",
      "conditions": [
        {
          "column": "rf_probability",
          "op": "<=",
          "quantile": 0.4,
          "value": null
        }
      ]
    },
    {
      "name": "high_rf_q60",
      "description": "higher RF probability",
      "conditions": [
        {
          "column": "rf_probability",
          "op": ">=",
          "quantile": 0.6,
          "value": null
        }
      ]
    },
    {
      "name": "level_far_q60",
      "description": "level far from signal open",
      "conditions": [
        {
          "column": "level_to_signal_open_atr",
          "op": ">=",
          "quantile": 0.6,
          "value": null
        }
      ]
    },
    {
      "name": "level_near_q40",
      "description": "level near signal open",
      "conditions": [
        {
          "column": "level_to_signal_open_atr",
          "op": "<=",
          "quantile": 0.4,
          "value": null
        }
      ]
    },
    {
      "name": "wick_touch_ext_le_0",
      "description": "touch-second close has not extended beyond level",
      "conditions": [
        {
          "column": "touch_extension_atr",
          "op": "<=",
          "quantile": null,
          "value": 0.0
        }
      ]
    },
    {
      "name": "late_touch_q40",
      "description": "not too early in signal bar",
      "conditions": [
        {
          "column": "pre_touch_seconds",
          "op": ">=",
          "quantile": 0.4,
          "value": null
        }
      ]
    },
    {
      "name": "side_sma_slope_up_q60",
      "description": "closed-bar SMA5 slope aligns with trade side",
      "conditions": [
        {
          "column": "side_sma5_slope_atr",
          "op": ">=",
          "quantile": 0.6,
          "value": null
        }
      ]
    },
    {
      "name": "side_sma_gap_up_q60",
      "description": "previous close is on the favorable side of SMA5",
      "conditions": [
        {
          "column": "side_sma5_gap_atr",
          "op": ">=",
          "quantile": 0.6,
          "value": null
        }
      ]
    },
    {
      "name": "low_eff_side_slope_q20_q60",
      "description": "low efficiency plus favorable SMA5 slope",
      "conditions": [
        {
          "column": "eff_300s",
          "op": "<=",
          "quantile": 0.2,
          "value": null
        },
        {
          "column": "side_sma5_slope_atr",
          "op": ">=",
          "quantile": 0.6,
          "value": null
        }
      ]
    },
    {
      "name": "low_atr_side_gap_q40_q60",
      "description": "low ATR percentile plus favorable SMA5 gap",
      "conditions": [
        {
          "column": "signal_atr_percentile",
          "op": "<=",
          "quantile": 0.4,
          "value": null
        },
        {
          "column": "side_sma5_gap_atr",
          "op": ">=",
          "quantile": 0.6,
          "value": null
        }
      ]
    },
    {
      "name": "ctx4h_side_up_q60",
      "description": "prior 4h return aligns with side",
      "conditions": [
        {
          "column": "ctx4h_side_return_atr",
          "op": ">=",
          "quantile": 0.6,
          "value": null
        }
      ]
    },
    {
      "name": "ctx12h_side_up_q60",
      "description": "prior 12h return aligns with side",
      "conditions": [
        {
          "column": "ctx12h_side_return_atr",
          "op": ">=",
          "quantile": 0.6,
          "value": null
        }
      ]
    },
    {
      "name": "low_eff_ctx4h_side_up_q20_q60",
      "description": "low efficiency plus favorable prior 4h return",
      "conditions": [
        {
          "column": "eff_300s",
          "op": "<=",
          "quantile": 0.2,
          "value": null
        },
        {
          "column": "ctx4h_side_return_atr",
          "op": ">=",
          "quantile": 0.6,
          "value": null
        }
      ]
    },
    {
      "name": "low_eff_ctx12h_side_up_q20_q60",
      "description": "low efficiency plus favorable prior 12h return",
      "conditions": [
        {
          "column": "eff_300s",
          "op": "<=",
          "quantile": 0.2,
          "value": null
        },
        {
          "column": "ctx12h_side_return_atr",
          "op": ">=",
          "quantile": 0.6,
          "value": null
        }
      ]
    },
    {
      "name": "low_eff_low_atr_ctx4h_up",
      "description": "low efficiency plus low ATR plus favorable prior 4h return",
      "conditions": [
        {
          "column": "eff_300s",
          "op": "<=",
          "quantile": 0.2,
          "value": null
        },
        {
          "column": "signal_atr_percentile",
          "op": "<=",
          "quantile": 0.4,
          "value": null
        },
        {
          "column": "ctx4h_side_return_atr",
          "op": ">=",
          "quantile": 0.6,
          "value": null
        }
      ]
    },
    {
      "name": "low_eff_low_atr_ctx12h_up",
      "description": "low efficiency plus low ATR plus favorable prior 12h return",
      "conditions": [
        {
          "column": "eff_300s",
          "op": "<=",
          "quantile": 0.2,
          "value": null
        },
        {
          "column": "signal_atr_percentile",
          "op": "<=",
          "quantile": 0.4,
          "value": null
        },
        {
          "column": "ctx12h_side_return_atr",
          "op": ">=",
          "quantile": 0.6,
          "value": null
        }
      ]
    }
  ],
  "outputs": {
    "candidate_rows_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_ethusdt_202503_202506_train1m_min5_candidates.csv",
    "split_rows_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_ethusdt_202503_202506_train1m_min5_splits.csv",
    "selected_rows_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_ethusdt_202503_202506_train1m_min5_selected.csv",
    "candidate_summary_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_ethusdt_202503_202506_train1m_min5_candidate_summary.csv",
    "selected_summary_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_ethusdt_202503_202506_train1m_min5_selected_summary.csv",
    "selected_trades_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_ethusdt_202503_202506_train1m_min5_selected_trades.csv",
    "report_md": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_ethusdt_202503_202506_train1m_min5_report.md"
  },
  "runtime_seconds": 22.152533054351807
}
```
