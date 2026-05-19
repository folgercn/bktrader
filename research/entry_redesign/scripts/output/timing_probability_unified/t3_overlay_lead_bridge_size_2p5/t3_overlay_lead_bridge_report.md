# T3 Overlay Lead Bridge

Research-only additive bridge from the T3 direct-entry findings back to the ETH pretouch research lead.

- Lead adverse ledger: `/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_lead_combo_lead_adverse10_trades.csv`
- Lead same-close ledger: `/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_lead_combo_lead_same_close_trades.csv`
- T3 lifecycle summary: `research/entry_redesign/scripts/output/timing_probability_unified/t3_filtered_external_all_speed_ge035_eth_next_adverse_size2p5_extended/t3_filtered_external_event_lifecycle_summary.json`
- T3 entry mode: `next_second_adverse`
- T3 size scale: `2.5`
- T3 schedule: `[0.5, 0.25]`

## Summary

| Variant | Calendar Sum | Worst Month | Neg Months | Trades |
|---|---:|---:|---:|---:|
| `lead_same_close` | 30.217222% | 0.000000% | 0 | 62 |
| `lead_adverse10` | 22.971648% | 0.000000% | 0 | 62 |
| `t3_overlay_eth_adverse_size2` | 14.300000% | -1.040000% | 3 | 163 |
| `lead_same_close_plus_t3_overlay` | 44.517222% | -0.590000% | 1 | 225 |
| `lead_adverse10_plus_t3_overlay` | 37.271648% | -0.590000% | 1 | 225 |

## Read

- This supports using T3 quality/direct-entry lessons as a lead enhancement layer, not as a replacement strategy.
- The combined rows are additive fixed-calendar accounting; promotion still requires exposure, drawdown, final-mark and slippage stress.
- The current overlay is ETH-only; BTC stayed negative in direct T3 tests and is intentionally excluded here.
