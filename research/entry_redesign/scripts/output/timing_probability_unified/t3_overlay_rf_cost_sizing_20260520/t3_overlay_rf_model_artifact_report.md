# T3 Overlay RF Model Artifact

This artifact is for ETHUSDT 1h testnet shadow T3 overlay quality sizing.

- Artifact: `data/pretouch_t3_overlay_rf_model.json`
- Version: `20260520_t3_overlay_rf_cost_v1`
- Trained at: `2026-05-20T00:00:00Z`
- Training source: `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_rf_cost_sizing_20260520/t3_overlay_rf_cost_base_trades.csv`
- Training rows: `71`
- Training months: `2025-06, 2025-07, 2025-08, 2025-09, 2025-10, 2025-11, 2025-12, 2026-01, 2026-02, 2026-03, 2026-04`
- Features: `rf_probability, speed_300s_abs, eff_300s, touch_extension_abs, pre_touch_seconds, roundtrip_cost_atr, side_is_short`
- In-sample RF accuracy: `0.873239`
- Live sizing policy: map RF probability into absolute T3 overlay quantity `0.20..0.40` ETH, apply cost penalty, then clamp back to `0.20..0.40` ETH.
- Default testnet quantity band at base `0.100` ETH: `0.20..0.40` ETH, equivalent to `0.08 * 2.5..5.0`.
- This is an accumulated-history shadow artifact; it does not claim to exactly reproduce the walk-forward research curve.

- Walk-forward evidence: `wf_t3_rf_cost_quantity_0p20_0p40_shadow` overlay `45.639102%`, delta vs fixed `34.236470pp`, lead adverse10 + overlay `68.610750%`; fixed overlay `11.402632%`, avg quantity `0.300663` ETH.
