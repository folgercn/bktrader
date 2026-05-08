# BTCUSDT 2026 Jan-Apr Direct Breakout

Scope: research-only. This removes VSL, removes `re_p`, and removes reclaim reentry. The first configured structural breakout in each signal bar opens real exposure immediately at the observed 1s close.

Accounting shown below uses 2 bps/side slippage plus maker entry 2 bps and market SL/exit 4 bps.

| Timeframe | Shape | Variant | Schedule | Realistic Return | Trades | Raw No Fee/Slip | 2bps Slip No Fee | Fees | Win Rate | Max DD | Exit Reasons | Avg Hold | Median Hold | Breakout Median Ext | Max Entries/Bar |
|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| `1d` | `original_t2` | `20_10` | `[0.2, 0.1]` | -0.1587% | 27 | 0.3813% | 0.1653% | 0.3244% | 44.44% | -1.18% | `InitialSL:15, TrailingSL:12, PT:0` | 40437.96s | 14418.00s | 1.2903 | 1 |
| `1d` | `baseline_plus_t3` | `20_10` | `[0.2, 0.1]` | -0.2852% | 34 | 0.3944% | 0.1225% | 0.4081% | 44.12% | -1.41% | `InitialSL:19, TrailingSL:15, PT:0` | 40592.29s | 22213.50s | 1.2796 | 1 |

## Exit Hold Diagnostics

| Timeframe | Shape | Variant | Exit Reason | Trades | Avg Hold | Median Hold | Win Rate |
|---|---|---|---|---:|---:|---:|---:|
| `1d` | `original_t2` | `20_10` | `InitialSL` | 15 | 37641.93s | 7836.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `TrailingSL` | 12 | 43933.00s | 48817.00s | 100.00% |
| `1d` | `original_t2` | `20_10` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `InitialSL` | 19 | 36650.26s | 8772.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `TrailingSL` | 15 | 45585.53s | 45377.00s | 100.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `PT` | 0 | 0.00s | 0.00s | 0.00% |

## Trade Slot Diagnostics

| Timeframe | Shape | Variant | Slot | Trades | Realistic Contribution | Raw Contribution | 2bps Slip Contribution | Fees | Win Rate | Avg Hold | Median Hold | Exit Reasons |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `1d` | `original_t2` | `20_10` | 0 | 27 | -0.1587% | 0.3813% | 0.1653% | 0.3244% | 44.44% | 40437.96s | 14418.00s | `{'InitialSL': 15, 'TrailingSL': 12}` |
| `1d` | `baseline_plus_t3` | `20_10` | 0 | 34 | -0.2852% | 0.3944% | 0.1225% | 0.4081% | 44.12% | 40592.29s | 22213.50s | `{'InitialSL': 19, 'TrailingSL': 15}` |

## MFE/MAE Diagnostics

| Timeframe | Shape | Variant | Group | Trades | Median MFE | Median MAE | Median MFE ATR | Median MAE ATR | MFE >= 10bps | MFE >= 20bps | Median Realized |
|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `1d` | `original_t2` | `20_10` | `overall` | 27 | 126.0018bps | 81.8675bps | 0.3910 | 0.3009 | 96.30% | 88.89% | -82.4780bps |
| `1d` | `original_t2` | `20_10` | `exit:InitialSL` | 15 | 60.5594bps | 117.9360bps | 0.1498 | 0.3039 | 93.33% | 80.00% | -118.5987bps |
| `1d` | `original_t2` | `20_10` | `exit:TrailingSL` | 12 | 268.0778bps | 46.1397bps | 0.6936 | 0.1167 | 100.00% | 100.00% | 142.2156bps |
| `1d` | `original_t2` | `20_10` | `filled:original_t2` | 27 | 126.0018bps | 81.8675bps | 0.3910 | 0.3009 | 96.30% | 88.89% | -82.4780bps |
| `1d` | `baseline_plus_t3` | `20_10` | `overall` | 34 | 128.0496bps | 90.5831bps | 0.3560 | 0.3011 | 97.06% | 91.18% | -85.2727bps |
| `1d` | `baseline_plus_t3` | `20_10` | `exit:InitialSL` | 19 | 60.5594bps | 117.9360bps | 0.1498 | 0.3069 | 94.74% | 84.21% | -118.2592bps |
| `1d` | `baseline_plus_t3` | `20_10` | `exit:TrailingSL` | 15 | 264.3545bps | 45.1870bps | 0.6635 | 0.1133 | 100.00% | 100.00% | 132.2913bps |
| `1d` | `baseline_plus_t3` | `20_10` | `filled:original_t2` | 27 | 126.0018bps | 81.8675bps | 0.3910 | 0.3009 | 96.30% | 88.89% | -82.4780bps |
| `1d` | `baseline_plus_t3` | `20_10` | `filled:t3_swing` | 7 | 140.0142bps | 117.1793bps | 0.3201 | 0.3142 | 100.00% | 100.00% | -99.6047bps |

## Breakout Attribution

| Timeframe | Configured Shape | Variant | Filled Shape | Trades | Win Rate | Avg PnL | Median PnL | Net PnL | Profit Factor |
|---|---|---|---|---:|---:|---:|---:|---:|---:|
| `1d` | `original_t2` | `20_10` | `original_t2` | 27 | 44.44% | 0.0330% | -0.8248% | 165.30 | 1.0439 |
| `1d` | `baseline_plus_t3` | `20_10` | `original_t2` | 27 | 44.44% | 0.0330% | -0.8248% | 167.59 | 1.0445 |
| `1d` | `baseline_plus_t3` | `20_10` | `t3_swing` | 7 | 42.86% | -0.0287% | -0.9960% | -45.07 | 0.9517 |

## Files

- Summary JSON: `research/btc_2026_jan_apr_1d_direct_breakout_shape_compare_mfe_summary.json`
- `1d` ledger: `research/tmp_btc_2026_jan_apr_1d_direct_breakout_shape_compare_mfe_1d_original_t2_20_10_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_btc_2026_jan_apr_1d_direct_breakout_shape_compare_mfe_1d_baseline_plus_t3_20_10_observed_close_ledger.csv`
