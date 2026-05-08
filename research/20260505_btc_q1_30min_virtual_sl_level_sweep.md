# BTCUSDT Q1 2026 30min Virtual-SL Level Sweep

Scope: research-only Python replay. No live or execution path is changed by this report.

## Setup

- Symbol/window: `BTCUSDT`, `2026-01-01T00:00:00+00:00` to `2026-03-31T23:59:59+00:00`
- Execution bars: continuous `1s` bars rebuilt from Binance trade archives
- Signal timeframe: `30min`
- Breakout shape: `baseline_plus_t3`
- `re_p` usage: none
- Cooldown: none
- Breakout arm: 1s high/low touching the structural breakout level
- Virtual Initial reference price: structural breakout level, not the observed trigger-second close
- Initial fixed stop: not using the old fixed `0.3 ATR`; virtual SL and real initial stop come from the sweep level
- Turn confirmation modes: `fixed` enters after price recovers from the SL level by offset ATR; `dynamic` tracks the post-SL local low/high and enters after recovery from that extreme by offset ATR.
- Downstream `SL-Reentry`: enabled. A real SL exit arms another no-`re_p` trigger from the observed real stop level plus/minus the same entry offset ATR.
- Trailing stop retained for trend management: `trailing_stop_atr=0.3`, activated after `0.5 ATR` unrealized profit
- Sizing: `reentry_size_schedule=[0.2, 0.1]`, `max_trades_per_bar=2`

## Results

| Variant | Turn | VSL ATR | Turn Offset ATR | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Avg Loss | Worst Loss | Entry Mix | Median SL->Entry |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|
| `fixed_vsl_0p2atr_turn_0p05atr` | `fixed` | 0.20 | 0.05 | 40,653.65 | -59.35% | -59.34% | 1972 | 8.72% | -8.54 | -0.1335% | -0.2538% | `SL-Reentry:1244, Zero-Initial-Reentry:728` | 70.0 |
| `fixed_vsl_0p2atr_turn_0p1atr` | `fixed` | 0.20 | 0.10 | 43,386.35 | -56.61% | -56.60% | 1822 | 14.05% | -6.37 | -0.1588% | -0.3698% | `SL-Reentry:1207, Zero-Initial-Reentry:615` | 147.0 |
| `fixed_vsl_0p2atr_turn_0p15atr` | `fixed` | 0.20 | 0.15 | 44,721.91 | -55.28% | -55.27% | 1662 | 16.91% | -6.11 | -0.1875% | -0.5357% | `SL-Reentry:1126, Zero-Initial-Reentry:536` | 257.0 |
| `fixed_vsl_0p2atr_turn_0p2atr` | `fixed` | 0.20 | 0.20 | 50,802.74 | -49.20% | -49.19% | 1389 | 21.53% | -5.43 | -0.2127% | -0.6288% | `SL-Reentry:909, Zero-Initial-Reentry:480` | 331.0 |
| `fixed_vsl_0p4atr_turn_0p05atr` | `fixed` | 0.40 | 0.05 | 51,369.80 | -48.63% | -48.62% | 1451 | 8.96% | -10.05 | -0.1335% | -0.2663% | `SL-Reentry:932, Zero-Initial-Reentry:519` | 66.0 |
| `fixed_vsl_0p4atr_turn_0p1atr` | `fixed` | 0.40 | 0.10 | 52,549.31 | -47.45% | -47.44% | 1343 | 12.21% | -8.72 | -0.1586% | -0.3527% | `SL-Reentry:908, Zero-Initial-Reentry:435` | 157.0 |
| `fixed_vsl_0p4atr_turn_0p15atr` | `fixed` | 0.40 | 0.15 | 56,077.46 | -43.92% | -43.91% | 1198 | 16.36% | -6.81 | -0.1845% | -0.4406% | `SL-Reentry:804, Zero-Initial-Reentry:394` | 231.0 |
| `fixed_vsl_0p4atr_turn_0p2atr` | `fixed` | 0.40 | 0.20 | 60,642.41 | -39.36% | -39.35% | 1032 | 20.83% | -5.57 | -0.2075% | -0.5671% | `SL-Reentry:681, Zero-Initial-Reentry:351` | 320.5 |
| `fixed_vsl_0p4atr_turn_0p3atr` | `fixed` | 0.40 | 0.30 | 65,527.49 | -34.47% | -34.46% | 814 | 25.68% | -5.10 | -0.2597% | -0.8094% | `SL-Reentry:508, Zero-Initial-Reentry:306` | 467.0 |
| `fixed_vsl_0p6atr_turn_0p1atr` | `fixed` | 0.60 | 0.10 | 62,716.36 | -37.28% | -37.27% | 975 | 12.10% | -7.63 | -0.1615% | -0.3930% | `SL-Reentry:659, Zero-Initial-Reentry:316` | 140.0 |
| `fixed_vsl_0p6atr_turn_0p2atr` | `fixed` | 0.60 | 0.20 | 66,446.27 | -33.55% | -33.54% | 798 | 18.42% | -6.62 | -0.2086% | -0.4686% | `SL-Reentry:541, Zero-Initial-Reentry:257` | 321.0 |
| `fixed_vsl_0p8atr_turn_0p1atr` | `fixed` | 0.80 | 0.10 | 71,059.94 | -28.94% | -28.93% | 687 | 11.94% | -8.87 | -0.1605% | -0.3428% | `SL-Reentry:449, Zero-Initial-Reentry:238` | 122.0 |
| `fixed_vsl_0p8atr_turn_0p2atr` | `fixed` | 0.80 | 0.20 | 75,848.84 | -24.15% | -24.15% | 527 | 19.92% | -6.52 | -0.2158% | -0.5719% | `SL-Reentry:333, Zero-Initial-Reentry:194` | 310.0 |
| `dynamic_vsl_0p2atr_turn_0p1atr` | `dynamic` | 0.20 | 0.10 | 37,150.24 | -62.85% | -62.84% | 2348 | 4.26% | -10.23 | -0.0978% | -0.3206% | `SL-Reentry:1373, Zero-Initial-Reentry:975` | 54.0 |
| `dynamic_vsl_0p2atr_turn_0p2atr` | `dynamic` | 0.20 | 0.20 | 31,997.53 | -68.00% | -68.00% | 2674 | 7.33% | -7.84 | -0.1090% | -0.5357% | `SL-Reentry:1819, Zero-Initial-Reentry:855` | 210.0 |
| `dynamic_vsl_0p4atr_turn_0p05atr` | `dynamic` | 0.40 | 0.05 | 52,298.03 | -47.70% | -47.69% | 1516 | 2.84% | -16.17 | -0.0916% | -0.2663% | `SL-Reentry:832, Zero-Initial-Reentry:684` | 16.0 |
| `dynamic_vsl_0p4atr_turn_0p1atr` | `dynamic` | 0.40 | 0.10 | 50,599.47 | -49.40% | -49.39% | 1616 | 4.39% | -10.80 | -0.0961% | -0.2925% | `SL-Reentry:950, Zero-Initial-Reentry:666` | 52.0 |
| `dynamic_vsl_0p4atr_turn_0p15atr` | `dynamic` | 0.40 | 0.15 | 47,518.08 | -52.48% | -52.47% | 1761 | 5.85% | -9.61 | -0.1030% | -0.3719% | `SL-Reentry:1125, Zero-Initial-Reentry:636` | 110.0 |
| `dynamic_vsl_0p4atr_turn_0p2atr` | `dynamic` | 0.40 | 0.20 | 44,593.30 | -55.41% | -55.40% | 1887 | 6.94% | -8.16 | -0.1073% | -0.4670% | `SL-Reentry:1293, Zero-Initial-Reentry:594` | 198.0 |
| `dynamic_vsl_0p6atr_turn_0p1atr` | `dynamic` | 0.60 | 0.10 | 63,561.13 | -36.44% | -36.43% | 1065 | 4.04% | -12.18 | -0.0961% | -0.3930% | `SL-Reentry:617, Zero-Initial-Reentry:448` | 41.0 |
| `dynamic_vsl_0p6atr_turn_0p2atr` | `dynamic` | 0.60 | 0.20 | 57,755.24 | -42.24% | -42.23% | 1269 | 5.83% | -9.39 | -0.1089% | -0.5068% | `SL-Reentry:855, Zero-Initial-Reentry:414` | 156.0 |

## Best By Return

- `fixed_vsl_0p8atr_turn_0p2atr`: return `-24.15%`, trades `527`, win `19.92%`, MaxDD `-24.15%`.

## Entry Diagnostics

This table uses gross paired price PnL before commissions. The main result table win rate uses the existing fee-adjusted balance-accounting summary.

| Variant | Reason | Trades | Gross Win Rate | Gross PnL | Avg Gross PnL |
|---|---|---:|---:|---:|---:|
| `fixed_vsl_0p2atr_turn_0p05atr` | `SL-Reentry` | 1244 | 13.91% | -9,548.09 | -0.0845% |
| `fixed_vsl_0p2atr_turn_0p05atr` | `Zero-Initial-Reentry` | 728 | 12.64% | -8,085.97 | -0.0847% |
| `fixed_vsl_0p2atr_turn_0p1atr` | `SL-Reentry` | 1207 | 21.62% | -9,264.40 | -0.0753% |
| `fixed_vsl_0p2atr_turn_0p1atr` | `Zero-Initial-Reentry` | 615 | 18.70% | -6,821.51 | -0.0828% |
| `fixed_vsl_0p2atr_turn_0p15atr` | `SL-Reentry` | 1126 | 25.40% | -10,424.24 | -0.0833% |
| `fixed_vsl_0p2atr_turn_0p15atr` | `Zero-Initial-Reentry` | 536 | 24.44% | -6,187.07 | -0.0839% |
| `fixed_vsl_0p2atr_turn_0p2atr` | `SL-Reentry` | 909 | 30.25% | -9,242.70 | -0.0847% |
| `fixed_vsl_0p2atr_turn_0p2atr` | `Zero-Initial-Reentry` | 480 | 30.83% | -5,252.25 | -0.0742% |
| `fixed_vsl_0p4atr_turn_0p05atr` | `SL-Reentry` | 932 | 13.73% | -7,915.58 | -0.0871% |
| `fixed_vsl_0p4atr_turn_0p05atr` | `Zero-Initial-Reentry` | 519 | 12.33% | -6,860.31 | -0.0900% |
| `fixed_vsl_0p4atr_turn_0p1atr` | `SL-Reentry` | 908 | 18.39% | -9,181.80 | -0.0926% |
| `fixed_vsl_0p4atr_turn_0p1atr` | `Zero-Initial-Reentry` | 435 | 18.39% | -5,691.58 | -0.0897% |
| `fixed_vsl_0p4atr_turn_0p15atr` | `SL-Reentry` | 804 | 25.25% | -8,174.17 | -0.0869% |
| `fixed_vsl_0p4atr_turn_0p15atr` | `Zero-Initial-Reentry` | 394 | 24.62% | -5,110.40 | -0.0868% |
| `fixed_vsl_0p4atr_turn_0p2atr` | `SL-Reentry` | 681 | 27.90% | -7,465.94 | -0.0871% |
| `fixed_vsl_0p4atr_turn_0p2atr` | `Zero-Initial-Reentry` | 351 | 33.05% | -3,730.69 | -0.0686% |
| `fixed_vsl_0p4atr_turn_0p3atr` | `SL-Reentry` | 508 | 35.43% | -6,474.57 | -0.0880% |
| `fixed_vsl_0p4atr_turn_0p3atr` | `Zero-Initial-Reentry` | 306 | 38.89% | -4,032.89 | -0.0816% |
| `fixed_vsl_0p6atr_turn_0p1atr` | `SL-Reentry` | 659 | 18.06% | -6,787.66 | -0.0873% |
| `fixed_vsl_0p6atr_turn_0p1atr` | `Zero-Initial-Reentry` | 316 | 17.72% | -4,864.55 | -0.0968% |
| `fixed_vsl_0p6atr_turn_0p2atr` | `SL-Reentry` | 541 | 29.02% | -6,058.02 | -0.0840% |
| `fixed_vsl_0p6atr_turn_0p2atr` | `Zero-Initial-Reentry` | 257 | 23.35% | -4,838.29 | -0.1135% |
| `fixed_vsl_0p8atr_turn_0p1atr` | `SL-Reentry` | 449 | 18.49% | -5,447.00 | -0.0916% |
| `fixed_vsl_0p8atr_turn_0p1atr` | `Zero-Initial-Reentry` | 238 | 14.71% | -4,114.66 | -0.1032% |
| `fixed_vsl_0p8atr_turn_0p2atr` | `SL-Reentry` | 333 | 29.73% | -4,204.51 | -0.0853% |
| `fixed_vsl_0p8atr_turn_0p2atr` | `Zero-Initial-Reentry` | 194 | 25.26% | -3,710.44 | -0.1098% |
| `dynamic_vsl_0p2atr_turn_0p1atr` | `SL-Reentry` | 1373 | 5.17% | -7,784.11 | -0.0744% |
| `dynamic_vsl_0p2atr_turn_0p1atr` | `Zero-Initial-Reentry` | 975 | 7.79% | -9,456.52 | -0.0771% |
| `dynamic_vsl_0p2atr_turn_0p2atr` | `SL-Reentry` | 1819 | 10.01% | -10,240.45 | -0.0692% |
| `dynamic_vsl_0p2atr_turn_0p2atr` | `Zero-Initial-Reentry` | 855 | 11.70% | -7,816.79 | -0.0780% |
| `dynamic_vsl_0p4atr_turn_0p05atr` | `SL-Reentry` | 832 | 3.73% | -5,321.45 | -0.0761% |
| `dynamic_vsl_0p4atr_turn_0p05atr` | `Zero-Initial-Reentry` | 684 | 4.09% | -8,404.95 | -0.0830% |
| `dynamic_vsl_0p4atr_turn_0p1atr` | `SL-Reentry` | 950 | 5.26% | -6,211.92 | -0.0745% |
| `dynamic_vsl_0p4atr_turn_0p1atr` | `Zero-Initial-Reentry` | 666 | 7.96% | -7,229.92 | -0.0749% |
| `dynamic_vsl_0p4atr_turn_0p15atr` | `SL-Reentry` | 1125 | 7.64% | -7,570.25 | -0.0751% |
| `dynamic_vsl_0p4atr_turn_0p15atr` | `Zero-Initial-Reentry` | 636 | 10.85% | -6,484.37 | -0.0727% |
| `dynamic_vsl_0p4atr_turn_0p2atr` | `SL-Reentry` | 1293 | 9.28% | -8,542.03 | -0.0707% |
| `dynamic_vsl_0p4atr_turn_0p2atr` | `Zero-Initial-Reentry` | 594 | 12.29% | -6,203.43 | -0.0766% |
| `dynamic_vsl_0p6atr_turn_0p1atr` | `SL-Reentry` | 617 | 6.16% | -4,416.56 | -0.0743% |
| `dynamic_vsl_0p6atr_turn_0p1atr` | `Zero-Initial-Reentry` | 448 | 6.47% | -5,783.69 | -0.0800% |
| `dynamic_vsl_0p6atr_turn_0p2atr` | `SL-Reentry` | 855 | 9.24% | -6,880.76 | -0.0783% |
| `dynamic_vsl_0p6atr_turn_0p2atr` | `Zero-Initial-Reentry` | 414 | 11.11% | -4,943.02 | -0.0779% |

## Pending Diagnostics

| Variant | Real SL | Virtual Expired Without SL | Armed By Reason | Triggered By Reason | Expired By Reason | Max Trades Blocked |
|---|---:|---:|---|---|---|---:|
| `fixed_vsl_0p2atr_turn_0p05atr` | 1972 | 280 | `Zero-Initial-Reentry:885, SL-Reentry:1972` | `Zero-Initial-Reentry:728, SL-Reentry:1244` | `SL-Reentry:212, Zero-Initial-Reentry:82` | 591 |
| `fixed_vsl_0p2atr_turn_0p1atr` | 1821 | 236 | `Zero-Initial-Reentry:763, SL-Reentry:1821` | `Zero-Initial-Reentry:615, SL-Reentry:1207` | `SL-Reentry:291, Zero-Initial-Reentry:107` | 364 |
| `fixed_vsl_0p2atr_turn_0p15atr` | 1661 | 219 | `Zero-Initial-Reentry:679, SL-Reentry:1661` | `Zero-Initial-Reentry:536, SL-Reentry:1126` | `SL-Reentry:342, Zero-Initial-Reentry:121` | 215 |
| `fixed_vsl_0p2atr_turn_0p2atr` | 1388 | 214 | `Zero-Initial-Reentry:660, SL-Reentry:1388` | `Zero-Initial-Reentry:480, SL-Reentry:909` | `SL-Reentry:360, Zero-Initial-Reentry:167` | 132 |
| `fixed_vsl_0p4atr_turn_0p05atr` | 1451 | 472 | `Zero-Initial-Reentry:610, SL-Reentry:1451` | `Zero-Initial-Reentry:519, SL-Reentry:932` | `SL-Reentry:154, Zero-Initial-Reentry:67` | 389 |
| `fixed_vsl_0p4atr_turn_0p1atr` | 1343 | 430 | `Zero-Initial-Reentry:535, SL-Reentry:1343` | `Zero-Initial-Reentry:435, SL-Reentry:908` | `SL-Reentry:209, Zero-Initial-Reentry:85` | 241 |
| `fixed_vsl_0p4atr_turn_0p15atr` | 1198 | 416 | `Zero-Initial-Reentry:509, SL-Reentry:1198` | `Zero-Initial-Reentry:394, SL-Reentry:804` | `SL-Reentry:244, Zero-Initial-Reentry:104` | 160 |
| `fixed_vsl_0p4atr_turn_0p2atr` | 1032 | 393 | `Zero-Initial-Reentry:485, SL-Reentry:1032` | `Zero-Initial-Reentry:351, SL-Reentry:681` | `SL-Reentry:276, Zero-Initial-Reentry:128` | 81 |
| `fixed_vsl_0p4atr_turn_0p3atr` | 814 | 376 | `Zero-Initial-Reentry:488, SL-Reentry:814` | `Zero-Initial-Reentry:306, SL-Reentry:508` | `SL-Reentry:267, Zero-Initial-Reentry:181` | 40 |
| `fixed_vsl_0p6atr_turn_0p1atr` | 974 | 646 | `Zero-Initial-Reentry:387, SL-Reentry:974` | `Zero-Initial-Reentry:316, SL-Reentry:659` | `SL-Reentry:123, Zero-Initial-Reentry:62` | 201 |
| `fixed_vsl_0p6atr_turn_0p2atr` | 798 | 598 | `Zero-Initial-Reentry:349, SL-Reentry:798` | `Zero-Initial-Reentry:257, SL-Reentry:541` | `SL-Reentry:173, Zero-Initial-Reentry:86` | 89 |
| `fixed_vsl_0p8atr_turn_0p1atr` | 687 | 774 | `Zero-Initial-Reentry:276, SL-Reentry:687` | `Zero-Initial-Reentry:238, SL-Reentry:449` | `SL-Reentry:119, Zero-Initial-Reentry:36` | 121 |
| `fixed_vsl_0p8atr_turn_0p2atr` | 527 | 739 | `Zero-Initial-Reentry:262, SL-Reentry:527` | `Zero-Initial-Reentry:194, SL-Reentry:333` | `Zero-Initial-Reentry:66, SL-Reentry:139` | 57 |
| `dynamic_vsl_0p2atr_turn_0p1atr` | 2348 | 389 | `Zero-Initial-Reentry:1172, SL-Reentry:2348` | `Zero-Initial-Reentry:975, SL-Reentry:1373` | `` | 1171 |
| `dynamic_vsl_0p2atr_turn_0p2atr` | 2674 | 340 | `Zero-Initial-Reentry:980, SL-Reentry:2674` | `Zero-Initial-Reentry:855, SL-Reentry:1819` | `Zero-Initial-Reentry:3, SL-Reentry:7` | 969 |
| `dynamic_vsl_0p4atr_turn_0p05atr` | 1516 | 613 | `Zero-Initial-Reentry:748, SL-Reentry:1516` | `Zero-Initial-Reentry:684, SL-Reentry:832` | `` | 748 |
| `dynamic_vsl_0p4atr_turn_0p1atr` | 1616 | 603 | `Zero-Initial-Reentry:725, SL-Reentry:1616` | `Zero-Initial-Reentry:666, SL-Reentry:950` | `` | 725 |
| `dynamic_vsl_0p4atr_turn_0p15atr` | 1761 | 574 | `Zero-Initial-Reentry:688, SL-Reentry:1761` | `Zero-Initial-Reentry:636, SL-Reentry:1125` | `SL-Reentry:1` | 687 |
| `dynamic_vsl_0p4atr_turn_0p2atr` | 1887 | 535 | `Zero-Initial-Reentry:646, SL-Reentry:1887` | `Zero-Initial-Reentry:594, SL-Reentry:1293` | `SL-Reentry:6, Zero-Initial-Reentry:2` | 638 |
| `dynamic_vsl_0p6atr_turn_0p1atr` | 1065 | 790 | `Zero-Initial-Reentry:482, SL-Reentry:1065` | `Zero-Initial-Reentry:448, SL-Reentry:617` | `` | 482 |
| `dynamic_vsl_0p6atr_turn_0p2atr` | 1269 | 742 | `Zero-Initial-Reentry:444, SL-Reentry:1269` | `Zero-Initial-Reentry:414, SL-Reentry:855` | `SL-Reentry:2, Zero-Initial-Reentry:1` | 441 |
