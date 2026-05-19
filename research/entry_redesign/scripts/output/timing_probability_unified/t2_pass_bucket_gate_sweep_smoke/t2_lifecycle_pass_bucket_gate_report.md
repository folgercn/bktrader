# T2 Lifecycle Pass-Bucket Gates

Research-only strict lifecycle sweep for shrinking the original_t2 full-size pass bucket.

- Timeframe: `1h`
- Reentry fill policy: `strict_next_second_cross`
- Months: 2026-01
- Symbols: ETHUSDT

## Candidate Summary

| Candidate | Calendar Sum | Delta | Worst Silo | Neg Silos | Trades | T2 Trades | T2 Net PnL | T3 Net PnL | Filters | Read |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ctx4h_skipfail_t3_60m` | -0.190000% | baseline | -0.190000% | 1 | 5 | 2 | -0.023470% | 0.012570% | `{"ctx_return_lookback_bars": 4, "max_atr_percentile": 40.0, "min_ctx_side_return_atr": 0.0}` | current executable reference: fail context skips lock; pass context trades full size |
| `ctx4h_min020_skipfail_t3_60m` | -0.190000% | +0.000000% | -0.190000% | 1 | 5 | 2 | -0.023470% | 0.012570% | `{"ctx_return_lookback_bars": 4, "max_atr_percentile": 40.0, "min_ctx_side_return_atr": 0.2}` | medium 4h continuation threshold |
| `ctx12h_min000_skipfail_t3_60m` | -0.110000% | +0.080000% | -0.110000% | 1 | 3 | 0 | 0.000000% | 0.012560% | `{"ctx_return_lookback_bars": 4, "max_atr_percentile": 40.0, "min_ctx12h_side_return_atr": 0.0, "min_ctx_side_return_atr": 0.0}` | keeps 4h pass and also requires 12h context not opposing the side |

## Reference Pass-Bucket Attribution

| Family | Bucket | Trades | Net After Fee | Gross | Fee | Win Rate | Worst Trade | Notional |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| `all_original_t2` | `all` | 2 | -0.083430% | -0.023474% | 0.059956% | 0.00% | -0.055619% | 29.978093% |
| `atr_percentile` | `20-30` | 2 | -0.083430% | -0.023474% | 0.059956% | 0.00% | -0.055619% | 29.978093% |
| `breakout_extension_atr` | `<0.02` | 2 | -0.083430% | -0.023474% | 0.059956% | 0.00% | -0.055619% | 29.978093% |
| `ctx12h_side_return_atr` | `-0.50-0` | 2 | -0.083430% | -0.023474% | 0.059956% | 0.00% | -0.055619% | 29.978093% |
| `ctx4h_side_return_atr` | `>=0.50` | 2 | -0.083430% | -0.023474% | 0.059956% | 0.00% | -0.055619% | 29.978093% |
| `exit_reason` | `SL` | 2 | -0.083430% | -0.023474% | 0.059956% | 0.00% | -0.055619% | 29.978093% |
| `pass_full_original_t2` | `all` | 2 | -0.083430% | -0.023474% | 0.059956% | 0.00% | -0.055619% | 29.978093% |
| `pre_touch_seconds` | `>=900` | 2 | -0.083430% | -0.023474% | 0.059956% | 0.00% | -0.055619% | 29.978093% |
| `side` | `long` | 2 | -0.083430% | -0.023474% | 0.059956% | 0.00% | -0.055619% | 29.978093% |
| `entry_reason` | `Zero-Initial-Reentry` | 1 | -0.055619% | -0.015640% | 0.039978% | 0.00% | -0.055619% | 19.989103% |
| `entry_reason` | `SL-Reentry` | 1 | -0.027812% | -0.007834% | 0.019978% | 0.00% | -0.027812% | 9.988990% |

## Read

- These results are promotion-comparable lifecycle returns, not adverse10 event-ledger returns.
- A candidate only matters if it improves calendar sum without creating a worse worst-silo profile or collapsing T3 contribution.
- Exact low_eff/RF remains a separate hook because lifecycle replay still needs event-time speed/efficiency/RF features.
