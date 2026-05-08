# BTCUSDT 2026 Jan-Apr Direct Breakout

Scope: research-only. This removes VSL, removes `re_p`, and removes reclaim reentry. The first configured structural breakout in each signal bar opens real exposure immediately at the observed 1s close. Exit variants keep the entry semantics fixed and only change stop/target handling after entry.

Accounting shown below uses 2 bps/side slippage plus maker entry 2 bps and market SL/exit 4 bps.

| Timeframe | Shape | Variant | Exit Policy | Schedule | Realistic Return | Trades | Raw No Fee/Slip | 2bps Slip No Fee | Fees | Win Rate | Max DD | Exit Reasons | Avg Hold | Median Hold | Breakout Median Ext | Max Entries/Bar |
|---|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| `1d` | `original_t2` | `20_10` | `baseline` | `[0.2, 0.1]` | -0.1587% | 27 | 0.3813% | 0.1653% | 0.3244% | 44.44% | -1.18% | `InitialSL:15, TrailingSL:12, PT:0` | 40437.96s | 14418.00s | 1.2903 | 1 |
| `1d` | `original_t2` | `20_10` | `be0p5` | `[0.2, 0.1]` | -0.1587% | 27 | 0.3813% | 0.1653% | 0.3244% | 44.44% | -1.18% | `InitialSL:15, TrailingSL:12, PT:0` | 40437.96s | 14418.00s | 1.2903 | 1 |
| `1d` | `original_t2` | `20_10` | `cost0p5` | `[0.2, 0.1]` | -0.1587% | 27 | 0.3813% | 0.1653% | 0.3244% | 44.44% | -1.18% | `InitialSL:15, TrailingSL:12, PT:0` | 40437.96s | 14418.00s | 1.2903 | 1 |
| `1d` | `original_t2` | `20_10` | `lock1p0_20bps` | `[0.2, 0.1]` | -0.1587% | 27 | 0.3813% | 0.1653% | 0.3244% | 44.44% | -1.18% | `InitialSL:15, TrailingSL:12, PT:0` | 40437.96s | 14418.00s | 1.2903 | 1 |
| `1d` | `original_t2` | `20_10` | `trail0p2_act0p5` | `[0.2, 0.1]` | 0.1562% | 27 | 0.6981% | 0.4812% | 0.3248% | 44.44% | -1.18% | `InitialSL:15, TrailingSL:12, PT:0` | 38128.26s | 12606.00s | 1.2903 | 1 |
| `1d` | `original_t2` | `20_10` | `trail0p5_act1p0` | `[0.2, 0.1]` | -0.6310% | 27 | -0.0939% | -0.3085% | 0.3248% | 29.63% | -1.68% | `InitialSL:19, TrailingSL:8, PT:0` | 72892.44s | 56398.00s | 1.2903 | 1 |
| `1d` | `original_t2` | `20_10` | `tp1p0` | `[0.2, 0.1]` | 0.0462% | 27 | 0.5872% | 0.3708% | 0.3248% | 44.44% | -1.18% | `InitialSL:15, TrailingSL:10, PT:0, TPxATR:2` | 40010.30s | 14418.00s | 1.2903 | 1 |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | `[0.2, 0.1]` | -0.2852% | 34 | 0.3944% | 0.1225% | 0.4081% | 44.12% | -1.41% | `InitialSL:19, TrailingSL:15, PT:0` | 40592.29s | 22213.50s | 1.2796 | 1 |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | `[0.2, 0.1]` | -0.2852% | 34 | 0.3944% | 0.1225% | 0.4081% | 44.12% | -1.41% | `InitialSL:19, TrailingSL:15, PT:0` | 40592.29s | 22213.50s | 1.2796 | 1 |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | `[0.2, 0.1]` | -0.2852% | 34 | 0.3944% | 0.1225% | 0.4081% | 44.12% | -1.41% | `InitialSL:19, TrailingSL:15, PT:0` | 40592.29s | 22213.50s | 1.2796 | 1 |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | `[0.2, 0.1]` | -0.2852% | 34 | 0.3944% | 0.1225% | 0.4081% | 44.12% | -1.41% | `InitialSL:19, TrailingSL:15, PT:0` | 40592.29s | 22213.50s | 1.2796 | 1 |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | `[0.2, 0.1]` | 0.2719% | 34 | 0.9553% | 0.6818% | 0.4091% | 44.12% | -1.41% | `InitialSL:19, TrailingSL:15, PT:0` | 38380.00s | 22213.50s | 1.2796 | 1 |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | `[0.2, 0.1]` | -1.3439% | 34 | -0.6720% | -0.9404% | 0.4082% | 26.47% | -2.16% | `InitialSL:25, TrailingSL:9, PT:0` | 73945.00s | 57877.50s | 1.2796 | 1 |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `[0.2, 0.1]` | -0.0805% | 34 | 0.6003% | 0.3280% | 0.4086% | 44.12% | -1.41% | `InitialSL:19, TrailingSL:13, PT:0, TPxATR:2` | 40252.68s | 22213.50s | 1.2796 | 1 |

## Exit Hold Diagnostics

| Timeframe | Shape | Variant | Exit Policy | Exit Reason | Trades | Avg Hold | Median Hold | Win Rate |
|---|---|---|---|---|---:|---:|---:|---:|
| `1d` | `original_t2` | `20_10` | `baseline` | `InitialSL` | 15 | 37641.93s | 7836.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `baseline` | `TrailingSL` | 12 | 43933.00s | 48817.00s | 100.00% |
| `1d` | `original_t2` | `20_10` | `baseline` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `be0p5` | `InitialSL` | 15 | 37641.93s | 7836.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `be0p5` | `TrailingSL` | 12 | 43933.00s | 48817.00s | 100.00% |
| `1d` | `original_t2` | `20_10` | `be0p5` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `cost0p5` | `InitialSL` | 15 | 37641.93s | 7836.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `cost0p5` | `TrailingSL` | 12 | 43933.00s | 48817.00s | 100.00% |
| `1d` | `original_t2` | `20_10` | `cost0p5` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `lock1p0_20bps` | `InitialSL` | 15 | 37641.93s | 7836.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `lock1p0_20bps` | `TrailingSL` | 12 | 43933.00s | 48817.00s | 100.00% |
| `1d` | `original_t2` | `20_10` | `lock1p0_20bps` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `trail0p2_act0p5` | `InitialSL` | 15 | 37641.93s | 7836.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `trail0p2_act0p5` | `TrailingSL` | 12 | 38736.17s | 44490.50s | 100.00% |
| `1d` | `original_t2` | `20_10` | `trail0p2_act0p5` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `trail0p5_act1p0` | `InitialSL` | 19 | 52397.47s | 12606.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `trail0p5_act1p0` | `TrailingSL` | 8 | 121568.00s | 137750.50s | 100.00% |
| `1d` | `original_t2` | `20_10` | `trail0p5_act1p0` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `tp1p0` | `InitialSL` | 15 | 37641.93s | 7836.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `tp1p0` | `TrailingSL` | 10 | 43246.70s | 48817.00s | 100.00% |
| `1d` | `original_t2` | `20_10` | `tp1p0` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `tp1p0` | `TPxATR` | 2 | 41591.00s | 41591.00s | 100.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | `InitialSL` | 19 | 36650.26s | 8772.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | `TrailingSL` | 15 | 45585.53s | 45377.00s | 100.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | `InitialSL` | 19 | 36650.26s | 8772.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | `TrailingSL` | 15 | 45585.53s | 45377.00s | 100.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | `InitialSL` | 19 | 36650.26s | 8772.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | `TrailingSL` | 15 | 45585.53s | 45377.00s | 100.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | `InitialSL` | 19 | 36650.26s | 8772.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | `TrailingSL` | 15 | 45585.53s | 45377.00s | 100.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | `InitialSL` | 19 | 36650.26s | 8772.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | `TrailingSL` | 15 | 40571.00s | 37359.00s | 100.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | `InitialSL` | 25 | 56213.48s | 25969.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | `TrailingSL` | 9 | 123199.22s | 136249.00s | 100.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `InitialSL` | 19 | 36650.26s | 8772.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `TrailingSL` | 13 | 45311.85s | 45377.00s | 100.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `TPxATR` | 2 | 41591.00s | 41591.00s | 100.00% |

## Trade Slot Diagnostics

| Timeframe | Shape | Variant | Exit Policy | Slot | Trades | Realistic Contribution | Raw Contribution | 2bps Slip Contribution | Fees | Win Rate | Avg Hold | Median Hold | Exit Reasons |
|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `1d` | `original_t2` | `20_10` | `baseline` | 0 | 27 | -0.1587% | 0.3813% | 0.1653% | 0.3244% | 44.44% | 40437.96s | 14418.00s | `{'InitialSL': 15, 'TrailingSL': 12}` |
| `1d` | `original_t2` | `20_10` | `be0p5` | 0 | 27 | -0.1587% | 0.3813% | 0.1653% | 0.3244% | 44.44% | 40437.96s | 14418.00s | `{'InitialSL': 15, 'TrailingSL': 12}` |
| `1d` | `original_t2` | `20_10` | `cost0p5` | 0 | 27 | -0.1587% | 0.3813% | 0.1653% | 0.3244% | 44.44% | 40437.96s | 14418.00s | `{'InitialSL': 15, 'TrailingSL': 12}` |
| `1d` | `original_t2` | `20_10` | `lock1p0_20bps` | 0 | 27 | -0.1587% | 0.3813% | 0.1653% | 0.3244% | 44.44% | 40437.96s | 14418.00s | `{'InitialSL': 15, 'TrailingSL': 12}` |
| `1d` | `original_t2` | `20_10` | `trail0p2_act0p5` | 0 | 27 | 0.1562% | 0.6981% | 0.4812% | 0.3248% | 44.44% | 38128.26s | 12606.00s | `{'InitialSL': 15, 'TrailingSL': 12}` |
| `1d` | `original_t2` | `20_10` | `trail0p5_act1p0` | 0 | 27 | -0.6310% | -0.0939% | -0.3085% | 0.3248% | 29.63% | 72892.44s | 56398.00s | `{'InitialSL': 19, 'TrailingSL': 8}` |
| `1d` | `original_t2` | `20_10` | `tp1p0` | 0 | 27 | 0.0462% | 0.5872% | 0.3708% | 0.3248% | 44.44% | 40010.30s | 14418.00s | `{'InitialSL': 15, 'TrailingSL': 10, 'TPxATR': 2}` |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | 0 | 34 | -0.2852% | 0.3944% | 0.1225% | 0.4081% | 44.12% | 40592.29s | 22213.50s | `{'InitialSL': 19, 'TrailingSL': 15}` |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | 0 | 34 | -0.2852% | 0.3944% | 0.1225% | 0.4081% | 44.12% | 40592.29s | 22213.50s | `{'InitialSL': 19, 'TrailingSL': 15}` |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | 0 | 34 | -0.2852% | 0.3944% | 0.1225% | 0.4081% | 44.12% | 40592.29s | 22213.50s | `{'InitialSL': 19, 'TrailingSL': 15}` |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | 0 | 34 | -0.2852% | 0.3944% | 0.1225% | 0.4081% | 44.12% | 40592.29s | 22213.50s | `{'InitialSL': 19, 'TrailingSL': 15}` |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | 0 | 34 | 0.2719% | 0.9553% | 0.6818% | 0.4091% | 44.12% | 38380.00s | 22213.50s | `{'InitialSL': 19, 'TrailingSL': 15}` |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | 0 | 34 | -1.3439% | -0.6720% | -0.9404% | 0.4082% | 26.47% | 73945.00s | 57877.50s | `{'InitialSL': 25, 'TrailingSL': 9}` |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | 0 | 34 | -0.0805% | 0.6003% | 0.3280% | 0.4086% | 44.12% | 40252.68s | 22213.50s | `{'InitialSL': 19, 'TrailingSL': 13, 'TPxATR': 2}` |

## MFE/MAE Diagnostics

| Timeframe | Shape | Variant | Exit Policy | Group | Trades | Median MFE | Median MAE | Median MFE ATR | Median MAE ATR | MFE >= 10bps | MFE >= 20bps | Median Realized |
|---|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `1d` | `original_t2` | `20_10` | `baseline` | `overall` | 27 | 126.0018bps | 81.8675bps | 0.3910 | 0.3009 | 96.30% | 88.89% | -82.4780bps |
| `1d` | `original_t2` | `20_10` | `baseline` | `exit:InitialSL` | 15 | 60.5594bps | 117.9360bps | 0.1498 | 0.3039 | 93.33% | 80.00% | -118.5987bps |
| `1d` | `original_t2` | `20_10` | `baseline` | `exit:TrailingSL` | 12 | 268.0778bps | 46.1397bps | 0.6936 | 0.1167 | 100.00% | 100.00% | 142.2156bps |
| `1d` | `original_t2` | `20_10` | `baseline` | `filled:original_t2` | 27 | 126.0018bps | 81.8675bps | 0.3910 | 0.3009 | 96.30% | 88.89% | -82.4780bps |
| `1d` | `original_t2` | `20_10` | `be0p5` | `overall` | 27 | 126.0018bps | 81.8675bps | 0.3910 | 0.3009 | 96.30% | 88.89% | -82.4780bps |
| `1d` | `original_t2` | `20_10` | `be0p5` | `exit:InitialSL` | 15 | 60.5594bps | 117.9360bps | 0.1498 | 0.3039 | 93.33% | 80.00% | -118.5987bps |
| `1d` | `original_t2` | `20_10` | `be0p5` | `exit:TrailingSL` | 12 | 268.0778bps | 46.1397bps | 0.6936 | 0.1167 | 100.00% | 100.00% | 142.2156bps |
| `1d` | `original_t2` | `20_10` | `be0p5` | `filled:original_t2` | 27 | 126.0018bps | 81.8675bps | 0.3910 | 0.3009 | 96.30% | 88.89% | -82.4780bps |
| `1d` | `original_t2` | `20_10` | `cost0p5` | `overall` | 27 | 126.0018bps | 81.8675bps | 0.3910 | 0.3009 | 96.30% | 88.89% | -82.4780bps |
| `1d` | `original_t2` | `20_10` | `cost0p5` | `exit:InitialSL` | 15 | 60.5594bps | 117.9360bps | 0.1498 | 0.3039 | 93.33% | 80.00% | -118.5987bps |
| `1d` | `original_t2` | `20_10` | `cost0p5` | `exit:TrailingSL` | 12 | 268.0778bps | 46.1397bps | 0.6936 | 0.1167 | 100.00% | 100.00% | 142.2156bps |
| `1d` | `original_t2` | `20_10` | `cost0p5` | `filled:original_t2` | 27 | 126.0018bps | 81.8675bps | 0.3910 | 0.3009 | 96.30% | 88.89% | -82.4780bps |
| `1d` | `original_t2` | `20_10` | `lock1p0_20bps` | `overall` | 27 | 126.0018bps | 81.8675bps | 0.3910 | 0.3009 | 96.30% | 88.89% | -82.4780bps |
| `1d` | `original_t2` | `20_10` | `lock1p0_20bps` | `exit:InitialSL` | 15 | 60.5594bps | 117.9360bps | 0.1498 | 0.3039 | 93.33% | 80.00% | -118.5987bps |
| `1d` | `original_t2` | `20_10` | `lock1p0_20bps` | `exit:TrailingSL` | 12 | 268.0778bps | 46.1397bps | 0.6936 | 0.1167 | 100.00% | 100.00% | 142.2156bps |
| `1d` | `original_t2` | `20_10` | `lock1p0_20bps` | `filled:original_t2` | 27 | 126.0018bps | 81.8675bps | 0.3910 | 0.3009 | 96.30% | 88.89% | -82.4780bps |
| `1d` | `original_t2` | `20_10` | `trail0p2_act0p5` | `overall` | 27 | 126.0018bps | 81.8675bps | 0.3910 | 0.3009 | 96.30% | 88.89% | -82.4780bps |
| `1d` | `original_t2` | `20_10` | `trail0p2_act0p5` | `exit:InitialSL` | 15 | 60.5594bps | 117.9360bps | 0.1498 | 0.3039 | 93.33% | 80.00% | -118.5987bps |
| `1d` | `original_t2` | `20_10` | `trail0p2_act0p5` | `exit:TrailingSL` | 12 | 246.7380bps | 46.1397bps | 0.6705 | 0.1167 | 100.00% | 100.00% | 176.0340bps |
| `1d` | `original_t2` | `20_10` | `trail0p2_act0p5` | `filled:original_t2` | 27 | 126.0018bps | 81.8675bps | 0.3910 | 0.3009 | 96.30% | 88.89% | -82.4780bps |
| `1d` | `original_t2` | `20_10` | `trail0p5_act1p0` | `overall` | 27 | 126.0018bps | 94.2966bps | 0.3910 | 0.3027 | 96.30% | 88.89% | -96.2176bps |
| `1d` | `original_t2` | `20_10` | `trail0p5_act1p0` | `exit:InitialSL` | 19 | 96.8224bps | 117.9360bps | 0.2651 | 0.3040 | 94.74% | 84.21% | -118.5987bps |
| `1d` | `original_t2` | `20_10` | `trail0p5_act1p0` | `exit:TrailingSL` | 8 | 472.3448bps | 43.0958bps | 1.2003 | 0.1167 | 100.00% | 100.00% | 259.4680bps |
| `1d` | `original_t2` | `20_10` | `trail0p5_act1p0` | `filled:original_t2` | 27 | 126.0018bps | 94.2966bps | 0.3910 | 0.3027 | 96.30% | 88.89% | -96.2176bps |
| `1d` | `original_t2` | `20_10` | `tp1p0` | `overall` | 27 | 126.0018bps | 81.8675bps | 0.3910 | 0.3009 | 96.30% | 88.89% | -82.4780bps |
| `1d` | `original_t2` | `20_10` | `tp1p0` | `exit:InitialSL` | 15 | 60.5594bps | 117.9360bps | 0.1498 | 0.3039 | 93.33% | 80.00% | -118.5987bps |
| `1d` | `original_t2` | `20_10` | `tp1p0` | `exit:TPxATR` | 2 | 353.0296bps | 43.2287bps | 1.0470 | 0.1126 | 100.00% | 100.00% | 350.3621bps |
| `1d` | `original_t2` | `20_10` | `tp1p0` | `exit:TrailingSL` | 10 | 256.7104bps | 46.1397bps | 0.6471 | 0.1167 | 100.00% | 100.00% | 130.5709bps |
| `1d` | `original_t2` | `20_10` | `tp1p0` | `filled:original_t2` | 27 | 126.0018bps | 81.8675bps | 0.3910 | 0.3009 | 96.30% | 88.89% | -82.4780bps |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | `overall` | 34 | 128.0496bps | 90.5831bps | 0.3560 | 0.3011 | 97.06% | 91.18% | -85.2727bps |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | `exit:InitialSL` | 19 | 60.5594bps | 117.9360bps | 0.1498 | 0.3069 | 94.74% | 84.21% | -118.2592bps |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | `exit:TrailingSL` | 15 | 264.3545bps | 45.1870bps | 0.6635 | 0.1133 | 100.00% | 100.00% | 132.2913bps |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | `filled:original_t2` | 27 | 126.0018bps | 81.8675bps | 0.3910 | 0.3009 | 96.30% | 88.89% | -82.4780bps |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | `filled:t3_swing` | 7 | 140.0142bps | 117.1793bps | 0.3201 | 0.3142 | 100.00% | 100.00% | -99.6047bps |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | `overall` | 34 | 128.0496bps | 90.5831bps | 0.3560 | 0.3011 | 97.06% | 91.18% | -85.2727bps |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | `exit:InitialSL` | 19 | 60.5594bps | 117.9360bps | 0.1498 | 0.3069 | 94.74% | 84.21% | -118.2592bps |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | `exit:TrailingSL` | 15 | 264.3545bps | 45.1870bps | 0.6635 | 0.1133 | 100.00% | 100.00% | 132.2913bps |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | `filled:original_t2` | 27 | 126.0018bps | 81.8675bps | 0.3910 | 0.3009 | 96.30% | 88.89% | -82.4780bps |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | `filled:t3_swing` | 7 | 140.0142bps | 117.1793bps | 0.3201 | 0.3142 | 100.00% | 100.00% | -99.6047bps |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | `overall` | 34 | 128.0496bps | 90.5831bps | 0.3560 | 0.3011 | 97.06% | 91.18% | -85.2727bps |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | `exit:InitialSL` | 19 | 60.5594bps | 117.9360bps | 0.1498 | 0.3069 | 94.74% | 84.21% | -118.2592bps |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | `exit:TrailingSL` | 15 | 264.3545bps | 45.1870bps | 0.6635 | 0.1133 | 100.00% | 100.00% | 132.2913bps |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | `filled:original_t2` | 27 | 126.0018bps | 81.8675bps | 0.3910 | 0.3009 | 96.30% | 88.89% | -82.4780bps |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | `filled:t3_swing` | 7 | 140.0142bps | 117.1793bps | 0.3201 | 0.3142 | 100.00% | 100.00% | -99.6047bps |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | `overall` | 34 | 128.0496bps | 90.5831bps | 0.3560 | 0.3011 | 97.06% | 91.18% | -85.2727bps |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | `exit:InitialSL` | 19 | 60.5594bps | 117.9360bps | 0.1498 | 0.3069 | 94.74% | 84.21% | -118.2592bps |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | `exit:TrailingSL` | 15 | 264.3545bps | 45.1870bps | 0.6635 | 0.1133 | 100.00% | 100.00% | 132.2913bps |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | `filled:original_t2` | 27 | 126.0018bps | 81.8675bps | 0.3910 | 0.3009 | 96.30% | 88.89% | -82.4780bps |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | `filled:t3_swing` | 7 | 140.0142bps | 117.1793bps | 0.3201 | 0.3142 | 100.00% | 100.00% | -99.6047bps |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | `overall` | 34 | 128.0496bps | 90.5831bps | 0.3560 | 0.3011 | 97.06% | 91.18% | -85.2727bps |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | `exit:InitialSL` | 19 | 60.5594bps | 117.9360bps | 0.1498 | 0.3069 | 94.74% | 84.21% | -118.2592bps |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | `exit:TrailingSL` | 15 | 249.0664bps | 45.1870bps | 0.6635 | 0.1133 | 100.00% | 100.00% | 175.6280bps |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | `filled:original_t2` | 27 | 126.0018bps | 81.8675bps | 0.3910 | 0.3009 | 96.30% | 88.89% | -82.4780bps |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | `filled:t3_swing` | 7 | 140.0142bps | 117.1793bps | 0.3201 | 0.3142 | 100.00% | 100.00% | -99.6047bps |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | `overall` | 34 | 133.0080bps | 109.6974bps | 0.3560 | 0.3036 | 97.06% | 91.18% | -104.9946bps |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | `exit:InitialSL` | 25 | 96.8224bps | 117.9360bps | 0.2651 | 0.3049 | 96.00% | 88.00% | -118.2592bps |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | `exit:TrailingSL` | 9 | 480.6285bps | 41.0046bps | 1.2163 | 0.1133 | 100.00% | 100.00% | 288.6916bps |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | `filled:original_t2` | 27 | 126.0018bps | 94.2966bps | 0.3910 | 0.3027 | 96.30% | 88.89% | -96.2176bps |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | `filled:t3_swing` | 7 | 140.0142bps | 117.1793bps | 0.3201 | 0.3215 | 100.00% | 100.00% | -113.8585bps |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `overall` | 34 | 128.0496bps | 90.5831bps | 0.3560 | 0.3011 | 97.06% | 91.18% | -85.2727bps |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `exit:InitialSL` | 19 | 60.5594bps | 117.9360bps | 0.1498 | 0.3069 | 94.74% | 84.21% | -118.2592bps |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `exit:TPxATR` | 2 | 353.0296bps | 43.2287bps | 1.0470 | 0.1126 | 100.00% | 100.00% | 350.3621bps |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `exit:TrailingSL` | 13 | 262.3749bps | 45.1870bps | 0.6496 | 0.1133 | 100.00% | 100.00% | 130.8039bps |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `filled:original_t2` | 27 | 126.0018bps | 81.8675bps | 0.3910 | 0.3009 | 96.30% | 88.89% | -82.4780bps |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `filled:t3_swing` | 7 | 140.0142bps | 117.1793bps | 0.3201 | 0.3142 | 100.00% | 100.00% | -99.6047bps |

## Breakout Attribution

| Timeframe | Configured Shape | Variant | Exit Policy | Filled Shape | Trades | Win Rate | Avg PnL | Median PnL | Net PnL | Profit Factor |
|---|---|---|---|---|---:|---:|---:|---:|---:|---:|
| `1d` | `original_t2` | `20_10` | `baseline` | `original_t2` | 27 | 44.44% | 0.0330% | -0.8248% | 165.30 | 1.0439 |
| `1d` | `original_t2` | `20_10` | `be0p5` | `original_t2` | 27 | 44.44% | 0.0330% | -0.8248% | 165.30 | 1.0439 |
| `1d` | `original_t2` | `20_10` | `cost0p5` | `original_t2` | 27 | 44.44% | 0.0330% | -0.8248% | 165.30 | 1.0439 |
| `1d` | `original_t2` | `20_10` | `lock1p0_20bps` | `original_t2` | 27 | 44.44% | 0.0330% | -0.8248% | 165.30 | 1.0439 |
| `1d` | `original_t2` | `20_10` | `trail0p2_act0p5` | `original_t2` | 27 | 44.44% | 0.0914% | -0.8248% | 481.17 | 1.1276 |
| `1d` | `original_t2` | `20_10` | `trail0p5_act1p0` | `original_t2` | 27 | 29.63% | -0.0535% | -0.9622% | -308.45 | 0.9336 |
| `1d` | `original_t2` | `20_10` | `tp1p0` | `original_t2` | 27 | 44.44% | 0.0712% | -0.8248% | 370.84 | 1.0983 |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | `original_t2` | 27 | 44.44% | 0.0330% | -0.8248% | 167.59 | 1.0445 |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | `t3_swing` | 7 | 42.86% | -0.0287% | -0.9960% | -45.07 | 0.9517 |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | `original_t2` | 27 | 44.44% | 0.0330% | -0.8248% | 167.59 | 1.0445 |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | `t3_swing` | 7 | 42.86% | -0.0287% | -0.9960% | -45.07 | 0.9517 |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | `original_t2` | 27 | 44.44% | 0.0330% | -0.8248% | 167.59 | 1.0445 |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | `t3_swing` | 7 | 42.86% | -0.0287% | -0.9960% | -45.07 | 0.9517 |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | `original_t2` | 27 | 44.44% | 0.0330% | -0.8248% | 167.59 | 1.0445 |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | `t3_swing` | 7 | 42.86% | -0.0287% | -0.9960% | -45.07 | 0.9517 |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | `original_t2` | 27 | 44.44% | 0.0914% | -0.8248% | 485.09 | 1.1286 |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | `t3_swing` | 7 | 42.86% | 0.1448% | -0.9960% | 196.66 | 1.2104 |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | `original_t2` | 27 | 29.63% | -0.0535% | -0.9622% | -305.01 | 0.9343 |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | `t3_swing` | 7 | 14.29% | -0.4506% | -1.1386% | -635.42 | 0.5619 |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `original_t2` | 27 | 44.44% | 0.0712% | -0.8248% | 373.03 | 1.099 |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `t3_swing` | 7 | 42.86% | -0.0287% | -0.9960% | -45.06 | 0.9518 |

## Files

- Summary JSON: `research/btc_2026_jan_apr_1d_direct_breakout_exit_variants_summary.json`
- `1d` ledger: `research/tmp_btc_2026_jan_apr_1d_direct_breakout_exit_variants_1d_original_t2_20_10_baseline_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_btc_2026_jan_apr_1d_direct_breakout_exit_variants_1d_original_t2_20_10_be0p5_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_btc_2026_jan_apr_1d_direct_breakout_exit_variants_1d_original_t2_20_10_cost0p5_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_btc_2026_jan_apr_1d_direct_breakout_exit_variants_1d_original_t2_20_10_lock1p0_20bps_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_btc_2026_jan_apr_1d_direct_breakout_exit_variants_1d_original_t2_20_10_trail0p2_act0p5_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_btc_2026_jan_apr_1d_direct_breakout_exit_variants_1d_original_t2_20_10_trail0p5_act1p0_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_btc_2026_jan_apr_1d_direct_breakout_exit_variants_1d_original_t2_20_10_tp1p0_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_btc_2026_jan_apr_1d_direct_breakout_exit_variants_1d_baseline_plus_t3_20_10_baseline_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_btc_2026_jan_apr_1d_direct_breakout_exit_variants_1d_baseline_plus_t3_20_10_be0p5_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_btc_2026_jan_apr_1d_direct_breakout_exit_variants_1d_baseline_plus_t3_20_10_cost0p5_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_btc_2026_jan_apr_1d_direct_breakout_exit_variants_1d_baseline_plus_t3_20_10_lock1p0_20bps_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_btc_2026_jan_apr_1d_direct_breakout_exit_variants_1d_baseline_plus_t3_20_10_trail0p2_act0p5_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_btc_2026_jan_apr_1d_direct_breakout_exit_variants_1d_baseline_plus_t3_20_10_trail0p5_act1p0_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_btc_2026_jan_apr_1d_direct_breakout_exit_variants_1d_baseline_plus_t3_20_10_tp1p0_observed_close_ledger.csv`
