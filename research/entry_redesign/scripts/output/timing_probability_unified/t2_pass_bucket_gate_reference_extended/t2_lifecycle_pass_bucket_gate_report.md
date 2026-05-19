# T2 Lifecycle Pass-Bucket Gates

Research-only strict lifecycle sweep for shrinking the original_t2 full-size pass bucket.

- Timeframe: `1h`
- Reentry fill policy: `strict_next_second_cross`
- Months: 2025-06, 2025-07, 2025-08, 2025-09, 2025-10, 2025-11, 2025-12, 2026-01, 2026-02, 2026-03, 2026-04
- Symbols: ETHUSDT, BTCUSDT

## Candidate Summary

| Candidate | Calendar Sum | Delta | Worst Silo | Neg Silos | Trades | T2 Trades | T2 Net PnL | T3 Net PnL | Filters | Read |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `ctx4h_skipfail_t3_60m` | -7.900000% | baseline | -0.740000% | 19 | 267 | 163 | -2.383800% | 3.983430% | `{"ctx_return_lookback_bars": 4, "max_atr_percentile": 40.0, "min_ctx_side_return_atr": 0.0}` | current executable reference: fail context skips lock; pass context trades full size |

## Reference Pass-Bucket Attribution

| Family | Bucket | Trades | Net After Fee | Gross | Fee | Win Rate | Worst Trade | Notional |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| `all_original_t2` | `all` | 163 | -7.713116% | -2.383797% | 5.329319% | 3.07% | -0.108473% | 2664.659696% |
| `pass_full_original_t2` | `all` | 163 | -7.713116% | -2.383797% | 5.329319% | 3.07% | -0.108473% | 2664.659696% |
| `exit_reason` | `SL` | 163 | -7.713116% | -2.383797% | 5.329319% | 3.07% | -0.108473% | 2664.659696% |
| `pre_touch_seconds` | `>=900` | 142 | -6.674873% | -2.004090% | 4.670783% | 3.52% | -0.108473% | 2335.391483% |
| `side` | `long` | 126 | -5.810579% | -1.658988% | 4.151590% | 3.97% | -0.108473% | 2075.795067% |
| `entry_reason` | `Zero-Initial-Reentry` | 93 | -5.467795% | -1.754277% | 3.713518% | 3.23% | -0.108473% | 1856.758873% |
| `ctx4h_side_return_atr` | `>=0.50` | 106 | -5.218748% | -1.824461% | 3.394287% | 0.94% | -0.108473% | 1697.143381% |
| `ctx12h_side_return_atr` | `>=0.30` | 97 | -4.653302% | -1.480011% | 3.173291% | 2.06% | -0.089294% | 1586.645469% |
| `breakout_extension_atr` | `<0.02` | 79 | -3.759685% | -1.264620% | 2.495065% | 0.00% | -0.091530% | 1247.532317% |
| `atr_percentile` | `10-20` | 52 | -2.404611% | -0.687729% | 1.716882% | 3.85% | -0.089294% | 858.441132% |
| `entry_reason` | `SL-Reentry` | 70 | -2.245321% | -0.629520% | 1.615802% | 2.86% | -0.077159% | 807.900824% |
| `atr_percentile` | `<10` | 48 | -2.215203% | -0.638684% | 1.576519% | 0.00% | -0.077132% | 788.259655% |
| `ctx12h_side_return_atr` | `<-0.50` | 45 | -2.177720% | -0.700721% | 1.476999% | 4.44% | -0.108473% | 738.499556% |
| `side` | `short` | 37 | -1.902538% | -0.724809% | 1.177729% | 0.00% | -0.102386% | 588.864629% |
| `atr_percentile` | `20-30` | 39 | -1.800387% | -0.562890% | 1.237497% | 2.56% | -0.102386% | 618.748581% |
| `breakout_extension_atr` | `0.02-0.05` | 35 | -1.713630% | -0.495435% | 1.218195% | 5.71% | -0.108473% | 609.097546% |
| `atr_percentile` | `30-40` | 24 | -1.292915% | -0.494494% | 0.798421% | 8.33% | -0.108473% | 399.210328% |
| `ctx4h_side_return_atr` | `0.30-0.50` | 27 | -1.120774% | -0.223016% | 0.897758% | 11.11% | -0.088709% | 448.878814% |

## Read

- These results are promotion-comparable lifecycle returns, not adverse10 event-ledger returns.
- A candidate only matters if it improves calendar sum without creating a worse worst-silo profile or collapsing T3 contribution.
- Exact low_eff/RF remains a separate hook because lifecycle replay still needs event-time speed/efficiency/RF features.
