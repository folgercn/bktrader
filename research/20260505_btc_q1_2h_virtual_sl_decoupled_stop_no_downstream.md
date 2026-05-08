# BTCUSDT Q1 2026 30min Virtual-SL Decoupled Real-Stop Sweep

Scope: research-only Python replay. No live or execution path is changed by this report.

## Setup

- Symbol/window: `BTCUSDT`, `2026-01-01T00:00:00+00:00` to `2026-03-31T23:59:59+00:00`
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
| `fixed_vsl_0p4atr_turn_0p1atr_realsl_vsl` | `vsl` | 0.40 | 0.10 |  |  | 91,203.42 | -8.80% | -8.78% | 171 | 19.88% | -3.69 | `Zero-Initial-Reentry:171` | 15.926 |
| `fixed_vsl_0p4atr_turn_0p2atr_realsl_vsl` | `vsl` | 0.40 | 0.20 |  |  | 93,521.21 | -6.48% | -6.46% | 148 | 30.41% | -1.20 | `Zero-Initial-Reentry:148` | 25.7281 |
| `fixed_vsl_0p6atr_turn_0p2atr_realsl_vsl` | `vsl` | 0.60 | 0.20 |  |  | 95,755.96 | -4.24% | -4.22% | 109 | 33.94% | -0.12 | `Zero-Initial-Reentry:109` | 26.6525 |
| `fixed_vsl_0p8atr_turn_0p2atr_realsl_vsl` | `vsl` | 0.80 | 0.20 |  |  | 95,748.60 | -4.25% | -4.23% | 79 | 26.58% | -3.31 | `Zero-Initial-Reentry:79` | 26.3937 |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p2atr` | `entry_atr` | 0.40 | 0.10 | 0.2 |  | 90,671.18 | -9.33% | -9.31% | 170 | 22.35% | -4.11 | `Zero-Initial-Reentry:170` | 20.0879 |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p3atr` | `entry_atr` | 0.40 | 0.10 | 0.3 |  | 92,076.53 | -7.92% | -7.91% | 168 | 35.12% | -1.66 | `Zero-Initial-Reentry:168` | 30.4382 |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.40 | 0.20 | 0.2 |  | 93,232.92 | -6.77% | -6.75% | 149 | 24.16% | -1.59 | `Zero-Initial-Reentry:149` | 19.9293 |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.40 | 0.20 | 0.3 |  | 93,089.64 | -6.91% | -6.89% | 148 | 32.43% | -1.63 | `Zero-Initial-Reentry:148` | 30.082 |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p2atr` | `entry_atr` | 0.60 | 0.10 | 0.2 |  | 93,922.52 | -6.08% | -6.06% | 132 | 27.27% | -1.35 | `Zero-Initial-Reentry:132` | 20.8931 |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p3atr` | `entry_atr` | 0.60 | 0.10 | 0.3 |  | 93,681.51 | -6.32% | -6.30% | 131 | 34.35% | -1.51 | `Zero-Initial-Reentry:131` | 31.3439 |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.60 | 0.20 | 0.2 |  | 95,018.15 | -4.98% | -4.96% | 112 | 26.79% | -1.18 | `Zero-Initial-Reentry:112` | 20.7214 |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.60 | 0.20 | 0.3 |  | 96,209.11 | -3.79% | -3.77% | 109 | 38.53% | 0.50 | `Zero-Initial-Reentry:109` | 31.3013 |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.80 | 0.20 | 0.2 |  | 95,766.76 | -4.23% | -4.21% | 80 | 22.50% | -3.57 | `Zero-Initial-Reentry:80` | 20.5399 |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.80 | 0.20 | 0.3 |  | 95,935.61 | -4.06% | -4.05% | 79 | 31.65% | -2.33 | `Zero-Initial-Reentry:79` | 31.1046 |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p05atr` | `extreme_buffer` | 0.40 | 0.10 |  | 0.05 | 92,062.69 | -7.94% | -8.12% | 165 | 38.18% | -1.69 | `Zero-Initial-Reentry:165` | 33.735 |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p1atr` | `extreme_buffer` | 0.40 | 0.10 |  | 0.1 | 92,585.43 | -7.41% | -7.60% | 163 | 41.72% | -1.16 | `Zero-Initial-Reentry:163` | 39.2057 |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.40 | 0.20 |  | 0.05 | 92,798.26 | -7.20% | -7.18% | 144 | 44.44% | -1.58 | `Zero-Initial-Reentry:144` | 46.7343 |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p1atr` | `extreme_buffer` | 0.40 | 0.20 |  | 0.1 | 92,363.61 | -7.64% | -7.62% | 143 | 45.45% | -1.95 | `Zero-Initial-Reentry:143` | 50.8609 |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p05atr` | `extreme_buffer` | 0.60 | 0.10 |  | 0.05 | 94,055.38 | -5.94% | -6.10% | 128 | 37.50% | -1.10 | `Zero-Initial-Reentry:128` | 33.4369 |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p1atr` | `extreme_buffer` | 0.60 | 0.10 |  | 0.1 | 93,789.13 | -6.21% | -6.37% | 127 | 39.37% | -1.42 | `Zero-Initial-Reentry:127` | 38.8092 |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.60 | 0.20 |  | 0.05 | 96,028.54 | -3.97% | -4.14% | 108 | 46.30% | 0.14 | `Zero-Initial-Reentry:108` | 49.4707 |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p1atr` | `extreme_buffer` | 0.60 | 0.20 |  | 0.1 | 95,497.98 | -4.50% | -4.66% | 108 | 46.30% | -0.48 | `Zero-Initial-Reentry:108` | 55.5827 |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.80 | 0.20 |  | 0.05 | 96,688.62 | -3.31% | -3.47% | 76 | 44.74% | -0.56 | `Zero-Initial-Reentry:76` | 48.1848 |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p1atr` | `extreme_buffer` | 0.80 | 0.20 |  | 0.1 | 96,338.43 | -3.66% | -3.82% | 75 | 44.00% | -1.31 | `Zero-Initial-Reentry:75` | 52.2276 |

## Best By Return

- `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p05atr`: return `-3.31%`, trades `76`, win `44.74%`, MaxDD `-3.47%`.

## Entry Diagnostics

This table uses gross paired price PnL before commissions. The main result table win rate uses the existing fee-adjusted balance-accounting summary.

| Variant | Reason | Trades | Gross Win Rate | Gross PnL | Avg Gross PnL |
|---|---|---:|---:|---:|---:|
| `fixed_vsl_0p4atr_turn_0p1atr_realsl_vsl` | `Zero-Initial-Reentry` | 171 | 21.64% | -2,428.88 | -0.0757% |
| `fixed_vsl_0p4atr_turn_0p2atr_realsl_vsl` | `Zero-Initial-Reentry` | 148 | 32.43% | -875.00 | -0.0365% |
| `fixed_vsl_0p6atr_turn_0p2atr_realsl_vsl` | `Zero-Initial-Reentry` | 109 | 36.70% | -88.50 | -0.0036% |
| `fixed_vsl_0p8atr_turn_0p2atr_realsl_vsl` | `Zero-Initial-Reentry` | 79 | 29.11% | -1,218.00 | -0.0816% |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 170 | 22.94% | -3,019.60 | -0.0949% |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 168 | 37.50% | -1,633.78 | -0.0545% |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 149 | 25.50% | -1,136.42 | -0.0440% |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 148 | 33.78% | -1,319.17 | -0.0529% |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 132 | 27.27% | -1,060.94 | -0.0369% |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 131 | 35.11% | -1,344.30 | -0.0491% |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 112 | 27.68% | -738.95 | -0.0334% |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 109 | 40.37% | 370.78 | 0.0174% |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 80 | 23.75% | -1,155.05 | -0.0789% |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 79 | 34.18% | -1,028.21 | -0.0642% |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 165 | 40.61% | -1,768.07 | -0.0585% |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p1atr` | `Zero-Initial-Reentry` | 163 | 44.17% | -1,292.08 | -0.0438% |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 144 | 46.53% | -1,775.90 | -0.0656% |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p1atr` | `Zero-Initial-Reentry` | 143 | 47.55% | -2,257.41 | -0.0844% |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 128 | 39.06% | -1,068.69 | -0.0372% |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p1atr` | `Zero-Initial-Reentry` | 127 | 40.94% | -1,386.99 | -0.0503% |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 108 | 49.07% | 141.34 | 0.0059% |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p1atr` | `Zero-Initial-Reentry` | 108 | 49.07% | -400.41 | -0.0204% |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 76 | 51.32% | -371.44 | -0.0196% |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p1atr` | `Zero-Initial-Reentry` | 75 | 50.67% | -746.11 | -0.0478% |

## Pending Diagnostics

| Variant | Real SL | Virtual Expired Without SL | Armed By Reason | Triggered By Reason | Expired By Reason | Max Trades Blocked |
|---|---:|---:|---|---|---|---:|
| `fixed_vsl_0p4atr_turn_0p1atr_realsl_vsl` | 171 | 135 | `Zero-Initial-Reentry:202` | `Zero-Initial-Reentry:171` | `Zero-Initial-Reentry:29` | 2 |
| `fixed_vsl_0p4atr_turn_0p2atr_realsl_vsl` | 148 | 127 | `Zero-Initial-Reentry:185` | `Zero-Initial-Reentry:148` | `Zero-Initial-Reentry:37` | 0 |
| `fixed_vsl_0p6atr_turn_0p2atr_realsl_vsl` | 109 | 166 | `Zero-Initial-Reentry:138` | `Zero-Initial-Reentry:109` | `Zero-Initial-Reentry:29` | 0 |
| `fixed_vsl_0p8atr_turn_0p2atr_realsl_vsl` | 79 | 197 | `Zero-Initial-Reentry:98` | `Zero-Initial-Reentry:79` | `Zero-Initial-Reentry:19` | 0 |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p2atr` | 170 | 135 | `Zero-Initial-Reentry:201` | `Zero-Initial-Reentry:170` | `Zero-Initial-Reentry:29` | 2 |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p3atr` | 168 | 131 | `Zero-Initial-Reentry:198` | `Zero-Initial-Reentry:168` | `Zero-Initial-Reentry:29` | 1 |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p2atr` | 149 | 129 | `Zero-Initial-Reentry:188` | `Zero-Initial-Reentry:149` | `Zero-Initial-Reentry:39` | 0 |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p3atr` | 148 | 127 | `Zero-Initial-Reentry:187` | `Zero-Initial-Reentry:148` | `Zero-Initial-Reentry:39` | 0 |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p2atr` | 132 | 173 | `Zero-Initial-Reentry:148` | `Zero-Initial-Reentry:132` | `Zero-Initial-Reentry:16` | 0 |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p3atr` | 131 | 172 | `Zero-Initial-Reentry:146` | `Zero-Initial-Reentry:131` | `Zero-Initial-Reentry:15` | 0 |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p2atr` | 112 | 165 | `Zero-Initial-Reentry:141` | `Zero-Initial-Reentry:112` | `Zero-Initial-Reentry:29` | 0 |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p3atr` | 109 | 167 | `Zero-Initial-Reentry:137` | `Zero-Initial-Reentry:109` | `Zero-Initial-Reentry:28` | 0 |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p2atr` | 80 | 198 | `Zero-Initial-Reentry:99` | `Zero-Initial-Reentry:80` | `Zero-Initial-Reentry:19` | 0 |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p3atr` | 79 | 196 | `Zero-Initial-Reentry:98` | `Zero-Initial-Reentry:79` | `Zero-Initial-Reentry:19` | 0 |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p05atr` | 165 | 132 | `Zero-Initial-Reentry:193` | `Zero-Initial-Reentry:165` | `Zero-Initial-Reentry:28` | 0 |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p1atr` | 163 | 132 | `Zero-Initial-Reentry:190` | `Zero-Initial-Reentry:163` | `Zero-Initial-Reentry:27` | 0 |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p05atr` | 144 | 123 | `Zero-Initial-Reentry:179` | `Zero-Initial-Reentry:144` | `Zero-Initial-Reentry:35` | 0 |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p1atr` | 143 | 123 | `Zero-Initial-Reentry:178` | `Zero-Initial-Reentry:143` | `Zero-Initial-Reentry:35` | 0 |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p05atr` | 128 | 173 | `Zero-Initial-Reentry:144` | `Zero-Initial-Reentry:128` | `Zero-Initial-Reentry:16` | 0 |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p1atr` | 127 | 173 | `Zero-Initial-Reentry:142` | `Zero-Initial-Reentry:127` | `Zero-Initial-Reentry:15` | 0 |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p05atr` | 108 | 167 | `Zero-Initial-Reentry:134` | `Zero-Initial-Reentry:108` | `Zero-Initial-Reentry:26` | 0 |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p1atr` | 108 | 167 | `Zero-Initial-Reentry:134` | `Zero-Initial-Reentry:108` | `Zero-Initial-Reentry:26` | 0 |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p05atr` | 76 | 194 | `Zero-Initial-Reentry:95` | `Zero-Initial-Reentry:76` | `Zero-Initial-Reentry:19` | 0 |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p1atr` | 75 | 194 | `Zero-Initial-Reentry:94` | `Zero-Initial-Reentry:75` | `Zero-Initial-Reentry:19` | 0 |
