# ETHUSDT Q1 2026 1h Virtual-SL Decoupled Real-Stop Sweep

Scope: research-only Python replay. No live or execution path is changed by this report.

## Setup

- Symbol/window: `ETHUSDT`, `2026-01-01T00:00:00+00:00` to `2026-03-31T23:59:59+00:00`
- Execution bars: continuous `1s` bars rebuilt from Binance trade archives
- Signal timeframe: `1h`
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
| `fixed_vsl_0p45atr_turn_0p1atr_entrysl_0p2atr` | `entry_atr` | 0.45 | 0.10 | 0.2 |  | 82,683.59 | -17.32% | -17.37% | 309 | 17.15% | -5.84 | `Zero-Initial-Reentry:309` | 19.0422 |
| `fixed_vsl_0p45atr_turn_0p1atr_entrysl_0p3atr` | `entry_atr` | 0.45 | 0.10 | 0.3 |  | 82,781.82 | -17.22% | -17.27% | 300 | 24.33% | -4.13 | `Zero-Initial-Reentry:300` | 28.8317 |
| `fixed_vsl_0p45atr_turn_0p1atr_extbuf_0p05atr` | `extreme_buffer` | 0.45 | 0.10 |  | 0.05 | 82,844.41 | -17.16% | -17.20% | 293 | 30.38% | -4.14 | `Zero-Initial-Reentry:293` | 29.547 |
| `fixed_vsl_0p45atr_turn_0p15atr_entrysl_0p2atr` | `entry_atr` | 0.45 | 0.15 | 0.2 |  | 84,848.33 | -15.15% | -15.20% | 274 | 17.88% | -5.01 | `Zero-Initial-Reentry:274` | 18.7624 |
| `fixed_vsl_0p45atr_turn_0p15atr_entrysl_0p3atr` | `entry_atr` | 0.45 | 0.15 | 0.3 |  | 85,502.11 | -14.50% | -14.54% | 271 | 26.20% | -3.03 | `Zero-Initial-Reentry:271` | 28.2222 |
| `fixed_vsl_0p45atr_turn_0p15atr_extbuf_0p05atr` | `extreme_buffer` | 0.45 | 0.15 |  | 0.05 | 86,691.65 | -13.31% | -13.35% | 263 | 36.12% | -2.02 | `Zero-Initial-Reentry:263` | 35.564 |
| `fixed_vsl_0p45atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.45 | 0.20 | 0.2 |  | 86,849.48 | -13.15% | -13.18% | 247 | 19.84% | -4.35 | `Zero-Initial-Reentry:247` | 18.6852 |
| `fixed_vsl_0p45atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.45 | 0.20 | 0.3 |  | 87,189.07 | -12.81% | -12.84% | 243 | 27.57% | -2.97 | `Zero-Initial-Reentry:243` | 28.0017 |
| `fixed_vsl_0p45atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.45 | 0.20 |  | 0.05 | 88,501.35 | -11.50% | -11.53% | 232 | 41.38% | -1.69 | `Zero-Initial-Reentry:232` | 40.3817 |
| `fixed_vsl_0p5atr_turn_0p1atr_entrysl_0p2atr` | `entry_atr` | 0.50 | 0.10 | 0.2 |  | 85,572.12 | -14.43% | -14.43% | 280 | 20.36% | -3.35 | `Zero-Initial-Reentry:280` | 19.3704 |
| `fixed_vsl_0p5atr_turn_0p1atr_entrysl_0p3atr` | `entry_atr` | 0.50 | 0.10 | 0.3 |  | 86,398.25 | -13.60% | -13.60% | 276 | 30.80% | -2.38 | `Zero-Initial-Reentry:276` | 29.0556 |
| `fixed_vsl_0p5atr_turn_0p1atr_extbuf_0p05atr` | `extreme_buffer` | 0.50 | 0.10 |  | 0.05 | 85,439.66 | -14.56% | -14.56% | 276 | 33.33% | -2.75 | `Zero-Initial-Reentry:276` | 28.7831 |
| `fixed_vsl_0p5atr_turn_0p15atr_entrysl_0p2atr` | `entry_atr` | 0.50 | 0.15 | 0.2 |  | 86,765.48 | -13.23% | -13.22% | 253 | 18.58% | -3.52 | `Zero-Initial-Reentry:253` | 19.2584 |
| `fixed_vsl_0p5atr_turn_0p15atr_entrysl_0p3atr` | `entry_atr` | 0.50 | 0.15 | 0.3 |  | 86,778.82 | -13.22% | -13.21% | 250 | 27.60% | -3.01 | `Zero-Initial-Reentry:250` | 28.9838 |
| `fixed_vsl_0p5atr_turn_0p15atr_extbuf_0p05atr` | `extreme_buffer` | 0.50 | 0.15 |  | 0.05 | 86,476.51 | -13.52% | -13.51% | 244 | 36.48% | -2.75 | `Zero-Initial-Reentry:244` | 35.6221 |
| `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.50 | 0.20 | 0.2 |  | 88,478.45 | -11.52% | -11.53% | 221 | 19.46% | -3.19 | `Zero-Initial-Reentry:221` | 18.7277 |
| `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.50 | 0.20 | 0.3 |  | 90,256.97 | -9.74% | -9.75% | 222 | 30.18% | -1.04 | `Zero-Initial-Reentry:222` | 28.0639 |
| `fixed_vsl_0p5atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.50 | 0.20 |  | 0.05 | 90,623.72 | -9.38% | -9.38% | 213 | 42.72% | -0.79 | `Zero-Initial-Reentry:213` | 42.5273 |
| `fixed_vsl_0p55atr_turn_0p1atr_entrysl_0p2atr` | `entry_atr` | 0.55 | 0.10 | 0.2 |  | 87,056.62 | -12.94% | -12.93% | 257 | 21.01% | -2.81 | `Zero-Initial-Reentry:257` | 19.5166 |
| `fixed_vsl_0p55atr_turn_0p1atr_entrysl_0p3atr` | `entry_atr` | 0.55 | 0.10 | 0.3 |  | 88,413.12 | -11.59% | -11.57% | 253 | 31.62% | -1.49 | `Zero-Initial-Reentry:253` | 29.4171 |
| `fixed_vsl_0p55atr_turn_0p1atr_extbuf_0p05atr` | `extreme_buffer` | 0.55 | 0.10 |  | 0.05 | 87,288.89 | -12.71% | -12.69% | 254 | 33.86% | -2.05 | `Zero-Initial-Reentry:254` | 30.9774 |
| `fixed_vsl_0p55atr_turn_0p15atr_entrysl_0p2atr` | `entry_atr` | 0.55 | 0.15 | 0.2 |  | 87,550.85 | -12.45% | -12.45% | 236 | 19.49% | -3.37 | `Zero-Initial-Reentry:236` | 19.5264 |
| `fixed_vsl_0p55atr_turn_0p15atr_entrysl_0p3atr` | `entry_atr` | 0.55 | 0.15 | 0.3 |  | 89,100.51 | -10.90% | -10.90% | 234 | 32.05% | -1.69 | `Zero-Initial-Reentry:234` | 29.3679 |
| `fixed_vsl_0p55atr_turn_0p15atr_extbuf_0p05atr` | `extreme_buffer` | 0.55 | 0.15 |  | 0.05 | 87,773.40 | -12.23% | -12.22% | 232 | 38.36% | -2.32 | `Zero-Initial-Reentry:232` | 38.6508 |
| `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.55 | 0.20 | 0.2 |  | 88,537.49 | -11.46% | -11.45% | 212 | 17.92% | -3.68 | `Zero-Initial-Reentry:212` | 19.5111 |
| `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.55 | 0.20 | 0.3 |  | 89,273.13 | -10.73% | -10.72% | 210 | 28.57% | -2.44 | `Zero-Initial-Reentry:210` | 29.2776 |
| `fixed_vsl_0p55atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.55 | 0.20 |  | 0.05 | 88,525.90 | -11.47% | -11.46% | 204 | 40.20% | -2.54 | `Zero-Initial-Reentry:204` | 47.1149 |

## Best By Return

- `fixed_vsl_0p5atr_turn_0p2atr_extbuf_0p05atr`: return `-9.38%`, trades `213`, win `42.72%`, MaxDD `-9.38%`.

## Entry Diagnostics

This table uses gross paired price PnL before commissions. The main result table win rate uses the existing fee-adjusted balance-accounting summary.

| Variant | Reason | Trades | Gross Win Rate | Gross PnL | Avg Gross PnL |
|---|---|---:|---:|---:|---:|
| `fixed_vsl_0p45atr_turn_0p1atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 309 | 19.09% | -6,275.15 | -0.1140% |
| `fixed_vsl_0p45atr_turn_0p1atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 300 | 27.00% | -6,460.79 | -0.1195% |
| `fixed_vsl_0p45atr_turn_0p1atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 293 | 35.15% | -6,565.07 | -0.1252% |
| `fixed_vsl_0p45atr_turn_0p15atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 274 | 18.98% | -5,256.85 | -0.1046% |
| `fixed_vsl_0p45atr_turn_0p15atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 271 | 28.41% | -4,594.34 | -0.0923% |
| `fixed_vsl_0p45atr_turn_0p15atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 263 | 40.30% | -3,584.04 | -0.0739% |
| `fixed_vsl_0p45atr_turn_0p2atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 247 | 21.46% | -4,138.91 | -0.0908% |
| `fixed_vsl_0p45atr_turn_0p2atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 243 | 29.63% | -3,875.90 | -0.0875% |
| `fixed_vsl_0p45atr_turn_0p2atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 232 | 46.12% | -2,802.25 | -0.0648% |
| `fixed_vsl_0p5atr_turn_0p1atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 280 | 22.50% | -4,243.40 | -0.0841% |
| `fixed_vsl_0p5atr_turn_0p1atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 276 | 34.06% | -3,540.91 | -0.0720% |
| `fixed_vsl_0p5atr_turn_0p1atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 276 | 37.68% | -4,541.09 | -0.0917% |
| `fixed_vsl_0p5atr_turn_0p15atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 253 | 20.55% | -3,981.85 | -0.0874% |
| `fixed_vsl_0p5atr_turn_0p15atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 250 | 30.80% | -4,064.52 | -0.0895% |
| `fixed_vsl_0p5atr_turn_0p15atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 244 | 40.98% | -4,518.92 | -0.1006% |
| `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 221 | 20.81% | -3,359.40 | -0.0830% |
| `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 222 | 32.88% | -1,426.07 | -0.0365% |
| `fixed_vsl_0p5atr_turn_0p2atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 213 | 46.95% | -1,300.40 | -0.0332% |
| `fixed_vsl_0p55atr_turn_0p1atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 257 | 22.57% | -3,438.44 | -0.0747% |
| `fixed_vsl_0p55atr_turn_0p1atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 253 | 34.78% | -2,166.58 | -0.0494% |
| `fixed_vsl_0p55atr_turn_0p1atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 254 | 37.80% | -3,313.97 | -0.0731% |
| `fixed_vsl_0p55atr_turn_0p15atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 236 | 21.61% | -3,694.79 | -0.0872% |
| `fixed_vsl_0p55atr_turn_0p15atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 234 | 35.04% | -2,148.99 | -0.0532% |
| `fixed_vsl_0p55atr_turn_0p15atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 232 | 42.24% | -3,636.40 | -0.0875% |
| `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 212 | 18.87% | -3,587.59 | -0.0935% |
| `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 210 | 30.48% | -2,888.28 | -0.0761% |
| `fixed_vsl_0p55atr_turn_0p2atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 204 | 43.63% | -3,869.42 | -0.1020% |

## Pending Diagnostics

| Variant | Real SL | Virtual Expired Without SL | Armed By Reason | Triggered By Reason | Expired By Reason | Max Trades Blocked |
|---|---:|---:|---|---|---|---:|
| `fixed_vsl_0p45atr_turn_0p1atr_entrysl_0p2atr` | 309 | 279 | `Zero-Initial-Reentry:354` | `Zero-Initial-Reentry:309` | `Zero-Initial-Reentry:45` | 0 |
| `fixed_vsl_0p45atr_turn_0p1atr_entrysl_0p3atr` | 300 | 278 | `Zero-Initial-Reentry:345` | `Zero-Initial-Reentry:300` | `Zero-Initial-Reentry:45` | 0 |
| `fixed_vsl_0p45atr_turn_0p1atr_extbuf_0p05atr` | 293 | 275 | `Zero-Initial-Reentry:336` | `Zero-Initial-Reentry:293` | `Zero-Initial-Reentry:43` | 0 |
| `fixed_vsl_0p45atr_turn_0p15atr_entrysl_0p2atr` | 274 | 272 | `Zero-Initial-Reentry:344` | `Zero-Initial-Reentry:274` | `Zero-Initial-Reentry:70` | 0 |
| `fixed_vsl_0p45atr_turn_0p15atr_entrysl_0p3atr` | 271 | 268 | `Zero-Initial-Reentry:341` | `Zero-Initial-Reentry:271` | `Zero-Initial-Reentry:70` | 0 |
| `fixed_vsl_0p45atr_turn_0p15atr_extbuf_0p05atr` | 263 | 267 | `Zero-Initial-Reentry:329` | `Zero-Initial-Reentry:263` | `Zero-Initial-Reentry:66` | 0 |
| `fixed_vsl_0p45atr_turn_0p2atr_entrysl_0p2atr` | 247 | 267 | `Zero-Initial-Reentry:338` | `Zero-Initial-Reentry:247` | `Zero-Initial-Reentry:91` | 0 |
| `fixed_vsl_0p45atr_turn_0p2atr_entrysl_0p3atr` | 243 | 265 | `Zero-Initial-Reentry:332` | `Zero-Initial-Reentry:243` | `Zero-Initial-Reentry:89` | 0 |
| `fixed_vsl_0p45atr_turn_0p2atr_extbuf_0p05atr` | 232 | 262 | `Zero-Initial-Reentry:318` | `Zero-Initial-Reentry:232` | `Zero-Initial-Reentry:86` | 0 |
| `fixed_vsl_0p5atr_turn_0p1atr_entrysl_0p2atr` | 280 | 311 | `Zero-Initial-Reentry:318` | `Zero-Initial-Reentry:280` | `Zero-Initial-Reentry:36` | 2 |
| `fixed_vsl_0p5atr_turn_0p1atr_entrysl_0p3atr` | 276 | 309 | `Zero-Initial-Reentry:314` | `Zero-Initial-Reentry:276` | `Zero-Initial-Reentry:36` | 2 |
| `fixed_vsl_0p5atr_turn_0p1atr_extbuf_0p05atr` | 276 | 308 | `Zero-Initial-Reentry:313` | `Zero-Initial-Reentry:276` | `Zero-Initial-Reentry:35` | 2 |
| `fixed_vsl_0p5atr_turn_0p15atr_entrysl_0p2atr` | 253 | 301 | `Zero-Initial-Reentry:306` | `Zero-Initial-Reentry:253` | `Zero-Initial-Reentry:53` | 0 |
| `fixed_vsl_0p5atr_turn_0p15atr_entrysl_0p3atr` | 250 | 300 | `Zero-Initial-Reentry:302` | `Zero-Initial-Reentry:250` | `Zero-Initial-Reentry:52` | 0 |
| `fixed_vsl_0p5atr_turn_0p15atr_extbuf_0p05atr` | 244 | 297 | `Zero-Initial-Reentry:296` | `Zero-Initial-Reentry:244` | `Zero-Initial-Reentry:52` | 0 |
| `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p2atr` | 221 | 294 | `Zero-Initial-Reentry:298` | `Zero-Initial-Reentry:221` | `Zero-Initial-Reentry:77` | 0 |
| `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p3atr` | 222 | 292 | `Zero-Initial-Reentry:298` | `Zero-Initial-Reentry:222` | `Zero-Initial-Reentry:76` | 0 |
| `fixed_vsl_0p5atr_turn_0p2atr_extbuf_0p05atr` | 213 | 291 | `Zero-Initial-Reentry:288` | `Zero-Initial-Reentry:213` | `Zero-Initial-Reentry:75` | 0 |
| `fixed_vsl_0p55atr_turn_0p1atr_entrysl_0p2atr` | 257 | 339 | `Zero-Initial-Reentry:287` | `Zero-Initial-Reentry:257` | `Zero-Initial-Reentry:29` | 1 |
| `fixed_vsl_0p55atr_turn_0p1atr_entrysl_0p3atr` | 253 | 335 | `Zero-Initial-Reentry:283` | `Zero-Initial-Reentry:253` | `Zero-Initial-Reentry:29` | 1 |
| `fixed_vsl_0p55atr_turn_0p1atr_extbuf_0p05atr` | 254 | 335 | `Zero-Initial-Reentry:283` | `Zero-Initial-Reentry:254` | `Zero-Initial-Reentry:28` | 1 |
| `fixed_vsl_0p55atr_turn_0p15atr_entrysl_0p2atr` | 236 | 328 | `Zero-Initial-Reentry:282` | `Zero-Initial-Reentry:236` | `Zero-Initial-Reentry:45` | 1 |
| `fixed_vsl_0p55atr_turn_0p15atr_entrysl_0p3atr` | 234 | 325 | `Zero-Initial-Reentry:280` | `Zero-Initial-Reentry:234` | `Zero-Initial-Reentry:45` | 1 |
| `fixed_vsl_0p55atr_turn_0p15atr_extbuf_0p05atr` | 232 | 323 | `Zero-Initial-Reentry:277` | `Zero-Initial-Reentry:232` | `Zero-Initial-Reentry:44` | 1 |
| `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p2atr` | 212 | 317 | `Zero-Initial-Reentry:275` | `Zero-Initial-Reentry:212` | `Zero-Initial-Reentry:63` | 0 |
| `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p3atr` | 210 | 316 | `Zero-Initial-Reentry:273` | `Zero-Initial-Reentry:210` | `Zero-Initial-Reentry:63` | 0 |
| `fixed_vsl_0p55atr_turn_0p2atr_extbuf_0p05atr` | 204 | 313 | `Zero-Initial-Reentry:267` | `Zero-Initial-Reentry:204` | `Zero-Initial-Reentry:63` | 0 |
