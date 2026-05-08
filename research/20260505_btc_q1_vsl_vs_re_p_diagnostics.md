# BTCUSDT Q1 2026 VSL vs re_p Diagnostics

Scope: research-only diagnostic. This compares the no-`re_p` VSL reclaim entries against the old research `re_p` anchor (`prev_low_1 + 0.1 ATR` for long, `prev_high_1 - 0.1 ATR` for short).

## Read

The `VSL 0.8 ATR + turn 0.2 ATR` candidates are not semantically far from the old `re_p` area. They do not use `re_p` as a planned fill, but the virtual stop level itself is very close to old `re_p` in the best-performing runs.

Therefore these variants should not be treated as an independent no-`re_p` baseline. They are better described as "price reaches the old reentry-price area, then reclaims by 0.2 ATR, then fills at the next observed 1s open."

## Median Distances

Positive `entry_vs_re_p_bps` means the observed entry is worse than old `re_p` for the trade direction. Positive `vsl_vs_re_p_bps` means the virtual stop is beyond old `re_p` in the breakout-favorable direction.

| Timeframe | Variant | Entries | Median VSL vs re_p | Median Trigger vs re_p | Median Entry vs re_p | Entry within 5 bps of re_p | Entry at/better than re_p |
|---|---|---:|---:|---:|---:|---:|---:|
| `30min` | `fixed_vsl_0p8atr_turn_0p2atr_realsl_vsl` | 226 | `1.21 bps` | `9.85 bps` | `15.48 bps` | `18.58%` | `12.83%` |
| `1h` | `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p05atr` | 140 | `1.38 bps` | `11.57 bps` | `17.40 bps` | `15.71%` | `12.14%` |
| `2h` | `fixed_vsl_0p8atr_turn_0p2atr_extbuf_0p05atr` | 76 | `-4.02 bps` | `11.05 bps` | `16.42 bps` | `17.11%` | `22.37%` |

## Implication

The earlier conclusion should be narrowed:

- The stale-virtual bug did explain the suspiciously low trade count in the first VSL sweep.
- Disabling downstream `SL-Reentry` and moving from `30min` to `1h/2h` did improve results.
- But the best `VSL 0.8` results are not clean evidence for a new no-`re_p` baseline, because they require price to reach almost the same area as the old `re_p`.

The next baseline candidate should avoid both planned `re_p` fills and hidden `re_p`-area dependency. A cleaner test should define entries from post-breakout observed price action, for example acceptance above/below the structural breakout, pullback depth relative to breakout-to-extreme move, or close-based continuation filters, instead of anchoring around a deep pullback level that empirically overlaps old `re_p`.

Raw diagnostic rows are written to `research/tmp_btc_q1_vsl_vs_re_p_diagnostics.csv`.
