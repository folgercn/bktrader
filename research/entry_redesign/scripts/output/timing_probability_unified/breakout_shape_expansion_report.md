# Breakout Shape Expansion — ETH Pretouch Timing Lead

Generated: 2026-05-18T03:02:13.650091+00:00

Scope: research-only, ETHUSDT 1h, 2025-06-01..2026-04-30, frozen `data/pretouch_model.json` `20260515_v1`.
Execution uses D0 same-close baseline plus next-second adverse fill stress. No live defaults are changed.

## Summary

| variant                  | raw_events | quality_events | model_advance_events | d0_traded_events | same_close_calendar_sum | same_close_worst_sm | same_close_neg_sm | adverse10_calendar_sum | adverse10_worst_sm | adverse10_neg_sm | avg_rf_probability |
| ------------------------ | ---------- | -------------- | -------------------- | ---------------- | ----------------------- | ------------------- | ----------------- | ---------------------- | ------------------ | ---------------- | ------------------ |
| restrictive_0p5bps       | 1810       | 736            | 561                  | 561              | -0.319441               | -0.116655           | 7                 | -1.029829              | -0.181796          | 10               | 0.578003           |
| strict_0bps              | 1844       | 745            | 569                  | 569              | -0.332641               | -0.116655           | 8                 | -1.052437              | -0.181796          | 10               | 0.578483           |
| near_equal_slack_0p25bps | 1864       | 751            | 574                  | 574              | -0.338424               | -0.116655           | 8                 | -1.063740              | -0.181796          | 10               | 0.578808           |
| near_equal_slack_0p5bps  | 1885       | 752            | 575                  | 575              | -0.333729               | -0.116655           | 8                 | -1.058893              | -0.181796          | 10               | 0.578185           |
| near_equal_slack_1p0bps  | 1906       | 755            | 578                  | 578              | -0.340747               | -0.116655           | 7                 | -1.068916              | -0.181796          | 10               | 0.578993           |

## Incremental Events vs Current Production Shape

| variant                  | extra_quality_events | extra_model_advance_events | extra_d0_traded_events | extra_same_close_calendar_sum | extra_same_close_worst_sm | extra_avg_rf_probability |
| ------------------------ | -------------------- | -------------------------- | ---------------------- | ----------------------------- | ------------------------- | ------------------------ |
| strict_0bps              | 9                    | 8                          | 8                      | -0.013199                     | -0.005201                 | 0.617778                 |
| near_equal_slack_0p25bps | 15                   | 13                         | 13                     | -0.018982                     | -0.008697                 | 0.618333                 |
| near_equal_slack_0p5bps  | 17                   | 15                         | 15                     | -0.013972                     | -0.008697                 | 0.610000                 |
| near_equal_slack_1p0bps  | 21                   | 19                         | 19                     | -0.016416                     | -0.015682                 | 0.642857                 |

## Canonical Lead Coverage

| variant                  | canonical_eth_events | missing_signal_bars | ready_events | ready_rate | unified_trades_ready | unified_trades_not_ready | ready_weighted_pnl | not_ready_weighted_pnl | total_weighted_pnl |
| ------------------------ | -------------------- | ------------------- | ------------ | ---------- | -------------------- | ------------------------ | ------------------ | ---------------------- | ------------------ |
| restrictive_0p5bps       | 154                  | 0                   | 151          | 0.980519   | 66                   | 2                        | 0.302105           | 0.007939               | 0.310044           |
| strict_0bps              | 154                  | 0                   | 154          | 1.000000   | 68                   | 0                        | 0.310044           | 0.000000               | 0.310044           |
| near_equal_slack_0p25bps | 154                  | 0                   | 154          | 1.000000   | 68                   | 0                        | 0.310044           | 0.000000               | 0.310044           |
| near_equal_slack_0p5bps  | 154                  | 0                   | 154          | 1.000000   | 68                   | 0                        | 0.310044           | 0.000000               | 0.310044           |
| near_equal_slack_1p0bps  | 154                  | 0                   | 154          | 1.000000   | 68                   | 0                        | 0.310044           | 0.000000               | 0.310044           |

## Fill Stress Matrix

| variant                  | scenario                | calendar_sum_gate_on | worst_sm_gate_on | neg_sm_count | trade_count_gate_on |
| ------------------------ | ----------------------- | -------------------- | ---------------- | ------------ | ------------------- |
| restrictive_0p5bps       | same_close_xslip0bps    | -0.319441            | -0.116655        | 7            | 561                 |
| restrictive_0p5bps       | next_close_xslip0bps    | -0.367571            | -0.115610        | 8            | 561                 |
| restrictive_0p5bps       | next_adverse_xslip0bps  | -0.442779            | -0.122389        | 9            | 561                 |
| restrictive_0p5bps       | next_adverse_xslip1bps  | -0.501484            | -0.128330        | 9            | 561                 |
| restrictive_0p5bps       | next_adverse_xslip3bps  | -0.618894            | -0.140211        | 9            | 561                 |
| restrictive_0p5bps       | next_adverse_xslip5bps  | -0.736304            | -0.152093        | 9            | 561                 |
| restrictive_0p5bps       | next_adverse_xslip7bps  | -0.853714            | -0.163974        | 9            | 561                 |
| restrictive_0p5bps       | next_adverse_xslip10bps | -1.029829            | -0.181796        | 10           | 561                 |
| strict_0bps              | same_close_xslip0bps    | -0.332641            | -0.116655        | 8            | 569                 |
| strict_0bps              | next_close_xslip0bps    | -0.379905            | -0.115610        | 8            | 569                 |
| strict_0bps              | next_adverse_xslip0bps  | -0.456892            | -0.122389        | 9            | 569                 |
| strict_0bps              | next_adverse_xslip1bps  | -0.516447            | -0.128330        | 9            | 569                 |
| strict_0bps              | next_adverse_xslip3bps  | -0.635555            | -0.140211        | 9            | 569                 |
| strict_0bps              | next_adverse_xslip5bps  | -0.754664            | -0.152093        | 9            | 569                 |
| strict_0bps              | next_adverse_xslip7bps  | -0.873773            | -0.163974        | 9            | 569                 |
| strict_0bps              | next_adverse_xslip10bps | -1.052437            | -0.181796        | 10           | 569                 |
| near_equal_slack_0p25bps | same_close_xslip0bps    | -0.338424            | -0.116655        | 8            | 574                 |
| near_equal_slack_0p25bps | next_close_xslip0bps    | -0.385523            | -0.115610        | 8            | 574                 |
| near_equal_slack_0p25bps | next_adverse_xslip0bps  | -0.462676            | -0.122389        | 9            | 574                 |
| near_equal_slack_0p25bps | next_adverse_xslip1bps  | -0.522782            | -0.128330        | 9            | 574                 |
| near_equal_slack_0p25bps | next_adverse_xslip3bps  | -0.642995            | -0.140211        | 9            | 574                 |
| near_equal_slack_0p25bps | next_adverse_xslip5bps  | -0.763208            | -0.152093        | 9            | 574                 |
| near_equal_slack_0p25bps | next_adverse_xslip7bps  | -0.883421            | -0.163974        | 9            | 574                 |
| near_equal_slack_0p25bps | next_adverse_xslip10bps | -1.063740            | -0.181796        | 10           | 574                 |
| near_equal_slack_0p5bps  | same_close_xslip0bps    | -0.333729            | -0.116655        | 8            | 575                 |
| near_equal_slack_0p5bps  | next_close_xslip0bps    | -0.380483            | -0.115610        | 8            | 575                 |
| near_equal_slack_0p5bps  | next_adverse_xslip0bps  | -0.457652            | -0.122389        | 9            | 575                 |
| near_equal_slack_0p5bps  | next_adverse_xslip1bps  | -0.517776            | -0.128330        | 9            | 575                 |
| near_equal_slack_0p5bps  | next_adverse_xslip3bps  | -0.638025            | -0.140211        | 9            | 575                 |
| near_equal_slack_0p5bps  | next_adverse_xslip5bps  | -0.758273            | -0.152093        | 9            | 575                 |
| near_equal_slack_0p5bps  | next_adverse_xslip7bps  | -0.878521            | -0.163974        | 9            | 575                 |
| near_equal_slack_0p5bps  | next_adverse_xslip10bps | -1.058893            | -0.181796        | 10           | 575                 |
| near_equal_slack_1p0bps  | same_close_xslip0bps    | -0.340747            | -0.116655        | 7            | 578                 |
| near_equal_slack_1p0bps  | next_close_xslip0bps    | -0.387225            | -0.115610        | 8            | 578                 |
| near_equal_slack_1p0bps  | next_adverse_xslip0bps  | -0.464835            | -0.122389        | 9            | 578                 |
| near_equal_slack_1p0bps  | next_adverse_xslip1bps  | -0.525243            | -0.128330        | 9            | 578                 |
| near_equal_slack_1p0bps  | next_adverse_xslip3bps  | -0.646059            | -0.140211        | 9            | 578                 |
| near_equal_slack_1p0bps  | next_adverse_xslip5bps  | -0.766875            | -0.152093        | 9            | 578                 |
| near_equal_slack_1p0bps  | next_adverse_xslip7bps  | -0.887692            | -0.163974        | 9            | 578                 |
| near_equal_slack_1p0bps  | next_adverse_xslip10bps | -1.068916            | -0.181796        | 10           | 578                 |

## Notes

- `restrictive_0p5bps` is the current production structure gate.
- `strict_0bps` restores original strict T2 comparison without the extra 0.5bps separation.
- `near_equal_slack_*` allows near-equal `prev2`/`prev1` structures and is deliberately not a live default.
- Rebuilt OHLC replay does not have real order-book spread; `roundtrip_cost_atr` is set to the live fallback `0.10`, while slippage stress is handled by the adverse-fill matrix.
- Feature signs are side-normalized to stay aligned with the 2026-05-15 research model artifact.
- The production-like D0 rebuilt replay is intentionally a different lens from the canonical `unified_trades.csv`, which uses the frozen canonical event source and timing-selected delays.

## Diagnostics

```json
{
  "symbol": "ETHUSDT",
  "model_version": "20260515_v1",
  "eval_start": "2025-06-01T00:00:00+00:00",
  "eval_end_exclusive": "2026-05-01T00:00:00+00:00",
  "speed_threshold": 0.228106,
  "max_pre_touch_seconds": 1800.0,
  "max_eff_300s": 1.0,
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
    "restrictive_0p5bps": {
      "dual_touch_same_second_skipped": 0,
      "bars_scanned": 1810,
      "raw_events": 1810,
      "quality_events": 736,
      "eval_events": 736
    },
    "strict_0bps": {
      "dual_touch_same_second_skipped": 0,
      "bars_scanned": 1844,
      "raw_events": 1844,
      "quality_events": 745,
      "eval_events": 745
    },
    "near_equal_slack_0p25bps": {
      "dual_touch_same_second_skipped": 0,
      "bars_scanned": 1864,
      "raw_events": 1864,
      "quality_events": 751,
      "eval_events": 751
    },
    "near_equal_slack_0p5bps": {
      "dual_touch_same_second_skipped": 0,
      "bars_scanned": 1885,
      "raw_events": 1885,
      "quality_events": 752,
      "eval_events": 752
    },
    "near_equal_slack_1p0bps": {
      "dual_touch_same_second_skipped": 0,
      "bars_scanned": 1906,
      "raw_events": 1906,
      "quality_events": 755,
      "eval_events": 755
    }
  },
  "runtime_seconds": 65.93300175666809
}
```
