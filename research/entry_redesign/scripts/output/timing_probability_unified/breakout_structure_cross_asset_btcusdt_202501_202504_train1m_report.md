# Breakout Structure Cross-Asset Validation — BTCUSDT

Generated: 2026-05-18T07:26:45.591444+00:00

Scope: research-only. This checks whether the ETH-derived structure family generalizes cross-asset; no live defaults are changed.

## Aggregate

| gate                    | forward_months | forward_events | trade_count | same_close_calendar_sum | same_close_worst_month | same_close_neg_months | adverse10_calendar_sum | adverse10_worst_month | adverse10_neg_months |
| ----------------------- | -------------- | -------------- | ----------- | ----------------------- | ---------------------- | --------------------- | ---------------------- | --------------------- | -------------------- |
| low_eff_low_atr_q20_q40 | 2              | 8              | 8           | -0.022965               | -0.018666              | 2                     | -0.031058              | -0.025623             | 2                    |
| baseline_model_advance  | 2              | 88             | 88          | -0.102010               | -0.067970              | 2                     | -0.198943              | -0.114854             | 2                    |

## Split Rows

| forward_month | gate                    | forward_events | conditions                                                           | same_close_calendar_sum | adverse10_calendar_sum | trade_count |
| ------------- | ----------------------- | -------------- | -------------------------------------------------------------------- | ----------------------- | ---------------------- | ----------- |
| 2025-03       | baseline_model_advance  | 45             | none                                                                 | -0.034040               | -0.084089              | 45          |
| 2025-03       | low_eff_low_atr_q20_q40 | 7              | eff_300s <= 0.82274938032 & signal_atr_percentile <= 0.2             | -0.018666               | -0.025623              | 7           |
| 2025-04       | baseline_model_advance  | 43             | none                                                                 | -0.067970               | -0.114854              | 43          |
| 2025-04       | low_eff_low_atr_q20_q40 | 1              | eff_300s <= 0.782012231416 & signal_atr_percentile <= 0.166666666667 | -0.004299               | -0.005435              | 1           |

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
  "train_months": 1,
  "bars_cache_dir": "research/historical_extension/bars_cache",
  "eval_start": "2025-01-02T00:00:00+00:00",
  "eval_end_exclusive": "2025-05-01T00:00:00+00:00",
  "raw_events": 623,
  "quality_events": 209,
  "model_advance_events": 166,
  "detector_diagnostics": {
    "dual_touch_same_second_skipped": 0,
    "bars_scanned": 623
  },
  "canonical_coverage": {
    "canonical_events": 0,
    "overlap_keys": 0
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
    "events_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_btcusdt_202501_202504_train1m_events.csv",
    "splits_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_btcusdt_202501_202504_train1m_splits.csv",
    "summary_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_btcusdt_202501_202504_train1m_summary.csv",
    "trades_csv": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_btcusdt_202501_202504_train1m_trades.csv",
    "report_md": "/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_btcusdt_202501_202504_train1m_report.md"
  },
  "runtime_seconds": 7.4433159828186035
}
```
