# ETHUSDT 2026 Jan-Apr Direct Breakout

Scope: research-only. This removes VSL, removes `re_p`, and removes reclaim reentry. The first configured structural breakout in each signal bar opens real exposure immediately at the observed 1s close. Exit variants keep the entry semantics fixed and only change stop/target handling after entry.

Accounting shown below uses 2 bps/side slippage plus maker entry 2 bps and market SL/exit 4 bps.

| Timeframe | Shape | Variant | Exit Policy | Schedule | Realistic Return | Trades | Raw No Fee/Slip | 2bps Slip No Fee | Fees | Win Rate | Max DD | Exit Reasons | Avg Hold | Median Hold | Breakout Median Ext | Max Entries/Bar |
|---|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| `1d` | `original_t2` | `20_10` | `baseline` | `[0.2, 0.1]` | -1.6688% | 27 | -1.1356% | -1.3496% | 0.3206% | 37.04% | -2.85% | `InitialSL:17, TrailingSL:10, PT:0` | 33141.67s | 13416.00s | 0.9552 | 1 |
| `1d` | `original_t2` | `20_10` | `be0p5` | `[0.2, 0.1]` | -1.6688% | 27 | -1.1356% | -1.3496% | 0.3206% | 37.04% | -2.85% | `InitialSL:17, TrailingSL:10, PT:0` | 33141.67s | 13416.00s | 0.9552 | 1 |
| `1d` | `original_t2` | `20_10` | `cost0p5` | `[0.2, 0.1]` | -1.6688% | 27 | -1.1356% | -1.3496% | 0.3206% | 37.04% | -2.85% | `InitialSL:17, TrailingSL:10, PT:0` | 33141.67s | 13416.00s | 0.9552 | 1 |
| `1d` | `original_t2` | `20_10` | `lock1p0_20bps` | `[0.2, 0.1]` | -1.6688% | 27 | -1.1356% | -1.3496% | 0.3206% | 37.04% | -2.85% | `InitialSL:17, TrailingSL:10, PT:0` | 33141.67s | 13416.00s | 0.9552 | 1 |
| `1d` | `original_t2` | `20_10` | `trail0p2_act0p5` | `[0.2, 0.1]` | -1.7865% | 27 | -1.2541% | -1.4675% | 0.3202% | 37.04% | -3.11% | `InitialSL:17, TrailingSL:10, PT:0` | 30314.11s | 10321.00s | 0.9552 | 1 |
| `1d` | `original_t2` | `20_10` | `trail0p5_act1p0` | `[0.2, 0.1]` | -3.3212% | 26 | -2.8163% | -3.0187% | 0.3086% | 15.38% | -3.53% | `InitialSL:22, TrailingSL:4, PT:0` | 68830.31s | 32560.50s | 1.0181 | 1 |
| `1d` | `original_t2` | `20_10` | `tp1p0` | `[0.2, 0.1]` | -2.0750% | 27 | -1.5441% | -1.7570% | 0.3196% | 37.04% | -3.25% | `InitialSL:17, TrailingSL:9, PT:0, TPxATR:1` | 32550.63s | 13416.00s | 0.9552 | 1 |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | `[0.2, 0.1]` | -0.7961% | 34 | -0.1189% | -0.3905% | 0.4067% | 44.12% | -2.74% | `InitialSL:19, TrailingSL:15, PT:0` | 34659.44s | 13597.00s | 1.0818 | 1 |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | `[0.2, 0.1]` | -0.7961% | 34 | -0.1189% | -0.3905% | 0.4067% | 44.12% | -2.74% | `InitialSL:19, TrailingSL:15, PT:0` | 34659.44s | 13597.00s | 1.0818 | 1 |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | `[0.2, 0.1]` | -0.7961% | 34 | -0.1189% | -0.3905% | 0.4067% | 44.12% | -2.74% | `InitialSL:19, TrailingSL:15, PT:0` | 34659.44s | 13597.00s | 1.0818 | 1 |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | `[0.2, 0.1]` | -0.7961% | 34 | -0.1189% | -0.3905% | 0.4067% | 44.12% | -2.74% | `InitialSL:19, TrailingSL:15, PT:0` | 34659.44s | 13597.00s | 1.0818 | 1 |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | `[0.2, 0.1]` | -0.6189% | 34 | 0.0592% | -0.2125% | 0.4070% | 44.12% | -2.39% | `InitialSL:19, TrailingSL:15, PT:0` | 31181.29s | 8171.00s | 1.0818 | 1 |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | `[0.2, 0.1]` | -1.7006% | 33 | -1.0494% | -1.3103% | 0.3981% | 21.21% | -3.78% | `InitialSL:26, TrailingSL:7, PT:0` | 73849.06s | 33652.00s | 1.2084 | 1 |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `[0.2, 0.1]` | -1.2059% | 34 | -0.5315% | -0.8019% | 0.4056% | 44.12% | -2.74% | `InitialSL:19, TrailingSL:14, PT:0, TPxATR:1` | 34190.09s | 13597.00s | 1.0818 | 1 |

## Exit Hold Diagnostics

| Timeframe | Shape | Variant | Exit Policy | Exit Reason | Trades | Avg Hold | Median Hold | Win Rate |
|---|---|---|---|---|---:|---:|---:|---:|
| `1d` | `original_t2` | `20_10` | `baseline` | `InitialSL` | 17 | 26816.29s | 6322.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `baseline` | `TrailingSL` | 10 | 43894.80s | 34735.00s | 100.00% |
| `1d` | `original_t2` | `20_10` | `baseline` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `be0p5` | `InitialSL` | 17 | 26816.29s | 6322.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `be0p5` | `TrailingSL` | 10 | 43894.80s | 34735.00s | 100.00% |
| `1d` | `original_t2` | `20_10` | `be0p5` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `cost0p5` | `InitialSL` | 17 | 26816.29s | 6322.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `cost0p5` | `TrailingSL` | 10 | 43894.80s | 34735.00s | 100.00% |
| `1d` | `original_t2` | `20_10` | `cost0p5` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `lock1p0_20bps` | `InitialSL` | 17 | 26816.29s | 6322.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `lock1p0_20bps` | `TrailingSL` | 10 | 43894.80s | 34735.00s | 100.00% |
| `1d` | `original_t2` | `20_10` | `lock1p0_20bps` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `trail0p2_act0p5` | `InitialSL` | 17 | 26816.29s | 6322.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `trail0p2_act0p5` | `TrailingSL` | 10 | 36260.40s | 17579.50s | 100.00% |
| `1d` | `original_t2` | `20_10` | `trail0p2_act0p5` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `trail0p5_act1p0` | `InitialSL` | 22 | 52169.55s | 17628.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `trail0p5_act1p0` | `TrailingSL` | 4 | 160464.50s | 111242.50s | 100.00% |
| `1d` | `original_t2` | `20_10` | `trail0p5_act1p0` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `tp1p0` | `InitialSL` | 17 | 26816.29s | 6322.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `tp1p0` | `TrailingSL` | 9 | 45323.00s | 38429.00s | 100.00% |
| `1d` | `original_t2` | `20_10` | `tp1p0` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `original_t2` | `20_10` | `tp1p0` | `TPxATR` | 1 | 15083.00s | 15083.00s | 100.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | `InitialSL` | 19 | 24503.32s | 6322.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | `TrailingSL` | 15 | 47523.87s | 28115.00s | 100.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | `InitialSL` | 19 | 24503.32s | 6322.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | `TrailingSL` | 15 | 47523.87s | 28115.00s | 100.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | `InitialSL` | 19 | 24503.32s | 6322.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | `TrailingSL` | 15 | 47523.87s | 28115.00s | 100.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | `InitialSL` | 19 | 24503.32s | 6322.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | `TrailingSL` | 15 | 47523.87s | 28115.00s | 100.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | `InitialSL` | 19 | 24503.32s | 6322.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | `TrailingSL` | 15 | 39640.07s | 13417.00s | 100.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | `InitialSL` | 26 | 55508.38s | 17628.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | `TrailingSL` | 7 | 141971.57s | 88324.00s | 100.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `InitialSL` | 19 | 24503.32s | 6322.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `TrailingSL` | 14 | 48701.21s | 23689.50s | 100.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `PT` | 0 | 0.00s | 0.00s | 0.00% |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `TPxATR` | 1 | 15083.00s | 15083.00s | 100.00% |

## Trade Slot Diagnostics

| Timeframe | Shape | Variant | Exit Policy | Slot | Trades | Realistic Contribution | Raw Contribution | 2bps Slip Contribution | Fees | Win Rate | Avg Hold | Median Hold | Exit Reasons |
|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `1d` | `original_t2` | `20_10` | `baseline` | 0 | 27 | -1.6688% | -1.1356% | -1.3496% | 0.3206% | 37.04% | 33141.67s | 13416.00s | `{'InitialSL': 17, 'TrailingSL': 10}` |
| `1d` | `original_t2` | `20_10` | `be0p5` | 0 | 27 | -1.6688% | -1.1356% | -1.3496% | 0.3206% | 37.04% | 33141.67s | 13416.00s | `{'InitialSL': 17, 'TrailingSL': 10}` |
| `1d` | `original_t2` | `20_10` | `cost0p5` | 0 | 27 | -1.6688% | -1.1356% | -1.3496% | 0.3206% | 37.04% | 33141.67s | 13416.00s | `{'InitialSL': 17, 'TrailingSL': 10}` |
| `1d` | `original_t2` | `20_10` | `lock1p0_20bps` | 0 | 27 | -1.6688% | -1.1356% | -1.3496% | 0.3206% | 37.04% | 33141.67s | 13416.00s | `{'InitialSL': 17, 'TrailingSL': 10}` |
| `1d` | `original_t2` | `20_10` | `trail0p2_act0p5` | 0 | 27 | -1.7865% | -1.2541% | -1.4675% | 0.3202% | 37.04% | 30314.11s | 10321.00s | `{'InitialSL': 17, 'TrailingSL': 10}` |
| `1d` | `original_t2` | `20_10` | `trail0p5_act1p0` | 0 | 26 | -3.3212% | -2.8163% | -3.0187% | 0.3086% | 15.38% | 68830.31s | 32560.50s | `{'InitialSL': 22, 'TrailingSL': 4}` |
| `1d` | `original_t2` | `20_10` | `tp1p0` | 0 | 27 | -2.0750% | -1.5441% | -1.7570% | 0.3196% | 37.04% | 32550.63s | 13416.00s | `{'InitialSL': 17, 'TrailingSL': 9, 'TPxATR': 1}` |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | 0 | 34 | -0.7961% | -0.1189% | -0.3905% | 0.4067% | 44.12% | 34659.44s | 13597.00s | `{'InitialSL': 19, 'TrailingSL': 15}` |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | 0 | 34 | -0.7961% | -0.1189% | -0.3905% | 0.4067% | 44.12% | 34659.44s | 13597.00s | `{'InitialSL': 19, 'TrailingSL': 15}` |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | 0 | 34 | -0.7961% | -0.1189% | -0.3905% | 0.4067% | 44.12% | 34659.44s | 13597.00s | `{'InitialSL': 19, 'TrailingSL': 15}` |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | 0 | 34 | -0.7961% | -0.1189% | -0.3905% | 0.4067% | 44.12% | 34659.44s | 13597.00s | `{'InitialSL': 19, 'TrailingSL': 15}` |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | 0 | 34 | -0.6189% | 0.0592% | -0.2125% | 0.4070% | 44.12% | 31181.29s | 8171.00s | `{'InitialSL': 19, 'TrailingSL': 15}` |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | 0 | 33 | -1.7006% | -1.0494% | -1.3103% | 0.3981% | 21.21% | 73849.06s | 33652.00s | `{'InitialSL': 26, 'TrailingSL': 7}` |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | 0 | 34 | -1.2059% | -0.5315% | -0.8019% | 0.4056% | 44.12% | 34190.09s | 13597.00s | `{'InitialSL': 19, 'TrailingSL': 14, 'TPxATR': 1}` |

## MFE/MAE Diagnostics

| Timeframe | Shape | Variant | Exit Policy | Group | Trades | Median MFE | Median MAE | Median MFE ATR | Median MAE ATR | MFE >= 10bps | MFE >= 20bps | Median Realized |
|---|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `1d` | `original_t2` | `20_10` | `baseline` | `overall` | 27 | 179.3661bps | 144.8341bps | 0.2993 | 0.3016 | 88.89% | 81.48% | -142.1100bps |
| `1d` | `original_t2` | `20_10` | `baseline` | `exit:InitialSL` | 17 | 74.3600bps | 156.9761bps | 0.1693 | 0.3034 | 82.35% | 70.59% | -158.1506bps |
| `1d` | `original_t2` | `20_10` | `baseline` | `exit:TrailingSL` | 10 | 323.4158bps | 38.2759bps | 0.7203 | 0.0869 | 100.00% | 100.00% | 177.5751bps |
| `1d` | `original_t2` | `20_10` | `baseline` | `filled:original_t2` | 27 | 179.3661bps | 144.8341bps | 0.2993 | 0.3016 | 88.89% | 81.48% | -142.1100bps |
| `1d` | `original_t2` | `20_10` | `be0p5` | `overall` | 27 | 179.3661bps | 144.8341bps | 0.2993 | 0.3016 | 88.89% | 81.48% | -142.1100bps |
| `1d` | `original_t2` | `20_10` | `be0p5` | `exit:InitialSL` | 17 | 74.3600bps | 156.9761bps | 0.1693 | 0.3034 | 82.35% | 70.59% | -158.1506bps |
| `1d` | `original_t2` | `20_10` | `be0p5` | `exit:TrailingSL` | 10 | 323.4158bps | 38.2759bps | 0.7203 | 0.0869 | 100.00% | 100.00% | 177.5751bps |
| `1d` | `original_t2` | `20_10` | `be0p5` | `filled:original_t2` | 27 | 179.3661bps | 144.8341bps | 0.2993 | 0.3016 | 88.89% | 81.48% | -142.1100bps |
| `1d` | `original_t2` | `20_10` | `cost0p5` | `overall` | 27 | 179.3661bps | 144.8341bps | 0.2993 | 0.3016 | 88.89% | 81.48% | -142.1100bps |
| `1d` | `original_t2` | `20_10` | `cost0p5` | `exit:InitialSL` | 17 | 74.3600bps | 156.9761bps | 0.1693 | 0.3034 | 82.35% | 70.59% | -158.1506bps |
| `1d` | `original_t2` | `20_10` | `cost0p5` | `exit:TrailingSL` | 10 | 323.4158bps | 38.2759bps | 0.7203 | 0.0869 | 100.00% | 100.00% | 177.5751bps |
| `1d` | `original_t2` | `20_10` | `cost0p5` | `filled:original_t2` | 27 | 179.3661bps | 144.8341bps | 0.2993 | 0.3016 | 88.89% | 81.48% | -142.1100bps |
| `1d` | `original_t2` | `20_10` | `lock1p0_20bps` | `overall` | 27 | 179.3661bps | 144.8341bps | 0.2993 | 0.3016 | 88.89% | 81.48% | -142.1100bps |
| `1d` | `original_t2` | `20_10` | `lock1p0_20bps` | `exit:InitialSL` | 17 | 74.3600bps | 156.9761bps | 0.1693 | 0.3034 | 82.35% | 70.59% | -158.1506bps |
| `1d` | `original_t2` | `20_10` | `lock1p0_20bps` | `exit:TrailingSL` | 10 | 323.4158bps | 38.2759bps | 0.7203 | 0.0869 | 100.00% | 100.00% | 177.5751bps |
| `1d` | `original_t2` | `20_10` | `lock1p0_20bps` | `filled:original_t2` | 27 | 179.3661bps | 144.8341bps | 0.2993 | 0.3016 | 88.89% | 81.48% | -142.1100bps |
| `1d` | `original_t2` | `20_10` | `trail0p2_act0p5` | `overall` | 27 | 179.3661bps | 144.8341bps | 0.2993 | 0.3016 | 88.89% | 81.48% | -142.1100bps |
| `1d` | `original_t2` | `20_10` | `trail0p2_act0p5` | `exit:InitialSL` | 17 | 74.3600bps | 156.9761bps | 0.1693 | 0.3034 | 82.35% | 70.59% | -158.1506bps |
| `1d` | `original_t2` | `20_10` | `trail0p2_act0p5` | `exit:TrailingSL` | 10 | 302.2854bps | 38.2759bps | 0.6050 | 0.0869 | 100.00% | 100.00% | 198.7155bps |
| `1d` | `original_t2` | `20_10` | `trail0p2_act0p5` | `filled:original_t2` | 27 | 179.3661bps | 144.8341bps | 0.2993 | 0.3016 | 88.89% | 81.48% | -142.1100bps |
| `1d` | `original_t2` | `20_10` | `trail0p5_act1p0` | `overall` | 26 | 179.5050bps | 147.8845bps | 0.3228 | 0.3038 | 88.46% | 84.62% | -142.8855bps |
| `1d` | `original_t2` | `20_10` | `trail0p5_act1p0` | `exit:InitialSL` | 22 | 145.3230bps | 152.1828bps | 0.2607 | 0.3053 | 86.36% | 81.82% | -148.7464bps |
| `1d` | `original_t2` | `20_10` | `trail0p5_act1p0` | `exit:TrailingSL` | 4 | 815.8317bps | 13.6664bps | 1.3632 | 0.0248 | 100.00% | 100.00% | 527.9531bps |
| `1d` | `original_t2` | `20_10` | `trail0p5_act1p0` | `filled:original_t2` | 26 | 179.5050bps | 147.8845bps | 0.3228 | 0.3038 | 88.46% | 84.62% | -142.8855bps |
| `1d` | `original_t2` | `20_10` | `tp1p0` | `overall` | 27 | 179.3661bps | 144.8341bps | 0.2993 | 0.3016 | 88.89% | 81.48% | -142.1100bps |
| `1d` | `original_t2` | `20_10` | `tp1p0` | `exit:InitialSL` | 17 | 74.3600bps | 156.9761bps | 0.1693 | 0.3034 | 82.35% | 70.59% | -158.1506bps |
| `1d` | `original_t2` | `20_10` | `tp1p0` | `exit:TPxATR` | 1 | 599.2719bps | 19.3820bps | 1.0843 | 0.0351 | 100.00% | 100.00% | 592.6307bps |
| `1d` | `original_t2` | `20_10` | `tp1p0` | `exit:TrailingSL` | 9 | 321.1798bps | 42.8553bps | 0.6971 | 0.1007 | 100.00% | 100.00% | 172.9692bps |
| `1d` | `original_t2` | `20_10` | `tp1p0` | `filled:original_t2` | 27 | 179.3661bps | 144.8341bps | 0.2993 | 0.3016 | 88.89% | 81.48% | -142.1100bps |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | `overall` | 34 | 181.9048bps | 143.1884bps | 0.3540 | 0.3009 | 91.18% | 85.29% | -134.5906bps |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | `exit:InitialSL` | 19 | 74.3600bps | 166.8528bps | 0.1693 | 0.3034 | 84.21% | 73.68% | -164.1143bps |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | `exit:TrailingSL` | 15 | 325.5481bps | 33.6966bps | 0.6971 | 0.0954 | 100.00% | 100.00% | 172.9692bps |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | `filled:original_t2` | 27 | 179.3661bps | 144.8341bps | 0.2993 | 0.3016 | 88.89% | 81.48% | -142.1100bps |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | `filled:t3_swing` | 7 | 244.9413bps | 89.5015bps | 0.5889 | 0.2130 | 100.00% | 100.00% | 127.9942bps |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | `overall` | 34 | 181.9048bps | 143.1884bps | 0.3540 | 0.3009 | 91.18% | 85.29% | -134.5906bps |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | `exit:InitialSL` | 19 | 74.3600bps | 166.8528bps | 0.1693 | 0.3034 | 84.21% | 73.68% | -164.1143bps |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | `exit:TrailingSL` | 15 | 325.5481bps | 33.6966bps | 0.6971 | 0.0954 | 100.00% | 100.00% | 172.9692bps |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | `filled:original_t2` | 27 | 179.3661bps | 144.8341bps | 0.2993 | 0.3016 | 88.89% | 81.48% | -142.1100bps |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | `filled:t3_swing` | 7 | 244.9413bps | 89.5015bps | 0.5889 | 0.2130 | 100.00% | 100.00% | 127.9942bps |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | `overall` | 34 | 181.9048bps | 143.1884bps | 0.3540 | 0.3009 | 91.18% | 85.29% | -134.5906bps |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | `exit:InitialSL` | 19 | 74.3600bps | 166.8528bps | 0.1693 | 0.3034 | 84.21% | 73.68% | -164.1143bps |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | `exit:TrailingSL` | 15 | 325.5481bps | 33.6966bps | 0.6971 | 0.0954 | 100.00% | 100.00% | 172.9692bps |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | `filled:original_t2` | 27 | 179.3661bps | 144.8341bps | 0.2993 | 0.3016 | 88.89% | 81.48% | -142.1100bps |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | `filled:t3_swing` | 7 | 244.9413bps | 89.5015bps | 0.5889 | 0.2130 | 100.00% | 100.00% | 127.9942bps |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | `overall` | 34 | 181.9048bps | 143.1884bps | 0.3540 | 0.3009 | 91.18% | 85.29% | -134.5906bps |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | `exit:InitialSL` | 19 | 74.3600bps | 166.8528bps | 0.1693 | 0.3034 | 84.21% | 73.68% | -164.1143bps |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | `exit:TrailingSL` | 15 | 325.5481bps | 33.6966bps | 0.6971 | 0.0954 | 100.00% | 100.00% | 172.9692bps |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | `filled:original_t2` | 27 | 179.3661bps | 144.8341bps | 0.2993 | 0.3016 | 88.89% | 81.48% | -142.1100bps |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | `filled:t3_swing` | 7 | 244.9413bps | 89.5015bps | 0.5889 | 0.2130 | 100.00% | 100.00% | 127.9942bps |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | `overall` | 34 | 179.7131bps | 143.1884bps | 0.3540 | 0.3009 | 91.18% | 85.29% | -134.5906bps |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | `exit:InitialSL` | 19 | 74.3600bps | 166.8528bps | 0.1693 | 0.3034 | 84.21% | 73.68% | -164.1143bps |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | `exit:TrailingSL` | 15 | 289.3092bps | 33.6966bps | 0.5864 | 0.0954 | 100.00% | 100.00% | 186.3940bps |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | `filled:original_t2` | 27 | 179.3661bps | 144.8341bps | 0.2993 | 0.3016 | 88.89% | 81.48% | -142.1100bps |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | `filled:t3_swing` | 7 | 244.9413bps | 89.5015bps | 0.5660 | 0.2130 | 100.00% | 100.00% | 162.7029bps |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | `overall` | 33 | 184.1656bps | 144.8341bps | 0.3618 | 0.3031 | 90.91% | 87.88% | -142.1100bps |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | `exit:InitialSL` | 26 | 145.3230bps | 153.9385bps | 0.2275 | 0.3049 | 88.46% | 84.62% | -151.8188bps |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | `exit:TrailingSL` | 7 | 766.8210bps | 27.1357bps | 1.5001 | 0.0531 | 100.00% | 100.00% | 522.3506bps |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | `filled:original_t2` | 26 | 179.5050bps | 147.8845bps | 0.2860 | 0.3038 | 88.46% | 84.62% | -142.8855bps |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | `filled:t3_swing` | 7 | 524.8970bps | 125.7347bps | 0.5889 | 0.3015 | 100.00% | 100.00% | -126.7608bps |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `overall` | 34 | 181.9048bps | 143.1884bps | 0.3540 | 0.3009 | 91.18% | 85.29% | -134.5906bps |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `exit:InitialSL` | 19 | 74.3600bps | 166.8528bps | 0.1693 | 0.3034 | 84.21% | 73.68% | -164.1143bps |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `exit:TPxATR` | 1 | 599.2719bps | 19.3820bps | 1.0843 | 0.0351 | 100.00% | 100.00% | 592.6307bps |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `exit:TrailingSL` | 14 | 323.3639bps | 38.2759bps | 0.6670 | 0.0981 | 100.00% | 100.00% | 172.3027bps |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `filled:original_t2` | 27 | 179.3661bps | 144.8341bps | 0.2993 | 0.3016 | 88.89% | 81.48% | -142.1100bps |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `filled:t3_swing` | 7 | 244.9413bps | 89.5015bps | 0.5889 | 0.2130 | 100.00% | 100.00% | 127.9942bps |

## Breakout Attribution

| Timeframe | Configured Shape | Variant | Exit Policy | Filled Shape | Trades | Win Rate | Avg PnL | Median PnL | Net PnL | Profit Factor |
|---|---|---|---|---|---:|---:|---:|---:|---:|---:|
| `1d` | `original_t2` | `20_10` | `baseline` | `original_t2` | 27 | 37.04% | -0.2462% | -1.4211% | -1,349.55 | 0.7715 |
| `1d` | `original_t2` | `20_10` | `be0p5` | `original_t2` | 27 | 37.04% | -0.2462% | -1.4211% | -1,349.55 | 0.7715 |
| `1d` | `original_t2` | `20_10` | `cost0p5` | `original_t2` | 27 | 37.04% | -0.2462% | -1.4211% | -1,349.55 | 0.7715 |
| `1d` | `original_t2` | `20_10` | `lock1p0_20bps` | `original_t2` | 27 | 37.04% | -0.2462% | -1.4211% | -1,349.55 | 0.7715 |
| `1d` | `original_t2` | `20_10` | `trail0p2_act0p5` | `original_t2` | 27 | 37.04% | -0.2694% | -1.4211% | -1,467.54 | 0.7513 |
| `1d` | `original_t2` | `20_10` | `trail0p5_act1p0` | `original_t2` | 26 | 15.38% | -0.5823% | -1.4289% | -3,018.75 | 0.5751 |
| `1d` | `original_t2` | `20_10` | `tp1p0` | `original_t2` | 27 | 37.04% | -0.3239% | -1.4211% | -1,757.00 | 0.7017 |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | `original_t2` | 27 | 37.04% | -0.2462% | -1.4211% | -1,350.03 | 0.7729 |
| `1d` | `baseline_plus_t3` | `20_10` | `baseline` | `t3_swing` | 7 | 71.43% | 0.6943% | 1.2799% | 959.56 | 2.2875 |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | `original_t2` | 27 | 37.04% | -0.2462% | -1.4211% | -1,350.03 | 0.7729 |
| `1d` | `baseline_plus_t3` | `20_10` | `be0p5` | `t3_swing` | 7 | 71.43% | 0.6943% | 1.2799% | 959.56 | 2.2875 |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | `original_t2` | 27 | 37.04% | -0.2462% | -1.4211% | -1,350.03 | 0.7729 |
| `1d` | `baseline_plus_t3` | `20_10` | `cost0p5` | `t3_swing` | 7 | 71.43% | 0.6943% | 1.2799% | 959.56 | 2.2875 |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | `original_t2` | 27 | 37.04% | -0.2462% | -1.4211% | -1,350.03 | 0.7729 |
| `1d` | `baseline_plus_t3` | `20_10` | `lock1p0_20bps` | `t3_swing` | 7 | 71.43% | 0.6943% | 1.2799% | 959.56 | 2.2875 |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | `original_t2` | 27 | 37.04% | -0.2694% | -1.4211% | -1,475.98 | 0.752 |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p2_act0p5` | `t3_swing` | 7 | 71.43% | 0.9081% | 1.6270% | 1,263.46 | 2.6992 |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | `original_t2` | 26 | 15.38% | -0.5818% | -1.4289% | -3,072.95 | 0.5744 |
| `1d` | `baseline_plus_t3` | `20_10` | `trail0p5_act1p0` | `t3_swing` | 7 | 42.86% | 1.2644% | -1.2676% | 1,762.61 | 2.0496 |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `original_t2` | 27 | 37.04% | -0.3239% | -1.4211% | -1,762.24 | 0.7028 |
| `1d` | `baseline_plus_t3` | `20_10` | `tp1p0` | `t3_swing` | 7 | 71.43% | 0.6943% | 1.2799% | 960.36 | 2.2939 |

## Files

- Summary JSON: `research/eth_2026_jan_apr_1d_direct_breakout_exit_variants_summary.json`
- `1d` ledger: `research/tmp_eth_2026_jan_apr_1d_direct_breakout_exit_variants_1d_original_t2_20_10_baseline_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_eth_2026_jan_apr_1d_direct_breakout_exit_variants_1d_original_t2_20_10_be0p5_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_eth_2026_jan_apr_1d_direct_breakout_exit_variants_1d_original_t2_20_10_cost0p5_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_eth_2026_jan_apr_1d_direct_breakout_exit_variants_1d_original_t2_20_10_lock1p0_20bps_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_eth_2026_jan_apr_1d_direct_breakout_exit_variants_1d_original_t2_20_10_trail0p2_act0p5_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_eth_2026_jan_apr_1d_direct_breakout_exit_variants_1d_original_t2_20_10_trail0p5_act1p0_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_eth_2026_jan_apr_1d_direct_breakout_exit_variants_1d_original_t2_20_10_tp1p0_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_eth_2026_jan_apr_1d_direct_breakout_exit_variants_1d_baseline_plus_t3_20_10_baseline_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_eth_2026_jan_apr_1d_direct_breakout_exit_variants_1d_baseline_plus_t3_20_10_be0p5_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_eth_2026_jan_apr_1d_direct_breakout_exit_variants_1d_baseline_plus_t3_20_10_cost0p5_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_eth_2026_jan_apr_1d_direct_breakout_exit_variants_1d_baseline_plus_t3_20_10_lock1p0_20bps_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_eth_2026_jan_apr_1d_direct_breakout_exit_variants_1d_baseline_plus_t3_20_10_trail0p2_act0p5_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_eth_2026_jan_apr_1d_direct_breakout_exit_variants_1d_baseline_plus_t3_20_10_trail0p5_act1p0_observed_close_ledger.csv`
- `1d` ledger: `research/tmp_eth_2026_jan_apr_1d_direct_breakout_exit_variants_1d_baseline_plus_t3_20_10_tp1p0_observed_close_ledger.csv`
