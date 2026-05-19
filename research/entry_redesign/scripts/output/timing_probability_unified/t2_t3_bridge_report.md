# T2/T3 Bridge Runner - 2026-05-18

Scope: research-only. This bridge report normalizes current T2 adverse10 event-ledger results and strict T3 lifecycle split results into percent units on the same month axis. It does not claim the two contracts are additive.

## Candidate Summary

| Family | Candidate | Contract | Scope | Calendar Sum | Delta | Worst Month | Neg Months | Trades | Read |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| t2_event_ledger | `canonical_lead` | next_adverse_xslip10bps | ETHUSDT only | 22.971648% | 0.000000% | 0.000000% | 0 | 62 | current production-aligned T2 lead under adverse10 |
| t2_event_ledger | `low_eff_rf_rank_median_000_combo` | next_adverse_xslip10bps | ETHUSDT only | 28.287125% | 5.315477% | -0.473563% | 2 | 89 | preferred hard-select additive T2 expansion to falsify |
| t2_event_ledger | `wf3_low_eff_low_atr_ctx4h_up_hard_combo` | next_adverse_xslip10bps | ETHUSDT only | 27.604805% | 4.633157% | -0.184894% | 1 | 77 | hard 4h context leg, useful proxy for scaled context control |
| t2_event_ledger | `wf3_low_eff_low_atr_ctx4h_scaled025_combo` | next_adverse_xslip10bps | ETHUSDT only | 27.768565% | 4.796917% | -0.137868% | 1 | 103 | derived per-trade scaled ledger; best current T2 risk-control leg |
| t3_strict_lifecycle | `t3_min_hold_sl_60m_t3_split` | strict_next_second_cross lifecycle | ETHUSDT/BTCUSDT T3 split | 3.845840% | n/a | -0.228120% | 2 | 100 | watch-only T3 risk-shaping leg; not additive to T2 event ledger yet |

## Monthly Bridge

| Month | T2 Lead Adverse10 | T2 Low-Eff RF Combo | T2 Extra | T2 Ctx4h Scaled | Ctx4h Extra | T3 60m Strict Split | Read |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 2025-06 | 4.493650% | 4.493650% | 0.000000% | 4.493650% | 0.000000% | -0.057750% | T3 split weakens a positive T2 month under strict lifecycle |
| 2025-07 | 4.640439% | 4.640439% | 0.000000% | 4.640439% | 0.000000% | 0.679510% | both positive; bridge replay should test coexistence |
| 2025-08 | 7.341680% | 7.341680% | 0.000000% | 7.341680% | 0.000000% | 0.127230% | both positive; bridge replay should test coexistence |
| 2025-09 | 5.100058% | 6.058320% | 0.958262% | 6.058320% | 0.958262% | 1.022350% | both positive; bridge replay should test coexistence |
| 2025-10 | 1.395821% | 1.395821% | 0.000000% | 1.074811% | -0.321010% | 0.618490% | both positive; bridge replay should test coexistence |
| 2025-11 | 0.000000% | 1.557188% | 1.557188% | 0.076992% | 0.076992% | 0.168500% | both positive; bridge replay should test coexistence |
| 2025-12 | 0.000000% | -0.473563% | -0.473563% | -0.137868% | -0.137868% | -0.228120% | both weak; residual-risk month |
| 2026-01 | 0.000000% | -0.372734% | -0.372734% | 0.319687% | 0.319687% | 0.059650% | T3 split offsets a weak T2 month, but contracts differ |
| 2026-02 | 0.000000% | 0.604408% | 0.604408% | 0.338019% | 0.338019% | 0.772600% | both positive; bridge replay should test coexistence |
| 2026-03 | 0.000000% | 1.982591% | 1.982591% | 2.538148% | 2.538148% | 0.636550% | both positive; bridge replay should test coexistence |
| 2026-04 | 0.000000% | 1.059325% | 1.059325% | 1.024687% | 1.024687% | 0.046830% | both positive; bridge replay should test coexistence |

## Data Caveat

- The T2 lead adverse10 per-trade artifact used by this bridge has active canonical lead rows in: 2025-06, 2025-07, 2025-08, 2025-09, 2025-10. Later months are zero in this bridge table because the current per-trade source is sparse, not because the retrain-forward lead has no value.
- Use `t2_t3_merge_matrix_report.md` for the production8 retrain-forward summary; use this bridge for month-axis comparison of currently available trade ledgers.

## Generated Ledgers

- Scaled ctx4h extra ledger: `/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_lead_combo_extra_adverse10_trades_wf3_low_eff_low_atr_ctx4h_scaled025.csv`
- Scaled ctx4h combo ledger: `/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_lead_combo_combo_adverse10_trades_wf3_low_eff_low_atr_ctx4h_scaled025.csv`

## Decision

- `low_eff_rf_rank_median_000_combo` remains the cleanest T2 additive leg to falsify under adverse10.
- `wf3_low_eff_low_atr_ctx4h_scaled025_combo` now has a generated per-month scaled ledger and is the cleaner T2 risk-control leg versus hard ctx4h.
- `t3_min_hold_sl_60m_t3_split` is a watch-only strict lifecycle leg; its positive split cannot be added to T2 adverse10 without a unified lifecycle bridge.
- Next implementation target: refresh T3 strict JSON from a long replay and then build a true unified lifecycle bridge.
