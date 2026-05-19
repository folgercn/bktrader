# Breakout Structure Cross-Asset Validation — BTCUSDT

Generated: 2026-05-18T07:22:29.906772+00:00

Scope: research-only. This checks whether the ETH-derived structure family generalizes cross-asset; no live defaults are changed.

## Aggregate

| gate                    | forward_months | forward_events | trade_count | same_close_calendar_sum | same_close_worst_month | same_close_neg_months | adverse10_calendar_sum | adverse10_worst_month | adverse10_neg_months |
| ----------------------- | -------------- | -------------- | ----------- | ----------------------- | ---------------------- | --------------------- | ---------------------- | --------------------- | -------------------- |
| low_eff_low_atr_q20_q40 | 8              | 38             | 38          | -0.007412               | -0.012545              | 4                     | -0.059469              | -0.020220             | 6                    |
| baseline_model_advance  | 8              | 385            | 385         | -0.185722               | -0.061181              | 7                     | -0.622999              | -0.131488             | 8                    |

## Split Rows

| forward_month | gate                    | forward_events | conditions                                                           | same_close_calendar_sum | adverse10_calendar_sum | trade_count |
| ------------- | ----------------------- | -------------- | -------------------------------------------------------------------- | ----------------------- | ---------------------- | ----------- |
| 2025-09       | baseline_model_advance  | 50             | none                                                                 | -0.043832               | -0.085054              | 50          |
| 2025-09       | low_eff_low_atr_q20_q40 | 4              | eff_300s <= 0.856005745436 & signal_atr_percentile <= 0.25           | 0.000200                | -0.000800              | 4           |
| 2025-10       | baseline_model_advance  | 43             | none                                                                 | -0.012654               | -0.057929              | 43          |
| 2025-10       | low_eff_low_atr_q20_q40 | 3              | eff_300s <= 0.830224152305 & signal_atr_percentile <= 0.3            | -0.008626               | -0.011369              | 3           |
| 2025-11       | baseline_model_advance  | 62             | none                                                                 | -0.061181               | -0.131488              | 62          |
| 2025-11       | low_eff_low_atr_q20_q40 | 6              | eff_300s <= 0.835754455478 & signal_atr_percentile <= 0.333333333333 | -0.003908               | -0.009684              | 6           |
| 2025-12       | baseline_model_advance  | 40             | none                                                                 | -0.020065               | -0.063655              | 40          |
| 2025-12       | low_eff_low_atr_q20_q40 | 3              | eff_300s <= 0.814825517846 & signal_atr_percentile <= 0.358333333333 | 0.000716                | -0.003797              | 3           |
| 2026-01       | baseline_model_advance  | 41             | none                                                                 | -0.025361               | -0.069635              | 41          |
| 2026-01       | low_eff_low_atr_q20_q40 | 6              | eff_300s <= 0.827325984288 & signal_atr_percentile <= 0.375          | -0.012545               | -0.020220              | 6           |
| 2026-02       | baseline_model_advance  | 43             | none                                                                 | -0.026386               | -0.082561              | 43          |
| 2026-02       | low_eff_low_atr_q20_q40 | 7              | eff_300s <= 0.78056144533 & signal_atr_percentile <= 0.375           | -0.009437               | -0.018210              | 7           |
| 2026-03       | baseline_model_advance  | 51             | none                                                                 | -0.001439               | -0.080415              | 51          |
| 2026-03       | low_eff_low_atr_q20_q40 | 5              | eff_300s <= 0.752295509609 & signal_atr_percentile <= 0.341666666667 | 0.019726                | 0.000953               | 5           |
| 2026-04       | baseline_model_advance  | 55             | none                                                                 | 0.005196                | -0.052263              | 55          |
| 2026-04       | low_eff_low_atr_q20_q40 | 4              | eff_300s <= 0.722556508464 & signal_atr_percentile <= 0.333333333333 | 0.006463                | 0.003659               | 4           |

## Interpretation

- `baseline_model_advance` is the full current-shape model-advance pool.
- `low_eff_low_atr_q20_q40` uses only trailing 3-month symbol-local quantiles, then trades the next calendar month.
- A cross-asset pass would require adverse10 improvement with acceptable worst-month and fewer negative months than the baseline.

## Diagnostics

```json
{
  "symbol": "BTCUSDT",
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
  "raw_events": 1748,
  "quality_events": 616,
  "model_advance_events": 497,
  "detector_diagnostics": {
    "dual_touch_same_second_skipped": 0,
    "bars_scanned": 1748
  },
  "canonical_coverage": {
    "canonical_events": 240,
    "canonical_start": "2025-06-01T18:30:00+00:00",
    "canonical_end": "2026-04-30T04:32:00+00:00",
    "current_shape_model_advance_events": 497,
    "overlap_keys": 64,
    "canonical_coverage_rate": 0.26666666666666666
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
    "events_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_btcusdt_train3m_events.csv",
    "splits_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_btcusdt_train3m_splits.csv",
    "summary_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_btcusdt_train3m_summary.csv",
    "trades_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_btcusdt_train3m_trades.csv",
    "report_md": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_btcusdt_train3m_report.md"
  },
  "runtime_seconds": 26.78265690803528
}
```
