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
- Downstream `SL-Reentry`: disabled.
- Trailing stop retained for trend management: `trailing_stop_atr=0.3`, activated after `0.5 ATR` unrealized profit
- Sizing: `reentry_size_schedule=[0.2, 0.1]`, `max_trades_per_bar=2`

## Results

| Variant | Real Stop | VSL ATR | Turn Offset ATR | Stop ATR | Buffer ATR | Final Balance | Return | Max DD | Trades | Win Rate | Sharpe | Entry Mix | Median Real Stop bps |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|
| `fixed_vsl_0p4atr_turn_0p1atr_realsl_vsl` | `vsl` | 0.40 | 0.10 |  |  | 72,116.93 | -27.88% | -27.87% | 559 | 12.34% | -9.46 | `Zero-Initial-Reentry:559` | 10.7222 |
| `fixed_vsl_0p4atr_turn_0p2atr_realsl_vsl` | `vsl` | 0.40 | 0.20 |  |  | 77,634.50 | -22.37% | -22.35% | 462 | 21.86% | -5.31 | `Zero-Initial-Reentry:462` | 15.7453 |
| `fixed_vsl_0p6atr_turn_0p2atr_realsl_vsl` | `vsl` | 0.60 | 0.20 |  |  | 82,371.95 | -17.63% | -17.61% | 326 | 16.26% | -7.23 | `Zero-Initial-Reentry:326` | 15.0887 |
| `fixed_vsl_0p8atr_turn_0p2atr_realsl_vsl` | `vsl` | 0.80 | 0.20 |  |  | 87,508.30 | -12.49% | -12.50% | 226 | 17.26% | -6.54 | `Zero-Initial-Reentry:226` | 15.4957 |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p2atr` | `entry_atr` | 0.40 | 0.10 | 0.2 |  | 72,001.65 | -28.00% | -27.98% | 562 | 11.39% | -9.28 | `Zero-Initial-Reentry:562` | 9.8196 |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p3atr` | `entry_atr` | 0.40 | 0.10 | 0.3 |  | 73,044.42 | -26.96% | -26.94% | 554 | 20.04% | -5.83 | `Zero-Initial-Reentry:554` | 14.7654 |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.40 | 0.20 | 0.2 |  | 77,412.10 | -22.59% | -22.57% | 476 | 15.76% | -6.19 | `Zero-Initial-Reentry:476` | 9.8524 |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.40 | 0.20 | 0.3 |  | 76,973.50 | -23.03% | -23.01% | 468 | 20.51% | -5.77 | `Zero-Initial-Reentry:468` | 14.7115 |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p2atr` | `entry_atr` | 0.60 | 0.10 | 0.2 |  | 79,917.54 | -20.08% | -20.07% | 384 | 11.20% | -9.53 | `Zero-Initial-Reentry:384` | 9.5421 |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p3atr` | `entry_atr` | 0.60 | 0.10 | 0.3 |  | 79,780.23 | -20.22% | -20.20% | 380 | 17.11% | -7.24 | `Zero-Initial-Reentry:380` | 14.3107 |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.60 | 0.20 | 0.2 |  | 82,532.46 | -17.47% | -17.45% | 331 | 9.97% | -8.81 | `Zero-Initial-Reentry:331` | 9.4946 |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.60 | 0.20 | 0.3 |  | 82,351.88 | -17.65% | -17.63% | 329 | 16.72% | -6.91 | `Zero-Initial-Reentry:329` | 14.2487 |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.80 | 0.20 | 0.2 |  | 87,425.19 | -12.57% | -12.56% | 230 | 10.43% | -8.00 | `Zero-Initial-Reentry:230` | 9.6459 |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.80 | 0.20 | 0.3 |  | 87,406.81 | -12.59% | -12.60% | 227 | 17.18% | -6.40 | `Zero-Initial-Reentry:227` | 14.4801 |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p05atr` | `extreme_buffer` | 0.40 | 0.10 |  | 0.05 | 72,604.72 | -27.40% | -27.38% | 536 | 23.69% | -5.81 | `Zero-Initial-Reentry:536` | 18.8712 |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p1atr` | `extreme_buffer` | 0.40 | 0.10 |  | 0.1 | 72,761.25 | -27.24% | -27.22% | 534 | 26.03% | -5.34 | `Zero-Initial-Reentry:534` | 21.9657 |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.40 | 0.20 |  | 0.05 | 76,813.70 | -23.19% | -23.17% | 444 | 30.41% | -4.59 | `Zero-Initial-Reentry:444` | 26.4198 |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p1atr` | `extreme_buffer` | 0.40 | 0.20 |  | 0.1 | 77,208.22 | -22.79% | -22.78% | 441 | 32.88% | -4.12 | `Zero-Initial-Reentry:441` | 29.2149 |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p05atr` | `extreme_buffer` | 0.60 | 0.10 |  | 0.05 | 81,008.22 | -18.99% | -18.98% | 374 | 24.33% | -5.09 | `Zero-Initial-Reentry:374` | 18.9749 |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p1atr` | `extreme_buffer` | 0.60 | 0.10 |  | 0.1 | 80,773.94 | -19.23% | -19.21% | 370 | 25.95% | -5.18 | `Zero-Initial-Reentry:370` | 21.2738 |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.60 | 0.20 |  | 0.05 | 83,049.12 | -16.95% | -16.93% | 316 | 26.90% | -4.74 | `Zero-Initial-Reentry:316` | 25.4456 |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p1atr` | `extreme_buffer` | 0.60 | 0.20 |  | 0.1 | 82,881.56 | -17.12% | -17.10% | 316 | 28.80% | -4.69 | `Zero-Initial-Reentry:316` | 27.9908 |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.80 | 0.20 |  | 0.05 | 87,261.22 | -12.74% | -12.75% | 223 | 28.70% | -4.51 | `Zero-Initial-Reentry:223` | 26.114 |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p1atr` | `extreme_buffer` | 0.80 | 0.20 |  | 0.1 | 86,890.94 | -13.11% | -13.12% | 222 | 29.28% | -4.76 | `Zero-Initial-Reentry:222` | 28.8192 |

## Best By Return

- `fixed_vsl_0p8atr_turn_0p2atr_realsl_vsl`: return `-12.49%`, trades `226`, win `17.26%`, MaxDD `-12.50%`.

## Entry Diagnostics

This table uses gross paired price PnL before commissions. The main result table win rate uses the existing fee-adjusted balance-accounting summary.

| Variant | Reason | Trades | Gross Win Rate | Gross PnL | Avg Gross PnL |
|---|---|---:|---:|---:|---:|
| `fixed_vsl_0p4atr_turn_0p1atr_realsl_vsl` | `Zero-Initial-Reentry` | 559 | 17.35% | -9,081.65 | -0.0972% |
| `fixed_vsl_0p4atr_turn_0p2atr_realsl_vsl` | `Zero-Initial-Reentry` | 462 | 31.82% | -6,229.66 | -0.0769% |
| `fixed_vsl_0p6atr_turn_0p2atr_realsl_vsl` | `Zero-Initial-Reentry` | 326 | 27.91% | -5,865.83 | -0.0986% |
| `fixed_vsl_0p8atr_turn_0p2atr_realsl_vsl` | `Zero-Initial-Reentry` | 226 | 27.43% | -4,127.42 | -0.0961% |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 562 | 13.70% | -9,110.54 | -0.0975% |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 554 | 25.27% | -8,205.83 | -0.0870% |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 476 | 19.12% | -6,025.13 | -0.0727% |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 468 | 26.92% | -6,746.21 | -0.0831% |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 384 | 15.89% | -6,507.94 | -0.0964% |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 380 | 26.05% | -6,742.09 | -0.1002% |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 331 | 15.71% | -5,518.06 | -0.0928% |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 329 | 25.23% | -5,764.65 | -0.0963% |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 230 | 15.22% | -4,078.58 | -0.0951% |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 227 | 25.11% | -4,191.20 | -0.0973% |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 536 | 34.33% | -9,251.61 | -0.1023% |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p1atr` | `Zero-Initial-Reentry` | 534 | 38.20% | -9,132.90 | -0.1010% |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 444 | 44.59% | -7,662.67 | -0.0990% |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p1atr` | `Zero-Initial-Reentry` | 441 | 47.39% | -7,304.08 | -0.0954% |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 374 | 37.43% | -5,633.49 | -0.0855% |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p1atr` | `Zero-Initial-Reentry` | 370 | 39.73% | -6,036.74 | -0.0921% |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 316 | 43.35% | -5,474.84 | -0.0962% |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p1atr` | `Zero-Initial-Reentry` | 316 | 45.57% | -5,665.50 | -0.0995% |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 223 | 44.39% | -4,511.51 | -0.1055% |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p1atr` | `Zero-Initial-Reentry` | 222 | 45.50% | -4,933.55 | -0.1165% |

## Pending Diagnostics

| Variant | Real SL | Virtual Expired Without SL | Armed By Reason | Triggered By Reason | Expired By Reason | Max Trades Blocked |
|---|---:|---:|---|---|---|---:|
| `fixed_vsl_0p4atr_turn_0p1atr_realsl_vsl` | 559 | 547 | `Zero-Initial-Reentry:670` | `Zero-Initial-Reentry:559` | `Zero-Initial-Reentry:109` | 2 |
| `fixed_vsl_0p4atr_turn_0p2atr_realsl_vsl` | 462 | 522 | `Zero-Initial-Reentry:629` | `Zero-Initial-Reentry:462` | `Zero-Initial-Reentry:167` | 0 |
| `fixed_vsl_0p6atr_turn_0p2atr_realsl_vsl` | 326 | 709 | `Zero-Initial-Reentry:435` | `Zero-Initial-Reentry:326` | `Zero-Initial-Reentry:107` | 1 |
| `fixed_vsl_0p8atr_turn_0p2atr_realsl_vsl` | 226 | 834 | `Zero-Initial-Reentry:306` | `Zero-Initial-Reentry:226` | `Zero-Initial-Reentry:79` | 1 |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p2atr` | 562 | 556 | `Zero-Initial-Reentry:673` | `Zero-Initial-Reentry:562` | `Zero-Initial-Reentry:109` | 2 |
| `fixed_vsl_0p4atr_turn_0p1atr_entrysl_0p3atr` | 554 | 548 | `Zero-Initial-Reentry:663` | `Zero-Initial-Reentry:554` | `Zero-Initial-Reentry:108` | 1 |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p2atr` | 476 | 531 | `Zero-Initial-Reentry:649` | `Zero-Initial-Reentry:476` | `Zero-Initial-Reentry:171` | 2 |
| `fixed_vsl_0p4atr_turn_0p2atr_entrysl_0p3atr` | 468 | 529 | `Zero-Initial-Reentry:637` | `Zero-Initial-Reentry:468` | `Zero-Initial-Reentry:168` | 1 |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p2atr` | 383 | 739 | `Zero-Initial-Reentry:458` | `Zero-Initial-Reentry:384` | `Zero-Initial-Reentry:72` | 2 |
| `fixed_vsl_0p6atr_turn_0p1atr_entrysl_0p3atr` | 379 | 737 | `Zero-Initial-Reentry:453` | `Zero-Initial-Reentry:380` | `Zero-Initial-Reentry:71` | 2 |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p2atr` | 331 | 717 | `Zero-Initial-Reentry:441` | `Zero-Initial-Reentry:331` | `Zero-Initial-Reentry:108` | 1 |
| `fixed_vsl_0p6atr_turn_0p2atr_entrysl_0p3atr` | 329 | 710 | `Zero-Initial-Reentry:438` | `Zero-Initial-Reentry:329` | `Zero-Initial-Reentry:107` | 1 |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p2atr` | 230 | 840 | `Zero-Initial-Reentry:311` | `Zero-Initial-Reentry:230` | `Zero-Initial-Reentry:80` | 1 |
| `fixed_vsl_0p8atr_turn_0p2atr_entrysl_0p3atr` | 227 | 834 | `Zero-Initial-Reentry:307` | `Zero-Initial-Reentry:227` | `Zero-Initial-Reentry:79` | 1 |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p05atr` | 536 | 536 | `Zero-Initial-Reentry:641` | `Zero-Initial-Reentry:536` | `Zero-Initial-Reentry:105` | 0 |
| `fixed_vsl_0p4atr_turn_0p1atr_extbuf_0p1atr` | 534 | 527 | `Zero-Initial-Reentry:637` | `Zero-Initial-Reentry:534` | `Zero-Initial-Reentry:103` | 0 |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p05atr` | 444 | 519 | `Zero-Initial-Reentry:605` | `Zero-Initial-Reentry:444` | `Zero-Initial-Reentry:161` | 0 |
| `fixed_vsl_0p4atr_turn_0p2atr_extbuf_0p1atr` | 441 | 516 | `Zero-Initial-Reentry:601` | `Zero-Initial-Reentry:441` | `Zero-Initial-Reentry:160` | 0 |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p05atr` | 373 | 730 | `Zero-Initial-Reentry:446` | `Zero-Initial-Reentry:374` | `Zero-Initial-Reentry:70` | 2 |
| `fixed_vsl_0p6atr_turn_0p1atr_extbuf_0p1atr` | 369 | 725 | `Zero-Initial-Reentry:442` | `Zero-Initial-Reentry:370` | `Zero-Initial-Reentry:70` | 2 |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p05atr` | 316 | 701 | `Zero-Initial-Reentry:424` | `Zero-Initial-Reentry:316` | `Zero-Initial-Reentry:107` | 0 |
| `fixed_vsl_0p6atr_turn_0p2atr_extbuf_0p1atr` | 316 | 695 | `Zero-Initial-Reentry:422` | `Zero-Initial-Reentry:316` | `Zero-Initial-Reentry:105` | 0 |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p05atr` | 223 | 828 | `Zero-Initial-Reentry:300` | `Zero-Initial-Reentry:223` | `Zero-Initial-Reentry:76` | 1 |
| `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p1atr` | 222 | 825 | `Zero-Initial-Reentry:299` | `Zero-Initial-Reentry:222` | `Zero-Initial-Reentry:76` | 1 |
