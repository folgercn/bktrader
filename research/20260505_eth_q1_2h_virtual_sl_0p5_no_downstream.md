# ETHUSDT Q1 2026 2h Virtual-SL Decoupled Real-Stop Sweep

Scope: research-only Python replay. No live or execution path is changed by this report.

## Setup

- Symbol/window: `ETHUSDT`, `2026-01-01T00:00:00+00:00` to `2026-03-31T23:59:59+00:00`
- Execution bars: continuous `1s` bars rebuilt from Binance trade archives
- Signal timeframe: `2h`
- Breakout shape: `baseline_plus_t3`
- `re_p` usage: none
- Cooldown: none
- Breakout arm: 1s high/low touching the structural breakout level
- Virtual Initial reference price: structural breakout level, not the observed trigger-second close
- Virtual SL is used only as fake-breakout filtering; real position stop is swept independently.
- Real stop modes: `vsl` keeps the old tight stop at the virtual SL level; `entry_atr` stops from the observed entry by fixed ATR; `extreme_buffer` stops outside the post-VSL local low/high by a small ATR buffer.
- Turn confirmation: `fixed` enters after price recovers from the SL level by offset ATR.
- Downstream `SL-Reentry`: disabled.
- Trailing stop retained for trend management: `trailing_stop_atr=0.3`, activated after `0.5 ATR` unrealized profit
- Sizing: `reentry_size_schedule=[0.2, 0.1]`, `max_trades_per_bar=2`

## Results

| Variant | Real Stop | VSL ATR | Turn Offset ATR | Stop ATR | Buffer ATR | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Entry Mix | Median Real Stop bps |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|
| `fixed_vsl_0p45atr_turn_0p1atr_entrysl_0p2atr` | `entry_atr` | 0.45 | 0.10 | 0.2 |  | 90,046.77 | -9.95% | -9.94% | 152 | 20.39% | -4.69 | `Zero-Initial-Reentry:152` | 30.2512 |
| `fixed_vsl_0p45atr_turn_0p1atr_entrysl_0p3atr` | `entry_atr` | 0.45 | 0.10 | 0.3 |  | 90,604.08 | -9.40% | -9.72% | 151 | 30.46% | -2.89 | `Zero-Initial-Reentry:151` | 45.4049 |
| `fixed_vsl_0p45atr_turn_0p1atr_extbuf_0p05atr` | `extreme_buffer` | 0.45 | 0.10 |  | 0.05 | 90,827.53 | -9.17% | -9.15% | 147 | 35.37% | -2.68 | `Zero-Initial-Reentry:147` | 47.0109 |
| `fixed_vsl_0p45atr_turn_0p15atr_entrysl_0p2atr` | `entry_atr` | 0.45 | 0.15 | 0.2 |  | 92,320.45 | -7.68% | -7.86% | 144 | 24.31% | -2.15 | `Zero-Initial-Reentry:144` | 29.2897 |
| `fixed_vsl_0p45atr_turn_0p15atr_entrysl_0p3atr` | `entry_atr` | 0.45 | 0.15 | 0.3 |  | 91,703.70 | -8.30% | -8.49% | 143 | 32.17% | -2.35 | `Zero-Initial-Reentry:143` | 43.7199 |
| `fixed_vsl_0p45atr_turn_0p15atr_extbuf_0p05atr` | `extreme_buffer` | 0.45 | 0.15 |  | 0.05 | 92,262.09 | -7.74% | -8.06% | 136 | 41.18% | -1.78 | `Zero-Initial-Reentry:136` | 55.9123 |
| `fixed_vsl_0p45atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.45 | 0.20 | 0.2 |  | 94,042.38 | -5.96% | -6.10% | 135 | 28.89% | -0.60 | `Zero-Initial-Reentry:135` | 28.8336 |
| `fixed_vsl_0p45atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.45 | 0.20 | 0.3 |  | 92,505.41 | -7.49% | -7.64% | 135 | 33.33% | -1.95 | `Zero-Initial-Reentry:135` | 41.7829 |
| `fixed_vsl_0p45atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.45 | 0.20 |  | 0.05 | 92,252.12 | -7.75% | -7.96% | 127 | 47.24% | -1.79 | `Zero-Initial-Reentry:127` | 66.7823 |
| `fixed_vsl_0p5atr_turn_0p1atr_entrysl_0p2atr` | `entry_atr` | 0.50 | 0.10 | 0.2 |  | 91,685.85 | -8.31% | -8.30% | 148 | 18.92% | -2.51 | `Zero-Initial-Reentry:148` | 30.2372 |
| `fixed_vsl_0p5atr_turn_0p1atr_entrysl_0p3atr` | `entry_atr` | 0.50 | 0.10 | 0.3 |  | 91,033.48 | -8.97% | -9.10% | 148 | 28.38% | -2.48 | `Zero-Initial-Reentry:148` | 45.356 |
| `fixed_vsl_0p5atr_turn_0p1atr_extbuf_0p05atr` | `extreme_buffer` | 0.50 | 0.10 |  | 0.05 | 90,380.51 | -9.62% | -9.60% | 148 | 29.05% | -2.60 | `Zero-Initial-Reentry:148` | 43.8284 |
| `fixed_vsl_0p5atr_turn_0p15atr_entrysl_0p2atr` | `entry_atr` | 0.50 | 0.15 | 0.2 |  | 91,725.99 | -8.27% | -8.26% | 131 | 19.85% | -3.73 | `Zero-Initial-Reentry:131` | 30.2367 |
| `fixed_vsl_0p5atr_turn_0p15atr_entrysl_0p3atr` | `entry_atr` | 0.50 | 0.15 | 0.3 |  | 93,071.84 | -6.93% | -7.21% | 131 | 32.82% | -1.56 | `Zero-Initial-Reentry:131` | 45.3302 |
| `fixed_vsl_0p5atr_turn_0p15atr_extbuf_0p05atr` | `extreme_buffer` | 0.50 | 0.15 |  | 0.05 | 92,135.65 | -7.86% | -7.96% | 126 | 40.48% | -2.11 | `Zero-Initial-Reentry:126` | 59.943 |
| `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.50 | 0.20 | 0.2 |  | 94,157.38 | -5.84% | -6.03% | 120 | 23.33% | -1.23 | `Zero-Initial-Reentry:120` | 29.4155 |
| `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.50 | 0.20 | 0.3 |  | 94,348.13 | -5.65% | -5.85% | 117 | 32.48% | -0.88 | `Zero-Initial-Reentry:117` | 43.7199 |
| `fixed_vsl_0p5atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.50 | 0.20 |  | 0.05 | 94,226.63 | -5.77% | -6.04% | 113 | 46.90% | -0.86 | `Zero-Initial-Reentry:113` | 70.0583 |
| `fixed_vsl_0p55atr_turn_0p1atr_entrysl_0p2atr` | `entry_atr` | 0.55 | 0.10 | 0.2 |  | 93,935.22 | -6.06% | -6.07% | 132 | 27.27% | -0.91 | `Zero-Initial-Reentry:132` | 29.6561 |
| `fixed_vsl_0p55atr_turn_0p1atr_entrysl_0p3atr` | `entry_atr` | 0.55 | 0.10 | 0.3 |  | 94,315.06 | -5.68% | -5.83% | 132 | 36.36% | -0.48 | `Zero-Initial-Reentry:132` | 44.4831 |
| `fixed_vsl_0p55atr_turn_0p1atr_extbuf_0p05atr` | `extreme_buffer` | 0.55 | 0.10 |  | 0.05 | 94,150.37 | -5.85% | -5.85% | 131 | 38.93% | -0.65 | `Zero-Initial-Reentry:131` | 41.9253 |
| `fixed_vsl_0p55atr_turn_0p15atr_entrysl_0p2atr` | `entry_atr` | 0.55 | 0.15 | 0.2 |  | 93,703.88 | -6.30% | -6.28% | 128 | 23.44% | -1.50 | `Zero-Initial-Reentry:128` | 30.1014 |
| `fixed_vsl_0p55atr_turn_0p15atr_entrysl_0p3atr` | `entry_atr` | 0.55 | 0.15 | 0.3 |  | 93,788.78 | -6.21% | -6.34% | 128 | 34.38% | -1.11 | `Zero-Initial-Reentry:128` | 45.0842 |
| `fixed_vsl_0p55atr_turn_0p15atr_extbuf_0p05atr` | `extreme_buffer` | 0.55 | 0.15 |  | 0.05 | 93,078.36 | -6.92% | -6.90% | 127 | 40.16% | -1.50 | `Zero-Initial-Reentry:127` | 52.9315 |
| `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.55 | 0.20 | 0.2 |  | 93,254.41 | -6.75% | -6.73% | 115 | 22.61% | -2.91 | `Zero-Initial-Reentry:115` | 30.2367 |
| `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.55 | 0.20 | 0.3 |  | 94,868.38 | -5.13% | -5.55% | 115 | 37.39% | -0.66 | `Zero-Initial-Reentry:115` | 45.3302 |
| `fixed_vsl_0p55atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.55 | 0.20 |  | 0.05 | 92,887.70 | -7.11% | -7.38% | 111 | 45.05% | -2.15 | `Zero-Initial-Reentry:111` | 65.0844 |

## Best By Return

- `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p3atr`: return `-5.13%`, trades `115`, win `37.39%`, MaxDD `-5.55%`.

## Entry Diagnostics

This table uses gross paired price PnL before commissions. The main result table win rate uses the existing fee-adjusted balance-accounting summary.

| Variant | Reason | Trades | Gross Win Rate | Gross PnL | Avg Gross PnL |
|---|---|---:|---:|---:|---:|
| `fixed_vsl_0p45atr_turn_0p1atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 152 | 20.39% | -4,329.19 | -0.1509% |
| `fixed_vsl_0p45atr_turn_0p1atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 151 | 31.13% | -3,790.43 | -0.1299% |
| `fixed_vsl_0p45atr_turn_0p1atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 147 | 36.05% | -3,698.58 | -0.1373% |
| `fixed_vsl_0p45atr_turn_0p15atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 144 | 24.31% | -2,315.57 | -0.0816% |
| `fixed_vsl_0p45atr_turn_0p15atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 143 | 32.17% | -2,938.83 | -0.1085% |
| `fixed_vsl_0p45atr_turn_0p15atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 136 | 41.91% | -2,630.63 | -0.1002% |
| `fixed_vsl_0p45atr_turn_0p2atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 135 | 28.89% | -856.64 | -0.0235% |
| `fixed_vsl_0p45atr_turn_0p2atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 135 | 34.07% | -2,428.98 | -0.0861% |
| `fixed_vsl_0p45atr_turn_0p2atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 127 | 48.03% | -2,980.11 | -0.1132% |
| `fixed_vsl_0p5atr_turn_0p1atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 148 | 20.27% | -2,817.99 | -0.1024% |
| `fixed_vsl_0p5atr_turn_0p1atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 148 | 29.73% | -3,519.15 | -0.1196% |
| `fixed_vsl_0p5atr_turn_0p1atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 148 | 30.41% | -4,179.63 | -0.1407% |
| `fixed_vsl_0p5atr_turn_0p15atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 131 | 19.85% | -3,385.14 | -0.1323% |
| `fixed_vsl_0p5atr_turn_0p15atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 131 | 33.59% | -2,008.57 | -0.0750% |
| `fixed_vsl_0p5atr_turn_0p15atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 126 | 41.27% | -3,134.94 | -0.1279% |
| `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 120 | 23.33% | -1,309.09 | -0.0521% |
| `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 117 | 32.48% | -1,205.50 | -0.0461% |
| `fixed_vsl_0p5atr_turn_0p2atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 113 | 47.79% | -1,479.88 | -0.0579% |
| `fixed_vsl_0p55atr_turn_0p1atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 132 | 27.27% | -1,085.51 | -0.0372% |
| `fixed_vsl_0p55atr_turn_0p1atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 132 | 36.36% | -716.86 | -0.0246% |
| `fixed_vsl_0p55atr_turn_0p1atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 131 | 38.93% | -905.48 | -0.0340% |
| `fixed_vsl_0p55atr_turn_0p15atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 128 | 24.22% | -1,476.64 | -0.0616% |
| `fixed_vsl_0p55atr_turn_0p15atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 128 | 35.16% | -1,405.67 | -0.0546% |
| `fixed_vsl_0p55atr_turn_0p15atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 127 | 40.94% | -2,159.55 | -0.0879% |
| `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 115 | 22.61% | -2,413.03 | -0.1087% |
| `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 115 | 37.39% | -770.21 | -0.0321% |
| `fixed_vsl_0p55atr_turn_0p2atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 111 | 45.05% | -2,924.20 | -0.1376% |

## Pending Diagnostics

| Variant | Real SL | Virtual Expired Without SL | Armed By Reason | Triggered By Reason | Expired By Reason | Max Trades Blocked |
|---|---:|---:|---|---|---|---:|
| `fixed_vsl_0p45atr_turn_0p1atr_entrysl_0p2atr` | 152 | 155 | `Zero-Initial-Reentry:181` | `Zero-Initial-Reentry:152` | `Zero-Initial-Reentry:18` | 11 |
| `fixed_vsl_0p45atr_turn_0p1atr_entrysl_0p3atr` | 151 | 155 | `Zero-Initial-Reentry:179` | `Zero-Initial-Reentry:151` | `Zero-Initial-Reentry:17` | 11 |
| `fixed_vsl_0p45atr_turn_0p1atr_extbuf_0p05atr` | 147 | 152 | `Zero-Initial-Reentry:175` | `Zero-Initial-Reentry:147` | `Zero-Initial-Reentry:17` | 11 |
| `fixed_vsl_0p45atr_turn_0p15atr_entrysl_0p2atr` | 144 | 151 | `Zero-Initial-Reentry:179` | `Zero-Initial-Reentry:144` | `Zero-Initial-Reentry:24` | 11 |
| `fixed_vsl_0p45atr_turn_0p15atr_entrysl_0p3atr` | 143 | 151 | `Zero-Initial-Reentry:177` | `Zero-Initial-Reentry:143` | `Zero-Initial-Reentry:23` | 11 |
| `fixed_vsl_0p45atr_turn_0p15atr_extbuf_0p05atr` | 136 | 151 | `Zero-Initial-Reentry:169` | `Zero-Initial-Reentry:136` | `Zero-Initial-Reentry:22` | 11 |
| `fixed_vsl_0p45atr_turn_0p2atr_entrysl_0p2atr` | 135 | 150 | `Zero-Initial-Reentry:173` | `Zero-Initial-Reentry:135` | `Zero-Initial-Reentry:27` | 11 |
| `fixed_vsl_0p45atr_turn_0p2atr_entrysl_0p3atr` | 135 | 149 | `Zero-Initial-Reentry:172` | `Zero-Initial-Reentry:135` | `Zero-Initial-Reentry:26` | 11 |
| `fixed_vsl_0p45atr_turn_0p2atr_extbuf_0p05atr` | 127 | 150 | `Zero-Initial-Reentry:162` | `Zero-Initial-Reentry:127` | `Zero-Initial-Reentry:24` | 11 |
| `fixed_vsl_0p5atr_turn_0p1atr_entrysl_0p2atr` | 148 | 172 | `Zero-Initial-Reentry:169` | `Zero-Initial-Reentry:148` | `Zero-Initial-Reentry:12` | 9 |
| `fixed_vsl_0p5atr_turn_0p1atr_entrysl_0p3atr` | 148 | 171 | `Zero-Initial-Reentry:168` | `Zero-Initial-Reentry:148` | `Zero-Initial-Reentry:11` | 9 |
| `fixed_vsl_0p5atr_turn_0p1atr_extbuf_0p05atr` | 148 | 167 | `Zero-Initial-Reentry:164` | `Zero-Initial-Reentry:148` | `Zero-Initial-Reentry:11` | 5 |
| `fixed_vsl_0p5atr_turn_0p15atr_entrysl_0p2atr` | 131 | 168 | `Zero-Initial-Reentry:160` | `Zero-Initial-Reentry:131` | `Zero-Initial-Reentry:20` | 9 |
| `fixed_vsl_0p5atr_turn_0p15atr_entrysl_0p3atr` | 131 | 167 | `Zero-Initial-Reentry:159` | `Zero-Initial-Reentry:131` | `Zero-Initial-Reentry:19` | 9 |
| `fixed_vsl_0p5atr_turn_0p15atr_extbuf_0p05atr` | 126 | 163 | `Zero-Initial-Reentry:150` | `Zero-Initial-Reentry:126` | `Zero-Initial-Reentry:19` | 5 |
| `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p2atr` | 120 | 162 | `Zero-Initial-Reentry:157` | `Zero-Initial-Reentry:120` | `Zero-Initial-Reentry:28` | 9 |
| `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p3atr` | 117 | 160 | `Zero-Initial-Reentry:154` | `Zero-Initial-Reentry:117` | `Zero-Initial-Reentry:28` | 9 |
| `fixed_vsl_0p5atr_turn_0p2atr_extbuf_0p05atr` | 113 | 160 | `Zero-Initial-Reentry:145` | `Zero-Initial-Reentry:113` | `Zero-Initial-Reentry:27` | 5 |
| `fixed_vsl_0p55atr_turn_0p1atr_entrysl_0p2atr` | 132 | 176 | `Zero-Initial-Reentry:154` | `Zero-Initial-Reentry:132` | `Zero-Initial-Reentry:14` | 8 |
| `fixed_vsl_0p55atr_turn_0p1atr_entrysl_0p3atr` | 132 | 174 | `Zero-Initial-Reentry:153` | `Zero-Initial-Reentry:132` | `Zero-Initial-Reentry:13` | 8 |
| `fixed_vsl_0p55atr_turn_0p1atr_extbuf_0p05atr` | 131 | 176 | `Zero-Initial-Reentry:150` | `Zero-Initial-Reentry:131` | `Zero-Initial-Reentry:14` | 5 |
| `fixed_vsl_0p55atr_turn_0p15atr_entrysl_0p2atr` | 128 | 176 | `Zero-Initial-Reentry:153` | `Zero-Initial-Reentry:128` | `Zero-Initial-Reentry:17` | 8 |
| `fixed_vsl_0p55atr_turn_0p15atr_entrysl_0p3atr` | 128 | 176 | `Zero-Initial-Reentry:152` | `Zero-Initial-Reentry:128` | `Zero-Initial-Reentry:16` | 8 |
| `fixed_vsl_0p55atr_turn_0p15atr_extbuf_0p05atr` | 127 | 172 | `Zero-Initial-Reentry:148` | `Zero-Initial-Reentry:127` | `Zero-Initial-Reentry:16` | 5 |
| `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p2atr` | 115 | 172 | `Zero-Initial-Reentry:147` | `Zero-Initial-Reentry:115` | `Zero-Initial-Reentry:24` | 8 |
| `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p3atr` | 115 | 171 | `Zero-Initial-Reentry:146` | `Zero-Initial-Reentry:115` | `Zero-Initial-Reentry:23` | 8 |
| `fixed_vsl_0p55atr_turn_0p2atr_extbuf_0p05atr` | 111 | 167 | `Zero-Initial-Reentry:139` | `Zero-Initial-Reentry:111` | `Zero-Initial-Reentry:23` | 5 |
