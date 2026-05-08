# BTCUSDT Q1 2026 30min Virtual-SL Decoupled Real-Stop Sweep

Scope: research-only Python replay. No live or execution path is changed by this report.

## Setup

- Symbol/window: `BTCUSDT`, `2026-01-01T00:00:00+00:00` to `2026-03-31T23:59:59+00:00`
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
| `fixed_vsl_0p4atr_turn_0p1atr_realsl_vsl` | `vsl` | 0.40 | 0.10 |  |  | 83,641.98 | -16.36% | -16.51% | 319 | 15.99% | -5.66 | `Zero-Initial-Reentry:319` | 13.1596 |
| `fixed_vsl_0p4atr_turn_0p2atr_realsl_vsl` | `vsl` | 0.40 | 0.20 |  |  | 87,173.90 | -12.83% | -12.83% | 261 | 27.59% | -3.20 | `Zero-Initial-Reentry:261` | 20.184 |
| `fixed_vsl_0p6atr_turn_0p2atr_realsl_vsl` | `vsl` | 0.60 | 0.20 |  |  | 90,338.27 | -9.66% | -9.64% | 186 | 26.88% | -3.63 | `Zero-Initial-Reentry:186` | 19.1382 |
| `fixed_vsl_0p8atr_turn_0p2atr_realsl_vsl` | `vsl` | 0.80 | 0.20 |  |  | 92,648.25 | -7.35% | -7.36% | 148 | 29.73% | -3.01 | `Zero-Initial-Reentry:148` | 19.2003 |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p2atr` | `entry_atr` | 0.40 | 0.10 | 0.2 |  | 83,789.72 | -16.21% | -16.36% | 319 | 17.55% | -4.62 | `Zero-Initial-Reentry:319` | 14.4093 |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p3atr` | `entry_atr` | 0.40 | 0.10 | 0.3 |  | 84,146.67 | -15.85% | -16.00% | 313 | 26.20% | -3.67 | `Zero-Initial-Reentry:313` | 21.5944 |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.40 | 0.20 | 0.2 |  | 86,306.00 | -13.69% | -13.68% | 270 | 19.26% | -4.68 | `Zero-Initial-Reentry:270` | 14.2384 |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.40 | 0.20 | 0.3 |  | 87,124.26 | -12.88% | -12.88% | 264 | 28.79% | -2.91 | `Zero-Initial-Reentry:264` | 21.3424 |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p2atr` | `entry_atr` | 0.60 | 0.10 | 0.2 |  | 88,691.18 | -11.31% | -11.29% | 229 | 20.09% | -3.77 | `Zero-Initial-Reentry:229` | 13.6987 |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p3atr` | `entry_atr` | 0.60 | 0.10 | 0.3 |  | 87,282.48 | -12.72% | -12.70% | 229 | 22.27% | -4.96 | `Zero-Initial-Reentry:229` | 20.534 |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.60 | 0.20 | 0.2 |  | 90,562.74 | -9.44% | -9.42% | 192 | 21.35% | -3.47 | `Zero-Initial-Reentry:192` | 13.1909 |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.60 | 0.20 | 0.3 |  | 90,208.93 | -9.79% | -9.77% | 188 | 27.66% | -3.56 | `Zero-Initial-Reentry:188` | 19.9164 |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.80 | 0.20 | 0.2 |  | 91,758.30 | -8.24% | -8.22% | 152 | 19.08% | -5.20 | `Zero-Initial-Reentry:152` | 14.2874 |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.80 | 0.20 | 0.3 |  | 93,016.90 | -6.98% | -6.99% | 147 | 32.65% | -2.31 | `Zero-Initial-Reentry:147` | 21.3872 |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p05atr` | `extreme_buffer` | 0.40 | 0.10 |  | 0.05 | 84,821.11 | -15.18% | -15.33% | 303 | 32.34% | -2.96 | `Zero-Initial-Reentry:303` | 25.2213 |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p1atr` | `extreme_buffer` | 0.40 | 0.10 |  | 0.1 | 84,495.45 | -15.50% | -15.66% | 303 | 33.99% | -3.01 | `Zero-Initial-Reentry:303` | 28.9803 |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.40 | 0.20 |  | 0.05 | 87,323.58 | -12.68% | -12.68% | 246 | 39.02% | -2.57 | `Zero-Initial-Reentry:246` | 36.3759 |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p1atr` | `extreme_buffer` | 0.40 | 0.20 |  | 0.1 | 87,421.50 | -12.58% | -12.58% | 244 | 40.98% | -2.46 | `Zero-Initial-Reentry:244` | 40.3455 |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p05atr` | `extreme_buffer` | 0.60 | 0.10 |  | 0.05 | 88,346.84 | -11.65% | -11.64% | 221 | 29.86% | -3.47 | `Zero-Initial-Reentry:221` | 23.4141 |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p1atr` | `extreme_buffer` | 0.60 | 0.10 |  | 0.1 | 87,841.19 | -12.16% | -12.14% | 221 | 31.22% | -3.84 | `Zero-Initial-Reentry:221` | 27.3834 |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.60 | 0.20 |  | 0.05 | 90,267.04 | -9.73% | -9.71% | 180 | 38.89% | -2.98 | `Zero-Initial-Reentry:180` | 35.1608 |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p1atr` | `extreme_buffer` | 0.60 | 0.20 |  | 0.1 | 90,335.59 | -9.66% | -9.65% | 179 | 40.78% | -2.84 | `Zero-Initial-Reentry:179` | 38.1841 |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.80 | 0.20 |  | 0.05 | 93,222.00 | -6.78% | -6.78% | 140 | 45.00% | -1.89 | `Zero-Initial-Reentry:140` | 34.2084 |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p1atr` | `extreme_buffer` | 0.80 | 0.20 |  | 0.1 | 93,064.25 | -6.94% | -6.94% | 140 | 45.71% | -1.99 | `Zero-Initial-Reentry:140` | 38.4972 |

## Best By Return

- `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p05atr`: return `-6.78%`, trades `140`, win `45.00%`, MaxDD `-6.78%`.

## Entry Diagnostics

This table uses gross paired price PnL before commissions. The main result table win rate uses the existing fee-adjusted balance-accounting summary.

| Variant | Reason | Trades | Gross Win Rate | Gross PnL | Avg Gross PnL |
|---|---|---:|---:|---:|---:|
| `fixed_vsl_0p4atr_turn_0p1atr_realsl_vsl` | `Zero-Initial-Reentry` | 319 | 19.75% | -4,948.33 | -0.0857% |
| `fixed_vsl_0p4atr_turn_0p2atr_realsl_vsl` | `Zero-Initial-Reentry` | 261 | 32.95% | -3,269.71 | -0.0656% |
| `fixed_vsl_0p6atr_turn_0p2atr_realsl_vsl` | `Zero-Initial-Reentry` | 186 | 28.49% | -2,648.82 | -0.0734% |
| `fixed_vsl_0p8atr_turn_0p2atr_realsl_vsl` | `Zero-Initial-Reentry` | 148 | 33.78% | -1,699.58 | -0.0607% |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 319 | 20.38% | -4,792.18 | -0.0826% |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 313 | 30.03% | -4,555.95 | -0.0805% |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 270 | 21.85% | -3,870.23 | -0.0783% |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 264 | 33.71% | -3,225.65 | -0.0629% |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 229 | 21.83% | -2,814.34 | -0.0657% |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 229 | 24.89% | -4,286.46 | -0.1012% |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 192 | 22.40% | -2,182.69 | -0.0602% |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 188 | 28.72% | -2,709.13 | -0.0743% |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 152 | 22.37% | -2,482.72 | -0.0864% |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 147 | 36.05% | -1,359.12 | -0.0490% |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 303 | 38.61% | -4,189.40 | -0.0745% |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p1atr` | `Zero-Initial-Reentry` | 303 | 41.58% | -4,532.26 | -0.0806% |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 246 | 48.78% | -3,619.02 | -0.0759% |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p1atr` | `Zero-Initial-Reentry` | 244 | 52.05% | -3,581.21 | -0.0759% |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 221 | 34.39% | -3,433.23 | -0.0822% |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p1atr` | `Zero-Initial-Reentry` | 221 | 36.65% | -3,958.93 | -0.0953% |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 180 | 43.33% | -2,908.57 | -0.0841% |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p1atr` | `Zero-Initial-Reentry` | 179 | 46.37% | -2,871.57 | -0.0835% |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 140 | 51.43% | -1,406.83 | -0.0511% |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p1atr` | `Zero-Initial-Reentry` | 140 | 52.14% | -1,567.57 | -0.0572% |

## Pending Diagnostics

| Variant | Real SL | Virtual Expired Without SL | Armed By Reason | Triggered By Reason | Expired By Reason | Max Trades Blocked |
|---|---:|---:|---|---|---|---:|
| `fixed_vsl_0p4atr_turn_0p1atr_realsl_vsl` | 319 | 252 | `Zero-Initial-Reentry:377` | `Zero-Initial-Reentry:319` | `Zero-Initial-Reentry:57` | 1 |
| `fixed_vsl_0p4atr_turn_0p2atr_realsl_vsl` | 261 | 245 | `Zero-Initial-Reentry:345` | `Zero-Initial-Reentry:261` | `Zero-Initial-Reentry:84` | 0 |
| `fixed_vsl_0p6atr_turn_0p2atr_realsl_vsl` | 186 | 335 | `Zero-Initial-Reentry:237` | `Zero-Initial-Reentry:186` | `Zero-Initial-Reentry:51` | 0 |
| `fixed_vsl_0p8atr_turn_0p2atr_realsl_vsl` | 148 | 402 | `Zero-Initial-Reentry:174` | `Zero-Initial-Reentry:148` | `Zero-Initial-Reentry:26` | 0 |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p2atr` | 319 | 252 | `Zero-Initial-Reentry:376` | `Zero-Initial-Reentry:319` | `Zero-Initial-Reentry:56` | 1 |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p3atr` | 313 | 247 | `Zero-Initial-Reentry:370` | `Zero-Initial-Reentry:313` | `Zero-Initial-Reentry:56` | 1 |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p2atr` | 270 | 248 | `Zero-Initial-Reentry:354` | `Zero-Initial-Reentry:270` | `Zero-Initial-Reentry:84` | 0 |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p3atr` | 264 | 242 | `Zero-Initial-Reentry:349` | `Zero-Initial-Reentry:264` | `Zero-Initial-Reentry:85` | 0 |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p2atr` | 229 | 352 | `Zero-Initial-Reentry:254` | `Zero-Initial-Reentry:229` | `Zero-Initial-Reentry:25` | 0 |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p3atr` | 229 | 349 | `Zero-Initial-Reentry:254` | `Zero-Initial-Reentry:229` | `Zero-Initial-Reentry:25` | 0 |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p2atr` | 192 | 337 | `Zero-Initial-Reentry:244` | `Zero-Initial-Reentry:192` | `Zero-Initial-Reentry:52` | 0 |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p3atr` | 188 | 336 | `Zero-Initial-Reentry:239` | `Zero-Initial-Reentry:188` | `Zero-Initial-Reentry:51` | 0 |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p2atr` | 152 | 403 | `Zero-Initial-Reentry:178` | `Zero-Initial-Reentry:152` | `Zero-Initial-Reentry:26` | 0 |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p3atr` | 147 | 403 | `Zero-Initial-Reentry:173` | `Zero-Initial-Reentry:147` | `Zero-Initial-Reentry:26` | 0 |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p05atr` | 303 | 245 | `Zero-Initial-Reentry:359` | `Zero-Initial-Reentry:303` | `Zero-Initial-Reentry:56` | 0 |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p1atr` | 303 | 243 | `Zero-Initial-Reentry:358` | `Zero-Initial-Reentry:303` | `Zero-Initial-Reentry:55` | 0 |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p05atr` | 246 | 233 | `Zero-Initial-Reentry:328` | `Zero-Initial-Reentry:246` | `Zero-Initial-Reentry:82` | 0 |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p1atr` | 244 | 230 | `Zero-Initial-Reentry:327` | `Zero-Initial-Reentry:244` | `Zero-Initial-Reentry:83` | 0 |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p05atr` | 221 | 348 | `Zero-Initial-Reentry:244` | `Zero-Initial-Reentry:221` | `Zero-Initial-Reentry:23` | 0 |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p1atr` | 221 | 344 | `Zero-Initial-Reentry:244` | `Zero-Initial-Reentry:221` | `Zero-Initial-Reentry:23` | 0 |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p05atr` | 180 | 331 | `Zero-Initial-Reentry:228` | `Zero-Initial-Reentry:180` | `Zero-Initial-Reentry:48` | 0 |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p1atr` | 179 | 326 | `Zero-Initial-Reentry:227` | `Zero-Initial-Reentry:179` | `Zero-Initial-Reentry:48` | 0 |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p05atr` | 140 | 394 | `Zero-Initial-Reentry:164` | `Zero-Initial-Reentry:140` | `Zero-Initial-Reentry:24` | 0 |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p1atr` | 140 | 393 | `Zero-Initial-Reentry:164` | `Zero-Initial-Reentry:140` | `Zero-Initial-Reentry:24` | 0 |
