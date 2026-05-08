# ETHUSDT Q1 2026 30m VSL Bar-Guard Friction Compare

Scope: research-only. `re_p` is not used for fills. Virtual VSL is zero-notional and does not count as a trade. A new Zero-Initial virtual can only be armed while `trades_in_bar == 0`.

| Mode | Net Return | Trades | Raw Price PnL No Fee/Slippage | Fee | Slippage Drag | Total Friction | Entry Reasons | Multi-ZI Bars | Max Entries/Bar |
|---|---:|---:|---:|---:|---:|---:|---|---:|---:|
| `no_downstream` | -23.02% | 432 | -0.2327% | 15.1895% | 7.5947% | 22.7842% | `{'Zero-Initial-Reentry': 432}` | 0 | 1 |
| `with_downstream` | -43.67% | 1153 | 0.5151% | 29.4538% | 14.7269% | 44.1806% | `{'SL-Reentry': 801, 'Zero-Initial-Reentry': 352}` | 0 | 3 |

- Summary JSON: `research/eth_2026_q1_30m_vsl_0p55_turn_0p1_entrysl_0p3_bar_guard_friction_compare_summary.json`
