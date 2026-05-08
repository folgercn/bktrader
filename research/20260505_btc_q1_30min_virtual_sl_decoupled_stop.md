# BTCUSDT Q1 2026 30min Virtual-SL Decoupled Real-Stop Sweep

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
- Virtual SL is used only as fake-breakout filtering; real position stop is swept independently.
- Real stop modes: `vsl` keeps the old tight stop at the virtual SL level; `entry_atr` stops from the observed entry by fixed ATR; `extreme_buffer` stops outside the post-VSL local low/high by a small ATR buffer.
- Turn confirmation: `fixed` enters after price recovers from the SL level by offset ATR.
- Downstream `SL-Reentry`: enabled. A real SL exit arms another no-`re_p` trigger from the observed real stop level plus/minus the same entry offset ATR.
- Trailing stop retained for trend management: `trailing_stop_atr=0.3`, activated after `0.5 ATR` unrealized profit
- Sizing: `reentry_size_schedule=[0.2, 0.1]`, `max_trades_per_bar=2`

## Results

| Variant | Real Stop | VSL ATR | Turn Offset ATR | Stop ATR | Buffer ATR | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Entry Mix | Median Real Stop bps |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|
| `fixed_vsl_0p4atr_turn_0p1atr_realsl_vsl` | `vsl` | 0.40 | 0.10 |  |  | 52,549.31 | -47.45% | -47.44% | 1343 | 12.21% | -8.72 | `SL-Reentry:908, Zero-Initial-Reentry:435` | 10.588 |
| `fixed_vsl_0p4atr_turn_0p2atr_realsl_vsl` | `vsl` | 0.40 | 0.20 |  |  | 60,642.41 | -39.36% | -39.35% | 1032 | 20.83% | -5.57 | `SL-Reentry:681, Zero-Initial-Reentry:351` | 15.4113 |
| `fixed_vsl_0p6atr_turn_0p2atr_realsl_vsl` | `vsl` | 0.60 | 0.20 |  |  | 66,446.27 | -33.55% | -33.54% | 798 | 18.42% | -6.62 | `SL-Reentry:541, Zero-Initial-Reentry:257` | 15.4451 |
| `fixed_vsl_0p8atr_turn_0p2atr_realsl_vsl` | `vsl` | 0.80 | 0.20 |  |  | 75,848.84 | -24.15% | -24.15% | 527 | 19.92% | -6.52 | `SL-Reentry:333, Zero-Initial-Reentry:194` | 15.6975 |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p2atr` | `entry_atr` | 0.40 | 0.10 | 0.2 |  | 53,810.86 | -46.19% | -46.18% | 1304 | 12.04% | -7.76 | `SL-Reentry:846, Zero-Initial-Reentry:458` | 9.9518 |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p3atr` | `entry_atr` | 0.40 | 0.10 | 0.3 |  | 50,596.23 | -49.40% | -49.39% | 1421 | 19.07% | -5.91 | `SL-Reentry:1012, Zero-Initial-Reentry:409` | 14.9869 |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.40 | 0.20 | 0.2 |  | 63,571.59 | -36.43% | -36.42% | 941 | 12.54% | -7.36 | `SL-Reentry:561, Zero-Initial-Reentry:380` | 9.6817 |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.40 | 0.20 | 0.3 |  | 62,356.26 | -37.64% | -37.63% | 972 | 19.65% | -5.76 | `SL-Reentry:607, Zero-Initial-Reentry:365` | 14.4971 |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p2atr` | `entry_atr` | 0.60 | 0.10 | 0.2 |  | 64,559.47 | -35.44% | -35.43% | 918 | 11.66% | -7.75 | `SL-Reentry:601, Zero-Initial-Reentry:317` | 9.9474 |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p3atr` | `entry_atr` | 0.60 | 0.10 | 0.3 |  | 60,280.76 | -39.72% | -39.71% | 1019 | 16.39% | -6.57 | `SL-Reentry:727, Zero-Initial-Reentry:292` | 14.5433 |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.60 | 0.20 | 0.2 |  | 70,125.49 | -29.87% | -29.86% | 720 | 10.42% | -8.03 | `SL-Reentry:435, Zero-Initial-Reentry:285` | 9.865 |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.60 | 0.20 | 0.3 |  | 67,765.90 | -32.23% | -32.22% | 767 | 17.08% | -6.59 | `SL-Reentry:507, Zero-Initial-Reentry:260` | 14.6798 |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.80 | 0.20 | 0.2 |  | 77,899.31 | -22.10% | -22.09% | 520 | 14.42% | -6.22 | `SL-Reentry:314, Zero-Initial-Reentry:206` | 10.1405 |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.80 | 0.20 | 0.3 |  | 75,803.31 | -24.20% | -24.20% | 535 | 19.25% | -6.52 | `SL-Reentry:341, Zero-Initial-Reentry:194` | 15.0603 |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p05atr` | `extreme_buffer` | 0.40 | 0.10 |  | 0.05 | 50,667.06 | -49.33% | -49.32% | 1387 | 25.02% | -4.44 | `SL-Reentry:1026, Zero-Initial-Reentry:361` | 18.7103 |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p1atr` | `extreme_buffer` | 0.40 | 0.10 |  | 0.1 | 45,861.16 | -54.14% | -54.13% | 1521 | 26.43% | -4.56 | `SL-Reentry:1173, Zero-Initial-Reentry:348` | 22.3991 |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.40 | 0.20 |  | 0.05 | 59,044.21 | -40.96% | -40.94% | 1090 | 32.11% | -3.32 | `SL-Reentry:770, Zero-Initial-Reentry:320` | 25.5659 |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p1atr` | `extreme_buffer` | 0.40 | 0.20 |  | 0.1 | 60,070.34 | -39.93% | -39.92% | 1054 | 34.91% | -2.99 | `SL-Reentry:744, Zero-Initial-Reentry:310` | 28.9248 |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p05atr` | `extreme_buffer` | 0.60 | 0.10 |  | 0.05 | 60,544.11 | -39.46% | -39.44% | 1010 | 25.45% | -4.84 | `SL-Reentry:744, Zero-Initial-Reentry:266` | 19.2476 |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p1atr` | `extreme_buffer` | 0.60 | 0.10 |  | 0.1 | 58,340.86 | -41.66% | -41.65% | 1081 | 27.20% | -4.47 | `SL-Reentry:821, Zero-Initial-Reentry:260` | 22.0883 |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.60 | 0.20 |  | 0.05 | 68,379.20 | -31.62% | -31.61% | 785 | 29.17% | -3.34 | `SL-Reentry:550, Zero-Initial-Reentry:235` | 25.0273 |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p1atr` | `extreme_buffer` | 0.60 | 0.20 |  | 0.1 | 67,593.23 | -32.41% | -32.40% | 805 | 31.68% | -3.29 | `SL-Reentry:571, Zero-Initial-Reentry:234` | 27.7594 |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.80 | 0.20 |  | 0.05 | 75,729.08 | -24.27% | -24.27% | 580 | 34.48% | -2.49 | `SL-Reentry:396, Zero-Initial-Reentry:184` | 27.8813 |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p1atr` | `extreme_buffer` | 0.80 | 0.20 |  | 0.1 | 74,057.48 | -25.94% | -25.99% | 599 | 34.89% | -2.97 | `SL-Reentry:420, Zero-Initial-Reentry:179` | 30.9624 |

## Best By Return

- `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p2atr`: return `-22.10%`, trades `520`, win `14.42%`, MaxDD `-22.09%`.

## Entry Diagnostics

This table uses gross paired price PnL before commissions. The main result table win rate uses the existing fee-adjusted balance-accounting summary.

| Variant | Reason | Trades | Gross Win Rate | Gross PnL | Avg Gross PnL |
|---|---|---:|---:|---:|---:|
| `fixed_vsl_0p4atr_turn_0p1atr_realsl_vsl` | `SL-Reentry` | 908 | 18.39% | -9,181.80 | -0.0926% |
| `fixed_vsl_0p4atr_turn_0p1atr_realsl_vsl` | `Zero-Initial-Reentry` | 435 | 18.39% | -5,691.58 | -0.0897% |
| `fixed_vsl_0p4atr_turn_0p2atr_realsl_vsl` | `SL-Reentry` | 681 | 27.90% | -7,465.94 | -0.0871% |
| `fixed_vsl_0p4atr_turn_0p2atr_realsl_vsl` | `Zero-Initial-Reentry` | 351 | 33.05% | -3,730.69 | -0.0686% |
| `fixed_vsl_0p6atr_turn_0p2atr_realsl_vsl` | `SL-Reentry` | 541 | 29.02% | -6,058.02 | -0.0840% |
| `fixed_vsl_0p6atr_turn_0p2atr_realsl_vsl` | `Zero-Initial-Reentry` | 257 | 23.35% | -4,838.29 | -0.1135% |
| `fixed_vsl_0p8atr_turn_0p2atr_realsl_vsl` | `SL-Reentry` | 333 | 29.73% | -4,204.51 | -0.0853% |
| `fixed_vsl_0p8atr_turn_0p2atr_realsl_vsl` | `Zero-Initial-Reentry` | 194 | 25.26% | -3,710.44 | -0.1098% |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p2atr` | `SL-Reentry` | 846 | 15.01% | -8,266.20 | -0.0888% |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 458 | 14.85% | -6,134.21 | -0.0905% |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p3atr` | `SL-Reentry` | 1012 | 25.69% | -10,117.01 | -0.0920% |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 409 | 24.69% | -4,951.11 | -0.0828% |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p2atr` | `SL-Reentry` | 561 | 15.33% | -6,129.61 | -0.0898% |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 380 | 18.16% | -4,667.75 | -0.0760% |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p3atr` | `SL-Reentry` | 607 | 25.21% | -6,345.20 | -0.0870% |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 365 | 27.67% | -4,594.10 | -0.0789% |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p2atr` | `SL-Reentry` | 601 | 14.98% | -6,259.01 | -0.0887% |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 317 | 15.46% | -4,796.81 | -0.0945% |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p3atr` | `SL-Reentry` | 727 | 24.48% | -8,294.14 | -0.0928% |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 292 | 23.63% | -4,784.10 | -0.1068% |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p2atr` | `SL-Reentry` | 435 | 15.63% | -4,756.94 | -0.0871% |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 285 | 14.39% | -4,554.64 | -0.0953% |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p3atr` | `SL-Reentry` | 507 | 26.23% | -5,789.71 | -0.0887% |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 260 | 22.31% | -4,516.91 | -0.1047% |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p2atr` | `SL-Reentry` | 314 | 21.02% | -2,579.28 | -0.0636% |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 206 | 14.08% | -3,738.02 | -0.1034% |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p3atr` | `SL-Reentry` | 341 | 27.27% | -4,219.67 | -0.0865% |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 194 | 22.68% | -3,815.55 | -0.1134% |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p05atr` | `SL-Reentry` | 1026 | 37.43% | -9,644.13 | -0.0812% |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 361 | 34.90% | -4,829.48 | -0.0954% |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p1atr` | `SL-Reentry` | 1173 | 37.68% | -12,846.97 | -0.0938% |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p1atr` | `Zero-Initial-Reentry` | 348 | 39.66% | -4,395.89 | -0.0946% |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p05atr` | `SL-Reentry` | 770 | 46.36% | -6,761.81 | -0.0646% |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 320 | 47.50% | -3,802.20 | -0.0783% |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p1atr` | `SL-Reentry` | 744 | 48.25% | -6,508.67 | -0.0625% |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p1atr` | `Zero-Initial-Reentry` | 310 | 50.00% | -3,454.23 | -0.0744% |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p05atr` | `SL-Reentry` | 744 | 37.10% | -8,878.32 | -0.0926% |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 266 | 39.10% | -2,923.25 | -0.0728% |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p1atr` | `SL-Reentry` | 821 | 38.12% | -9,532.40 | -0.0931% |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p1atr` | `Zero-Initial-Reentry` | 260 | 41.92% | -3,047.73 | -0.0774% |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p05atr` | `SL-Reentry` | 550 | 47.09% | -4,499.41 | -0.0598% |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 235 | 42.13% | -3,426.02 | -0.0886% |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p1atr` | `SL-Reentry` | 571 | 48.34% | -4,961.36 | -0.0658% |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p1atr` | `Zero-Initial-Reentry` | 234 | 45.30% | -3,294.76 | -0.0855% |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p05atr` | `SL-Reentry` | 396 | 50.25% | -2,460.83 | -0.0422% |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 184 | 44.57% | -3,434.87 | -0.1040% |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p1atr` | `SL-Reentry` | 420 | 49.52% | -3,652.85 | -0.0599% |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p1atr` | `Zero-Initial-Reentry` | 179 | 45.81% | -3,571.73 | -0.1124% |

## Pending Diagnostics

| Variant | Real SL | Virtual Expired Without SL | Armed By Reason | Triggered By Reason | Expired By Reason | Max Trades Blocked |
|---|---:|---:|---|---|---|---:|
| `fixed_vsl_0p4atr_turn_0p1atr_realsl_vsl` | 1343 | 430 | `Zero-Initial-Reentry:535, SL-Reentry:1343` | `Zero-Initial-Reentry:435, SL-Reentry:908` | `SL-Reentry:209, Zero-Initial-Reentry:85` | 241 |
| `fixed_vsl_0p4atr_turn_0p2atr_realsl_vsl` | 1032 | 393 | `Zero-Initial-Reentry:485, SL-Reentry:1032` | `Zero-Initial-Reentry:351, SL-Reentry:681` | `SL-Reentry:276, Zero-Initial-Reentry:128` | 81 |
| `fixed_vsl_0p6atr_turn_0p2atr_realsl_vsl` | 798 | 598 | `Zero-Initial-Reentry:349, SL-Reentry:798` | `Zero-Initial-Reentry:257, SL-Reentry:541` | `SL-Reentry:173, Zero-Initial-Reentry:86` | 89 |
| `fixed_vsl_0p8atr_turn_0p2atr_realsl_vsl` | 527 | 739 | `Zero-Initial-Reentry:262, SL-Reentry:527` | `Zero-Initial-Reentry:194, SL-Reentry:333` | `Zero-Initial-Reentry:66, SL-Reentry:139` | 57 |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p2atr` | 1304 | 455 | `Zero-Initial-Reentry:571, SL-Reentry:1304` | `Zero-Initial-Reentry:458, SL-Reentry:846` | `SL-Reentry:189, Zero-Initial-Reentry:94` | 288 |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p3atr` | 1421 | 410 | `Zero-Initial-Reentry:509, SL-Reentry:1421` | `Zero-Initial-Reentry:409, SL-Reentry:1012` | `SL-Reentry:203, Zero-Initial-Reentry:87` | 219 |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p2atr` | 941 | 437 | `Zero-Initial-Reentry:528, SL-Reentry:941` | `Zero-Initial-Reentry:380, SL-Reentry:561` | `Zero-Initial-Reentry:142, SL-Reentry:254` | 132 |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p3atr` | 972 | 418 | `Zero-Initial-Reentry:510, SL-Reentry:972` | `Zero-Initial-Reentry:365, SL-Reentry:607` | `Zero-Initial-Reentry:138, SL-Reentry:263` | 108 |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p2atr` | 917 | 645 | `Zero-Initial-Reentry:392, SL-Reentry:917` | `Zero-Initial-Reentry:317, SL-Reentry:601` | `SL-Reentry:136, Zero-Initial-Reentry:65` | 190 |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p3atr` | 1018 | 619 | `Zero-Initial-Reentry:362, SL-Reentry:1018` | `Zero-Initial-Reentry:292, SL-Reentry:727` | `SL-Reentry:139, Zero-Initial-Reentry:62` | 160 |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p2atr` | 720 | 631 | `Zero-Initial-Reentry:385, SL-Reentry:720` | `Zero-Initial-Reentry:285, SL-Reentry:435` | `SL-Reentry:179, Zero-Initial-Reentry:96` | 109 |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p3atr` | 767 | 609 | `Zero-Initial-Reentry:357, SL-Reentry:767` | `Zero-Initial-Reentry:260, SL-Reentry:507` | `SL-Reentry:172, Zero-Initial-Reentry:90` | 94 |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p2atr` | 520 | 766 | `Zero-Initial-Reentry:279, SL-Reentry:520` | `Zero-Initial-Reentry:206, SL-Reentry:314` | `SL-Reentry:137, Zero-Initial-Reentry:70` | 72 |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p3atr` | 535 | 746 | `Zero-Initial-Reentry:265, SL-Reentry:535` | `Zero-Initial-Reentry:194, SL-Reentry:341` | `SL-Reentry:142, Zero-Initial-Reentry:69` | 54 |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p05atr` | 1386 | 363 | `Zero-Initial-Reentry:446, SL-Reentry:1386` | `Zero-Initial-Reentry:361, SL-Reentry:1026` | `SL-Reentry:206, Zero-Initial-Reentry:75` | 164 |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p1atr` | 1520 | 328 | `Zero-Initial-Reentry:420, SL-Reentry:1520` | `Zero-Initial-Reentry:348, SL-Reentry:1173` | `SL-Reentry:211, Zero-Initial-Reentry:63` | 145 |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p05atr` | 1089 | 345 | `Zero-Initial-Reentry:433, SL-Reentry:1089` | `Zero-Initial-Reentry:320, SL-Reentry:770` | `SL-Reentry:250, Zero-Initial-Reentry:107` | 75 |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p1atr` | 1054 | 349 | `Zero-Initial-Reentry:420, SL-Reentry:1054` | `Zero-Initial-Reentry:310, SL-Reentry:744` | `SL-Reentry:246, Zero-Initial-Reentry:104` | 69 |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p05atr` | 1009 | 563 | `Zero-Initial-Reentry:323, SL-Reentry:1009` | `Zero-Initial-Reentry:266, SL-Reentry:744` | `SL-Reentry:157, Zero-Initial-Reentry:50` | 115 |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p1atr` | 1080 | 545 | `Zero-Initial-Reentry:311, SL-Reentry:1080` | `Zero-Initial-Reentry:260, SL-Reentry:821` | `SL-Reentry:150, Zero-Initial-Reentry:46` | 114 |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p05atr` | 785 | 545 | `Zero-Initial-Reentry:310, SL-Reentry:785` | `Zero-Initial-Reentry:235, SL-Reentry:550` | `SL-Reentry:176, Zero-Initial-Reentry:72` | 61 |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p1atr` | 805 | 537 | `Zero-Initial-Reentry:306, SL-Reentry:805` | `Zero-Initial-Reentry:234, SL-Reentry:571` | `SL-Reentry:181, Zero-Initial-Reentry:69` | 55 |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p05atr` | 580 | 691 | `Zero-Initial-Reentry:247, SL-Reentry:580` | `Zero-Initial-Reentry:184, SL-Reentry:396` | `SL-Reentry:142, Zero-Initial-Reentry:60` | 45 |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p1atr` | 599 | 693 | `Zero-Initial-Reentry:242, SL-Reentry:599` | `Zero-Initial-Reentry:179, SL-Reentry:420` | `SL-Reentry:137, Zero-Initial-Reentry:60` | 45 |
