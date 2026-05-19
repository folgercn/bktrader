# T2 Lifecycle Pass-Bucket Gates

Research-only strict lifecycle sweep for shrinking the original_t2 full-size pass bucket.

- Timeframe: `1h`
- Reentry fill policy: `strict_next_second_cross`
- Months: 2025-06, 2025-07, 2025-08, 2025-09, 2025-10, 2025-11, 2025-12, 2026-01, 2026-02, 2026-03, 2026-04
- Symbols: ETHUSDT, BTCUSDT

## Candidate Summary

| Candidate | Calendar Sum | Delta | Worst Silo | Neg Silos | Trades | T2 Trades | T2 Net PnL | T3 Net PnL | Filters | Read |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `pretouch_max900_skipfail_t3_60m` | -4.960000% | baseline | -0.680000% | 18 | 207 | 103 | -1.434690% | 3.985080% | `{"ctx_return_lookback_bars": 4, "max_atr_percentile": 40.0, "max_pre_touch_seconds": 900.0, "min_ctx_side_return_atr": 0.0}` | mirrors the T3 touch-timing cap for original_t2 and removes very late signal-bar touches |

## Reference Pass-Bucket Attribution

| Family | Bucket | Trades | Net After Fee | Gross | Fee | Win Rate | Worst Trade | Notional |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| `all_original_t2` | `all` | 103 | -4.749478% | -1.434678% | 3.314800% | 2.91% | -0.102386% | 1657.399763% |
| `pass_full_original_t2` | `all` | 103 | -4.749478% | -1.434678% | 3.314800% | 2.91% | -0.102386% | 1657.399763% |
| `exit_reason` | `SL` | 103 | -4.749478% | -1.434678% | 3.314800% | 2.91% | -0.102386% | 1657.399763% |
| `side` | `long` | 78 | -3.520862% | -0.985164% | 2.535698% | 3.85% | -0.089294% | 1267.848776% |
| `ctx4h_side_return_atr` | `>=0.50` | 71 | -3.393470% | -1.156176% | 2.237294% | 0.00% | -0.089294% | 1118.646809% |
| `entry_reason` | `Zero-Initial-Reentry` | 57 | -3.326556% | -1.049603% | 2.276953% | 3.51% | -0.102386% | 1138.476559% |
| `ctx12h_side_return_atr` | `>=0.30` | 63 | -2.916771% | -0.879695% | 2.037076% | 1.59% | -0.089294% | 1018.538161% |
| `pre_touch_seconds` | `>=900` | 61 | -2.742136% | -0.745421% | 1.996715% | 4.92% | -0.102386% | 998.357448% |
| `atr_percentile` | `10-20` | 36 | -1.609143% | -0.411352% | 1.197791% | 5.56% | -0.089294% | 598.895435% |
| `entry_reason` | `SL-Reentry` | 46 | -1.422922% | -0.385076% | 1.037846% | 2.17% | -0.066992% | 518.923205% |
| `pre_touch_seconds` | `600-900` | 27 | -1.382239% | -0.523394% | 0.858845% | 0.00% | -0.087326% | 429.422624% |
| `atr_percentile` | `<10` | 30 | -1.308876% | -0.370559% | 0.938317% | 0.00% | -0.073435% | 469.158489% |
| `ctx12h_side_return_atr` | `<-0.50` | 26 | -1.261980% | -0.423214% | 0.838765% | 3.85% | -0.102386% | 419.382574% |
| `side` | `short` | 25 | -1.228616% | -0.449514% | 0.779102% | 0.00% | -0.102386% | 389.550987% |
| `breakout_extension_atr` | `<0.02` | 26 | -1.206203% | -0.407645% | 0.798557% | 0.00% | -0.087326% | 399.278636% |
| `atr_percentile` | `20-30` | 23 | -1.093883% | -0.374636% | 0.719247% | 0.00% | -0.102386% | 359.623742% |
| `breakout_extension_atr` | `0.10-0.20` | 21 | -1.051368% | -0.392181% | 0.659188% | 0.00% | -0.102386% | 329.593817% |
| `breakout_extension_atr` | `0.02-0.05` | 20 | -1.030943% | -0.351383% | 0.679560% | 0.00% | -0.085218% | 339.779882% |

## Read

- These results are promotion-comparable lifecycle returns, not adverse10 event-ledger returns.
- A candidate only matters if it improves calendar sum without creating a worse worst-silo profile or collapsing T3 contribution.
- Exact low_eff/RF remains a separate hook because lifecycle replay still needs event-time speed/efficiency/RF features.
