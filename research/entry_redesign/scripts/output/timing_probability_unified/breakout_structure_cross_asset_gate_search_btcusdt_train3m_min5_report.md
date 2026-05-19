# Breakout Structure Cross-Asset Gate Search — BTCUSDT

Generated: 2026-05-18T07:45:18.520647+00:00

Scope: research-only. All thresholds are trailing-window quantiles; forward rows are out-of-sample by month.

## Candidate Aggregate

| gate                       | forward_months | forward_events | trade_count | same_close_calendar_sum | same_close_worst_month | same_close_neg_months | adverse10_calendar_sum | adverse10_worst_month | adverse10_neg_months |
| -------------------------- | -------------- | -------------- | ----------- | ----------------------- | ---------------------- | --------------------- | ---------------------- | --------------------- | -------------------- |
| low_eff_high_speed_q20_q60 | 8              | 12             | 12          | -0.019682               | -0.012202              | 4                     | -0.033263              | -0.015985             | 7                    |
| low_eff_low_atr_q20_q40    | 8              | 38             | 38          | -0.007412               | -0.012545              | 4                     | -0.059469              | -0.020220             | 6                    |
| high_speed_q80             | 8              | 78             | 78          | -0.014542               | -0.016582              | 6                     | -0.095944              | -0.027991             | 7                    |
| low_atr_side_gap_q40_q60   | 8              | 58             | 58          | -0.036498               | -0.011141              | 6                     | -0.098773              | -0.020125             | 8                    |
| low_eff_side_slope_q20_q60 | 8              | 44             | 44          | -0.066566               | -0.033020              | 5                     | -0.116035              | -0.038928             | 8                    |
| low_eff_low_atr_q30_q50    | 8              | 73             | 73          | -0.030541               | -0.019080              | 4                     | -0.117570              | -0.029733             | 7                    |
| wick_touch_ext_le_0        | 8              | 65             | 65          | -0.033285               | -0.026330              | 5                     | -0.118083              | -0.036566             | 6                    |
| low_rf_q40                 | 8              | 148            | 148         | -0.004647               | -0.029152              | 4                     | -0.145597              | -0.042317             | 7                    |
| high_speed_q60             | 8              | 155            | 155         | 0.017624                | -0.017139              | 4                     | -0.148395              | -0.032526             | 8                    |
| low_eff_q20                | 8              | 95             | 95          | -0.073534               | -0.019918              | 7                     | -0.189908              | -0.038924             | 8                    |
| low_atr_q40                | 8              | 152            | 152         | -0.058291               | -0.025482              | 6                     | -0.236807              | -0.044793             | 8                    |
| level_far_q60              | 8              | 151            | 151         | -0.099886               | -0.061646              | 6                     | -0.253161              | -0.077405             | 8                    |
| side_sma_gap_up_q60        | 8              | 167            | 167         | -0.081522               | -0.028851              | 6                     | -0.261830              | -0.051836             | 8                    |
| low_eff_q30                | 8              | 141            | 141         | -0.100134               | -0.036312              | 8                     | -0.265304              | -0.066395             | 8                    |
| side_sma_slope_up_q60      | 8              | 168            | 168         | -0.095917               | -0.070967              | 4                     | -0.282602              | -0.103352             | 8                    |
| level_near_q40             | 8              | 165            | 165         | -0.104435               | -0.035413              | 6                     | -0.295512              | -0.060669             | 8                    |
| late_touch_q40             | 8              | 227            | 227         | -0.104753               | -0.038384              | 6                     | -0.355740              | -0.063895             | 8                    |
| high_rf_q60                | 8              | 163            | 163         | -0.132185               | -0.063177              | 5                     | -0.357226              | -0.102467             | 8                    |
| baseline_model_advance     | 8              | 385            | 385         | -0.185722               | -0.061181              | 7                     | -0.622999              | -0.131488             | 8                    |

## Selected Aggregate

| gate                 | forward_months | forward_events | trade_count | same_close_calendar_sum | same_close_worst_month | same_close_neg_months | adverse10_calendar_sum | adverse10_worst_month | adverse10_neg_months |
| -------------------- | -------------- | -------------- | ----------- | ----------------------- | ---------------------- | --------------------- | ---------------------- | --------------------- | -------------------- |
| walkforward_selected | 8              | 345            | 345         | -0.188765               | -0.061181              | 7                     | -0.558606              | -0.131488             | 8                    |

## Split Decisions

| forward_month | selected_gate          | selected_conditions              | train_adverse10_calendar_sum | train_trade_count | same_close_calendar_sum | adverse10_calendar_sum | trade_count |
| ------------- | ---------------------- | -------------------------------- | ---------------------------- | ----------------- | ----------------------- | ---------------------- | ----------- |
| 2025-09       | baseline_model_advance | none                             | -0.239418                    | 112               | -0.043832               | -0.085054              | 50          |
| 2025-10       | baseline_model_advance | none                             | -0.272050                    | 124               | -0.012654               | -0.057929              | 43          |
| 2025-11       | baseline_model_advance | none                             | -0.257084                    | 127               | -0.061181               | -0.131488              | 62          |
| 2025-12       | baseline_model_advance | none                             | -0.274470                    | 155               | -0.020065               | -0.063655              | 40          |
| 2026-01       | baseline_model_advance | none                             | -0.253071                    | 145               | -0.025361               | -0.069635              | 41          |
| 2026-02       | baseline_model_advance | none                             | -0.264778                    | 143               | -0.026386               | -0.082561              | 43          |
| 2026-03       | high_speed_q80         | speed_300s_atr >= 0.529794588825 | 0.001694                     | 25                | -0.004483               | -0.016022              | 11          |
| 2026-04       | baseline_model_advance | none                             | -0.232611                    | 135               | 0.005196                | -0.052263              | 55          |

## Interpretation

- Candidate aggregate applies every gate family to every forward month.
- Selected aggregate chooses the best positive train gate each month; if no eligible gate is positive it falls back to baseline.
- Promotion needs positive adverse10, acceptable worst month, and non-trivial trade count without relying on the selected fallback.

## Diagnostics

```json
{
  "symbol": "BTCUSDT",
  "events_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_btcusdt_train3m_events.csv",
  "bars_cache_dir": "/Users/wuyaocheng/Downloads/bkTrader/research/probabilistic_v6_runs/walkforward_delay60_original_t2_feature60_valbest/bars_cache",
  "eval_start": "2025-06-01T00:00:00+00:00",
  "eval_end_exclusive": "2026-05-01T00:00:00+00:00",
  "train_months": 3,
  "min_train_trades": 5,
  "events": 497,
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
    }
  ],
  "outputs": {
    "candidate_rows_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_btcusdt_train3m_min5_candidates.csv",
    "split_rows_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_btcusdt_train3m_min5_splits.csv",
    "selected_rows_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_btcusdt_train3m_min5_selected.csv",
    "candidate_summary_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_btcusdt_train3m_min5_candidate_summary.csv",
    "selected_summary_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_btcusdt_train3m_min5_selected_summary.csv",
    "selected_trades_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_btcusdt_train3m_min5_selected_trades.csv",
    "report_md": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_btcusdt_train3m_min5_report.md"
  },
  "runtime_seconds": 73.62134671211243
}
```
