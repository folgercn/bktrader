# ETHUSDT 2026 Jan-Apr Direct Breakout

Scope: research-only. This removes VSL, removes `re_p`, and removes reclaim reentry. The first configured structural breakout in each signal bar opens real exposure immediately at the observed 1s close.

Accounting shown below uses 2 bps/side slippage plus maker entry 2 bps and market SL/exit 4 bps.

| Timeframe | Shape | Variant | Schedule | Realistic Return | Trades | Raw No Fee/Slip | 2bps Slip No Fee | Fees | Win Rate | Max DD | Exit Reasons | Avg Hold | Median Hold | Breakout Median Ext | Max Entries/Bar |
|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| `1d` | `original_t2` | `20_10` | `[0.2, 0.1]` | -1.6688% | 27 | -1.1356% | -1.3496% | 0.3206% | 37.04% | -2.85% | `InitialSL:17, TrailingSL:10, PT:0` | 33141.67s | 13416.00s | 0.9552 | 1 |
| `1d` | `baseline_plus_t3` | `20_10` | `[0.2, 0.1]` | -0.7961% | 34 | -0.1189% | -0.3905% | 0.4067% | 44.12% | -2.74% | `InitialSL:19, TrailingSL:15, PT:0` | 34659.44s | 13597.00s | 1.0818 | 1 |

## Exit Hold Diagnostics

| Timeframe | Shape | Variant | Exit Reason | Trades | Avg Hold | Median Hold | Win Rate |
|---|---|---|---|---:|---:|---:|---:|
| `1d` | `original_t2` | `20_10` | `InitialSL` | 17 | 26816.29s | 6322.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `TrailingSL` | 10 | 43894.80s | 34735.00s | 100.00% |
| `1d` | `original_t2` | `20_10` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `InitialSL` | 19 | 24503.32s | 6322.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `TrailingSL` | 15 | 47523.87s | 28115.00s | 100.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `PT` | 0 | 0.00s | 0.00s | 0.00% |

## Trade Slot Diagnostics

| Timeframe | Shape | Variant | Slot | Trades | Realistic Contribution | Raw Contribution | 2bps Slip Contribution | Fees | Win Rate | Avg Hold | Median Hold | Exit Reasons |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `1d` | `original_t2` | `20_10` | 0 | 27 | -1.6688% | -1.1356% | -1.3496% | 0.3206% | 37.04% | 33141.67s | 13416.00s | `{'InitialSL': 17, 'TrailingSL': 10}` |
| `1d` | `baseline_plus_t3` | `20_10` | 0 | 34 | -0.7961% | -0.1189% | -0.3905% | 0.4067% | 44.12% | 34659.44s | 13597.00s | `{'InitialSL': 19, 'TrailingSL': 15}` |

## MFE/MAE Diagnostics

| Timeframe | Shape | Variant | Group | Trades | Median MFE | Median MAE | Median MFE ATR | Median MAE ATR | MFE >= 10bps | MFE >= 20bps | Median Realized |
|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `1d` | `original_t2` | `20_10` | `overall` | 27 | 179.3661bps | 144.8341bps | 0.2993 | 0.3016 | 88.89% | 81.48% | -142.1100bps |
| `1d` | `original_t2` | `20_10` | `exit:InitialSL` | 17 | 74.3600bps | 156.9761bps | 0.1693 | 0.3034 | 82.35% | 70.59% | -158.1506bps |
| `1d` | `original_t2` | `20_10` | `exit:TrailingSL` | 10 | 323.4158bps | 38.2759bps | 0.7203 | 0.0869 | 100.00% | 100.00% | 177.5751bps |
| `1d` | `original_t2` | `20_10` | `filled:original_t2` | 27 | 179.3661bps | 144.8341bps | 0.2993 | 0.3016 | 88.89% | 81.48% | -142.1100bps |
| `1d` | `baseline_plus_t3` | `20_10` | `overall` | 34 | 181.9048bps | 143.1884bps | 0.3540 | 0.3009 | 91.18% | 85.29% | -134.5906bps |
| `1d` | `baseline_plus_t3` | `20_10` | `exit:InitialSL` | 19 | 74.3600bps | 166.8528bps | 0.1693 | 0.3034 | 84.21% | 73.68% | -164.1143bps |
| `1d` | `baseline_plus_t3` | `20_10` | `exit:TrailingSL` | 15 | 325.5481bps | 33.6966bps | 0.6971 | 0.0954 | 100.00% | 100.00% | 172.9692bps |
| `1d` | `baseline_plus_t3` | `20_10` | `filled:original_t2` | 27 | 179.3661bps | 144.8341bps | 0.2993 | 0.3016 | 88.89% | 81.48% | -142.1100bps |
| `1d` | `baseline_plus_t3` | `20_10` | `filled:t3_swing` | 7 | 244.9413bps | 89.5015bps | 0.5889 | 0.2130 | 100.00% | 100.00% | 127.9942bps |

## Breakout Attribution

| Timeframe | Configured Shape | Variant | Filled Shape | Trades | Win Rate | Avg PnL | Median PnL | Net PnL | Profit Factor |
|---|---|---|---|---:|---:|---:|---:|---:|---:|
| `1d` | `original_t2` | `20_10` | `original_t2` | 27 | 37.04% | -0.2462% | -1.4211% | -1,349.55 | 0.7715 |
| `1d` | `baseline_plus_t3` | `20_10` | `original_t2` | 27 | 37.04% | -0.2462% | -1.4211% | -1,350.03 | 0.7729 |
| `1d` | `baseline_plus_t3` | `20_10` | `t3_swing` | 7 | 71.43% | 0.6943% | 1.2799% | 959.56 | 2.2875 |

## Files

- Summary JSON: `research/eth_2026_jan_apr_1d_direct_breakout_shape_compare_mfe_summary.json`
- `1d` ledger: `research/tmp_eth_2026_jan_apr_1d_direct_breakout_shape_compare_mfe_1d_original_t2_20_10_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_eth_2026_jan_apr_1d_direct_breakout_shape_compare_mfe_1d_baseline_plus_t3_20_10_observed_close_ledger.csv`
