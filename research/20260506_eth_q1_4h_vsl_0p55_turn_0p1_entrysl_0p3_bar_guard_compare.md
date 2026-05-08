# ETHUSDT Q1 2026 4h VSL Bar-Guard Compare

Scope: research-only. `re_p` is not used for fills. Virtual VSL is zero-notional and does not count as a trade. A new Zero-Initial virtual can only be armed while `trades_in_bar == 0`.

| Mode | Return | Trades | Win | Max DD | Entry Reasons | Multi-ZI Bars | Max ZI Per Bar | Max Entries Per Bar |
|---|---:|---:|---:|---:|---|---:|---:|---:|
| `no_downstream` | -4.78% | 64 | 31.25% | -5.34% | `{'Zero-Initial-Reentry': 64}` | 0 | 1 | 1 |
| `with_downstream` | -7.90% | 172 | 33.72% | -8.23% | `{'SL-Reentry': 128, 'Zero-Initial-Reentry': 44}` | 0 | 1 | 2 |

- Summary JSON: `research/eth_2026_q1_4h_vsl_0p55_turn_0p1_entrysl_0p3_bar_guard_compare_summary.json`
