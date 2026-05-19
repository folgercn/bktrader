# Breakout Structure Expansion Decision - 2026-05-18

Scope: research-only. This note summarizes the current ETHUSDT 1h pretouch timing / breakout-structure expansion line after adding 4h/12h context-overlay checks. It does not change live defaults.

## Current Research Lead

- Production-aligned canonical event source: `pretouch_small_pullback_rf_q50_speed300_ge_q10_touch30m_eff300le1`
- Canonical CSV: `research/tick_flow_event_sources/20260514_pretouch_full_window/feature_filtered_seed_events/robust_quality/pretouch_small_pullback_rf_q50_speed300_ge_q10_touch30m_eff300le1.csv`
- Model artifact: `data/pretouch_model.json` (`20260515_v1`)
- Live-like exit contract for research validation: `trail_start_r=1.5`, `max_hold_hours=2.0`
- Fill stress used here: `next_adverse_xslip10bps`, not optimistic `re_p`

## Main Result

`wf3_low_eff_low_atr` is still the strongest clean ETH-local expansion candidate by return, but it is not cross-asset robust and should not be promoted as a live default.

The new 4h/12h context overlays do not increase total return versus bare `wf3`; they reduce tail risk and trade count. The best risk profile is currently `wf3_low_eff_low_atr_ctx12h_up`, with `ctx4h_up` close behind.

Position scaling is more promising than hard filtering: keeping all `wf3` events but scaling context-failed events to 25% size keeps most of the return lift while materially improving worst month.

## Lead Combo Replay

Canonical lead is preserved; expansion events are non-overlapping by `(signal_start, side)`.

| variant | extra_events | combo_adverse10 | delta_vs_lead | worst_sm | neg_sm | trades |
|---|---:|---:|---:|---:|---:|---:|
| `wf5_low_eff_q20` | 71 | 0.285020 | 0.055303 | -0.019114 | 2 | 133 |
| `wf3_low_eff_low_atr` | 41 | 0.282598 | 0.052882 | -0.010290 | 2 | 103 |
| `wf3_low_eff_low_atr_ctx4h_up` | 15 | 0.276048 | 0.046332 | -0.001849 | 1 | 77 |
| `wf3_low_eff_low_atr_ctx12h_up` | 14 | 0.268912 | 0.039195 | -0.001586 | 1 | 76 |

## Retrain Forward Check

Retrain uses the `production8` feature contract and evaluates forward after `2025-11-01`.

| pool | total_events | forward_events | forward_adverse10 | worst_sm | neg_sm | trades |
|---|---:|---:|---:|---:|---:|---:|
| `combo_wf5_low_eff_q20` | 225 | 157 | 0.423931 | -0.019235 | 2 | 149 |
| `combo_wf3_low_eff_low_atr` | 195 | 124 | 0.408512 | -0.002244 | 1 | 115 |
| `combo_wf3_low_eff_low_atr_ctx12h_up` | 168 | 99 | 0.371748 | 0.011073 | 0 | 91 |
| `combo_wf3_low_eff_low_atr_ctx4h_scaled025` | 195 | 124 | 0.370266 | 0.006097 | 0 | 115 |
| `combo_wf3_low_eff_low_atr_ctx4h_up` | 169 | 100 | 0.357531 | 0.006612 | 0 | 92 |
| `canonical_only` | 154 | 86 | 0.299716 | 0.002688 | 0 | 78 |

## Context Sizing Sensitivity

This keeps all non-overlapping `wf3_low_eff_low_atr` events, but scales events that fail the context overlay instead of removing them.

| context | fail_weight | combo_adverse10 | delta_vs_lead | worst_sm | neg_sm | trades |
|---|---:|---:|---:|---:|---:|---:|
| `ctx4h_up` | 0.25 | 0.277686 | 0.047969 | -0.001379 | 1 | 103 |
| `ctx12h_up` | 0.00 | 0.268912 | 0.039195 | -0.001586 | 1 | 76 |
| `ctx12h_up` | 0.25 | 0.272333 | 0.042617 | -0.001800 | 2 | 103 |
| `ctx4h_up` | 0.00 | 0.276048 | 0.046332 | -0.001849 | 1 | 77 |
| bare `wf3` | 1.00 | 0.282598 | 0.052882 | -0.010290 | 2 | 103 |

Read: `ctx4h_up` with failed-context events at 25% size keeps about 91% of bare `wf3` adverse10 lift (`0.047969 / 0.052882`) while improving worst month from `-0.010290` to `-0.001379`.

The same sizing idea also survives the retrain forward check: `combo_wf3_low_eff_low_atr_ctx4h_scaled025` keeps all `115` forward trades, raises forward adverse10 from canonical `0.299716` to `0.370266`, and keeps worst month positive at `0.006097`.

## Early ETH History Check

On ETH 2025-03..2025-06, using the prebuilt current-shape event source and 1-month train gate rows, context sizing is not robust enough:

| variant | events | adverse10 | worst_sm | neg_sm | trades |
|---|---:|---:|---:|---:|---:|
| `ctx12h_up hard` | 5 | 0.002595 | -0.003435 | 1 | 5 |
| `ctx12h_up fail_weight=0.25` | 17 | -0.009415 | -0.004335 | 3 | 17 |
| `ctx4h_up hard` | 8 | -0.025601 | -0.022166 | 2 | 8 |
| `ctx4h_up fail_weight=0.25` | 17 | -0.030562 | -0.025482 | 3 | 17 |
| bare `low_eff_low_atr` | 17 | -0.045447 | -0.035431 | 2 | 17 |

Read: context sizing reduces the early-history loss versus bare `low_eff_low_atr`, but only `ctx12h_up hard` is slightly positive and it has just 5 trades. This blocks live promotion.

## Context Model Sizing

I also tested a trailing RF sizing overlay on the `low_eff_low_atr` event family. The model uses only prior-window labels under `next_adverse_xslip10bps`, then sizes the next month. A follow-up rerun added hard-select variants (`*_000`) that can zero out model-rejected events instead of keeping a 25% floor.

| sample | best model/fixed variant | adverse10 | worst_sm | neg_months | read |
|---|---|---:|---:|---:|---|
| ETH 2025-06..2026-04 | `rf_rank_median_000` | 0.073127 | -0.004736 | 2 | Bare event family was 0.072854; hard-select adds almost no return, but improves worst month and active trade count falls to 28/42. |
| ETH 2025-03..2025-06 | `ctx12h_scaled025` | -0.009415 | -0.004335 | 3 | Best RF variant was still negative; model does not rescue early ETH. |
| BTC 2025-06..2026-04 | `rf_binary_000` | -0.008731 | -0.009684 | 1 | Hard-select reduces loss more than fixed context overlays, but active trade count falls to 11/38 and the total remains negative. |

The RF overlay is not promotion-worthy. Train windows are tiny and train AUC frequently reads `1.0`, which is a classic overfit smell rather than real evidence. The hard-select rerun improved risk filtering, but the useful result is still negative: adding a small model on top of `low_eff_low_atr` does not create a robust 10-20% candidate.

## Context Model Lead Combo

I then replayed the model-selected events as an additive leg on top of canonical lead, removing overlap by `(signal_start, side)`. This checks whether the probability overlay adds to the current research lead instead of only looking good as a standalone pool.

| variant | active_source | extra_events | extra_adverse10 | combo_adverse10 | delta_vs_lead | worst_sm | neg_sm | combo_trades | read |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `low_eff_rf_rank_median_000` | 28 | 27 | 0.053155 | 0.282871 | 0.053155 | -0.004736 | 2 | 89 | Nearly the same lift as bare `wf3_low_eff_low_atr`, with fewer events and better tail. |
| `wide_rf_binary_000` | 60 | 57 | 0.052375 | 0.282091 | 0.052375 | -0.042275 | 2 | 119 | Same return lift, but the wide-pool tail is too deep. |
| `low_eff_rf_rank_q60_000` | 21 | 20 | 0.037911 | 0.267627 | 0.037911 | -0.004736 | 2 | 82 | More conservative than median, but gives up too much return. |
| `low_eff_rf_binary_000` | 8 | 7 | 0.032647 | 0.262364 | 0.032647 | -0.005066 | 1 | 69 | Fewer negative months, but sample and lift are too small. |
| `low_eff_rf_rank_q70_000` | 5 | 5 | 0.010347 | 0.240063 | 0.010347 | -0.005066 | 2 | 67 | Over-filtered. |
| `wide_rf_rank_q70_000` | 103 | 97 | -0.005620 | 0.224097 | -0.005620 | -0.044589 | 2 | 159 | Fails as additive leg. |
| `wide_rf_binary_025` | 395 | 362 | -0.086876 | 0.142841 | -0.086876 | -0.063290 | 3 | 424 | Rejected; 25% floor keeps too much adverse selection. |

Read: hard-select probability can compress the late-ETH additive leg. The best form is not the wide pool; it is `low_eff_low_atr` plus median hard model selection. Pushing to q60/q70 or fixed `probability >= 0.5` improves selectivity but loses too much return. This is a cleaner research candidate than bare `wf3`, but early ETH standalone validation still blocks promotion.

## Wide Pool Context Model

To check whether the fixed `low_eff_low_atr` gate was too narrow, I reran the same trailing context-aware sizing model on the wider `baseline_model_advance` event pool.

| sample | best variant | best adverse10 | baseline adverse10 | read |
|---|---|---:|---:|---|
| ETH 2025-06..2026-04 | `rf_binary_000` | 0.079500 | -0.530945 | Hard-select flips the late ETH wide pool positive, but worst month is still -0.042275 and only 60/395 events remain active. |
| ETH 2025-03..2025-06 | `rf_rank_q70_000` | -0.031129 | -0.390524 | Large loss reduction, still negative across all 3 forward months. |
| BTC 2025-06..2026-04 | `rf_binary_000` | -0.078599 | -0.622999 | Large loss reduction, but all 8 months remain negative and only 46/385 events remain active. |

This confirms the wider current-shape pool has too much adverse selection. A context-aware probability overlay can suppress damage, and hard zero-selection can create a positive late-ETH slice, but it does not survive early ETH or BTC falsification.

## Robustness Read

- BTC 2025-06..2026-04 did not produce positive alpha from these simple structure gates; the best gates mainly reduced losses.
- ETH early 2025-03..2025-06 also weakens the full-history claim. Plain `low_eff_low_atr_q20_q40` was negative; `ctx4h fail_weight=0.25` was still negative; `ctx12h_up` was only slightly positive with 5 trades, too small for promotion.
- Dynamic selectors remain noisy and worse than fixed gates in several runs.
- Context-aware RF sizing on top of `low_eff_low_atr` is also not enough: it only marginally improves ETH late, fails early ETH, and remains negative on BTC.
- Additive lead-combo replay improves the late-ETH story for `low_eff_rf_rank_median_000`, but this does not remove the early-history/BTC block.
- Wider-pool context modeling reduces losses sharply; the hard-select rerun flips ETH late positive, but early ETH and BTC remain negative, so the issue is upstream event quality, not just sizing.
- Monthly gates remain excluded. The only longer context used here is prior closed-bar 4h/12h return.

## Decision

1. Keep production defaults unchanged.
2. Treat `wf3_low_eff_low_atr` as the aggressive ETH-local research expansion.
3. Treat `wf3_low_eff_low_atr_ctx12h_up` as the conservative risk-overlay candidate, not as a return amplifier.
4. Treat `wf3_low_eff_low_atr_ctx4h_up` with `fail_weight=0.25` as the best post-2025-11 sizing-control variant, but not robust enough for live promotion.
5. Reject the first context-aware RF sizing overlay as a promotion path; it is too small-sample and does not fix early ETH/BTC.
6. Keep `low_eff_rf_rank_median_000` as the next late-ETH additive candidate to falsify: it matches `wf3` return lift with fewer extra events and better worst month, but it still fails early ETH standalone.
7. Reject the wider `baseline_model_advance` pool as an alpha source despite late-ETH `rf_binary_000` turning positive; it is useful only as a falsification baseline until early ETH and BTC also pass.
8. Do not promote `wf5_low_eff_q20` yet despite higher retrain return; its combo replay tail is materially worse and it lacks the robustness story.

## Next Research Slice

Focus on ETH-specific event-source rebuild and regime partitioning using only intraday context:

- rebuild pretouch event source upstream instead of adding overlays to the current-shape pool;
- use `eff_300s`, low ATR percentile, 4h/12h side-return, and event-age/touch-extension as first-class event-generation or model-selection features;
- rebuild or re-partition the event source before more live-facing work; current context sizing improves losses but fails early-history robustness;
- keep validation in three tiers: lead combo replay, retrain forward, then early ETH / BTC falsification;
- require any candidate to beat canonical adverse10 while keeping worst month non-negative or near-flat under `next_adverse_xslip10bps`.

Primary artifacts:

- `research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_lead_expansion_combo_report.md`
- `research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_model_retrain_report.md`
- `research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_context_sizing_sensitivity_report.md`
- `research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_context_sizing_history_ethusdt_202503_202506_train1m_report.md`
- `research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_context_model_sizing_ethusdt_train3m_report.md`
- `research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_context_model_sizing_ethusdt_202503_202506_train1m_report.md`
- `research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_context_model_sizing_btcusdt_train3m_report.md`
- `research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_context_model_sizing_ethusdt_train3m_baseline_pool_report.md`
- `research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_context_model_sizing_ethusdt_202503_202506_train1m_baseline_pool_report.md`
- `research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_context_model_sizing_btcusdt_train3m_baseline_pool_report.md`
- `research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_context_model_lead_combo_report.md`
- `research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_ethusdt_train3m_min5_report.md`
- `research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_cross_asset_gate_search_btcusdt_train3m_min5_report.md`
- `research/entry_redesign/breakout_structure_robustness_matrix_20260518.md`
