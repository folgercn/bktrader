# BTCUSDT 2026 Jan-Apr 1d VSL 0.55 Turn 0.10 Entry SL 0.30

Scope: research-only. `re_p` is not used for fills. Virtual VSL is zero-notional and does not count as a trade. A new Zero-Initial virtual can only be armed while `trades_in_bar == 0`.

Accounting shown below uses 2 bps/side slippage plus maker entry 2 bps and market SL/exit 4 bps.

| Mode | Realistic Return | Trades | Raw No Fee/Slip | 2bps Slip No Fee | Fees | Entry Reasons | Max ZI/Bar | Max Entries/Bar |
|---|---:|---:|---:|---:|---:|---|---:|---:|
| `no_downstream` | -2.1739% | 8 | -2.0167% | -2.0797% | 0.0950% | `{'Zero-Initial-Reentry': 8}` | 1 | 1 |
| `with_downstream` | -1.7549% | 29 | -1.2826% | -1.4713% | 0.2881% | `{'Zero-Initial-Reentry': 8, 'SL-Reentry': 21}` | 1 | 2 |

- Summary JSON: `research/btc_2026_jan_apr_1d_vsl_0p55_turn_0p1_entrysl_0p3_summary.json`
