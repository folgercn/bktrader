# ETHUSDT Q1 2026 30min Virtual-SL Decoupled Real-Stop Sweep

Scope: research-only Python replay. No live or execution path is changed by this report.

## Setup

- Symbol/window: `ETHUSDT`, `2026-01-01T00:00:00+00:00` to `2026-03-31T23:59:59+00:00`
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
| `fixed_vsl_0p45atr_turn_0p1atr_entrysl_0p2atr` | `entry_atr` | 0.45 | 0.10 | 0.2 |  | 75,236.98 | -24.76% | -24.75% | 550 | 19.09% | -3.58 | `Zero-Initial-Reentry:550` | 13.3275 |
| `fixed_vsl_0p45atr_turn_0p1atr_entrysl_0p3atr` | `entry_atr` | 0.45 | 0.10 | 0.3 |  | 75,087.19 | -24.91% | -24.90% | 540 | 26.67% | -3.25 | `Zero-Initial-Reentry:540` | 19.9914 |
| `fixed_vsl_0p45atr_turn_0p1atr_extbuf_0p05atr` | `extreme_buffer` | 0.45 | 0.10 |  | 0.05 | 75,910.81 | -24.09% | -24.07% | 526 | 32.89% | -2.57 | `Zero-Initial-Reentry:526` | 23.0951 |
| `fixed_vsl_0p45atr_turn_0p15atr_entrysl_0p2atr` | `entry_atr` | 0.45 | 0.15 | 0.2 |  | 75,842.33 | -24.16% | -24.14% | 502 | 17.53% | -4.89 | `Zero-Initial-Reentry:502` | 13.5354 |
| `fixed_vsl_0p45atr_turn_0p15atr_entrysl_0p3atr` | `entry_atr` | 0.45 | 0.15 | 0.3 |  | 76,302.23 | -23.70% | -23.72% | 491 | 25.46% | -3.83 | `Zero-Initial-Reentry:491` | 20.3086 |
| `fixed_vsl_0p45atr_turn_0p15atr_extbuf_0p05atr` | `extreme_buffer` | 0.45 | 0.15 |  | 0.05 | 77,155.33 | -22.84% | -22.87% | 478 | 35.15% | -2.77 | `Zero-Initial-Reentry:478` | 27.4777 |
| `fixed_vsl_0p45atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.45 | 0.20 | 0.2 |  | 77,994.92 | -22.01% | -21.99% | 457 | 18.16% | -4.61 | `Zero-Initial-Reentry:457` | 13.6191 |
| `fixed_vsl_0p45atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.45 | 0.20 | 0.3 |  | 78,738.68 | -21.26% | -21.25% | 448 | 25.45% | -3.27 | `Zero-Initial-Reentry:448` | 20.4048 |
| `fixed_vsl_0p45atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.45 | 0.20 |  | 0.05 | 78,856.74 | -21.14% | -21.13% | 438 | 37.44% | -2.47 | `Zero-Initial-Reentry:438` | 31.2162 |
| `fixed_vsl_0p5atr_turn_0p1atr_entrysl_0p2atr` | `entry_atr` | 0.50 | 0.10 | 0.2 |  | 76,248.20 | -23.75% | -23.74% | 498 | 16.87% | -4.47 | `Zero-Initial-Reentry:498` | 13.195 |
| `fixed_vsl_0p5atr_turn_0p1atr_entrysl_0p3atr` | `entry_atr` | 0.50 | 0.10 | 0.3 |  | 75,524.59 | -24.48% | -24.46% | 497 | 23.74% | -4.14 | `Zero-Initial-Reentry:497` | 19.8085 |
| `fixed_vsl_0p5atr_turn_0p1atr_extbuf_0p05atr` | `extreme_buffer` | 0.50 | 0.10 |  | 0.05 | 75,431.70 | -24.57% | -24.55% | 484 | 28.51% | -3.89 | `Zero-Initial-Reentry:484` | 23.0377 |
| `fixed_vsl_0p5atr_turn_0p15atr_entrysl_0p2atr` | `entry_atr` | 0.50 | 0.15 | 0.2 |  | 77,950.97 | -22.05% | -22.03% | 451 | 16.19% | -4.66 | `Zero-Initial-Reentry:451` | 13.1965 |
| `fixed_vsl_0p5atr_turn_0p15atr_entrysl_0p3atr` | `entry_atr` | 0.50 | 0.15 | 0.3 |  | 77,874.33 | -22.13% | -22.15% | 446 | 24.44% | -3.99 | `Zero-Initial-Reentry:446` | 19.7603 |
| `fixed_vsl_0p5atr_turn_0p15atr_extbuf_0p05atr` | `extreme_buffer` | 0.50 | 0.15 |  | 0.05 | 78,186.87 | -21.81% | -21.82% | 436 | 34.40% | -3.19 | `Zero-Initial-Reentry:436` | 27.7375 |
| `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.50 | 0.20 | 0.2 |  | 78,777.81 | -21.22% | -21.24% | 410 | 15.37% | -6.00 | `Zero-Initial-Reentry:410` | 13.174 |
| `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.50 | 0.20 | 0.3 |  | 79,599.74 | -20.40% | -20.50% | 400 | 23.50% | -4.27 | `Zero-Initial-Reentry:400` | 19.7611 |
| `fixed_vsl_0p5atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.50 | 0.20 |  | 0.05 | 79,438.16 | -20.56% | -20.58% | 394 | 35.28% | -3.42 | `Zero-Initial-Reentry:394` | 31.9065 |
| `fixed_vsl_0p55atr_turn_0p1atr_entrysl_0p2atr` | `entry_atr` | 0.55 | 0.10 | 0.2 |  | 75,868.72 | -24.13% | -24.12% | 471 | 14.23% | -6.25 | `Zero-Initial-Reentry:471` | 13.3178 |
| `fixed_vsl_0p55atr_turn_0p1atr_entrysl_0p3atr` | `entry_atr` | 0.55 | 0.10 | 0.3 |  | 75,533.39 | -24.47% | -24.45% | 468 | 20.09% | -5.07 | `Zero-Initial-Reentry:468` | 19.9571 |
| `fixed_vsl_0p55atr_turn_0p1atr_extbuf_0p05atr` | `extreme_buffer` | 0.55 | 0.10 |  | 0.05 | 76,663.92 | -23.34% | -23.32% | 455 | 26.59% | -3.86 | `Zero-Initial-Reentry:455` | 22.766 |
| `fixed_vsl_0p55atr_turn_0p15atr_entrysl_0p2atr` | `entry_atr` | 0.55 | 0.15 | 0.2 |  | 78,550.15 | -21.45% | -21.43% | 435 | 16.78% | -4.73 | `Zero-Initial-Reentry:435` | 13.4637 |
| `fixed_vsl_0p55atr_turn_0p15atr_entrysl_0p3atr` | `entry_atr` | 0.55 | 0.15 | 0.3 |  | 77,638.73 | -22.36% | -22.35% | 432 | 22.45% | -4.69 | `Zero-Initial-Reentry:432` | 20.1396 |
| `fixed_vsl_0p55atr_turn_0p15atr_extbuf_0p05atr` | `extreme_buffer` | 0.55 | 0.15 |  | 0.05 | 77,960.23 | -22.04% | -22.02% | 422 | 30.57% | -3.72 | `Zero-Initial-Reentry:422` | 27.7369 |
| `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.55 | 0.20 | 0.2 |  | 80,103.01 | -19.90% | -19.88% | 394 | 16.24% | -4.96 | `Zero-Initial-Reentry:394` | 13.5104 |
| `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.55 | 0.20 | 0.3 |  | 79,284.78 | -20.72% | -20.74% | 390 | 22.31% | -4.99 | `Zero-Initial-Reentry:390` | 20.2655 |
| `fixed_vsl_0p55atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.55 | 0.20 |  | 0.05 | 79,711.74 | -20.29% | -20.30% | 381 | 34.91% | -3.56 | `Zero-Initial-Reentry:381` | 31.9558 |

## Best By Return

- `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p2atr`: return `-19.90%`, trades `394`, win `16.24%`, MaxDD `-19.88%`.

## Entry Diagnostics

This table uses gross paired price PnL before commissions. The main result table win rate uses the existing fee-adjusted balance-accounting summary.

| Variant | Reason | Trades | Gross Win Rate | Gross PnL | Avg Gross PnL |
|---|---|---:|---:|---:|---:|
| `fixed_vsl_0p45atr_turn_0p1atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 550 | 21.45% | -6,035.78 | -0.0644% |
| `fixed_vsl_0p45atr_turn_0p1atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 540 | 29.44% | -6,500.35 | -0.0698% |
| `fixed_vsl_0p45atr_turn_0p1atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 526 | 39.16% | -5,963.30 | -0.0660% |
| `fixed_vsl_0p45atr_turn_0p15atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 502 | 18.92% | -6,951.83 | -0.0824% |
| `fixed_vsl_0p45atr_turn_0p15atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 491 | 28.11% | -6,761.34 | -0.0801% |
| `fixed_vsl_0p45atr_turn_0p15atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 478 | 43.10% | -6,207.36 | -0.0753% |
| `fixed_vsl_0p45atr_turn_0p2atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 457 | 20.13% | -6,041.47 | -0.0768% |
| `fixed_vsl_0p45atr_turn_0p2atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 448 | 28.57% | -5,497.49 | -0.0698% |
| `fixed_vsl_0p45atr_turn_0p2atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 438 | 47.03% | -5,773.33 | -0.0733% |
| `fixed_vsl_0p5atr_turn_0p1atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 498 | 19.68% | -6,670.11 | -0.0775% |
| `fixed_vsl_0p5atr_turn_0p1atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 497 | 27.36% | -7,492.94 | -0.0869% |
| `fixed_vsl_0p5atr_turn_0p1atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 484 | 35.74% | -7,945.31 | -0.0955% |
| `fixed_vsl_0p5atr_turn_0p15atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 451 | 18.40% | -6,372.02 | -0.0807% |
| `fixed_vsl_0p5atr_turn_0p15atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 446 | 27.58% | -6,636.97 | -0.0838% |
| `fixed_vsl_0p5atr_turn_0p15atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 436 | 41.28% | -6,556.31 | -0.0861% |
| `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 410 | 16.59% | -6,882.62 | -0.0964% |
| `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 400 | 26.50% | -6,298.22 | -0.0885% |
| `fixed_vsl_0p5atr_turn_0p2atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 394 | 44.16% | -6,645.19 | -0.0961% |
| `fixed_vsl_0p55atr_turn_0p1atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 471 | 16.35% | -7,920.10 | -0.0983% |
| `fixed_vsl_0p55atr_turn_0p1atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 468 | 24.79% | -8,422.88 | -0.1047% |
| `fixed_vsl_0p55atr_turn_0p1atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 455 | 34.29% | -7,533.29 | -0.0966% |
| `fixed_vsl_0p55atr_turn_0p15atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 435 | 19.31% | -6,252.73 | -0.0816% |
| `fixed_vsl_0p55atr_turn_0p15atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 432 | 25.69% | -7,328.50 | -0.0976% |
| `fixed_vsl_0p55atr_turn_0p15atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 422 | 38.39% | -7,272.08 | -0.0994% |
| `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p2atr` | `Zero-Initial-Reentry` | 394 | 17.77% | -5,963.31 | -0.0848% |
| `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p3atr` | `Zero-Initial-Reentry` | 390 | 25.38% | -7,005.46 | -0.1016% |
| `fixed_vsl_0p55atr_turn_0p2atr_extbuf_0p05atr` | `Zero-Initial-Reentry` | 381 | 42.78% | -6,817.60 | -0.1010% |

## Pending Diagnostics

| Variant | Real SL | Virtual Expired Without SL | Armed By Reason | Triggered By Reason | Expired By Reason | Max Trades Blocked |
|---|---:|---:|---|---|---|---:|
| `fixed_vsl_0p45atr_turn_0p1atr_entrysl_0p2atr` | 550 | 595 | `Zero-Initial-Reentry:642` | `Zero-Initial-Reentry:550` | `Zero-Initial-Reentry:89` | 3 |
| `fixed_vsl_0p45atr_turn_0p1atr_entrysl_0p3atr` | 540 | 593 | `Zero-Initial-Reentry:629` | `Zero-Initial-Reentry:540` | `Zero-Initial-Reentry:86` | 3 |
| `fixed_vsl_0p45atr_turn_0p1atr_extbuf_0p05atr` | 526 | 590 | `Zero-Initial-Reentry:611` | `Zero-Initial-Reentry:526` | `Zero-Initial-Reentry:84` | 1 |
| `fixed_vsl_0p45atr_turn_0p15atr_entrysl_0p2atr` | 502 | 583 | `Zero-Initial-Reentry:629` | `Zero-Initial-Reentry:502` | `Zero-Initial-Reentry:125` | 2 |
| `fixed_vsl_0p45atr_turn_0p15atr_entrysl_0p3atr` | 491 | 580 | `Zero-Initial-Reentry:615` | `Zero-Initial-Reentry:491` | `Zero-Initial-Reentry:123` | 1 |
| `fixed_vsl_0p45atr_turn_0p15atr_extbuf_0p05atr` | 478 | 576 | `Zero-Initial-Reentry:597` | `Zero-Initial-Reentry:478` | `Zero-Initial-Reentry:118` | 1 |
| `fixed_vsl_0p45atr_turn_0p2atr_entrysl_0p2atr` | 457 | 572 | `Zero-Initial-Reentry:615` | `Zero-Initial-Reentry:457` | `Zero-Initial-Reentry:156` | 2 |
| `fixed_vsl_0p45atr_turn_0p2atr_entrysl_0p3atr` | 448 | 567 | `Zero-Initial-Reentry:604` | `Zero-Initial-Reentry:448` | `Zero-Initial-Reentry:155` | 1 |
| `fixed_vsl_0p45atr_turn_0p2atr_extbuf_0p05atr` | 438 | 559 | `Zero-Initial-Reentry:587` | `Zero-Initial-Reentry:438` | `Zero-Initial-Reentry:148` | 1 |
| `fixed_vsl_0p5atr_turn_0p1atr_entrysl_0p2atr` | 497 | 649 | `Zero-Initial-Reentry:581` | `Zero-Initial-Reentry:498` | `Zero-Initial-Reentry:80` | 3 |
| `fixed_vsl_0p5atr_turn_0p1atr_entrysl_0p3atr` | 496 | 645 | `Zero-Initial-Reentry:577` | `Zero-Initial-Reentry:497` | `Zero-Initial-Reentry:78` | 2 |
| `fixed_vsl_0p5atr_turn_0p1atr_extbuf_0p05atr` | 483 | 641 | `Zero-Initial-Reentry:563` | `Zero-Initial-Reentry:484` | `Zero-Initial-Reentry:78` | 1 |
| `fixed_vsl_0p5atr_turn_0p15atr_entrysl_0p2atr` | 451 | 633 | `Zero-Initial-Reentry:564` | `Zero-Initial-Reentry:451` | `Zero-Initial-Reentry:110` | 3 |
| `fixed_vsl_0p5atr_turn_0p15atr_entrysl_0p3atr` | 445 | 631 | `Zero-Initial-Reentry:557` | `Zero-Initial-Reentry:446` | `Zero-Initial-Reentry:108` | 3 |
| `fixed_vsl_0p5atr_turn_0p15atr_extbuf_0p05atr` | 435 | 625 | `Zero-Initial-Reentry:542` | `Zero-Initial-Reentry:436` | `Zero-Initial-Reentry:105` | 1 |
| `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p2atr` | 410 | 621 | `Zero-Initial-Reentry:556` | `Zero-Initial-Reentry:410` | `Zero-Initial-Reentry:143` | 2 |
| `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p3atr` | 400 | 619 | `Zero-Initial-Reentry:545` | `Zero-Initial-Reentry:400` | `Zero-Initial-Reentry:143` | 1 |
| `fixed_vsl_0p5atr_turn_0p2atr_extbuf_0p05atr` | 394 | 618 | `Zero-Initial-Reentry:531` | `Zero-Initial-Reentry:394` | `Zero-Initial-Reentry:135` | 1 |
| `fixed_vsl_0p55atr_turn_0p1atr_entrysl_0p2atr` | 470 | 692 | `Zero-Initial-Reentry:541` | `Zero-Initial-Reentry:471` | `Zero-Initial-Reentry:69` | 1 |
| `fixed_vsl_0p55atr_turn_0p1atr_entrysl_0p3atr` | 467 | 690 | `Zero-Initial-Reentry:536` | `Zero-Initial-Reentry:468` | `Zero-Initial-Reentry:68` | 0 |
| `fixed_vsl_0p55atr_turn_0p1atr_extbuf_0p05atr` | 454 | 679 | `Zero-Initial-Reentry:521` | `Zero-Initial-Reentry:455` | `Zero-Initial-Reentry:66` | 0 |
| `fixed_vsl_0p55atr_turn_0p15atr_entrysl_0p2atr` | 434 | 679 | `Zero-Initial-Reentry:526` | `Zero-Initial-Reentry:435` | `Zero-Initial-Reentry:90` | 1 |
| `fixed_vsl_0p55atr_turn_0p15atr_entrysl_0p3atr` | 431 | 678 | `Zero-Initial-Reentry:522` | `Zero-Initial-Reentry:432` | `Zero-Initial-Reentry:89` | 1 |
| `fixed_vsl_0p55atr_turn_0p15atr_extbuf_0p05atr` | 421 | 669 | `Zero-Initial-Reentry:509` | `Zero-Initial-Reentry:422` | `Zero-Initial-Reentry:87` | 0 |
| `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p2atr` | 394 | 666 | `Zero-Initial-Reentry:511` | `Zero-Initial-Reentry:394` | `Zero-Initial-Reentry:116` | 1 |
| `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p3atr` | 389 | 665 | `Zero-Initial-Reentry:505` | `Zero-Initial-Reentry:390` | `Zero-Initial-Reentry:114` | 1 |
| `fixed_vsl_0p55atr_turn_0p2atr_extbuf_0p05atr` | 380 | 655 | `Zero-Initial-Reentry:492` | `Zero-Initial-Reentry:381` | `Zero-Initial-Reentry:111` | 0 |
