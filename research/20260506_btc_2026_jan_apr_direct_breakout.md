# BTCUSDT 2026 Jan-Apr Direct Breakout

Scope: research-only. This removes VSL, removes `re_p`, and removes reclaim reentry. The first `baseline_plus_t3` breakout in each signal bar opens real exposure immediately at the observed 1s close.

Accounting shown below uses 2 bps/side slippage plus maker entry 2 bps and market SL/exit 4 bps.

| Timeframe | Realistic Return | Trades | Raw No Fee/Slip | 2bps Slip No Fee | Fees | Win Rate | Max DD | Entry Reasons | Breakout Median Ext | Max Entries/Bar |
|---|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|
| `1h` | -12.0251% | 845 | 4.1736% | -2.6356% | 9.5390% | 37.16% | -3.36% | `{'Direct-Breakout': 845}` | 0.7076 | 1 |
| `4h` | -5.2135% | 206 | -1.2254% | -2.8407% | 2.4141% | 36.41% | -2.84% | `{'Direct-Breakout': 206}` | 1.1255 | 1 |
| `1d` | -0.2852% | 34 | 0.3944% | 0.1225% | 0.4081% | 44.12% | -1.41% | `{'Direct-Breakout': 34}` | 1.2796 | 1 |

## Breakout Attribution

| Timeframe | Shape | Trades | Win Rate | Avg PnL | Median PnL | Net PnL | Profit Factor |
|---|---|---:|---:|---:|---:|---:|---:|
| `1h` | `original_t2` | 654 | 38.23% | -0.0046% | -0.1417% | -612.28 | 0.9667 |
| `1h` | `t3_swing` | 191 | 33.51% | -0.0536% | -0.1825% | -2,023.30 | 0.6654 |
| `4h` | `original_t2` | 159 | 37.11% | -0.0864% | -0.3568% | -2,719.49 | 0.6995 |
| `4h` | `t3_swing` | 47 | 34.04% | -0.0126% | -0.3353% | -121.24 | 0.9574 |
| `1d` | `original_t2` | 27 | 44.44% | 0.0330% | -0.8248% | 167.59 | 1.0445 |
| `1d` | `t3_swing` | 7 | 42.86% | -0.0287% | -0.9960% | -45.07 | 0.9517 |

## Files

- Summary JSON: `research/btc_2026_jan_apr_direct_breakout_summary.json`
- `1h` ledger: `research/tmp_btc_2026_jan_apr_direct_breakout_1h_observed_close_ledger.csv`
- `4h` ledger: `research/tmp_btc_2026_jan_apr_direct_breakout_4h_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_btc_2026_jan_apr_direct_breakout_1d_observed_close_ledger.csv`
