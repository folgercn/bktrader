# BTCUSDT 2026 Jan-Apr Direct Breakout

Scope: research-only. This removes VSL, removes `re_p`, and removes reclaim reentry. The first `baseline_plus_t3` breakout in each signal bar opens real exposure immediately at the observed 1s close.

Accounting shown below uses 2 bps/side slippage plus maker entry 2 bps and market SL/exit 4 bps.

| Timeframe | Variant | Schedule | Realistic Return | Trades | Raw No Fee/Slip | 2bps Slip No Fee | Fees | Win Rate | Max DD | Exit Reasons | Avg Hold | Median Hold | Breakout Median Ext | Max Entries/Bar |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| `1h` | `20_10` | `[0.2, 0.1]` | -16.5001% | 1504 | 5.6308% | -3.8494% | 12.9134% | 14.03% | -4.03% | `InitialSL:1293, TrailingSL:211, PT:0` | 364.34s | 64.00s | 0.7611 | 2 |

## Exit Hold Diagnostics

| Timeframe | Variant | Exit Reason | Trades | Avg Hold | Median Hold | Win Rate |
|---|---|---|---:|---:|---:|---:|
| `1h` | `20_10` | `InitialSL` | 1293 | 214.93s | 48.00s | 0.00% |
| `1h` | `20_10` | `TrailingSL` | 211 | 1279.96s | 721.00s | 100.00% |
| `1h` | `20_10` | `PT` | 0 | 0.00s | 0.00s | 0.00% |

## Trade Slot Diagnostics

| Timeframe | Variant | Slot | Trades | Realistic Contribution | Raw Contribution | 2bps Slip Contribution | Fees | Win Rate | Avg Hold | Median Hold | Exit Reasons |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `1h` | `20_10` | 0 | 847 | -10.5538% | 5.5823% | -1.3291% | 9.3024% | 16.06% | 391.11s | 63.00s | `{'InitialSL': 711, 'TrailingSL': 136}` |
| `1h` | `20_10` | 1 | 657 | -5.9463% | 0.0485% | -2.5203% | 3.6110% | 11.42% | 329.83s | 65.00s | `{'InitialSL': 582, 'TrailingSL': 75}` |

## Breakout Attribution

| Timeframe | Variant | Shape | Trades | Win Rate | Avg PnL | Median PnL | Net PnL | Profit Factor |
|---|---|---|---:|---:|---:|---:|---:|---:|
| `1h` | `20_10` | `original_t2` | 1162 | 14.80% | -0.0168% | -0.0795% | -2,166.48 | 0.8392 |
| `1h` | `20_10` | `t3_swing` | 342 | 11.40% | -0.0376% | -0.0852% | -1,682.94 | 0.5957 |

## Files

- Summary JSON: `research/btc_2026_jan_apr_1h_direct_breakout_sl0p1_second_breakout_summary.json`
- `1h` ledger: `research/tmp_btc_2026_jan_apr_1h_direct_breakout_sl0p1_second_breakout_1h_20_10_observed_close_ledger.csv`
