# ETHUSDT Q1 2026 VSL 0.5 ATR Around Sweep

Scope: research-only. Downstream `SL-Reentry` disabled. `re_p` is not used for fills; distance to old `re_p` is measured for diagnostics.

| Timeframe | Best Variant | Return | Trades | Win Rate | Max DD | Median VSL vs re_p | Median Entry vs re_p | Entry within 5 bps of re_p | Entry at/better than re_p |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `30min` | `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p2atr` | -19.90% | 394 | 16.24% | -19.88% | 10.7536 | 28.7624 | 5.33% | 3.05% |
| `1h` | `fixed_vsl_0p5atr_turn_0p2atr_extbuf_0p05atr` | -9.38% | 213 | 42.72% | -9.38% | 19.486 | 43.8565 | 3.29% | 0.94% |
| `2h` | `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p3atr` | -5.13% | 115 | 37.39% | -5.55% | 25.1518 | 58.6539 | 2.61% | 4.35% |

## Top 8 By Timeframe

| Timeframe | Variant | Real Stop | VSL ATR | Turn ATR | Return | Trades | Win Rate | Median VSL vs re_p | Median Entry vs re_p |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|
| `30min` | `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.55 | 0.20 | -19.90% | 394 | 16.24% | 10.7536 | 28.7624 |
| `30min` | `fixed_vsl_0p55atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.55 | 0.20 | -20.29% | 381 | 34.91% | 10.8463 | 28.9949 |
| `30min` | `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.50 | 0.20 | -20.40% | 400 | 23.50% | 12.7815 | 30.7809 |
| `30min` | `fixed_vsl_0p5atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.50 | 0.20 | -20.56% | 394 | 35.28% | 13.2364 | 31.4057 |
| `30min` | `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.55 | 0.20 | -20.72% | 390 | 22.31% | 10.7536 | 28.7624 |
| `30min` | `fixed_vsl_0p45atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.45 | 0.20 | -21.14% | 438 | 37.44% | 14.3714 | 32.2517 |
| `30min` | `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.50 | 0.20 | -21.22% | 410 | 15.37% | 12.7815 | 30.7809 |
| `30min` | `fixed_vsl_0p45atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.45 | 0.20 | -21.26% | 448 | 25.45% | 14.2418 | 32.1412 |
| `1h` | `fixed_vsl_0p5atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.50 | 0.20 | -9.38% | 213 | 42.72% | 19.486 | 43.8565 |
| `1h` | `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.50 | 0.20 | -9.74% | 222 | 30.18% | 19.1183 | 43.7369 |
| `1h` | `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.55 | 0.20 | -10.73% | 210 | 28.57% | 19.1853 | 43.168 |
| `1h` | `fixed_vsl_0p55atr_turn_0p15atr_entrysl_0p3atr` | `entry_atr` | 0.55 | 0.15 | -10.90% | 234 | 32.05% | 15.4881 | 35.0398 |
| `1h` | `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.55 | 0.20 | -11.46% | 212 | 17.92% | 18.3029 | 42.4446 |
| `1h` | `fixed_vsl_0p55atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.55 | 0.20 | -11.47% | 204 | 40.20% | 18.3029 | 42.4446 |
| `1h` | `fixed_vsl_0p45atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.45 | 0.20 | -11.50% | 232 | 41.38% | 20.6312 | 44.9933 |
| `1h` | `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.50 | 0.20 | -11.52% | 221 | 19.46% | 19.486 | 43.8565 |
| `2h` | `fixed_vsl_0p55atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.55 | 0.20 | -5.13% | 115 | 37.39% | 25.1518 | 58.6539 |
| `2h` | `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p3atr` | `entry_atr` | 0.50 | 0.20 | -5.65% | 117 | 32.48% | 32.397 | 62.9065 |
| `2h` | `fixed_vsl_0p55atr_turn_0p1atr_entrysl_0p3atr` | `entry_atr` | 0.55 | 0.10 | -5.68% | 132 | 36.36% | 22.4341 | 40.0801 |
| `2h` | `fixed_vsl_0p5atr_turn_0p2atr_extbuf_0p05atr` | `extreme_buffer` | 0.50 | 0.20 | -5.77% | 113 | 46.90% | 32.8605 | 63.2297 |
| `2h` | `fixed_vsl_0p5atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.50 | 0.20 | -5.84% | 120 | 23.33% | 32.3039 | 61.6683 |
| `2h` | `fixed_vsl_0p55atr_turn_0p1atr_extbuf_0p05atr` | `extreme_buffer` | 0.55 | 0.10 | -5.85% | 131 | 38.93% | 21.9731 | 41.1685 |
| `2h` | `fixed_vsl_0p45atr_turn_0p2atr_entrysl_0p2atr` | `entry_atr` | 0.45 | 0.20 | -5.96% | 135 | 28.89% | 35.6841 | 66.967 |
| `2h` | `fixed_vsl_0p55atr_turn_0p1atr_entrysl_0p2atr` | `entry_atr` | 0.55 | 0.10 | -6.06% | 132 | 27.27% | 22.4341 | 40.0801 |