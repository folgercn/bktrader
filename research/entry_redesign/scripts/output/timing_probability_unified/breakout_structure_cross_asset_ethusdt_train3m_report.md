# Breakout Structure Cross-Asset Validation — ETHUSDT

Generated: 2026-05-18T07:23:43.191159+00:00

Scope: research-only. This checks whether the ETH-derived structure family generalizes cross-asset; no live defaults are changed.

## Aggregate

| gate                    | forward_months | forward_events | trade_count | same_close_calendar_sum | same_close_worst_month | same_close_neg_months | adverse10_calendar_sum | adverse10_worst_month | adverse10_neg_months |
| ----------------------- | -------------- | -------------- | ----------- | ----------------------- | ---------------------- | --------------------- | ---------------------- | --------------------- | -------------------- |
| low_eff_low_atr_q20_q40 | 8              | 42             | 42          | 0.122025                | -0.010761              | 1                     | 0.072854               | -0.012840             | 2                    |
| baseline_model_advance  | 8              | 395            | 395         | -0.013631               | -0.080559              | 4                     | -0.530945              | -0.137240             | 7                    |

## Split Rows

| forward_month | gate                    | forward_events | conditions                                                           | same_close_calendar_sum | adverse10_calendar_sum | trade_count |
| ------------- | ----------------------- | -------------- | -------------------------------------------------------------------- | ----------------------- | ---------------------- | ----------- |
| 2025-09       | baseline_model_advance  | 50             | none                                                                 | -0.030508               | -0.087162              | 50          |
| 2025-09       | low_eff_low_atr_q20_q40 | 1              | eff_300s <= 0.775059996308 & signal_atr_percentile <= 0.291666666667 | 0.010425                | 0.009583               | 1           |
| 2025-10       | baseline_model_advance  | 47             | none                                                                 | -0.080559               | -0.137240              | 47          |
| 2025-10       | low_eff_low_atr_q20_q40 | 2              | eff_300s <= 0.792247877386 & signal_atr_percentile <= 0.266666666667 | -0.010761               | -0.012840              | 2           |
| 2025-11       | baseline_model_advance  | 58             | none                                                                 | 0.002162                | -0.067044              | 58          |
| 2025-11       | low_eff_low_atr_q20_q40 | 7              | eff_300s <= 0.829766179423 & signal_atr_percentile <= 0.25           | 0.016961                | 0.008626               | 7           |
| 2025-12       | baseline_model_advance  | 49             | none                                                                 | -0.065459               | -0.126334              | 49          |
| 2025-12       | low_eff_low_atr_q20_q40 | 9              | eff_300s <= 0.811297387944 & signal_atr_percentile <= 0.208333333333 | 0.000899                | -0.010290              | 9           |
| 2026-01       | baseline_model_advance  | 59             | none                                                                 | -0.039055               | -0.114596              | 59          |
| 2026-01       | low_eff_low_atr_q20_q40 | 5              | eff_300s <= 0.793343727708 & signal_atr_percentile <= 0.208333333333 | 0.008021                | 0.003008               | 5           |
| 2026-02       | baseline_model_advance  | 45             | none                                                                 | 0.128830                | 0.060452               | 45          |
| 2026-02       | low_eff_low_atr_q20_q40 | 4              | eff_300s <= 0.783358229057 & signal_atr_percentile <= 0.25           | 0.018394                | 0.012771               | 4           |
| 2026-03       | baseline_model_advance  | 48             | none                                                                 | 0.070376                | -0.012268              | 48          |
| 2026-03       | low_eff_low_atr_q20_q40 | 10             | eff_300s <= 0.771440028148 & signal_atr_percentile <= 0.375          | 0.049582                | 0.038316               | 10          |
| 2026-04       | baseline_model_advance  | 39             | none                                                                 | 0.000582                | -0.046753              | 39          |
| 2026-04       | low_eff_low_atr_q20_q40 | 4              | eff_300s <= 0.741917639949 & signal_atr_percentile <= 0.375          | 0.028506                | 0.023680               | 4           |

## Interpretation

- `baseline_model_advance` is the full current-shape model-advance pool.
- `low_eff_low_atr_q20_q40` uses only trailing 3-month symbol-local quantiles, then trades the next calendar month.
- A cross-asset pass would require adverse10 improvement with acceptable worst-month and fewer negative months than the baseline.

## Diagnostics

```json
{
  "symbol": "ETHUSDT",
  "model_version": "20260515_v1",
  "model_features": [
    "roundtrip_cost_atr",
    "prev1_range_atr",
    "prev1_close_pos_side",
    "level_to_signal_open_atr",
    "touch_extension_atr",
    "speed_300s_atr",
    "eff_300s",
    "pre_touch_seconds"
  ],
  "variant": {
    "name": "restrictive_0p5bps",
    "mode": "restrictive",
    "bps": 0.5,
    "description": "current production: prev2 must be separated by +0.5bps"
  },
  "train_months": 3,
  "eval_start": "2025-06-01T00:00:00+00:00",
  "eval_end_exclusive": "2026-05-01T00:00:00+00:00",
  "raw_events": 1810,
  "quality_events": 736,
  "model_advance_events": 561,
  "detector_diagnostics": {
    "dual_touch_same_second_skipped": 0,
    "bars_scanned": 1810
  },
  "canonical_coverage": {
    "canonical_events": 154,
    "canonical_start": "2025-06-01T14:03:00+00:00",
    "canonical_end": "2026-04-30T15:31:00+00:00",
    "current_shape_model_advance_events": 561,
    "overlap_keys": 48,
    "canonical_coverage_rate": 0.3116883116883117
  },
  "base_share": 0.8,
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
  "outputs": {
    "events_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_ethusdt_train3m_events.csv",
    "splits_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_ethusdt_train3m_splits.csv",
    "summary_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_ethusdt_train3m_summary.csv",
    "trades_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_ethusdt_train3m_trades.csv",
    "report_md": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_ethusdt_train3m_report.md"
  },
  "runtime_seconds": 25.51013493537903
}
```
