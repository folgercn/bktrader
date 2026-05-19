# T2/T3 Merge Matrix - 2026-05-18

Scope: research-only. This report evaluates whether the current ETH pretouch breakout research line can be merged with the Kiro `t2-t3-union-strategy` line. It does not change live defaults.

## Key Read

- Merge the T3 framework and strict-fill audit into the current research process.
- Do not merge historical T3 lifecycle headline returns; they depended on optimistic `re_p` fills.
- Treat `t3_min_hold_sl_60m` as a strict-fill watch leg inside research union tests, not as a default stop-loss rule.
- Current strict lifecycle pass-bucket read: original_t2 is the main drag; T2 disabled + T3 60m is the new strict floor to beat.
- Exact RF-event injection is now tested and does not beat the T2-disabled floor; the next probability-model lever must change post-touch entry, not only event selection.

## Metric Matrix

| Family | Candidate | Primary Metric | Value | Delta | Worst Metric | Worst | Neg | Trades | Read |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| current_breakout_expansion | `low_eff_rf_rank_median_000` | combo_adverse10_calendar_sum | 0.282871 | 0.053155 | combo_adverse10_worst_sm | -0.004736 | 2 | 89 | late-ETH additive candidate only; still blocked by early ETH/BTC falsification |
| current_breakout_expansion | `wide_rf_binary_000` | combo_adverse10_calendar_sum | 0.282091 | 0.052375 | combo_adverse10_worst_sm | -0.042275 | 2 | 119 | useful sensitivity row, not the preferred merge leg |
| current_breakout_expansion | `low_eff_rf_rank_q60_000` | combo_adverse10_calendar_sum | 0.267627 | 0.037911 | combo_adverse10_worst_sm | -0.004736 | 2 | 82 | useful sensitivity row, not the preferred merge leg |
| current_breakout_retrain | `canonical_only` | forward_adverse10_calendar_sum | 0.299716 | 0.000000 | forward_adverse10_worst_sm | 0.002688 | 0 | 78 | reference row for current breakout line |
| current_breakout_retrain | `combo_wf3_low_eff_low_atr` | forward_adverse10_calendar_sum | 0.408512 | 0.108797 | forward_adverse10_worst_sm | -0.002244 | 1 | 115 | reference row for current breakout line |
| current_breakout_retrain | `combo_wf3_low_eff_low_atr_ctx4h_scaled025` | forward_adverse10_calendar_sum | 0.370266 | 0.070551 | forward_adverse10_worst_sm | 0.006097 | 0 | 115 | best current ETH-local sizing-control shape |
| current_breakout_retrain | `combo_wf3_low_eff_low_atr_ctx12h_up` | forward_adverse10_calendar_sum | 0.371748 | 0.072032 | forward_adverse10_worst_sm | 0.011073 | 0 | 91 | reference row for current breakout line |
| t3_strict_lifecycle | `strict_baseline` | calendar_silo_sum_pct | -30.980000 | 0.000000 | worst_calendar_silo_pct | -2.190000 | 22 | 659 | strict lifecycle baseline/control; source=/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/t2_t3_merge_t3_strict_baseline_extended/t3_lifecycle_stability_summary.json |
| t3_strict_lifecycle | `t3_min_hold_sl_60m` | calendar_silo_sum_pct | -24.480000 | 6.500000 | worst_calendar_silo_pct | -2.150000 | 22 | 610 | strict T3 watch leg; positive T3 split but total lifecycle still weak; source=/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/t2_t3_merge_t3_exit_60m_extended/t3_lifecycle_exit_sweep_summary.json |
| t2_strict_lifecycle_context_sizing | `original_t2_ctx4h_scaled025` | calendar_silo_sum_pct | -18.050000 | 12.930000 | worst_calendar_silo_pct | -1.290000 | 22 | 659 | strict lifecycle bridge for the current ctx4h scaled-sizing idea; T2 PnL -3.596480%, T3 PnL -1.603750%, size fails 2899; T3 overrides={}; source=/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/t2_lifecycle_context_sizing_extended/t2_lifecycle_context_sizing_summary.json |
| t2_t3_strict_lifecycle_union | `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | calendar_silo_sum_pct | -11.680000 | n/a | worst_calendar_silo_pct | -1.100000 | 21 | 610 | strict lifecycle union row with T2 ctx4h sizing and T3 exit overrides; T2 PnL -3.513910%, T3 PnL 3.854600%, size fails 2895; T3 overrides={"min_hold_seconds_before_sl": 3600.0}; source=/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/t2_t3_lifecycle_union_combined/t2_lifecycle_context_sizing_summary.json |
| t2_t3_strict_lifecycle_multiplier_sensitivity | `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | calendar_silo_sum_pct | -9.130000 | n/a | worst_calendar_silo_pct | -0.880000 | 20 | 610 | strict lifecycle union row with T2 ctx4h sizing and T3 exit overrides; T2 PnL -2.720670%, T3 PnL 3.856350%, size fails 2895; T3 overrides={"min_hold_seconds_before_sl": 3600.0}; source=/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/t2_ctx4h_multiplier_sensitivity_extended/t2_lifecycle_context_sizing_summary.json |
| t2_t3_strict_lifecycle_multiplier_sensitivity | `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | calendar_silo_sum_pct | -7.410000 | n/a | worst_calendar_silo_pct | -0.740000 | 19 | 255 | strict lifecycle union row with T2 ctx4h sizing and T3 exit overrides; T2 PnL -2.191290%, T3 PnL 3.857520%, size fails 2895; T3 overrides={"min_hold_seconds_before_sl": 3600.0}; source=/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/t2_ctx4h_multiplier_sensitivity_extended/t2_lifecycle_context_sizing_summary.json |
| t2_t3_strict_lifecycle_skipfail | `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | calendar_silo_sum_pct | -7.900000 | n/a | worst_calendar_silo_pct | -0.740000 | 19 | 267 | strict lifecycle union row with T2 ctx4h sizing and T3 exit overrides; T2 PnL -2.383800%, T3 PnL 3.983430%, size fails 2958; T3 overrides={"min_hold_seconds_before_sl": 3600.0}; source=/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/t2_ctx4h_skipfail_extended/t2_lifecycle_context_sizing_summary.json |
| t2_t3_strict_lifecycle_pass_bucket_reference | `ctx4h_skipfail_t3_60m` | calendar_silo_sum_pct | -7.900000 | n/a | worst_calendar_silo_pct | -0.740000 | 19 | 267 | strict lifecycle union row with T2 ctx4h sizing and T3 exit overrides; T2 PnL -2.383800%, T3 PnL 3.983430%, size fails 2958; T3 overrides={"min_hold_seconds_before_sl": 3600.0}; source=/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/t2_pass_bucket_gate_reference_extended/t2_lifecycle_pass_bucket_gate_summary.json |
| t2_t3_strict_lifecycle_pass_bucket_gate | `pretouch_max900_skipfail_t3_60m` | calendar_silo_sum_pct | -4.960000 | n/a | worst_calendar_silo_pct | -0.680000 | 18 | 207 | strict lifecycle union row with T2 ctx4h sizing and T3 exit overrides; T2 PnL -1.434690%, T3 PnL 3.985080%, size fails 4429; T3 overrides={"min_hold_seconds_before_sl": 3600.0}; source=/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/t2_pass_bucket_gate_pretouch900_extended/t2_lifecycle_pass_bucket_gate_summary.json |
| t2_t3_strict_lifecycle_t2_disabled_floor | `t2_disabled_t3_60m` | calendar_silo_sum_pct | -0.190000 | n/a | worst_calendar_silo_pct | -0.460000 | 12 | 104 | strict lifecycle union row with T2 ctx4h sizing and T3 exit overrides; T2 PnL 0.000000%, T3 PnL 3.988470%, size fails 3639; T3 overrides={"min_hold_seconds_before_sl": 3600.0}; source=/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/t2_pass_bucket_gate_t2_disabled_extended/t2_lifecycle_pass_bucket_gate_summary.json |
| t2_t3_strict_lifecycle_external_probability | `low_eff_rf_rank_median_external_t3_60m` | calendar_silo_sum_pct | -0.590000 | n/a | worst_calendar_silo_pct | -0.460000 | 12 | 113 | strict lifecycle bridge for probability-selected external T2 events; external events 28, locks 27, trades 9, external PnL -0.121280%, T3 PnL 3.988540%; source=/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/t2_external_low_eff_rf_median_extended/t2_lifecycle_external_event_gate_summary.json |

## T3 Exposure Notes

- t3_min_hold_sl_60m: T3 PnL 3.845829%, ex-final-mark 3.655098%, FinalMark 0.190731%/1, T3 DD -0.254383%, p90 hold 6245.20s, worst MAE -477.5244bp (source=/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/t2_t3_merge_t3_exposure_60m_extended/t3_lifecycle_exposure_audit_summary.json)

## Merge Decisions

| Surface | Decision | Reason | Next Action |
| --- | --- | --- | --- |
| T2 canonical + current breakout expansion | merge as additive candidate | Both are event-ledger/retrain outputs under next_adverse_xslip10bps and already de-overlap by (signal_start, side). | Keep low_eff_rf_rank_median_000 and ctx4h_scaled025 as the current ETH-local falsification set. |
| T2 canonical + T3 generator/quality/probability harness | merge the framework and keep strict lifecycle as the comparison contract | T3 structure is mutually exclusive with T2 and uses compatible features; the bridge replay now prevents adding adverse10 ledger returns to lifecycle returns. | Use the strict lifecycle union row as the research comparison surface; keep adverse10 ledgers as diagnostic provenance only. |
| T3 historical lifecycle positives | do not merge | Historical reentry_window allowed optimistic same-second/non-cross re_p fills; strict replay invalidated the large T3 contribution. | Treat Task 14-17 historical T3 positive results as suspect-only provenance. |
| T3 min_hold_sl_60m | watch-only merge leg | It improves strict T3 split and survives final-mark exclusion, but increases exposure and total strict lifecycle remains negative. | Keep it inside strict lifecycle union tests only; do not promote it to live stop semantics. |
| T2 ctx4h sizing + T3 min_hold_sl_60m | promising bridge result, not promotion-ready | The two risk-shaping layers stack under strict_next_second_cross, but the combined fixed-calendar lifecycle remains negative. | Attack residual T2 loss and BTC drag before any live/default discussion. |
| T2 ctx4h fail multiplier sensitivity | zero-fail exposure is the best strict research row; skip-fail is the cleaner executable challenger | Reducing failed-context original_t2 exposure improves total lifecycle and worst silo; skip-fail keeps most of the improvement without relying on zero-notional lock occupancy. | Use skip-fail as the cleaner next implementation surface, while retaining scaled000 as the upper-bound research control. |
| T2 pass/full bucket | reject current original_t2 pass bucket as a trading leg | Strict lifecycle attribution shows all simple pass buckets remain fee-negative; disabling original_t2 beats ctx4h skip-fail and pre_touch<=900. | Do not spend more sweeps on ctx4h/pass timing knobs; either disable original_t2 or implement an exact event-time probability/RF lifecycle hook. |
| low_eff/RF external event hook | reject as-is under strict lifecycle | The RF-selected event set can be injected as explicit locks, but only a minority become strict reentry trades and the external leg is still net negative. | Use RF only with a post-touch entry/confirmation redesign; do not treat the adverse10 event-ledger result as lifecycle-ready. |

## Lifecycle Contract Readiness

| Candidate | Current Contract | Strict Lifecycle Ready | Blocker | Next Action |
| --- | --- | --- | --- | --- |
| `low_eff_rf_rank_median_000_combo` | T2 next_adverse_xslip10bps event ledger | no | No executable lifecycle breakout-lock rule yet; current row is additive ledger selection. | Translate the selected low-eff/RF leg into a replay-time original_t2 quality gate or keep it ledger-only. |
| `wf3_low_eff_low_atr_ctx4h_scaled025_combo` | T2 next_adverse_xslip10bps event ledger with derived scaled sizing | yes | Bridge hook has a full fixed-calendar lifecycle result for the original_t2 ctx4h scaled-sizing approximation; exact event-ledger parity is still pending. | Use the lifecycle bridge result to decide whether ctx4h sizing deserves exact parity work or should be rejected. |
| `strict_baseline` | baseline_plus_t3 strict_next_second_cross lifecycle | yes | None for T3 baseline; it is already the strict lifecycle control. | Use as lifecycle control, not as a profitability benchmark for adverse10 T2 ledgers. |
| `t3_min_hold_sl_60m` | baseline_plus_t3 strict_next_second_cross lifecycle with T3-only exit override | yes | Watch-only because total lifecycle remains negative and exposure risk must stay visible. | Only compare against T2 once T2 expansion is replayed through the same lifecycle contract. |

## Next Promotion Contract

1. Use strict lifecycle over ETHUSDT/BTCUSDT `2025-06..2026-04` as the only promotion-comparable contract.
2. Keep `next_adverse_xslip10bps` T2 ledgers as diagnostics; do not add them to lifecycle returns.
3. Keep long context filters bounded to 4h-12h and avoid month-level gates.
4. Next research lever: attack remaining pass bucket loss or implement an exact low-eff/RF lifecycle hook.
5. Promotion gate: no `re_p`, no live/default change unless total lifecycle, worst silo, and exposure audit all improve.
