# BTCUSDT 2026 Jan-Apr Direct Breakout

Scope: research-only. This removes VSL, removes `re_p`, and removes reclaim reentry. The first `baseline_plus_t3` breakout in each signal bar opens real exposure immediately at the observed 1s close.

Accounting shown below uses 2 bps/side slippage plus maker entry 2 bps and market SL/exit 4 bps.

| Timeframe | Realistic Return | Trades | Raw No Fee/Slip | 2bps Slip No Fee | Fees | Win Rate | Max DD | Exit Reasons | Avg Hold | Median Hold | Breakout Median Ext | Max Entries/Bar |
|---|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| `30min` | -23.6796% | 1646 | 6.0768% | -7.0110% | 17.2344% | 34.45% | -7.61% | `InitialSL:1079, TrailingSL:567, PT:0` | 758.79s | 397.00s | 0.5554 | 1 |

## Exit Hold Diagnostics

| Timeframe | Exit Reason | Trades | Avg Hold | Median Hold | Win Rate |
|---|---|---:|---:|---:|---:|
| `30min` | `InitialSL` | 1079 | 626.32s | 297.00s | 0.00% |
| `30min` | `TrailingSL` | 567 | 1010.88s | 605.00s | 100.00% |
| `30min` | `PT` | 0 | 0.00s | 0.00s | 0.00% |

## Breakout Attribution

| Timeframe | Shape | Trades | Win Rate | Avg PnL | Median PnL | Net PnL | Profit Factor |
|---|---|---:|---:|---:|---:|---:|---:|
| `30min` | `original_t2` | 1287 | 34.65% | -0.0223% | -0.1104% | -5,552.95 | 0.7893 |
| `30min` | `t3_swing` | 359 | 33.70% | -0.0209% | -0.1116% | -1,458.10 | 0.8059 |

## Files

- Summary JSON: `research/btc_2026_jan_apr_30min_direct_breakout_summary.json`
- `30min` ledger: `research/tmp_btc_2026_jan_apr_30min_direct_breakout_30min_observed_close_ledger.csv`
