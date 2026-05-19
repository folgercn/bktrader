# Breakout Structure Cross-Asset Validation — ETHUSDT

Generated: 2026-05-18T08:55:04.232070+00:00

Scope: research-only. This checks whether the ETH-derived structure family generalizes cross-asset; no live defaults are changed.

## Aggregate

| gate                    | forward_months | forward_events | trade_count | same_close_calendar_sum | same_close_worst_month | same_close_neg_months | adverse10_calendar_sum | adverse10_worst_month | adverse10_neg_months |
| ----------------------- | -------------- | -------------- | ----------- | ----------------------- | ---------------------- | --------------------- | ---------------------- | --------------------- | -------------------- |
| low_eff_low_atr_q20_q40 | 2              | 15             | 15          | -0.027164               | -0.031413              | 1                     | -0.044765              | -0.042891             | 2                    |
| baseline_model_advance  | 2              | 115            | 115         | -0.113973               | -0.089283              | 2                     | -0.261026              | -0.160892             | 2                    |

## Split Rows

| forward_month | gate                    | forward_events | conditions                                                           | same_close_calendar_sum | adverse10_calendar_sum | trade_count |
| ------------- | ----------------------- | -------------- | -------------------------------------------------------------------- | ----------------------- | ---------------------- | ----------- |
| 2025-05       | baseline_model_advance  | 56             | none                                                                 | -0.024691               | -0.100134              | 56          |
| 2025-05       | low_eff_low_atr_q20_q40 | 9              | eff_300s <= 0.810627896781 & signal_atr_percentile <= 0.208333333333 | -0.031413               | -0.042891              | 9           |
| 2025-06       | baseline_model_advance  | 59             | none                                                                 | -0.089283               | -0.160892              | 59          |
| 2025-06       | low_eff_low_atr_q20_q40 | 6              | eff_300s <= 0.794435946553 & signal_atr_percentile <= 0.358333333333 | 0.004249                | -0.001875              | 6           |

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
  "train_months": 2,
  "bars_cache_dir": "research/probabilistic_v6_runs/2025_m03_m06_original_t2_delay60/bars_cache",
  "eval_start": "2025-03-01T00:00:00+00:00",
  "eval_end_exclusive": "2025-07-01T00:00:00+00:00",
  "raw_events": 667,
  "quality_events": 267,
  "model_advance_events": 207,
  "detector_diagnostics": {
    "dual_touch_same_second_skipped": 0,
    "bars_scanned": 667
  },
  "canonical_coverage": {
    "canonical_events": 13,
    "canonical_start": "2025-06-01T14:03:00+00:00",
    "canonical_end": "2025-06-30T02:55:00+00:00",
    "current_shape_model_advance_events": 207,
    "overlap_keys": 3,
    "canonical_coverage_rate": 0.23076923076923078
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
    "events_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_ethusdt_202503_202506_train2m_events.csv",
    "splits_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_ethusdt_202503_202506_train2m_splits.csv",
    "summary_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_ethusdt_202503_202506_train2m_summary.csv",
    "trades_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_ethusdt_202503_202506_train2m_trades.csv",
    "report_md": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_ethusdt_202503_202506_train2m_report.md"
  },
  "runtime_seconds": 5.885515213012695
}
```
