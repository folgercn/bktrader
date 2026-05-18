# Breakout Structure Cross-Asset Gate Search — ETHUSDT

Generated: 2026-05-18T08:59:10.713841+00:00

Scope: research-only. All thresholds are trailing-window quantiles; forward rows are out-of-sample by month.

## Candidate Aggregate

| gate                           | forward_months | forward_events | trade_count | same_close_calendar_sum | same_close_worst_month | same_close_neg_months | adverse10_calendar_sum | adverse10_worst_month | adverse10_neg_months |
| ------------------------------ | -------------- | -------------- | ----------- | ----------------------- | ---------------------- | --------------------- | ---------------------- | --------------------- | -------------------- |
| low_eff_low_atr_q20_q40        | 8              | 42             | 42          | 0.122025                | -0.010761              | 1                     | 0.072854               | -0.012840             | 2                    |
| low_eff_low_atr_ctx4h_up       | 8              | 16             | 16          | 0.084547                | 0.000000               | 0                     | 0.066304               | -0.001849             | 1                    |
| low_eff_ctx4h_side_up_q20_q60  | 8              | 38             | 38          | 0.104054                | -0.013100              | 2                     | 0.059408               | -0.018948             | 3                    |
| low_eff_low_atr_ctx12h_up      | 8              | 15             | 15          | 0.078744                | -0.006925              | 1                     | 0.059167               | -0.008168             | 2                    |
| low_eff_ctx12h_side_up_q20_q60 | 8              | 40             | 40          | 0.101024                | -0.014317              | 3                     | 0.053181               | -0.018858             | 4                    |
| low_eff_low_atr_q30_q50        | 8              | 68             | 68          | 0.129092                | -0.032769              | 2                     | 0.048193               | -0.043119             | 2                    |
| low_eff_q20                    | 8              | 90             | 90          | 0.166553                | -0.012039              | 4                     | 0.039887               | -0.022803             | 4                    |
| low_eff_side_slope_q20_q60     | 8              | 40             | 40          | 0.080044                | -0.013100              | 4                     | 0.033415               | -0.015473             | 4                    |
| low_eff_high_speed_q20_q60     | 8              | 17             | 17          | 0.003994                | -0.010623              | 5                     | -0.014359              | -0.013457             | 5                    |
| low_eff_q30                    | 8              | 125            | 125         | 0.156165                | -0.053911              | 3                     | -0.015499              | -0.073094             | 3                    |
| low_atr_side_gap_q40_q60       | 8              | 57             | 57          | 0.022090                | -0.033624              | 3                     | -0.047584              | -0.051692             | 4                    |
| low_rf_q40                     | 8              | 155            | 155         | 0.058720                | -0.026356              | 3                     | -0.095684              | -0.045633             | 6                    |
| wick_touch_ext_le_0            | 8              | 102            | 102         | 0.042505                | -0.026318              | 3                     | -0.099473              | -0.037999             | 5                    |
| side_sma_slope_up_q60          | 8              | 154            | 154         | 0.099233                | -0.028404              | 4                     | -0.101036              | -0.055997             | 5                    |
| high_speed_q80                 | 8              | 82             | 82          | -0.013623               | -0.026813              | 4                     | -0.108317              | -0.043188             | 6                    |
| ctx4h_side_up_q60              | 8              | 148            | 148         | 0.061673                | -0.030636              | 4                     | -0.136374              | -0.050421             | 6                    |
| low_atr_q40                    | 8              | 162            | 162         | 0.055729                | -0.039955              | 1                     | -0.140102              | -0.064602             | 6                    |
| ctx12h_side_up_q60             | 8              | 156            | 156         | 0.047014                | -0.061282              | 4                     | -0.157075              | -0.082782             | 6                    |
| high_speed_q60                 | 8              | 167            | 167         | 0.015333                | -0.030870              | 5                     | -0.191405              | -0.061863             | 7                    |
| level_far_q60                  | 8              | 156            | 156         | -0.030335               | -0.041430              | 4                     | -0.227875              | -0.065201             | 6                    |
| side_sma_gap_up_q60            | 8              | 158            | 158         | -0.021124               | -0.053994              | 6                     | -0.228882              | -0.089830             | 6                    |
| level_near_q40                 | 8              | 159            | 159         | -0.018678               | -0.026966              | 5                     | -0.230277              | -0.058323             | 6                    |
| late_touch_q40                 | 8              | 225            | 225         | 0.026800                | -0.058460              | 4                     | -0.259323              | -0.093199             | 6                    |
| high_rf_q60                    | 8              | 171            | 171         | -0.069650               | -0.071699              | 6                     | -0.329114              | -0.109546             | 6                    |
| baseline_model_advance         | 8              | 395            | 395         | -0.013631               | -0.080559              | 4                     | -0.530945              | -0.137240             | 7                    |

## Selected Aggregate

| gate                 | forward_months | forward_events | trade_count | same_close_calendar_sum | same_close_worst_month | same_close_neg_months | adverse10_calendar_sum | adverse10_worst_month | adverse10_neg_months |
| -------------------- | -------------- | -------------- | ----------- | ----------------------- | ---------------------- | --------------------- | ---------------------- | --------------------- | -------------------- |
| walkforward_selected | 8              | 140            | 140         | -0.069868               | -0.080559              | 5                     | -0.234890              | -0.137240             | 6                    |

## Split Decisions

| forward_month | selected_gate                  | selected_conditions                                                                                   | train_adverse10_calendar_sum | train_trade_count | same_close_calendar_sum | adverse10_calendar_sum | trade_count |
| ------------- | ------------------------------ | ----------------------------------------------------------------------------------------------------- | ---------------------------- | ----------------- | ----------------------- | ---------------------- | ----------- |
| 2025-09       | baseline_model_advance         | none                                                                                                  | -0.498884                    | 166               | -0.030508               | -0.087162              | 50          |
| 2025-10       | baseline_model_advance         | none                                                                                                  | -0.425139                    | 157               | -0.080559               | -0.137240              | 47          |
| 2025-11       | low_eff_low_atr_ctx4h_up       | eff_300s <= 0.829766179423 & signal_atr_percentile <= 0.25 & ctx4h_side_return_atr >= 0.204573447079  | 0.005476                     | 6                 | 0.002202                | -0.001849              | 3           |
| 2025-12       | low_eff_q20                    | eff_300s <= 0.811297387944                                                                            | 0.010227                     | 31                | -0.005446               | -0.019774              | 12          |
| 2026-01       | low_eff_ctx12h_side_up_q20_q60 | eff_300s <= 0.793343727708 & ctx12h_side_return_atr >= 0.394091928159                                 | 0.002471                     | 16                | -0.002958               | -0.011644              | 7           |
| 2026-02       | low_eff_ctx12h_side_up_q20_q60 | eff_300s <= 0.783358229057 & ctx12h_side_return_atr >= 0.61147220646                                  | 0.029693                     | 16                | 0.034988                | 0.031151               | 3           |
| 2026-03       | low_eff_low_atr_ctx4h_up       | eff_300s <= 0.771440028148 & signal_atr_percentile <= 0.375 & ctx4h_side_return_atr >= 0.211094301007 | 0.031731                     | 8                 | 0.024115                | 0.021070               | 3           |
| 2026-04       | side_sma_slope_up_q60          | side_sma5_slope_atr >= 0.0568221116115                                                                | 0.084814                     | 61                | -0.011703               | -0.029443              | 15          |

## Interpretation

- Candidate aggregate applies every gate family to every forward month.
- Selected aggregate chooses the best positive train gate each month; if no eligible gate is positive it falls back to baseline.
- Promotion needs positive adverse10, acceptable worst month, and non-trivial trade count without relying on the selected fallback.

## Diagnostics

```json
{
  "symbol": "ETHUSDT",
  "events_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_ethusdt_train3m_events.csv",
  "bars_cache_dir": "/Users/wuyaocheng/Downloads/bkTrader/research/probabilistic_v6_runs/walkforward_delay60_original_t2_feature60_valbest/bars_cache",
  "eval_start": "2025-06-01T00:00:00+00:00",
  "eval_end_exclusive": "2026-05-01T00:00:00+00:00",
  "train_months": 3,
  "min_train_trades": 5,
  "events": 561,
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
    "candidate_rows_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_ethusdt_train3m_min5_candidates.csv",
    "split_rows_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_ethusdt_train3m_min5_splits.csv",
    "selected_rows_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_ethusdt_train3m_min5_selected.csv",
    "candidate_summary_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_ethusdt_train3m_min5_candidate_summary.csv",
    "selected_summary_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_ethusdt_train3m_min5_selected_summary.csv",
    "selected_trades_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_ethusdt_train3m_min5_selected_trades.csv",
    "report_md": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_ethusdt_train3m_min5_report.md"
  },
  "runtime_seconds": 96.21892929077148
}
```
