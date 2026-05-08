# BTCUSDT 2026 Jan-Apr Direct Breakout

Scope: research-only. This removes VSL, removes `re_p`, and removes reclaim reentry. The first `baseline_plus_t3` breakout in each signal bar opens real exposure immediately at the observed 1s close.

Accounting shown below uses 2 bps/side slippage plus maker entry 2 bps and market SL/exit 4 bps.

| Timeframe | Variant | Schedule | Realistic Return | Trades | Raw No Fee/Slip | 2bps Slip No Fee | Fees | Win Rate | Max DD | Exit Reasons | Avg Hold | Median Hold | Breakout Median Ext | Max Entries/Bar |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| `30min` | `20_10` | `[0.2, 0.1]` | -28.7352% | 2422 | 7.0319% | -9.0377% | 20.6534% | 33.90% | -9.60% | `InitialSL:1601, TrailingSL:821, PT:0` | 709.72s | 364.00s | 0.7326 | 2 |
| `30min` | `10_20` | `[0.1, 0.2]` | -23.7752% | 2422 | 4.9622% | -7.6447% | 16.8375% | 33.90% | -7.97% | `InitialSL:1601, TrailingSL:821, PT:0` | 709.72s | 364.00s | 0.7326 | 2 |
| `30min` | `20_20` | `[0.2, 0.2]` | -33.4281% | 2422 | 8.0628% | -10.9718% | 23.8714% | 33.90% | -11.52% | `InitialSL:1601, TrailingSL:821, PT:0` | 709.72s | 364.00s | 0.7326 | 2 |

## Exit Hold Diagnostics

| Timeframe | Variant | Exit Reason | Trades | Avg Hold | Median Hold | Win Rate |
|---|---|---|---:|---:|---:|---:|
| `30min` | `20_10` | `InitialSL` | 1601 | 562.06s | 277.00s | 0.00% |
| `30min` | `20_10` | `TrailingSL` | 821 | 997.66s | 617.00s | 100.00% |
| `30min` | `20_10` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `30min` | `10_20` | `InitialSL` | 1601 | 562.06s | 277.00s | 0.00% |
| `30min` | `10_20` | `TrailingSL` | 821 | 997.66s | 617.00s | 100.00% |
| `30min` | `10_20` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `30min` | `20_20` | `InitialSL` | 1601 | 562.06s | 277.00s | 0.00% |
| `30min` | `20_20` | `TrailingSL` | 821 | 997.66s | 617.00s | 100.00% |
| `30min` | `20_20` | `PT` | 0 | 0.00s | 0.00s | 0.00% |

## Trade Slot Diagnostics

| Timeframe | Variant | Slot | Trades | Realistic Contribution | Raw Contribution | 2bps Slip Contribution | Fees | Win Rate | Avg Hold | Median Hold | Exit Reasons |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `30min` | `20_10` | 0 | 1645 | -23.0343% | 6.0475% | -7.0162% | 16.7108% | 34.41% | 758.83s | 396.00s | `{'InitialSL': 1079, 'TrailingSL': 566}` |
| `30min` | `20_10` | 1 | 777 | -5.7009% | 0.9844% | -2.0215% | 3.9426% | 32.82% | 605.73s | 319.00s | `{'InitialSL': 522, 'TrailingSL': 255}` |
| `30min` | `10_20` | 0 | 1645 | -11.9242% | 3.0053% | -3.5421% | 8.6616% | 34.41% | 758.83s | 396.00s | `{'InitialSL': 1079, 'TrailingSL': 566}` |
| `30min` | `10_20` | 1 | 777 | -11.8510% | 1.9568% | -4.1026% | 8.1759% | 32.82% | 605.73s | 319.00s | `{'InitialSL': 522, 'TrailingSL': 255}` |
| `30min` | `20_20` | 0 | 1645 | -22.3803% | 6.0890% | -6.9671% | 16.2193% | 34.41% | 758.83s | 396.00s | `{'InitialSL': 1079, 'TrailingSL': 566}` |
| `30min` | `20_20` | 1 | 777 | -11.0479% | 1.9738% | -4.0047% | 7.6522% | 32.82% | 605.73s | 319.00s | `{'InitialSL': 522, 'TrailingSL': 255}` |

## Breakout Attribution

| Timeframe | Variant | Shape | Trades | Win Rate | Avg PnL | Median PnL | Net PnL | Profit Factor |
|---|---|---|---:|---:|---:|---:|---:|---:|
| `30min` | `20_10` | `original_t2` | 1899 | 34.07% | -0.0244% | -0.1088% | -7,200.36 | 0.7774 |
| `30min` | `20_10` | `t3_swing` | 523 | 33.27% | -0.0221% | -0.1134% | -1,837.29 | 0.798 |
| `30min` | `10_20` | `original_t2` | 1899 | 34.07% | -0.0244% | -0.1088% | -6,128.84 | 0.7628 |
| `30min` | `10_20` | `t3_swing` | 523 | 33.27% | -0.0221% | -0.1134% | -1,515.84 | 0.7863 |
| `30min` | `20_20` | `original_t2` | 1899 | 34.07% | -0.0244% | -0.1088% | -8,762.01 | 0.7711 |
| `30min` | `20_20` | `t3_swing` | 523 | 33.27% | -0.0221% | -0.1134% | -2,209.76 | 0.7925 |

## Files

- Summary JSON: `research/btc_2026_jan_apr_30min_direct_breakout_second_slot_schedules_summary.json`
- `30min` ledger: `research/tmp_btc_2026_jan_apr_30min_direct_breakout_second_slot_30min_20_10_observed_close_ledger.csv`
- `30min` ledger: `research/tmp_btc_2026_jan_apr_30min_direct_breakout_second_slot_30min_10_20_observed_close_ledger.csv`
- `30min` ledger: `research/tmp_btc_2026_jan_apr_30min_direct_breakout_second_slot_30min_20_20_observed_close_ledger.csv`
